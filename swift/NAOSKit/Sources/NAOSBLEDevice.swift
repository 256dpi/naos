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

/// The NAOS BLE specific errors.
public enum NAOSBLEError: LocalizedError {
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
	public let peripheral: Peripheral
	private var _advertisementData: [String: Any]
	private var _rssi: NSNumber
	private let lock = NSLock()
	private var service: Service?
	private var characteristic: Characteristic?
	private var streamSubscription: AnyCancellable?
	private var eventSubscription: AnyCancellable?

	public var advertisementData: [String: Any] {
		lock.withLock { _advertisementData }
	}

	public var rssi: NSNumber {
		lock.withLock { _rssi }
	}

	init(manager: CentralManager, peripheral: Peripheral, advertisementData: [String: Any], rssi: NSNumber) {
		self.manager = manager
		self.peripheral = peripheral
		self._advertisementData = advertisementData
		self._rssi = rssi

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

	func update(advertisementData: [String: Any], rssi: NSNumber) {
		lock.withLock {
			_advertisementData = advertisementData
			_rssi = rssi
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

	/// Ends the transport stream so the channel detects the loss.
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
