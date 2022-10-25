//
//  Created by Joël Gähwiler on 05.04.17.
//  Copyright © 2017 Joël Gähwiler. All rights reserved.
//

import Cocoa
import CoreBluetooth

internal let NAOSDeviceService = CBUUID(string: "632FBA1B-4861-4E4F-8103-FFEE9D5033B5")

internal enum NAOSDeviceCharacteristic: String {
	case description = "87BFFDCF-0704-22A2-9C4A-7A61BC8C1726"
	case lock = "F7A5FBA4-4084-239B-684D-07D5902EB591"
	case command = "F1634D43-7F82-8891-B440-BAE5D1529229"
	case paramsList = "AC2289D1-231B-B78B-DF48-7D951A6EA665"
	case paramsSelect = "CFC9706D-406F-CCBE-4240-F88D6ED4BACD"
	case paramsValue = "01CA5446-8EE1-7E99-2041-6884B01E71B3"

	func cbuuid() -> CBUUID {
		return CBUUID(string: rawValue)
	}

	static let refreshable = [
		description, lock, paramsList,
	]

	static let all = refreshable + [command, paramsSelect, paramsValue]
}

public struct NAOSDeviceDescriptor: Hashable {
	public var name: String

	public func hash(into hasher: inout Hasher) {
		hasher.combine(name)
	}

	public static func == (lhs: NAOSDeviceDescriptor, rhs: NAOSDeviceDescriptor) -> Bool {
		return lhs.name == rhs.name
	}

	public static let deviceType = NAOSDeviceDescriptor(name: "device_type")
	public static let deviceName = NAOSDeviceDescriptor(name: "device_name")
	public static let firmwareVersion = NAOSDeviceDescriptor(name: "firmware_version")
	public static let conenctionStatus = NAOSDeviceDescriptor(name: "connection_status")
	public static let batteryLevel = NAOSDeviceDescriptor(name: "battery_level")
	public static let uptime = NAOSDeviceDescriptor(name: "uptime")
	public static let freeHeap = NAOSDeviceDescriptor(name: "free_heap")
	public static let runningPartition = NAOSDeviceDescriptor(name: "running_partition")
	public static let wifiRSSI = NAOSDeviceDescriptor(name: "wifi_rssi")
	public static let cp0Usage = NAOSDeviceDescriptor(name: "cpu0_usage")
	public static let cpu1Usage = NAOSDeviceDescriptor(name: "cpu1_usage")

	public func title() -> String {
		return name.split(separator: "_").map { str in str.capitalized }.joined(separator: " ")
	}

	public func format(value: String) -> String {
		let num = Double(value) ?? 0
		switch self {
		case .conenctionStatus:
			return value.capitalized
		case .batteryLevel:
			return String(format: "%.0f%%", num * 100)
		case .uptime:
			let formatter = DateComponentsFormatter()
			formatter.allowedUnits = [.hour, .minute, .second]
			formatter.unitsStyle = .abbreviated
			return formatter.string(from: num / 1000) ?? ""
		case .freeHeap:
			return ByteCountFormatter.string(from: Measurement(value: num, unit: .bytes), countStyle: .memory)
		case .runningPartition:
			return value
		case .wifiRSSI:
			var signal = (100 - (num * -1)) * 2
			if signal > 100 {
				signal = 100
			} else if signal < 0 {
				signal = 0
			}
			return String(format: "%.0f%%", signal)
		case .cp0Usage:
			return String(format: "%.0f%% (Sys)", num * 100)
		case .cpu1Usage:
			return String(format: "%.0f%% (App)", num * 100)
		default:
			return value
		}
	}
}

public enum NAOSDeviceParameterType: String {
	case string = "s"
	case bool = "b"
	case long = "l"
	case double = "d"
	case action = "a"
}

public struct NAOSDeviceParameterMode: OptionSet {
	public let rawValue: Int

	public init(rawValue: Int) {
		self.rawValue = rawValue
	}

