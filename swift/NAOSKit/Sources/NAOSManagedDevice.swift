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

	public static let deviceID = NAOSParameter(name: "device-id")
	public static let deviceName = NAOSParameter(name: "device-name")
	public static let appType = NAOSParameter(name: "app-name")
	public static let appVersion = NAOSParameter(name: "app-version")
	public static let connectionStatus = NAOSParameter(name: "connection-status")
	public static let battery = NAOSParameter(name: "battery")
	public static let uptime = NAOSParameter(name: "uptime")

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
		default:
			return value
		}
	}
}

/// The delegate implemented by objects
public protocol NAOSManagedDeviceDelegate {
	func naosDeviceDidUpdate(device: NAOSManagedDevice, parameter: NAOSParameter)
}

/// A lifecycle event emitted by a managed device.
public enum NAOSManagedDeviceEvent {
	case connected
	case disconnected
}

public enum NAOSManagedError: LocalizedError {
	case notConnected
	case stopped

	public var errorDescription: String? {
		switch self {
		case .notConnected:
			return "Device not connected."
		case .stopped:
			return "Device has been stopped."
		}
	}
}

public class NAOSManagedDevice: NSObject {
	private var mutex = AsyncSemaphore(value: 1)
	private var session: NAOSSession?
	private var updaterTask: Task<Void, Never>?
	private var eventSubs: [UUID: AsyncStream<NAOSManagedDeviceEvent>.Continuation] = [:]
	private var eventSubsLock = NSLock()
	private var stopped = false

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
	public private(set) var mtu: UInt16 = 0

	private var password: String = ""
	private let stateLock = NSLock()
	private var _availableParameters: [NAOSParameter] = []
	private var _parameters: [NAOSParameter: String] = [:]
	private var _relayDevices: [NAOSDevice] = []

	public private(set) var availableParameters: [NAOSParameter] {
		get { stateLock.withLock { _availableParameters } }
		set { stateLock.withLock { _availableParameters = newValue } }
	}
	public var parameters: [NAOSParameter: String] {
		get { stateLock.withLock { _parameters } }
		set { stateLock.withLock { _parameters = newValue } }
	}
	public private(set) var relayDevices: [NAOSDevice] {
		get { stateLock.withLock { _relayDevices } }
		set { stateLock.withLock { _relayDevices = newValue } }
	}

	public init(device: NAOSDevice) {
		// initialize instance
		self.device = device

		// finish init
		super.init()

		// initialize device name and type
		parameters[.deviceName] = device.name()
		parameters[.appType] = "unknown"

		// run updater
		updaterTask = Task { [weak self] in
			while !Task.isCancelled {
				// wait a second
				try? await Task.sleep(for: .seconds(1))
				if Task.isCancelled { break }

				// get strong reference for this iteration
				guard let self else { break }

				// collect updates
				var updates = [NAOSParamUpdate]()
				try? await self.useSession { session in
					updates = try await NAOSParams.collect(session: session, refs: nil, since: self.maxAge)
				}

				// apply updates
				await self.mutex.wait()
				for update in updates {
					if let param = (self.availableParameters.first { p in p.ref == update.ref }) {
						self.parameters[param] = String(data: update.value, encoding: .utf8) ?? ""
						self.maxAge = max(self.maxAge, update.age)
					}
				}
				self.mutex.signal()

				// call delegate if present
				if let d = self.delegate {
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

	deinit {
		updaterTask?.cancel()
		stopped = true
	}

	/// Returns a new subscription stream for lifecycle events. The stream is
	/// buffered (4) and drops events if the consumer is too slow. It ends when
	/// the managed device is deallocated.
	public func events() -> AsyncStream<NAOSManagedDeviceEvent> {
		let id = UUID()
		return AsyncStream(bufferingPolicy: .bufferingNewest(4)) { continuation in
			self.eventSubsLock.lock()
			self.eventSubs[id] = continuation
			self.eventSubsLock.unlock()

			continuation.onTermination = { @Sendable _ in
				self.eventSubsLock.lock()
				self.eventSubs.removeValue(forKey: id)
				self.eventSubsLock.unlock()
			}
		}
	}

	private func emitEvent(_ event: NAOSManagedDeviceEvent) {
		eventSubsLock.lock()
		let subs = Array(eventSubs.values)
		eventSubsLock.unlock()
		for cont in subs {
			cont.yield(event)
		}
	}

	/// Connect will initiate a connection to a device.
	public func connect() async throws {
		// acquire mutex
		await mutex.wait()
		defer { mutex.signal() }

		// check state
		if stopped {
			throw NAOSManagedError.stopped
		}
		if connected {
			return
		}

		// open channel
		let ch = try await device.open()
		channel = ch

		// set flag
		connected = true

		// read lock status
		try await withSession { session in
			locked = try await session.status().contains(.locked)
		}

		// reset max age
		maxAge = 0

		// emit connected
		emitEvent(.connected)

		// watch for transport loss
		Task { [weak self] in
			await ch.done
			guard let self else { return }
			await self.didDisconnect(error: nil)
		}
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
		return device.type() + ": " + (parameters[.deviceName] ?? "") + " (" + (parameters[.appType] ?? "") + ")"
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

		// write parameter
		try await withSession { session in
			let value = (parameters[parameter] ?? "").data(using: .utf8) ?? Data()
			try await NAOSParams.write(session: session, ref: parameter.ref, value: value)
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
		if stopped {
			throw NAOSManagedError.stopped
		}
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
		if stopped {
			throw NAOSManagedError.stopped
		}
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

	/// Stop will disconnect and permanently disable the device.
	public func stop() async {
		// disconnect
		try? await disconnect()

		// cancel updater
		updaterTask?.cancel()
		updaterTask = nil

		// finish all event streams
		eventSubsLock.lock()
		let subs = eventSubs
		eventSubs = [:]
		eventSubsLock.unlock()
		for (_, cont) in subs {
			cont.finish()
		}

		// set flag
		stopped = true
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

	private func didDisconnect(error: Error?) async {
		// acquire mutex
		await mutex.wait()
		defer { mutex.signal() }

		// check state
		if !connected {
			return
		}

		// clear session
		session?.cleanup()
		session = nil

		// close channel
		channel?.close()
		channel = nil

		// set flag
		connected = false

		// emit disconnected
		emitEvent(.disconnected)
	}
}
