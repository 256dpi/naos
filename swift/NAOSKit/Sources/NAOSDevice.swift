//
//  Created by Joël Gähwiler on 05.04.17.
//  Copyright © 2017 Joël Gähwiler. All rights reserved.
//

import Combine
import Foundation
import Semaphore

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
	public var ref: UInt8 = 0

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
	case missingSession

	public var errorDescription: String? {
		switch self {
		case .serviceNotFound:
			return "Device service not found."
		case .characteristicNotFound:
			return "Device characteristic not found."
		case .missingSession:
			return "Could not open session."
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
	private var manager: NAOSManager
	private var mutex = AsyncSemaphore(value: 1)
	private var paramSession: NAOSSession?
	private var refreshing: Bool = false
	private var updater: AnyCancellable?
	private var readier: AnyCancellable?
	private var updateReady: CheckedContinuation<Void, Never>?

	internal var peripheral: NAOSPeripheral
	internal var updatable: Set<NAOSParameter> = Set()
	internal var maxAge: UInt64 = 0

	public var delegate: NAOSDeviceDelegate?
	public private(set) var connected: Bool = false
	public private(set) var protected: Bool = false
	public private(set) var locked: Bool = false
	public private(set) var availableParameters: [NAOSParameter] = []
	public var parameters: [NAOSParameter: String] = [:]

	internal init(peripheral: NAOSPeripheral, manager: NAOSManager) {
		// initialize instance
		self.peripheral = peripheral
		self.manager = manager

		// finish init
		super.init()

		// initialize device name
		parameters[.deviceName] = peripheral.name()
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

				// use session or legacy attributes
				if peripheral.exists(char: .msg) {
					try? await withParamSession { session in
						// create endpoint
						let endpoint = NAOSParamsEndpoint(session: session, timeout: 5)

						// collet parameters
						var updates: [NAOSParamUpdate] = []
						do {
							updates = try await endpoint.collect(refs: nil, since: maxAge)
						} catch {
							mutex.signal()
							return
						}

						// update parameters
						for update in updates {
							if let param = (availableParameters.first { p in p.ref == update.ref }) {
								parameters[param] = String(data: update.value, encoding: .utf8)!
								maxAge = max(maxAge, update.age)
							}
						}

						// release mutex
						mutex.signal()

						// notify manager
						manager.didUpdateDevice(device: self)

						// call delegate if present
						if let d = delegate {
							for update in updates {
								DispatchQueue.main.async {
									if let param = (self.availableParameters.first { p in p.ref == update.ref }) {
										d.naosDeviceDidUpdate(device: self, parameter: param)
									}
								}
							}
						}
					}
				} else {
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
	}

	/// Connect will initiate a connection to a device.
	public func connect() async throws {
		// acquire mutex
		await mutex.wait()
		defer { mutex.signal() }

		// connect
		try await peripheral.connect()

		// discover
		try await peripheral.discover()

		// set flag
		connected = true

		// read lock
		let lock = try await peripheral.read(char: .lock)

		// save lock status
		locked = lock == "locked"

		// save if this device is protected
		if locked {
			protected = true
		}

		// reset max aage
		maxAge = 0

		// cancel previous subscriptions
		updater?.cancel()
		readier?.cancel()

		// subscribe to value updates if not using session
		if !peripheral.exists(char: .msg) {
			updater = peripheral.receive(char: .update, operation: { data in
				Task {
					// acquire mutex
					await self.mutex.wait()
					defer { self.mutex.signal() }

					// get name
					let name = String(data: data, encoding: .utf8) ?? ""

					// find parameter
					let param = self.availableParameters.first { param in
						param.name == name
					}
					if param == nil {
						return
					}

					// add to set
					self.updatable.insert(param!)
				}
			})
		}

		// subscribe to flash updates
		readier = peripheral.receive(char: .flash, operation: { _ in
			Task {
				// acquire mutex
				await self.mutex.wait()
				defer { self.mutex.signal() }

				// handle flash
				self.updateReady?.resume()
			}
		})
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
		let lock = try await peripheral.read(char: .lock)

		// save lock status
		locked = lock == "locked"

		// save if this device is protected and stop
		if locked {
			protected = true
			return
		}

		// use session or legacy attributes
		if peripheral.exists(char: .msg) {
			try await withParamSession { session in
				// create endpoint
				let endpoint = NAOSParamsEndpoint(session: session, timeout: 5)

				// list parameters
				let list = try await endpoint.list()

				// save parameters
				availableParameters = []
				for info in list {
					availableParameters.append(NAOSParameter(name: info.name, type: info.type, mode: info.mode, ref: info.ref))
				}

				// prepare map
				let map = availableParameters.filter { p in
					p.type != .action
				}.map { p in
					p.ref
				}

				// refresh parameters
				for update in try await endpoint.collect(refs: map, since: 0) {
					let param = availableParameters.first(where: { p in p.ref == update.ref })!
					parameters[param] = String(data: update.value, encoding: .utf8)!
					maxAge = max(maxAge, update.age)
				}
			}
		} else {
			// read system parameters
			try await peripheral.write(char: .list, data: "system")
			let system = try await peripheral.read(char: .list)

			// read application parameters
			try await peripheral.write(char: .list, data: "application")
			let application = try await peripheral.read(char: .list)

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
				try await peripheral.write(char: .select, data: parameter.name)

				// read parameter
				parameters[parameter] = try await peripheral.read(char: .value)
			}
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
		try await peripheral.write(char: .lock, data: password)

		// read lock
		let lock = try await peripheral.read(char: .lock)

		return lock == "unlocked"
	}

	/// Read will read the specified parameter. The result is place into the parameters dictionary.
	public func read(parameter: NAOSParameter) async throws {
		// acquire mutex
		await mutex.wait()
		defer { mutex.signal() }

		// use session or legacy attributes
		if peripheral.exists(char: .msg) {
			try await withParamSession { session in
				// create endpoint
				let endpoint = NAOSParamsEndpoint(session: session, timeout: 5)

				// read value
				let value = try await endpoint.read(ref: parameter.ref)

				// write parameter
				parameters[parameter] = String(data: value, encoding: String.Encoding.utf8)
			}
		} else {
			// select parameter
			try await peripheral.write(char: .select, data: parameter.name)

			// write parameter
			parameters[parameter] = try await peripheral.read(char: .value)
		}

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

		// use session or legacy attributes
		if peripheral.exists(char: .msg) {
			try await withParamSession { session in
				// create endpoint
				let endpoint = NAOSParamsEndpoint(session: session, timeout: 5)

				// write parameter
				try await endpoint.write(ref: parameter.ref, value: parameters[parameter]!.data(using: .utf8)!)
			}
		} else {
			// select parameter
			try await peripheral.write(char: .select, data: parameter.name)

			// write parameter
			try await peripheral.write(char: .value, data: parameters[parameter]!)
		}

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
	public func flash(data: Data, progress: @escaping (NAOSProgress) -> Void) async throws {
		// acquire mutex
		await mutex.wait()
		defer { mutex.signal() }
		
		// check characteristic
		if peripheral.exists(char: .msg) {
			// open session
			if let session = try await NAOSSession.open(peripheral: peripheral, timeout: 5) {
				// create endpoint
				let endpoint = NAOSUpdateEndpoint(session: session)
				
				// get time
				let start = Date()
				
				// run update
				try await endpoint.run(image: data) { offset in
					let diff = Date().timeIntervalSince(start)
					progress(NAOSProgress(done: offset, total: data.count, rate: Double(offset) / diff, percent: 100 / Double(data.count) * Double(offset)))
				}
				
				// end session
				try await session.end(timeout: 5)
				
				return
			}
		}

		// begin flash
		try await peripheral.write(char: .flash, data: String(format: "b%d", data.count))

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
			try await peripheral.write(char: .flash, data: buf, confirm: num % 5 == 0)

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
		try await peripheral.write(char: .flash, data: "f")
	}

	/// Session will create a new session and return it.
	public func session(timeout: TimeInterval) async throws -> NAOSSession? {
		// acquire mutex
		await mutex.wait()
		defer { mutex.signal() }

		// check characteristic
		if !peripheral.exists(char: .msg) {
			return nil
		}

		return try await NAOSSession.open(peripheral: peripheral, timeout: timeout)
	}

	/// Disconnect will close the connection to the device.
	public func disconnect() async throws {
		// acquire mutex
		await mutex.wait()
		defer { mutex.signal() }

		// clear session
		paramSession = nil

		// lock again if protected
		if protected {
			locked = true
		}

		// set flag
		connected = false

		// cancel subscription
		updater?.cancel()

		// disconnect from device
		try await peripheral.disconnect()
	}

	// Helpers

	private func withParamSession(callback: (NAOSSession) async throws -> Void) async throws {
		// ensure session
		if paramSession == nil {
			if let sess = try await NAOSSession.open(peripheral: peripheral, timeout: 5) {
				paramSession = sess
			} else {
				throw NAOSError.missingSession
			}
		}

		// yield session
		do {
			try await callback(paramSession!)
		} catch {
			paramSession = nil
			throw error
		}
	}

	// NAOSManager

	internal func didDisconnect(error: Error) async {
		// acquire mutex
		await mutex.wait()
		defer { mutex.signal() }

		// clear session
		paramSession = nil

		// lock again if protected
		if protected {
			locked = true
		}

		// set flag
		connected = false

		// cancel subscription
		updater?.cancel()

		// call delegate if available
		if let d = delegate {
			DispatchQueue.main.async {
				d.naosDeviceDidDisconnect(device: self, error: error)
			}
		}
	}
}
