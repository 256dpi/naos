//
//  Created by Joël Gähwiler on 05.04.17.
//  Copyright © 2017 Joël Gähwiler. All rights reserved.
//

import AsyncBluetooth
import Cocoa
import Combine
import CoreBluetooth

internal let NAOSService = CBUUID(string: "632FBA1B-4861-4E4F-8103-FFEE9D5033B5")

internal enum NAOSCharacteristic: String {
	case lock = "F7A5FBA4-4084-239B-684D-07D5902EB591"
	case list = "AC2289D1-231B-B78B-DF48-7D951A6EA665"
	case select = "CFC9706D-406F-CCBE-4240-F88D6ED4BACD"
	case value = "01CA5446-8EE1-7E99-2041-6884B01E71B3"
	case update = "87BFFDCF-0704-22A2-9C4A-7A61BC8C1726"

	func cbuuid() -> CBUUID {
		return CBUUID(string: rawValue)
	}

	static let all: [NAOSCharacteristic] = [.lock, .list, .select, .value, .update]
}

public enum NAOSType: String {
	case string = "s"
	case bool = "b"
	case long = "l"
	case double = "d"
	case action = "a"
}

public struct NAOSMode: OptionSet {
	public let rawValue: Int

	public init(rawValue: Int) {
		self.rawValue = rawValue
	}

	public static let volatile = NAOSMode(rawValue: 1 << 0)
	public static let system = NAOSMode(rawValue: 1 << 1)
	public static let application = NAOSMode(rawValue: 1 << 2)
	public static let _public = NAOSMode(rawValue: 1 << 3)
	public static let locked = NAOSMode(rawValue: 1 << 4)
}

public struct NAOSParameter: Hashable {
	public var name: String
	public var type: NAOSType
	public var mode: NAOSMode

	public func hash(into hasher: inout Hasher) {
		hasher.combine(name)
	}

	public static func == (lhs: NAOSParameter, rhs: NAOSParameter) -> Bool {
		return lhs.name == rhs.name && lhs.type == rhs.type
	}

	public static let deviceName = NAOSParameter(name: "device-name", type: .string, mode: .system)
	public static let deviceType = NAOSParameter(name: "device-type", type: .string, mode: .system)
	public static let connectionStatus = NAOSParameter(name: "connection-status", type: .string, mode: .system)

//	public func format(value: String) -> String {
//		let num = Double(value) ?? 0
//		switch self {
//		case .conenctionStatus:
//			return value.capitalized
//		case .batteryLevel:
//			return String(format: "%.0f%%", num * 100)
//		case .uptime:
//			let formatter = DateComponentsFormatter()
//			formatter.allowedUnits = [.hour, .minute, .second]
//			formatter.unitsStyle = .abbreviated
//			return formatter.string(from: num / 1000) ?? ""
//		case .freeHeap:
//			return ByteCountFormatter.string(from: Measurement(value: num, unit: .bytes), countStyle: .memory)
//		case .runningPartition:
//			return value
//		case .wifiRSSI:
//			var signal = (100 - (num * -1)) * 2
//			if signal > 100 {
//				signal = 100
//			} else if signal < 0 {
//				signal = 0
//			}
//			return String(format: "%.0f%%", signal)
//		case .cp0Usage:
//			return String(format: "%.0f%% (Sys)", num * 100)
//		case .cpu1Usage:
//			return String(format: "%.0f%% (App)", num * 100)
//		default:
//			return value
//		}
//	}
}

public enum NAOSError: LocalizedError {
	case serviceNotFound
	case characteristicNotFound

	public var errorDescription: String? {
		switch self {
		case .serviceNotFound:
			return "Device service not found."
		case .characteristicNotFound:
			return "Device characteristic not found."
		}
	}
}

public protocol NAOSDeviceDelegate {
	func naosDeviceDidConnect(device: NAOSDevice)
	func naosDeviceDidUnlock(device: NAOSDevice)
	func naosDeviceDidRefresh(device: NAOSDevice)
	func naosDeviceDidUpdate(device: NAOSDevice, parameter: NAOSParameter)
	func naosDeviceDidDisconnect(device: NAOSDevice, error: Error?)
}

public class NAOSDevice: NSObject {
	internal var peripheral: Peripheral
	private var manager: NAOSManager
	private var service: Service?
	private var mutex: DispatchSemaphore
	private var refreshing: Bool = false
	private var subscription: AnyCancellable?

	public var delegate: NAOSDeviceDelegate?
	public private(set) var protected: Bool = false
	public private(set) var locked: Bool = false
	public private(set) var availableParameters: [NAOSParameter] = []
	public var parameters: [NAOSParameter: String] = [:]

	init(peripheral: Peripheral, manager: NAOSManager) {
		// initialize instance
		self.peripheral = peripheral
		self.manager = manager

		// create mutex
		mutex = DispatchSemaphore(value: 1)

		// initialize super
		super.init()
	}

	public func connect() async throws {
		// acquire mutex
		mutex.wait()
		defer { mutex.signal() }

		// connect
		try await manager.centralManager.connect(peripheral, options: nil)

		// discover services
		try await peripheral.discoverServices([NAOSService])

		// find service
		for svc in peripheral.discoveredServices ?? [] {
			if svc.uuid == NAOSService {
				service = svc
			}
		}
		if service == nil {
			throw NAOSError.serviceNotFound
		}

		// discover characteristics
		try await peripheral.discoverCharacteristics(nil, for: service!)

		// enable notifications for characteristics that support indication
		for char in service!.discoveredCharacteristics ?? [] {
			if char.properties.contains(.indicate) {
				try await peripheral.setNotifyValue(true, for: char)
			}
		}

		// subscribe to value updates
		subscription?.cancel()
		subscription = peripheral.characteristicValueUpdatedPublisher.sink { char in
			Task {
				// skip if refreshing
				if self.refreshing {
					return
				}

				// get value
				var value = ""
				if char.value != nil {
					value = String(data: char.value!, encoding: .utf8) ?? ""
				}

				// get characteristic
				let char = self.fromRawCharacteristic(char: char)
				if char != .update {
					return
				}

				// find parameter
				let param = self.availableParameters.first { param in
					param.name == value
				}
				if param == nil {
					return
				}

				// update parameter
				try await self.read(parameter: param!)

				print("updated", value)
			}
		}

		// call delegate if available
		if let d = delegate {
			DispatchQueue.main.async {
				d.naosDeviceDidConnect(device: self)
			}
		}
	}

