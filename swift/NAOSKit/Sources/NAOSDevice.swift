//
//  Created by Joël Gähwiler on 05.04.17.
//  Copyright © 2017 Joël Gähwiler. All rights reserved.
//

import AsyncBluetooth
import Combine
import CoreBluetooth
import Semaphore

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

/// The available parameter types.
public enum NAOSType: UInt8 {
	case raw
	case string
	case bool
	case long
	case double
	case action

	public static func parse(str: String) -> NAOSType {
		switch str {
		case "s": return .string
		case "b": return .bool
		case "l": return .long
		case "d": return .double
		case "a": return .action
		default: return .raw
		}
	}
}

/// The available parameter modes.
public struct NAOSMode: OptionSet {
	public let rawValue: UInt8

	public init(rawValue: UInt8) {
		self.rawValue = rawValue
	}

	public static let volatile = NAOSMode(rawValue: 1 << 0)
	public static let system = NAOSMode(rawValue: 1 << 1)
	public static let application = NAOSMode(rawValue: 1 << 2)
	public static let locked = NAOSMode(rawValue: 1 << 4)

	public static func parse(str: String) -> NAOSMode {
		var mode = NAOSMode()
		if str.contains("v") {
			mode.insert(.volatile)
		}
		if str.contains("s") {
			mode.insert(.system)
		}
		if str.contains("a") {
			mode.insert(.application)
		}
		if str.contains("l") {
			mode.insert(.locked)
		}
		return mode
	}
}

/// The object representing a single NAOS parameter.
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

	public static let deviceName = NAOSParameter(
		name: "device-name", type: .string, mode: .system)
	public static let deviceType = NAOSParameter(
		name: "device-type", type: .string, mode: .system)
	public static let connectionStatus = NAOSParameter(
		name: "connection-status", type: .string, mode: .system)
	public static let battery = NAOSParameter(name: "battery", type: .double, mode: .system)
	public static let uptime = NAOSParameter(name: "uptime", type: .long, mode: .system)
	public static let freeHeap = NAOSParameter(name: "free-heap", type: .long, mode: .system)
	public static let wifiRSSI = NAOSParameter(name: "wifi-rssi", type: .long, mode: .system)
	public static let cpuUsage0 = NAOSParameter(
		name: "cpu-usage0", type: .double, mode: .system)
	public static let cpuUsage1 = NAOSParameter(
		name: "cpu-usage1", type: .double, mode: .system)

	public func format(value: String) -> String {
		let num = Double(value) ?? 0
		switch self {
		case .connectionStatus:
			return value.capitalized
		case .battery:
			return String(format: "%.0f%%", num * 100)
		case .uptime:
			let formatter = DateComponentsFormatter()
			formatter.allowedUnits = [.hour, .minute, .second]
			formatter.unitsStyle = .abbreviated
			return formatter.string(from: num / 1000) ?? ""
		case .freeHeap:
			return ByteCountFormatter.string(
				from: Measurement(value: num, unit: .bytes), countStyle: .memory)
		case .wifiRSSI:
			var signal = (100 - (num * -1)) * 2
			if signal > 100 {
				signal = 100
			} else if signal < 0 {
				signal = 0
			}
			return String(format: "%.0f%%", signal)
		case .cpuUsage0:
			return String(format: "%.0f%% (Sys)", num * 100)
		case .cpuUsage1:
			return String(format: "%.0f%% (App)", num * 100)
		default:
			return value
		}
	}
}

/// The NAOS specific errors.
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

/// The NAOS update progress.
public struct NAOSProgress {
	public var done: Int
	public var total: Int
	public var rate: Double
	public var percent: Double
}

/// The delegate implemented by objects
public protocol NAOSDeviceDelegate {
	func naosDeviceDidUpdate(device: NAOSDevice, parameter: NAOSParameter)
	func naosDeviceDidDisconnect(device: NAOSDevice, error: Error)
}

