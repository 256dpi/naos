//
//  Created by Joël Gähwiler on 05.04.17.
//  Copyright © 2017 Joël Gähwiler. All rights reserved.
//

import Cocoa
import CoreBluetooth

internal let NAOSDeviceService = CBUUID(string: "632FBA1B-4861-4E4F-8103-FFEE9D5033B5")

internal enum NAOSDeviceCharacteristic: String {
	case identity = "276A12D7-1008-DDBB-CA4F-67D654EF9E45"
	case lock = "F7A5FBA4-4084-239B-684D-07D5902EB591"
	case description = "87BFFDCF-0704-22A2-9C4A-7A61BC8C1726"
	case settingsList = "DEAEE42C-B5EB-80A9-BB4C-5C88E55F285D"
	case settingsSelect = "A97F99BB-339B-87BD-B848-2D7A62CCF37B"
	case settingsValue = "C8BEBECB-E7E1-50A0-614A-0AFF25C9947E"
	case command = "F1634D43-7F82-8891-B440-BAE5D1529229"
	case paramsList = "AC2289D1-231B-B78B-DF48-7D951A6EA665"
	case paramsSelect = "CFC9706D-406F-CCBE-4240-F88D6ED4BACD"
	case paramsValue = "01CA5446-8EE1-7E99-2041-6884B01E71B3"

	func cbuuid() -> CBUUID {
		return CBUUID(string: rawValue)
	}

	static let refreshable = [
		identity, lock, description, settingsList, paramsList,
	]

	static let all = refreshable + [settingsSelect, settingsValue, command, paramsSelect, paramsValue]
}

public struct NAOSDeviceSetting: Hashable {
	public var name: String

	public func hash(into hasher: inout Hasher) {
		hasher.combine(name)
	}

	public static func == (lhs: NAOSDeviceSetting, rhs: NAOSDeviceSetting) -> Bool {
		return lhs.name == rhs.name
	}

	public static let wifiSSID = NAOSDeviceSetting(name: "wifi-ssid")
	public static let wifiPassword = NAOSDeviceSetting(name: "wifi-password")
	public static let mqttHost = NAOSDeviceSetting(name: "mqtt-host")
	public static let mqttPort = NAOSDeviceSetting(name: "mqtt-port")
	public static let mqttClientID = NAOSDeviceSetting(name: "mqtt-client-id")
	public static let mqttUsername = NAOSDeviceSetting(name: "mqtt-username")
	public static let mqttPassword = NAOSDeviceSetting(name: "mqtt-password")
	public static let deviceName = NAOSDeviceSetting(name: "device-name")
	public static let baseTopic = NAOSDeviceSetting(name: "base-topic")

	public static let all = [
		wifiSSID, wifiPassword, mqttHost, mqttPort, mqttClientID,
		mqttUsername, mqttPassword, deviceName, baseTopic,
	]
}

public enum NAOSDeviceCommand: String {
	case ping
	case reboot
	case restartWifi = "restart-wifi"
	case restartMQTT = "restart-mqtt"
}

public enum NAOSDeviceParameterType: String {
	case string = "s"
	case bool = "b"
	case long = "l"
	case double = "d"
}

public struct NAOSDeviceParameter: Hashable {
	public var name: String
	public var type: NAOSDeviceParameterType

	public func hash(into hasher: inout Hasher) {
		hasher.combine(name)
	}

	public static func == (lhs: NAOSDeviceParameter, rhs: NAOSDeviceParameter) -> Bool {
		return lhs.name == rhs.name && lhs.type == rhs.type
	}
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
	public private(set) var deviceType: String = ""
	public private(set) var deviceName: String = ""
	public private(set) var connectionStatus: String = ""
	public private(set) var batteryLevel: Float = -1
	public private(set) var protected: Bool = false
	public private(set) var locked: Bool = false
	public private(set) var availableSettings: [NAOSDeviceSetting] = []
	public var settings: [NAOSDeviceSetting: String] = [:]
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
	private var currentSetting: Int = -1
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

	public func name() -> String {
		return deviceType + " (" + (settings[.deviceName] ?? deviceName) + ")"
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

	public func write(setting: NAOSDeviceSetting) {
		// return if not available
		if availableSettings.firstIndex(of: setting) == nil {
			return
		}

		// get characteristic
		guard let selectChar = towRawCharacteristic(property: .settingsSelect) else {
			raiseError(error: NAOSDeviceError.characteristicNotFound)
			return
		}

		// get characteristic
		guard let valueChar = towRawCharacteristic(property: .settingsValue) else {
			raiseError(error: NAOSDeviceError.characteristicNotFound)
			return
		}

		// select setting
		peripheral.writeValue(setting.name.data(using: .utf8)!, for: selectChar, type: .withResponse)

		// write setting
		peripheral.writeValue(settings[setting]!.data(using: .utf8)!, for: valueChar, type: .withResponse)

		// notify manager
		manager.didUpdateDevice(device: self)
	}

	public func execute(cmd: NAOSDeviceCommand) {
		// get characteristic
		guard let c = towRawCharacteristic(property: .command) else {
			raiseError(error: NAOSDeviceError.characteristicNotFound)
			return
		}

		// write system command
		peripheral.writeValue(cmd.rawValue.data(using: .utf8)!, for: c, type: .withResponse)
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
		if rawChar.uuid == NAOSDeviceCharacteristic.identity.cbuuid() {
			// parse key-value
			let kv = parseKeyValue(value: value)

			// set device type and name
			deviceType = kv["device_type"] ?? ""
			deviceName = kv["device_name"] ?? ""

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

		} else if rawChar.uuid == NAOSDeviceCharacteristic.description.cbuuid() {
			// parse key-value
			let kv = parseKeyValue(value: value)

			// set connection status and battery level
			connectionStatus = kv["connection_status"] ?? ""
			batteryLevel = Float(kv["battery_level"] ?? "-1") ?? -1

			// notify delegate and return immediately if not refreshing
			if !refreshing {
				if let d = delegate {
					d.naosDeviceDidUpdateConnectionStatus(device: self)
				}
			}

		} else if rawChar.uuid == NAOSDeviceCharacteristic.settingsList.cbuuid() {
			// reset list
			availableSettings = []

			// save settings
			for name in value.split(separator: ",") {
				availableSettings.append(NAOSDeviceSetting(name: String(name)))
			}

			// queue first setting if refreshing
			if refreshing, availableSettings.count > 0 {
				// set index
				currentSetting = 0

				// select setting
				peripheral.writeValue(availableSettings[currentSetting].name.data(using: .utf8)!, for: towRawCharacteristic(property: .settingsSelect)!, type: .withResponse)

				// read setting
				peripheral.readValue(for: towRawCharacteristic(property: .settingsValue)!)
			}

		} else if rawChar.uuid == NAOSDeviceCharacteristic.settingsValue.cbuuid() {
			// update setting
			settings[availableSettings[currentSetting]] = value

			// increment setting
			currentSetting += 1

			// check overflow
			if currentSetting == availableSettings.count {
				currentSetting = -1
			} else {
				// select setting
				peripheral.writeValue(availableSettings[currentSetting].name.data(using: .utf8)!, for: towRawCharacteristic(property: .settingsSelect)!, type: .withResponse)

				// read setting
				peripheral.readValue(for: towRawCharacteristic(property: .settingsValue)!)
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
				availableParameters.append(NAOSDeviceParameter(name: name, type: type))
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
			let value = String(pair[1])
			kv[key] = value
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
