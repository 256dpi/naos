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

class NAOSBLEDevice: NAOSDevice {
	let manager: CentralManager
	let peripheral: Peripheral
	var service: Service?
	var characteristic: Characteristic?

	init(manager: CentralManager, peripheral: Peripheral) {
		self.manager = manager
		self.peripheral = peripheral
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

		// discovet device
		try await withTimeout(seconds: 10) {
			// discover service
			try await self.peripheral.discoverServices([bleService])

			// find service
			for s in self.peripheral.discoveredServices ?? [] {
				if s.uuid == bleService {
					self.service = s
				}
			}
			if self.service == nil {
				throw NAOSBLEError.serviceNotFound
			}

			// discover characteristics
			try await self.peripheral.discoverCharacteristics(nil, for: self.service!)

			// find characteristic
			for c in self.service!.discoveredCharacteristics ?? [] {
				if c.uuid == bleCharacteristic {
					self.characteristic = c
				}
			}
			if self.characteristic == nil {
				throw NAOSBLEError.characteristicNotFound
			}

			// enable indication notifications
			try await self.peripheral.setNotifyValue(true, for: self.characteristic!)
		}

		return await bleChannel.create(peripheral: self)
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
		// read value
		try await withTimeout(seconds: 2) {
			try await self.peripheral.writeValue(data, for: self.characteristic!, type: .withoutResponse)
		}
	}

	func close() async throws {
		// disconenct
		try await manager.cancelPeripheralConnection(peripheral)

		// clear state
		service = nil
		characteristic = nil
	}
}

class bleChannel: NAOSChannel {
	private var peripheral: NAOSBLEDevice
	private var subscription: AnyCancellable
	private var queues: [NAOSQueue] = []

	static func create(peripheral: NAOSBLEDevice) async -> bleChannel {
		// open stream
		let (stream, subscription) = await peripheral.stream()

		// create channel
		let ch = bleChannel(peripheral: peripheral, subscription: subscription)

		// run forwarder
		Task {
			for await data in stream {
				for queue in ch.queues {
					queue.send(value: data)
				}
			}
		}

		return ch
	}

	init(peripheral: NAOSBLEDevice, subscription: AnyCancellable) {
		self.peripheral = peripheral
		self.subscription = subscription
	}

	public func subscribe(queue: NAOSQueue) {
		if queues.first(where: { $0 === queue }) == nil {
			queues.append(queue)
		}
	}

	public func unsubscribe(queue: NAOSQueue) {
		queues.removeAll { $0 === queue }
	}

	public func write(data: Data) async throws {
		try await peripheral.write(data: data)
	}

	public func close() {
		subscription.cancel()
		Task { try await peripheral.close() }
	}
}
