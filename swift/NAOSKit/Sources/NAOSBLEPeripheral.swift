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

class NAOSBLEPeripheral {
	let man: CentralManager
	let raw: Peripheral

	var svc: Service? = nil
	var char: Characteristic? = nil

	init(man: CentralManager, raw: Peripheral) {
		self.man = man
		self.raw = raw
	}

	func name() -> String {
		raw.name ?? "Unnamed"
	}

	func identifier() -> UUID {
		return raw.identifier
	}

	func connect() async throws {
		try await man.connect(raw)
	
		try await withTimeout(seconds: 10) {
			// discover service
			try await self.raw.discoverServices([bleService])

			// find service
			for s in self.raw.discoveredServices ?? [] {
				if s.uuid == bleService {
					self.svc = s
				}
			}
			if self.svc == nil {
				throw NAOSBLEError.serviceNotFound
			}

			// discover characteristics
			try await self.raw.discoverCharacteristics(nil, for: self.svc!)

			// find characteristic
			for c in self.svc!.discoveredCharacteristics ?? [] {
				if c.uuid == bleCharacteristic {
					self.char = c
				}
			}
			if self.char == nil {
				throw NAOSBLEError.characteristicNotFound
			}

			// enable indication notifications
			try await self.raw.setNotifyValue(true, for: self.char!)
		}
	}
	
	func channel() async -> NAOSChannel {
		return await bleChannel.create(peripheral: self)
	}

	func write(data: Data) async throws {
		// read value
		try await withTimeout(seconds: 2) {
			try await self.raw.writeValue(data, for: self.char!, type: .withoutResponse)
		}
	}

	func stream() async -> (AsyncStream<Data>, AnyCancellable) {
		// prepare stream
		var continuation: AsyncStream<Data>.Continuation?
		let stream = AsyncStream<Data> { c in
			continuation = c
		}

		// create subscription
		let subscription = raw.characteristicValueUpdatedPublisher.sink { rawChar in
			// check characteristic
			if rawChar.uuid != bleCharacteristic {
				return
			}

			// get data
			guard let data = rawChar.value else {
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

	func disconnect() async throws {
		// disconenct
		try await man.cancelPeripheralConnection(raw)

		// clear state
		svc = nil
		char = nil
	}
}

class bleChannel: NAOSChannel {
	private var peripheral: NAOSBLEPeripheral
	private var subscription: AnyCancellable
	private var queues: [NAOSQueue] = []
	
	static func create(peripheral: NAOSBLEPeripheral) async -> bleChannel {
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
	
	init(peripheral: NAOSBLEPeripheral, subscription: AnyCancellable) {
		self.peripheral = peripheral
		self.subscription = subscription
	}
	
	public func subscribe(queue: NAOSQueue) {
		if queues.first(where: {$0 === queue}) == nil {
			queues.append(queue)
		}
	}
	
	public func unsubscribe(queue: NAOSQueue) {
		queues.removeAll{$0 === queue}
	}
	
	public func write(data: Data) async throws {
		try await peripheral.write(data: data)
	}
	
	public func close() {
		subscription.cancel()
	}
}
