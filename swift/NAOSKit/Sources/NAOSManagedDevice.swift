//
//  Created by Joël Gähwiler on 05.04.17.
//  Copyright © 2017 Joël Gähwiler. All rights reserved.
//

import Foundation
import Semaphore

/// The object representing a single NAOS parameter.
public struct NAOSParameter: Hashable {
	public var name: String
	public var type: NAOSParamType = .raw
	public var mode: NAOSParamMode = .init(rawValue: 0)
	public var ref: UInt8 = 0

	public func hash(into hasher: inout Hasher) {
		hasher.combine(name)
	}

	public static func == (lhs: NAOSParameter, rhs: NAOSParameter) -> Bool {
		return lhs.name == rhs.name
	}

	public static let deviceName = NAOSParameter(name: "device-name")
	public static let deviceType = NAOSParameter(name: "device-type")
	public static let connectionStatus = NAOSParameter(name: "connection-status")
	public static let battery = NAOSParameter(name: "battery")
	public static let uptime = NAOSParameter(name: "uptime")
	public static let freeHeap = NAOSParameter(name: "free-heap")
	public static let freeHeapInt = NAOSParameter(name: "free-heap-int")

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
		case .freeHeap, .freeHeapInt:
			return ByteCountFormatter.string(
				from: Measurement(value: num, unit: .bytes), countStyle: .memory)
		default:
			return value
		}
	}
}

/// The delegate implemented by objects
public protocol NAOSManagedDeviceDelegate {
	func naosDeviceDidUpdate(device: NAOSManagedDevice, parameter: NAOSParameter)
	func naosDeviceDidDisconnect(device: NAOSManagedDevice, error: Error)
}

public enum NAOSManagedError: LocalizedError {
	case notConnected

	public var errorDescription: String? {
		switch self {
		case .notConnected:
			return "Device not connected."
		}
	}
}

public class NAOSManagedDevice: NSObject {
	private var mutex = AsyncSemaphore(value: 1)
	private var session: NAOSSession?

	public private(set) var device: NAOSDevice
	public private(set) var channel: NAOSChannel? = nil
	var updatable: Set<NAOSParameter> = Set()
	var maxAge: UInt64 = 0

	public var delegate: NAOSManagedDeviceDelegate?
	public private(set) var connected: Bool = false
	public private(set) var canUpdate: Bool = false
	public private(set) var canFS: Bool = false
	public private(set) var canRelay: Bool = false
	public private(set) var hasMetrics: Bool = false
	public private(set) var locked: Bool = false
	public private(set) var availableParameters: [NAOSParameter] = []
	public var parameters: [NAOSParameter: String] = [:]
	private var password: String = ""
	public private(set) var relayDevices: [NAOSDevice] = []
	public private(set) var mtu: UInt16 = 0