	public static let volatile = NAOSDeviceParameterMode(rawValue: 1 << 0)
	public static let system = NAOSDeviceParameterMode(rawValue: 1 << 1)
	public static let application = NAOSDeviceParameterMode(rawValue: 1 << 2)
}

public struct NAOSDeviceParameter: Hashable {
	public var name: String
	public var type: NAOSDeviceParameterType
	public var mode: NAOSDeviceParameterMode

	public func hash(into hasher: inout Hasher) {
		hasher.combine(name)
	}

	public static func == (lhs: NAOSDeviceParameter, rhs: NAOSDeviceParameter) -> Bool {
		return lhs.name == rhs.name && lhs.type == rhs.type && lhs.mode == rhs.mode
	}

	public static let deviceName = NAOSDeviceParameter(name: "device-name", type: .string, mode: .system)
}

public enum NAOSDeviceError: LocalizedError {
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
	func naosDeviceDidUpdateConnectionStatus(device: NAOSDevice)
	func naosDeviceDidUnlock(device: NAOSDevice)
	func naosDeviceDidRefresh(device: NAOSDevice)
	func naosDeviceDidError(device: NAOSDevice, error: Error)
	func naosDeviceDidDisconnect(device: NAOSDevice, error: Error?)
}

public class NAOSDevice: NSObject, CBPeripheralDelegate {
	public private(set) var descriptors: [NAOSDeviceDescriptor: String] = [:]
	public private(set) var protected: Bool = false
	public private(set) var locked: Bool = false
	public private(set) var availableParameters: [NAOSDeviceParameter] = []
	public var parameters: [NAOSDeviceParameter: String] = [:]
	public var delegate: NAOSDeviceDelegate?

	internal var peripheral: CBPeripheral
	private var proxy: NAOSDeviceProxy!
	private var manager: NAOSManager
	private var service: CBService?
	private var initialRefresh: Bool = true
	private var refreshing: Bool = false
	private var tracker: [NAOSDeviceCharacteristic: Bool] = [:]
	private var currentParameter: Int = -1
	private var errorOccurred: Bool = false

	init(peripheral: CBPeripheral, manager: NAOSManager) {
		// initialize instance
		self.peripheral = peripheral
		self.manager = manager

		// initialize tracker
		for c in NAOSDeviceCharacteristic.all {
			tracker[c] = false
		}

		super.init()

		// create proxy and set delegate
		proxy = NAOSDeviceProxy(parent: self)
		peripheral.delegate = proxy

		// start initial refresh
		connect()
	}

	public func connect() {
		// connect to device
		manager.centralManager.connect(peripheral, options: nil)
	}

	public func refresh() {
		// immediately return if already refreshing
		if refreshing {
			return
		}

		// set flag
		refreshing = true

		// iterate over all readable properties
		for char in NAOSDeviceCharacteristic.refreshable {
			// get characteristic
			guard let c = towRawCharacteristic(property: char) else {
				raiseError(error: NAOSDeviceError.characteristicNotFound)
				return
			}

			// track request
			tracker[char] = true

			// issue read request
			peripheral.readValue(for: c)
		}
	}

	private func finishRefresh() {
		// set flag
		refreshing = false

		// check for initial refresh
		if initialRefresh {
			// disconnect from device
			disconnect()

			// update state
			initialRefresh = false

			// notify manager
			manager.didPrepareDevice(device: self)

			return
		}

		// notify manager
		manager.didUpdateDevice(device: self)

		// call delegate if available
		if let d = delegate {
			d.naosDeviceDidRefresh(device: self)
		}
	}

	public func title() -> String {
		return (descriptors[.deviceType] ?? "") + " (" + (parameters[.deviceName] ?? (descriptors[.deviceName] ?? "")) + ")"
	}

	public func unlock(password: String) {
		// get characteristic
		guard let c = towRawCharacteristic(property: .lock) else {
			raiseError(error: NAOSDeviceError.characteristicNotFound)
			return
		}

		// write unlock command
		peripheral.writeValue(password.data(using: .utf8)!, for: c, type: .withResponse)
	}