public class NAOSDevice: NSObject {
	internal var peripheral: Peripheral
	private var manager: NAOSManager
	private var service: Service?
	private var mutex = AsyncSemaphore(value: 1)
	private var refreshing: Bool = false
	private var subscription: AnyCancellable?
	private var updateReady: CheckedContinuation<Void, Never>?
	internal var updatable: Set<NAOSParameter> = Set()

	public var delegate: NAOSDeviceDelegate?
	public private(set) var connected: Bool = false
	public private(set) var protected: Bool = false
	public private(set) var locked: Bool = false
	public private(set) var availableParameters: [NAOSParameter] = []
	public var parameters: [NAOSParameter: String] = [:]
	internal var begins: [String: CheckedContinuation<UInt16, Never>] = [:]
	internal var sessions: [UInt16: NAOSSession] = [:]

	internal init(peripheral: Peripheral, manager: NAOSManager) {
		// initialize instance
		self.peripheral = peripheral
		self.manager = manager

		// finish init
		super.init()

		// initialize device name
		parameters[.deviceName] = peripheral.name
		parameters[.deviceType] = "unknown"

		// run updater
		Task {
			while true {
				// wait a second
				try await Task.sleep(nanoseconds: 1_000_000_000)

				// acquire mutex
				await mutex.wait()

				// skip if not connected or refreshing
				if !connected || refreshing {
					// release mutex
					mutex.signal()

					continue
				}

				// copy and clear updatable params
				let params = updatable
				updatable = Set()

				// release mutex
				mutex.signal()

				// attempt to read params
				for param in params {
					try? await self.read(parameter: param)
				}
			}
		}
	}

	/// Connect will initiate a connection to a device.
	public func connect() async throws {
		// acquire mutex
		await mutex.wait()
		defer { mutex.signal() }

		// connect
		try await manager.centralManager.connect(peripheral, options: nil)

		// discover services
		try await withTimeout(seconds: 2) {
			try await self.peripheral.discoverServices([NAOSService])
		}

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
		try await withTimeout(seconds: 1) {
			try await self.peripheral.discoverCharacteristics(nil, for: self.service!)
		}

		// enable notifications for characteristics that support indication
		for char in service!.discoveredCharacteristics ?? [] {
			if char.properties.contains(.indicate) {
				try await withTimeout(seconds: 1) {
					try await self.peripheral.setNotifyValue(true, for: char)
				}
			}
		}

		// set flag
		connected = true

		// read lock
		let lock = try await read(char: .lock)

		// save lock status
		locked = lock == "locked"

		// save if this device is protected
		if locked {
			protected = true
		}

		// subscribe to value updates
		subscription?.cancel()
		subscription = peripheral.characteristicValueUpdatedPublisher.sink { rawChar in
			// get value
			let rawValue = rawChar.value

			// subscriptions are handled in separate tasks that wait for other actions to complete first
			Task {
				// acquire mutex
				await self.mutex.wait()
				defer { self.mutex.signal() }

				// get value
				var value = ""
				if rawValue != nil {
					value = String(data: rawValue!, encoding: .utf8) ?? ""
				}

				// get characteristic
				let char = self.fromRawCharacteristic(char: rawChar)

				// handle flash
				if char == .flash {
					self.updateReady?.resume()
					return
				}

				// handle msg
				if char == .msg {
					// verify data
					guard let data = rawValue else {
						return
					}

					// verify size and version
					if data.count < 4 || data[0] != 1 {
						print("invalid message")
						return
					}

					// read session ID
					let sid = readUint16(data: Data(data[1 ... 2]))

					// read endpoint ID
					let eid = data[3]

					// handle "begin" replies
					if eid == 0 {
						// get handle
						let handle = String(data: Data(data[4...]), encoding: .utf8)!

						// get continuation
						guard let continuation = self.begins[handle] else {
							print("missing continuation for message")
							return
						}

						// resume continuation
						continuation.resume(returning: sid)

						return
					}

					// get session
					guard let session = self.sessions[sid] else {
						print("missing session for message")
						return
					}

					// dispatch message
					session.dispatch(msg: NAOSMessage(endpoint: eid, data: Data(data[4...])))

					return
				}

				// check update
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

				// add to set
				self.updatable.insert(param!)
			}
		}
	}

