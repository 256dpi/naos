//
//  Created by Joël Gähwiler on 20.08.23.
//  Copyright © 2017 Joël Gähwiler. All rights reserved.
//

import AsyncBluetooth
import Combine
import CoreBluetooth
import Foundation

let NAOSService = CBUUID(string: "632FBA1B-4861-4E4F-8103-FFEE9D5033B5")
let NAOSCharacteristic = CBUUID(string: "0360744B-A61B-00AD-C945-37f3634130F3")

class NAOSPeripheral {
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
	}

	func discover() async throws {
		try await withTimeout(seconds: 10) {
			// discover service
			try await self.raw.discoverServices([NAOSService])

			// find service
			for s in self.raw.discoveredServices ?? [] {
				if s.uuid == NAOSService {
					self.svc = s
				}
			}
			if self.svc == nil {
				throw NAOSError.serviceNotFound
			}

			// discover characteristics
			try await self.raw.discoverCharacteristics(nil, for: self.svc!)

			// find characteristic
			for c in self.svc!.discoveredCharacteristics ?? [] {
				if c.uuid == NAOSCharacteristic {
					self.char = c
				}
			}
			if self.char == nil {
				throw NAOSError.characteristicNotFound
			}

			// enable indication notifications
			try await self.raw.setNotifyValue(true, for: self.char!)
		}
	}

	func write(data: Data, confirm: Bool) async throws {
		// read value
		try await withTimeout(seconds: 2) {
			try await self.raw.writeValue(
				data, for: self.char!, type: confirm ? .withResponse : .withoutResponse)
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
			if rawChar.uuid != NAOSCharacteristic {
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