	public func refresh() async throws {
		// acquire mutex
		mutex.wait()
		defer { mutex.signal() }

		// manage flag
		refreshing = true
		defer { refreshing = false }

		// read lock
		let lock = try await read(char: .lock)

		// save lock status
		locked = lock == "locked"

		// save if this device is protected
		if locked {
			protected = true
		}

		// read list
		let list = try await read(char: .list)

		// reset list
		availableParameters = []

		// save parameters
		let segments = list.split(separator: ",")
		for s in segments {
			let subSegments = s.split(separator: ":")
			let name = String(subSegments[0])
			let type = NAOSType(rawValue: String(subSegments[1])) ?? .string
			let rawMode = String(subSegments[2])
			var mode = NAOSMode()
			if rawMode.contains("v") {
				mode.insert(.volatile)
			}
			if rawMode.contains("s") {
				mode.insert(.system)
			}
			if rawMode.contains("a") {
				mode.insert(.application)
			}
			if rawMode.contains("p") {
				mode.insert(._public)
			}
			if rawMode.contains("l") {
				mode.insert(.locked)
			}
			availableParameters.append(NAOSParameter(name: name, type: type, mode: mode))
		}

		// refresh parameters
		for parameter in availableParameters {
			// select parameter
			try await write(char: .select, data: parameter.name)

			// read parameter
			parameters[parameter] = try await read(char: .value)
		}

		// notify manager
		manager.didRefreshDevice(device: self)

		// call delegate if available
		if let d = delegate {
			DispatchQueue.main.async {
				d.naosDeviceDidRefresh(device: self)
			}
		}
	}

	public func title() -> String {
		return (parameters[.deviceName] ?? "") + " (" + (parameters[.deviceType] ?? "") + ")"
	}

	public func unlock(password: String) async throws -> Bool {
		// acquire mutex
		mutex.wait()
		defer { mutex.signal() }

		// write lock
		try await write(char: .lock, data: password)

		// read lock
		let lock = try await read(char: .lock)

		// check lock
		if lock != "unlocked" {
			return false
		}

		// call delegate if present
		if let d = delegate {
			DispatchQueue.main.async {
				d.naosDeviceDidUnlock(device: self)
			}
		}

		return true
	}

	public func read(parameter: NAOSParameter) async throws {
		// acquire mutex
		mutex.wait()
		defer { mutex.signal() }

		// select parameter
		try await write(char: .select, data: parameter.name)

		// write parameter
		parameters[parameter] = try await read(char: .value)

		// notify manager
		manager.didRefreshDevice(device: self)

		// call delegate if present
		if let d = delegate {
			DispatchQueue.main.async {
				d.naosDeviceDidUpdate(device: self, parameter: parameter)
			}
		}
	}

	public func write(parameter: NAOSParameter) async throws {
		// acquire mutex
		mutex.wait()
		defer { mutex.signal() }

		// select parameter
		try await write(char: .select, data: parameter.name)

		// write parameter
		try await write(char: .value, data: parameters[parameter]!)

		// notify manager
		manager.didRefreshDevice(device: self)

		// call delegate if present
		if let d = delegate {
			DispatchQueue.main.async {
				d.naosDeviceDidUpdate(device: self, parameter: parameter)
			}
		}
	}

	public func disconnect() async throws {
		// acquire mutex
		mutex.wait()
		defer { mutex.signal() }

		// lock again if protected
		if protected {
			locked = true
		}

		// cancel subscription
		subscription?.cancel()

		// disconnect from device
		try await manager.centralManager.cancelPeripheralConnection(peripheral)

		// call delegate if present
		if let d = delegate {
			DispatchQueue.main.async {
				d.naosDeviceDidDisconnect(device: self, error: nil)
			}
		}
	}

	// Helpers

	internal func read(char: NAOSCharacteristic) async throws -> String {
		// get characteristic
		guard let char = towRawCharacteristic(char: char) else {
			throw NAOSError.characteristicNotFound
		}

		// read value
		try await peripheral.readValue(for: char)

		// parse string
		let str = String(data: char.value ?? Data(capacity: 0), encoding: .utf8) ?? ""

		return str
	}

	internal func write(char: NAOSCharacteristic, data: String) async throws {
		// get characteristic
		guard let char = towRawCharacteristic(char: char) else {
			throw NAOSError.characteristicNotFound
		}

		// read value
		try await peripheral.writeValue(data.data(using: .utf8)!, for: char, type: .withResponse)
	}

	private func fromRawCharacteristic(char: Characteristic) -> NAOSCharacteristic? {
		for property in NAOSCharacteristic.all {
			if property.cbuuid() == char.uuid {
				return property
			}
		}

		return nil
	}

	private func towRawCharacteristic(char: NAOSCharacteristic) -> Characteristic? {
		if let s = service {
			if let cs = s.discoveredCharacteristics {
				for c in cs {
					if c.uuid == char.cbuuid() {
						return c
					}
				}
			}
		}

		return nil
	}
}