	/// Refresh will perform a full device refresh and update all parameters.
	public func refresh() async throws {
		// acquire mutex
		await mutex.wait()
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

		// read system parameters
		try await write(char: .list, data: "system")
		let system = try await read(char: .list)

		// read application parameters
		try await write(char: .list, data: "application")
		let application = try await read(char: .list)

		// read list
		let list = system + "," + application

		// reset list
		availableParameters = []

		// save parameters
		let segments = list.split(separator: ",")
		for s in segments {
			let subSegments = s.split(separator: ":")
			if subSegments.count != 3 {
				continue
			}
			let name = String(subSegments[0])
			let type = NAOSType.parse(str: String(subSegments[1]))
			let mode = NAOSMode.parse(str: String(subSegments[2]))
			availableParameters.append(
				NAOSParameter(name: name, type: type, mode: mode))
		}

		// refresh parameters
		for parameter in availableParameters {
			// select parameter
			try await write(char: .select, data: parameter.name)

			// read parameter
			parameters[parameter] = try await read(char: .value)
		}

		// notify manager
		manager.didUpdateDevice(device: self)
	}

	/// Returns the title of the device.
	public func title() -> String {
		// TODO: Precompute during refresh and updates?
		(parameters[.deviceName] ?? "") + " (" + (parameters[.deviceType] ?? "") + ")"
	}

	/// Unlock will attempt to unlock the device and returns its success.
	public func unlock(password: String) async throws -> Bool {
		// acquire mutex
		await mutex.wait()
		defer { mutex.signal() }

		// write lock
		try await write(char: .lock, data: password)

		// read lock
		let lock = try await read(char: .lock)

		return lock == "unlocked"
	}

	/// Read will read the specified parameter. The result is place into the parameters dictionary.
	public func read(parameter: NAOSParameter) async throws {
		// acquire mutex
		await mutex.wait()
		defer { mutex.signal() }

		// select parameter
		try await write(char: .select, data: parameter.name)

		// write parameter
		parameters[parameter] = try await read(char: .value)

		// notify manager
		manager.didUpdateDevice(device: self)

		// call delegate if present
		if let d = delegate {
			DispatchQueue.main.async {
				d.naosDeviceDidUpdate(device: self, parameter: parameter)
			}
		}
	}

	/// Write will write the specified parameter. The value is taken from the parameters dictionary.
	public func write(parameter: NAOSParameter) async throws {
		// acquire mutex
		await mutex.wait()
		defer { mutex.signal() }

		// select parameter
		try await write(char: .select, data: parameter.name)

		// write parameter
		try await write(char: .value, data: parameters[parameter]!)

		// notify manager
		manager.didUpdateDevice(device: self)

		// call delegate if present
		if let d = delegate {
			DispatchQueue.main.async {
				d.naosDeviceDidUpdate(device: self, parameter: parameter)
			}
		}
	}

