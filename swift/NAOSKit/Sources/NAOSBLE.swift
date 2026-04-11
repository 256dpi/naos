//
//  Created by Joël Gähwiler on 20.08.23.
//  Copyright © 2017 Joël Gähwiler. All rights reserved.
//

import AsyncBluetooth
import Combine
import CoreBluetooth
import Foundation

let bleService = CBUUID(string: "632FBA1B-4861-4E4F-8103-FFEE9D5033B5")
let bleCharacteristic = CBUUID(string: "0360744B-A61B-00AD-C945-37f3634130F3")

/// A discovered BLE device descriptor.
public struct NAOSBLEDescriptor: Hashable, @unchecked Sendable {
	public let identifier: UUID
	public let name: String
	public let advertisementData: [String: Any]
	public let rssi: NSNumber
	let manager: CentralManager
	let peripheral: Peripheral

	init(manager: CentralManager, peripheral: Peripheral, advertisementData: [String: Any], rssi: NSNumber) {
		self.manager = manager
		self.identifier = peripheral.identifier
		self.name = peripheral.name ?? "Unnamed"
		self.advertisementData = advertisementData
		self.rssi = rssi
		self.peripheral = peripheral
	}

	public func hash(into hasher: inout Hasher) {
		hasher.combine(identifier)
	}

	public static func == (lhs: NAOSBLEDescriptor, rhs: NAOSBLEDescriptor) -> Bool {
		lhs.identifier == rhs.identifier
	}
}

/// The delegate protocol to be implemented to handle NAOSManager events.
public protocol NAOSBLEManagerDelegate {
	/// The manager discovered a new device.
	func naosBLEManagerDidDiscoverDevice(manager: NAOSBLEManager, descriptor: NAOSBLEDescriptor)

	/// The manager did reset either because of a Bluetooth availability change or a manual reset().
	func naosBLEManagerDidReset(manager: NAOSBLEManager)
}

/// The main class that handles NAOS device discovery and handling.
public class NAOSBLEManager: NSObject {
	var delegate: NAOSBLEManagerDelegate?
	var centralManager: CentralManager!
	private var descriptors: [UUID: NAOSBLEDescriptor]
	private var subscription: AnyCancellable?
	private var queue = DispatchQueue(label: "devices", attributes: .concurrent)

	/// Initializes the manager and sets the specified class as the delegate.
	public init(delegate: NAOSBLEManagerDelegate?) {
		// set delegate
		self.delegate = delegate

		// initialize devices
		descriptors = [:]

		// finish init
		super.init()

		// create central manager
		centralManager = CentralManager()

		// disable logging
		AsyncBluetoothLogging.setEnabled(false)

		// start scanning
		Task { try await self.start() }
	}

	private func start() async throws {
		// subscribe events
		subscription = await centralManager.eventPublisher.sink(receiveValue: { event in
			switch event {
			case .didUpdateState(let state):
				switch state {
				case .poweredOn:
					// ignore
					break
				case .poweredOff:
					// stop running scan
					Task {
						await self.centralManager.stopScan()
					}
				default:
					break
				}
			case .didDisconnectPeripheral:
				break
			default:
				break
			}
		})

		// scan forever
		Task {
			while true {
				do {
					try await self.scan()
				} catch {
					print("error while scanning: ", error.localizedDescription)
					try? await Task.sleep(for: .seconds(2))
				}
			}
		}
	}

	/// Reset discovered devices.
	public func reset() {
		// clear devices
		queue.sync(flags: .barrier) {
			descriptors.removeAll()
		}

		// call callback if present
		if let d = delegate {
			DispatchQueue.main.async {
				d.naosBLEManagerDidReset(manager: self)
			}
		}
	}

	private func scan() async throws {
		// check if powered on
		while centralManager.bluetoothState != .poweredOn {
			try? await Task.sleep(for: .seconds(1))
		}

		// wait until ready
		try await centralManager.waitUntilReady()

		// create scan stream
		let stream = try await centralManager.scanForPeripherals(withServices: [bleService])

		// handle discovered peripherals
		for await scanData in stream {
			let descriptor = NAOSBLEDescriptor(
				manager: centralManager,
				peripheral: scanData.peripheral,
				advertisementData: scanData.advertisementData,
				rssi: scanData.rssi
			)

			// update existing descriptor if already known
			if hasDescriptor(peripheral: scanData.peripheral) {
				queue.sync(flags: .barrier) {
					descriptors[descriptor.identifier] = descriptor
				}
				continue
			}

			// otherwise, add descriptor
			queue.sync(flags: .barrier) {
				descriptors[descriptor.identifier] = descriptor
			}

			// call callback if present
			if let d = delegate {
				DispatchQueue.main.async {
					d.naosBLEManagerDidDiscoverDevice(manager: self, descriptor: descriptor)
				}
			}
		}
	}

	// Helpers

	private func hasDescriptor(peripheral: Peripheral) -> Bool {
		var found = false
		queue.sync {
			found = descriptors[peripheral.identifier] != nil
		}
		return found
	}
}


enum NAOSBLEError: LocalizedError {
	case serviceNotFound
	case characteristicNotFound

	public var errorDescription: String? {
		switch self {
		case .serviceNotFound:
			return "BLE service not found."
		case .characteristicNotFound:
			return "BLE characteristic not found."
		}
	}
}