	public func write(parameter: NAOSDeviceParameter) {
		// return if not available
		if availableParameters.firstIndex(of: parameter) == nil {
			return
		}

		// get characteristic
		guard let selectChar = towRawCharacteristic(property: .paramsSelect) else {
			raiseError(error: NAOSDeviceError.characteristicNotFound)
			return
		}

		// get characteristic
		guard let valueChar = towRawCharacteristic(property: .paramsValue) else {
			raiseError(error: NAOSDeviceError.characteristicNotFound)
			return
		}

		// select parameter
		peripheral.writeValue(parameter.name.data(using: .utf8)!, for: selectChar, type: .withResponse)

		// write parameter
		peripheral.writeValue(parameters[parameter]!.data(using: .utf8)!, for: valueChar, type: .withResponse)

		// notify manager
		manager.didUpdateDevice(device: self)
	}

	public func disconnect() {
		// disconnect from device
		manager.centralManager.cancelPeripheralConnection(peripheral)
	}

	// NAOSManager

	internal func forwardDidConnect() {
		// discover service
		peripheral.discoverServices([NAOSDeviceService])
	}

	internal func forwardDidFailToConnect(error: Error?) {
		// check error
		if let e = error {
			raiseError(error: e)
			return
		}
	}

	internal func forwardDidDisconnect(error: Error?) {
		// lock again if protected
		if protected {
			locked = true
		}

		// return immediately if an error occurred beforehand
		if errorOccurred {
			return
		}

		// call delegate if present
		if let d = delegate {
			d.naosDeviceDidDisconnect(device: self, error: error)
		}
	}

	// NAOSDeviceProxy

	internal func peripheralDidDiscoverServices(error: Error?) {
		// check error
		if let e = error {
			raiseError(error: e)
			return
		}

		// save service reference
		for svc in peripheral.services ?? [] {
			if svc.uuid == NAOSDeviceService {
				service = svc
			}
		}

		// check existence of service
		guard let ps = service else {
			raiseError(error: NAOSDeviceError.serviceNotFound)
			return
		}

		// discover characteristics
		peripheral.discoverCharacteristics(nil, for: ps)
	}

	internal func peripheralDidDiscoverCharacteristicsFor(service: CBService, error: Error?) {
		// check error
		if let e = error {
			raiseError(error: e)
			return
		}

		// go through all characteristics
		for chr in service.characteristics ?? [] {
			// enable notifications for characteristics that support indication
			if chr.properties.contains(.indicate) {
				peripheral.setNotifyValue(true, for: chr)
			}
		}

		// perform initial refresh
		if initialRefresh {
			refresh()
			return
		}

		// call delegate if available
		if let d = delegate {
			d.naosDeviceDidConnect(device: self)
		}
	}