	/// Flash will upload the provided firmware to the device and reset the device when done.
	public func flash(data: Data, progress: (NAOSProgress) -> Void) async throws {
		// acquire mutex
		await mutex.wait()
		defer { mutex.signal() }

		// begin flash
		try await write(char: .flash, data: String(format: "b%d", data.count))

		// await signal (without holding mutex)
		mutex.signal()
		do {
			try await withTimeout(seconds: 30) {
				await withCheckedContinuation { (continuation: CheckedContinuation<Void, Never>) in
					self.updateReady = continuation
				}
			}
		} catch {
			await mutex.wait()
			throw error
		}
		await mutex.wait()

		// get time
		let start = Date()

		// call progress callback
		progress(NAOSProgress(done: 0, total: data.count, rate: 0, percent: 0))

		// write chunks
		var num = 0
		for i in stride(from: 0, to: data.count, by: 500) {
			// determine end
			var end = i + 500
			if end > data.count {
				end = data.count
			}

			// prepare data
			var buf = Data()
			buf.append("w".data(using: .utf8)!)
			buf.append(data[i ..< end])

			// write data
			try await write(char: .flash, data: buf, confirm: num % 5 == 0)

			// increment
			num += 1

			// call progress callback
			let diff = Date().timeIntervalSince(start)
			progress(NAOSProgress(done: end, total: data.count, rate: Double(end) / diff, percent: 100 / Double(data.count) * Double(end)))
		}

		// call progress callback
		let diff = Date().timeIntervalSince(start)
		progress(NAOSProgress(done: data.count, total: data.count, rate: Double(data.count) / diff, percent: 100))

		// finish flash
		try await write(char: .flash, data: "f")
	}

	/// Session will create a new session and return it.
	public func session(timeout: TimeInterval) async throws -> NAOSSession? {
		// TODO: Lock mutex (arrange with async updates).

		// check characteristic
		if towRawCharacteristic(char: .msg) == nil {
			return nil
		}

		// genereate handle
		let handle = randomString(length: 16)

		// prepare message
		var msg = Data([1, 0, 0, 0])
		msg.append(handle.data(using: .utf8)!)

		// send "begin" command
		try await write(char: .msg, data: msg, confirm: false)

		// await response
		let sid = try await withTimeout(seconds: timeout) {
			await withCheckedContinuation { (continuation: CheckedContinuation<UInt16, Never>) in
				// store continuation
				self.begins[handle] = continuation
			}
		}

		// clear continuation
		begins[handle] = nil

		// create session
		let session = NAOSSession(device: self, id: sid)

		// store session
		sessions[sid] = session

		return session
	}

	/// Disconnect will close the connection to the device.
	public func disconnect() async throws {
		// acquire mutex
		await mutex.wait()
		defer { mutex.signal() }

		// lock again if protected
		if protected {
			locked = true
		}

		// set flag
		connected = false

		// cancel subscription
		subscription?.cancel()

		// disconnect from device
		try await manager.centralManager.cancelPeripheralConnection(peripheral)
	}

	// NAOSManager

	internal func didDisconnect(error: Error) async {
		// acquire mutex
		await mutex.wait()
		defer { mutex.signal() }

		// lock again if protected
		if protected {
			locked = true
		}

		// set flag
		connected = false

		// cancel subscription
		subscription?.cancel()

		// call delegate if available
		if let d = delegate {
			DispatchQueue.main.async {
				d.naosDeviceDidDisconnect(device: self, error: error)
			}
		}
	}

	// NAOSSession

	internal func send(data: Data) async throws {
		// acquire mutex
		await mutex.wait()
		defer { mutex.signal() }

		// send message
		try await write(char: .msg, data: data, confirm: false)
	}

	// Helpers

	internal func read(char: NAOSCharacteristic) async throws -> String {
		// get characteristic
		guard let char = towRawCharacteristic(char: char) else {
			throw NAOSError.characteristicNotFound
		}

		// read value
		try await withTimeout(seconds: 2) {
			try await self.peripheral.readValue(for: char)
		}

		// parse string
		let str = String(data: char.value ?? Data(capacity: 0), encoding: .utf8) ?? ""

		return str
	}

	internal func write(char: NAOSCharacteristic, data: String) async throws {
		try await write(char: char, data: data.data(using: .utf8)!, confirm: true)
	}

	internal func write(char: NAOSCharacteristic, data: Data, confirm: Bool) async throws {
		// get characteristic
		guard let char = towRawCharacteristic(char: char) else {
			throw NAOSError.characteristicNotFound
		}

		// read value
		try await withTimeout(seconds: 2) {
			try await self.peripheral.writeValue(data, for: char, type: confirm ? .withResponse : .withoutResponse)
		}
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