	public init(device: NAOSDevice) {
		// initialize instance
		self.device = device

		// finish init
		super.init()

		// initialize device name and type
		parameters[.deviceName] = device.name()
		parameters[.deviceType] = "unknown"

		// run updater
		Task {
			while true {
				// wait a second
				try await Task.sleep(for: .seconds(1))

				// collect updates
				var updates = [NAOSParamUpdate]()
				try? await useSession { session in
					updates = try await NAOSParams.collect(session: session, refs: nil, since: maxAge)
				}

				// update parameters
				for update in updates {
					if let param = (availableParameters.first { p in p.ref == update.ref }) {
						parameters[param] = String(data: update.value, encoding: .utf8)!
						maxAge = max(maxAge, update.age)
					}
				}

				// call delegate if present
				if let d = delegate {
					for update in updates {
						DispatchQueue.main.async {
							if let param =
								(self.availableParameters.first { p in
									p.ref == update.ref
								})
							{
								d.naosDeviceDidUpdate(
									device: self, parameter: param)
							}
						}
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

		// chceck state
		if connected {
			return
		}

		// open channel
		channel = try await device.open()

		// set flag
		connected = true

		// read lock status
		try await withSession { session in
			locked = try await session.status().contains(.locked)
		}

		// reset max aage
		maxAge = 0
	}

	/// Refresh will perform a full device refresh and update all parameters.
	public func refresh() async throws {
		// acquire mutex
		await mutex.wait()
		defer { mutex.signal() }

		// check state
		if !connected {
			throw NAOSManagedError.notConnected
		}

		// use session
		try await withSession { session in
			// check endpoint existence
			self.canUpdate = try await session.query(endpoint: NAOSUpdate.endpoint)
			self.canFS = try await session.query(endpoint: NAOSFS.endpoint)
			self.canRelay = try await session.query(endpoint: NAOSRelay.endpoint)
			self.hasMetrics = try await session.query(endpoint: NAOSMetrics.endpoint)

			// list parameters
			let list = try await NAOSParams.list(session: session)

			// save parameters
			availableParameters = []
			for info in list {
				availableParameters.append(
					NAOSParameter(
						name: info.name, type: info.type, mode: info.mode,
						ref: info.ref))
			}

			// prepare map
			let map = availableParameters.filter { p in p.type != .action }.map { p in p.ref }

			// refresh parameters
			for update in try await NAOSParams.collect(session: session, refs: map, since: 0) {
				if let param = availableParameters.first(where: { p in p.ref == update.ref }) {
					parameters[param] = String(data: update.value, encoding: .utf8) ?? ""
					maxAge = max(maxAge, update.age)
				}
			}

			// scan relay devices
			if self.canRelay {
				self.relayDevices = try (await NAOSRelay.scan(session: session)).map { device in
					NAOSRelayDevice(host: self, device: device)
				}
			}
			
			// get MTU
			self.mtu = try await session.getMTU()
		}
	}

	/// Returns the title of the device.
	public func title() -> String {
		// TODO: Precompute during refresh and updates?
		
		// format title
		return (parameters[.deviceName] ?? "") + " (" + (parameters[.deviceType] ?? "") + ")"
	}

	/// Unlock will attempt to unlock the device and returns its success.
	public func unlock(password: String) async throws -> Bool {
		// acquire mutex
		await mutex.wait()
		defer { mutex.signal() }

		// check state
		if !connected {
			throw NAOSManagedError.notConnected
		}

		// save password
		self.password = password

		// read lock status
		try await withSession { session in
			if try await session.unlock(password: password) {
				locked = false
			}
		}

		return !locked
	}

	/// Read will read the specified parameter. The result is placed into the parameters dictionary.
	public func read(parameter: NAOSParameter) async throws {
		// acquire mutex
		await mutex.wait()
		defer { mutex.signal() }

		// check state
		if !connected {
			throw NAOSManagedError.notConnected
		}

		// use session
		try await withSession { session in
			// read value
			let value = try await NAOSParams.read(session: session, ref: parameter.ref)

			// write parameter
			parameters[parameter] = String(data: value, encoding: String.Encoding.utf8)
		}

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

		// check state
		if !connected {
			throw NAOSManagedError.notConnected
		}

		// use session
		try await withSession { session in
			// write parameter
			try await NAOSParams.write(
				session: session,
				ref: parameter.ref,
				value: parameters[parameter]!.data(using: .utf8)!)
		}

		// call delegate if present
		if let d = delegate {
			DispatchQueue.main.async {
				d.naosDeviceDidUpdate(device: self, parameter: parameter)
			}
		}
	}

	/// NewSession will create a new session and return it.
	public func newSession(timeout: TimeInterval = 5) async throws -> NAOSSession {
		// acquire mutex
		await mutex.wait()
		defer { mutex.signal() }

		// check state
		if !connected {
			throw NAOSManagedError.notConnected
		}

		// open new session
		return try await openSession(timeout: timeout)
	}

	/// UseSession will yield the managed session.
	public func useSession(callback: (NAOSSession) async throws -> Void) async throws {
		// acquire mutex
		await mutex.wait()
		defer { mutex.signal() }

		// check state
		if !connected {
			throw NAOSManagedError.notConnected
		}

		// yield session
		try await withSession(callback: callback)
	}

	/// Disconnect will close the connection to the device.
	public func disconnect() async throws {
		// acquire mutex
		await mutex.wait()
		defer { mutex.signal() }

		// check state
		if !connected {
			return
		}

		// cleanup session
		session?.cleanup()
		session = nil

		// close channel
		channel?.close()
		channel = nil

		// set flag
		connected = false
	}

	// Helpers

	private func openSession(timeout: TimeInterval = 5) async throws -> NAOSSession {
		// open session
		let session = try await NAOSSession.open(channel: channel!, timeout: timeout)

		// try to unlock if locked
		if !password.isEmpty {
			if try (await session.status()).contains(.locked) {
				_ = try await session.unlock(password: password)
			}
		}

		return session
	}

	private func withSession(callback: (NAOSSession) async throws -> Void) async throws {
		// ensure session
		if session == nil {
			session = try await openSession()
		}

		// yield session
		do {
			try await callback(session!)
		} catch {
			session?.cleanup()
			session = nil
			throw error
		}
	}

	// NAOSManager

	func didDisconnect(error: Error) async {
		// acquire mutex
		await mutex.wait()
		defer { mutex.signal() }

		// clear session
		session?.cleanup()
		session = nil

		// close channel
		channel?.close()
		channel = nil

		// set flag
		connected = false

		// call delegate if available
		if let d = delegate {
			DispatchQueue.main.async {
				d.naosDeviceDidDisconnect(device: self, error: error)
			}
		}
	}
}
