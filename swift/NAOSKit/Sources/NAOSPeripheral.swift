//
//  Created by Joël Gähwiler on 20.08.23.
//  Copyright © 2017 Joël Gähwiler. All rights reserved.
//

import AsyncBluetooth
import Combine
import CoreBluetooth
import Foundation

internal let NAOSService = CBUUID(string: "632FBA1B-4861-4E4F-8103-FFEE9D5033B5")

internal enum NAOSCharacteristic: String {
	case lock = "F7A5FBA4-4084-239B-684D-07D5902EB591"
	case list = "AC2289D1-231B-B78B-DF48-7D951A6EA665"
	case select = "CFC9706D-406F-CCBE-4240-F88D6ED4BACD"
	case value = "01CA5446-8EE1-7E99-2041-6884B01E71B3"
	case update = "87BFFDCF-0704-22A2-9C4A-7A61BC8C1726"
	case flash = "6C114DA1-9AA9-1687-5341-A1fE4C991390"
	case msg = "0360744B-A61B-00AD-C945-37f3634130F3"

	func cbuuid() -> CBUUID {
		return CBUUID(string: rawValue)
	}

	static let all: [NAOSCharacteristic] = [.lock, .list, .select, .value, .update, .flash, .msg]
}

internal class NAOSPeripheral {
	let man: CentralManager
	let raw: Peripheral

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
	}

	func discover() async throws {
		try await withTimeout(seconds: 10) {
			// discover service
			try await self.raw.discoverServices([NAOSService])

			// find service
			var service: Service?
			for svc in self.raw.discoveredServices ?? [] {
				if svc.uuid == NAOSService {
					service = svc
				}
			}
			if service == nil {
				throw NAOSError.serviceNotFound
			}

			// discover characteristics
			try await self.raw.discoverCharacteristics(nil, for: service!)

			// enable notifications for characteristics that support indication
			for char in service!.discoveredCharacteristics ?? [] {
				if char.properties.contains(.indicate) {
					try await self.raw.setNotifyValue(true, for: char)
				}
			}
		}
	}

	func exists(char: NAOSCharacteristic) -> Bool {
		towRawCharacteristic(char: char) != nil
	}

	func read(char: NAOSCharacteristic) async throws -> String {
		// get characteristic
		guard let char = towRawCharacteristic(char: char) else {
			throw NAOSError.characteristicNotFound
		}

		// read value
		try await withTimeout(seconds: 2) {
			try await self.raw.readValue(for: char)
		}

		// parse string
		let str = String(data: char.value ?? Data(capacity: 0), encoding: .utf8) ?? ""

		return str
	}

	func write(char: NAOSCharacteristic, data: String) async throws {
		try await write(char: char, data: data.data(using: .utf8)!, confirm: true)
	}

	func write(char: NAOSCharacteristic, data: Data, confirm: Bool) async throws {
		// get characteristic
		guard let char = towRawCharacteristic(char: char) else {
			throw NAOSError.characteristicNotFound
		}

		// read value
		try await withTimeout(seconds: 2) {
			try await self.raw.writeValue(data, for: char, type: confirm ? .withResponse : .withoutResponse)
		}
	}

	func receive(char: NAOSCharacteristic, operation: @escaping (Data) -> Void) -> AnyCancellable {
		// create subscription
		return raw.characteristicValueUpdatedPublisher.sink { rawChar in
			// check characteristic
			if rawChar.uuid != char.cbuuid() {
				return
			}

			// get data
			guard let data = rawChar.value else {
				return
			}

			// yield message
			operation(data)
		}
	}

	func stream(char: NAOSCharacteristic) async -> (AsyncStream<Data>, AnyCancellable) {
		// prepare stream
		var continuation: AsyncStream<Data>.Continuation?
		let stream = AsyncStream<Data> { c in
			continuation = c
		}

		// create subscription
		let subscription = raw.characteristicValueUpdatedPublisher.sink { rawChar in
			// check characteristic
			if rawChar.uuid != char.cbuuid() {
				return
			}

			// get data
			guard let data = rawChar.value else {
				return
			}

			// yield message
			continuation!.yield(data)
		}

		return (stream, AnyCancellable {
			subscription.cancel()
			continuation!.finish()
		})
	}

	func disconnect() async throws {
		try await man.cancelPeripheralConnection(raw)
	}

	// Helpers

	private func fromRawCharacteristic(char: Characteristic) -> NAOSCharacteristic? {
		for c in NAOSCharacteristic.all {
			if c.cbuuid() == char.uuid {
				return c
			}
		}

		return nil
	}

	private func towRawCharacteristic(char: NAOSCharacteristic) -> Characteristic? {
		for s in raw.discoveredServices ?? [] {
			for c in s.discoveredCharacteristics ?? [] {
				if c.uuid == char.cbuuid() {
					return c
				}
			}
		}

		return nil
	}
}