	internal func peripheralDidUpdateValueFor(rawChar: CBCharacteristic, error: Error?) {
		// check error
		if let e = error {
			raiseError(error: e)
			return
		}

		// get string from value
		var value = ""
		if let v = rawChar.value {
			if let s = String(data: v, encoding: .utf8) {
				value = s
			}
		}

		// get characteristic
		guard let char = fromRawCharacteristic(characteristic: rawChar) else {
			raiseError(error: NAOSDeviceError.characteristicNotFound)
			return
		}

		// handle characteristic
		if rawChar.uuid == NAOSDeviceCharacteristic.description.cbuuid() {
			// set descriptors
			print("description", value)
			for (key, value) in parseKeyValue(value: value) {
				descriptors[NAOSDeviceDescriptor(name: key)] = value
			}

			// notify delegate and return immediately if not refreshing
			if !refreshing {
				if let d = delegate {
					d.naosDeviceDidUpdateConnectionStatus(device: self)
				}
			}

		} else if rawChar.uuid == NAOSDeviceCharacteristic.lock.cbuuid() {
			// save lock status
			locked = value == "locked"

			// save if this device is protected
			if locked {
				protected = true
			}

			// notify delegate and return immediately if not refreshing
			if !refreshing, !locked {
				if let d = delegate {
					d.naosDeviceDidUnlock(device: self)
				}
			}

		} else if rawChar.uuid == NAOSDeviceCharacteristic.paramsList.cbuuid() {
			// reset list
			availableParameters = []

			// save parameters
			let segments = value.split(separator: ",")
			for s in segments {
				let subSegments = s.split(separator: ":")
				let name = String(subSegments[0])
				let type = NAOSDeviceParameterType(rawValue: String(subSegments[1])) ?? .string
				let rawMode = String(subSegments[2])
				var mode = NAOSDeviceParameterMode()
				if rawMode.contains("v") {
					mode.insert(.volatile)
				}
				if rawMode.contains("s") {
					mode.insert(.system)
				}
				if rawMode.contains("a") {
					mode.insert(.application)
				}
				availableParameters.append(NAOSDeviceParameter(name: name, type: type, mode: mode))
			}

			// queue first parameter if refreshing
			if refreshing, availableParameters.count > 0 {
				// set index
				currentParameter = 0

				// select parameter
				peripheral.writeValue(availableParameters[currentParameter].name.data(using: .utf8)!, for: towRawCharacteristic(property: .paramsSelect)!, type: .withResponse)

				// read parameter
				peripheral.readValue(for: towRawCharacteristic(property: .paramsValue)!)
			}

		} else if rawChar.uuid == NAOSDeviceCharacteristic.paramsValue.cbuuid() {
			// update parameter
			parameters[availableParameters[currentParameter]] = value

			// increment parameter
			currentParameter += 1

			// check overflow
			if currentParameter == availableParameters.count {
				currentParameter = -1
			} else {
				// select parameter
				peripheral.writeValue(availableParameters[currentParameter].name.data(using: .utf8)!, for: towRawCharacteristic(property: .paramsSelect)!, type: .withResponse)

				// read parameter
				peripheral.readValue(for: towRawCharacteristic(property: .paramsValue)!)
			}
		}

		// check if refreshing and property is marked to be refreshed
		if refreshing {
			// unmark property if marked
			if tracker[char]! {
				tracker[char] = false
			}

			// return if one property is still flagged to be refreshed
			for (_, v) in tracker {
				if v {
					return
				}
			}

			// return if parameters are not finished
			if currentParameter >= 0 {
				return
			}

			// finish refresh
			finishRefresh()
		}
	}

	internal func peripheralDidWriteValueFor(rawChar: CBCharacteristic, error: Error?) {
		// check error
		if let e = error {
			raiseError(error: e)
			return
		}
	}

	// Helpers

	private func raiseError(error: Error) {
		// check for initial refresh
		if initialRefresh {
			// notify manager
			manager.failedToPrepareDevice(device: self, error: error)
		} else {
			// call delegate if available
			if let d = delegate {
				d.naosDeviceDidError(device: self, error: error)
			}
		}

		// set flag
		errorOccurred = true

		// disconnect device
		disconnect()
	}

	private func parseKeyValue(value: String) -> [String: String] {
		var kv = [String: String]()
		for item in value.split(separator: ",") {
			let pair = item.split(separator: "=")
			let key = String(pair[0])
			if pair.count > 1 {
				kv[key] = String(pair[1])
			} else {
				kv[key] = ""
			}
		}
		return kv
	}

	private func fromRawCharacteristic(characteristic: CBCharacteristic) -> NAOSDeviceCharacteristic? {
		for property in NAOSDeviceCharacteristic.all {
			if property.cbuuid() == characteristic.uuid {
				return property
			}
		}

		return nil
	}

	private func towRawCharacteristic(property: NAOSDeviceCharacteristic) -> CBCharacteristic? {
		if let s = service {
			if let cs = s.characteristics {
				for c in cs {
					if c.uuid == property.cbuuid() {
						return c
					}
				}
			}
		}

		return nil
	}
}