public class NAOSBLEDevice: NAOSDevice {
	private let manager: CentralManager
	private let peripheral: Peripheral
	private let lock = NSLock()
	private var service: Service?
	private var characteristic: Characteristic?
	private var streamSubscription: AnyCancellable?
	private var eventSubscription: AnyCancellable?

	public init(descriptor: NAOSBLEDescriptor) {
		self.manager = descriptor.manager
		self.peripheral = descriptor.peripheral

		// watch for BLE disconnects to end the transport stream
		Task { [weak self] in
			guard let self else { return }
			self.eventSubscription = await self.manager.eventPublisher.sink(receiveValue: { event in
				guard case .didDisconnectPeripheral(let peripheral, _, _) = event else {
					return
				}
				guard peripheral.identifier == self.peripheral.identifier else {
					return
				}
				self.handleDisconnect()
			})
		}
	}

	public func type() -> String {
		return "BLE"
	}
	
	public func id() -> String {
		return "ble/" + peripheral.identifier.uuidString
	}

	public func name() -> String {
		peripheral.name ?? "Unnamed"
	}

	public func open() async throws -> NAOSChannel {
		// connect
		try await manager.connect(peripheral)

		// discover device
		try await withTimeout(seconds: 10) {
			// discover service
			try await self.peripheral.discoverServices([bleService])

			// find service
			for s in self.peripheral.discoveredServices ?? [] {
				if s.uuid == bleService {
					self.lock.withLock {
						self.service = s
					}
				}
			}
			guard let service = self.lock.withLock({ self.service }) else {
				throw NAOSBLEError.serviceNotFound
			}

			// discover characteristics
			try await self.peripheral.discoverCharacteristics(nil, for: service)

			// find characteristic
			for c in service.discoveredCharacteristics ?? [] {
				if c.uuid == bleCharacteristic {
					self.lock.withLock {
						self.characteristic = c
					}
				}
			}
			guard let characteristic = self.lock.withLock({ self.characteristic }) else {
				throw NAOSBLEError.characteristicNotFound
			}

			// enable indication notifications
			try await self.peripheral.setNotifyValue(true, for: characteristic)
		}

		let (stream, subscription) = await stream()

		// store subscription to cancel on disconnect
		lock.withLock {
			streamSubscription = subscription
		}

		return NAOSChannel(
			transport: bleTransport(peripheral: self, stream: stream, subscription: subscription),
			device: self,
			width: 10,
			onClose: { [weak self] in
				self?.lock.withLock {
					self?.streamSubscription = nil
				}
			}
		)
	}

	func handleDisconnect() {
		let sub = lock.withLock { () -> AnyCancellable? in
			let s = streamSubscription
			streamSubscription = nil
			return s
		}
		sub?.cancel()
	}

	func stream() async -> (AsyncStream<Data>, AnyCancellable) {
		// prepare stream
		var continuation: AsyncStream<Data>.Continuation?
		let stream = AsyncStream<Data> { c in
			continuation = c
		}

		// create subscription
		let subscription = await peripheral.characteristicValueUpdatedPublisher.sink { event in
			// check characteristic
			if event.characteristic.uuid != bleCharacteristic {
				return
			}

			// get data
			guard let data = event.value else {
				return
			}

			// yield message
			continuation!.yield(data)
		}

		return (
			stream,
			AnyCancellable {
				subscription.cancel()
				continuation!.finish()
			}
		)
	}

	func write(data: Data) async throws {
		let activeCharacteristic = try lock.withLock { () throws -> Characteristic in
			guard let characteristic = self.characteristic else {
				throw NAOSBLEError.characteristicNotFound
			}
			return characteristic
		}

		// read value
		try await withTimeout(seconds: 2) {
			try await self.peripheral.writeValue(data, for: activeCharacteristic, type: .withoutResponse)
		}
	}

	func close() async throws {
		lock.withLock {
			service = nil
			characteristic = nil
			streamSubscription = nil
		}

		// disconnect if still connected
		if peripheral.state == .connected || peripheral.state == .connecting {
			try await manager.cancelPeripheralConnection(peripheral)
		}
	}
}

private actor dataStreamReader {
	private var iterator: AsyncStream<Data>.Iterator

	init(stream: AsyncStream<Data>) {
		self.iterator = stream.makeAsyncIterator()
	}

	func next() async -> Data? {
		var iterator = self.iterator
		let value = await iterator.next()
		self.iterator = iterator
		return value
	}
}

final class bleTransport: NAOSTransport {
	private let peripheral: NAOSBLEDevice
	private let subscription: AnyCancellable
	private let reader: dataStreamReader
	private let lock = NSLock()
	private var closed = false

	init(peripheral: NAOSBLEDevice, stream: AsyncStream<Data>, subscription: AnyCancellable) {
		self.peripheral = peripheral
		self.subscription = subscription
		self.reader = dataStreamReader(stream: stream)
	}

	func read() async throws -> Data {
		guard let data = await reader.next() else {
			throw NAOSTransportError.closed
		}
		return data
	}

	func write(data: Data) async throws {
		try await peripheral.write(data: data)
	}

	func close() {
		let shouldClose = lock.withLock {
			if closed {
				return false
			}
			closed = true
			return true
		}

		if !shouldClose {
			return
		}

		subscription.cancel()
		Task {
			do {
				try await self.peripheral.close()
			} catch {
				print("error closing BLE connection: \(error.localizedDescription)")
			}
		}
	}
}
