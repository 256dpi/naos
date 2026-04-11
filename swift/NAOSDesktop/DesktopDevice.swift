//
//  Created by Joël Gähwiler on 05.04.17.
//  Copyright © 2017 Joël Gähwiler. All rights reserved.
//

import Foundation
import NAOSKit

/// The object representing a single NAOS parameter.
public struct Parameter: Hashable {
	public var name: String
	public var type: NAOSParamType = .raw
	public var mode: NAOSParamMode = .init(rawValue: 0)
	public var ref: UInt8 = 0

	public func hash(into hasher: inout Hasher) {
		hasher.combine(name)
	}

	public static func == (lhs: Parameter, rhs: Parameter) -> Bool {
		return lhs.name == rhs.name
	}

	public static let deviceID = Parameter(name: "device-id")
	public static let deviceName = Parameter(name: "device-name")
	public static let appType = Parameter(name: "app-name")
	public static let appVersion = Parameter(name: "app-version")
	public static let connectionStatus = Parameter(name: "connection-status")
	public static let battery = Parameter(name: "battery")
	public static let uptime = Parameter(name: "uptime")

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

/// The delegate implemented by objects to handle desktop device events.
public protocol DesktopDeviceDelegate {
	func naosDeviceDidUpdate(device: DesktopDevice, parameter: Parameter)
}

/// A managed device extended with Desktop-specific state and behavior.
public class DesktopDevice: NAOSManagedDevice {
	private var updaterTask: Task<Void, Never>?
	private let stateLock = NSLock()
	private var _availableParameters: [Parameter] = []
	private var _parameters: [Parameter: String] = [:]
	private var _relayDevices: [NAOSDevice] = []
	var updatable: Set<Parameter> = Set()
	var maxAge: UInt64 = 0

	public var delegate: DesktopDeviceDelegate?
	public private(set) var canUpdate: Bool = false
	public private(set) var canFS: Bool = false
	public private(set) var canRelay: Bool = false
	public private(set) var hasMetrics: Bool = false
	public private(set) var mtu: UInt16 = 0

	public private(set) var availableParameters: [Parameter] {
		get { stateLock.withLock { _availableParameters } }
		set { stateLock.withLock { _availableParameters = newValue } }
	}
	public var parameters: [Parameter: String] {
		get { stateLock.withLock { _parameters } }
		set { stateLock.withLock { _parameters = newValue } }
	}
	public private(set) var relayDevices: [NAOSDevice] {
		get { stateLock.withLock { _relayDevices } }
		set { stateLock.withLock { _relayDevices = newValue } }
	}

	public override init(device: NAOSDevice) {
		super.init(device: device)

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
				for update in updates {
					if let param = (self.availableParameters.first { p in p.ref == update.ref }) {
						self.parameters[param] = String(data: update.value, encoding: .utf8) ?? ""
						self.maxAge = max(self.maxAge, update.age)
					}
				}

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
	}

	/// Refresh will perform a full device refresh and update all parameters.
	public func refresh() async throws {
		// check state
		if !active {
			throw NAOSManagedDeviceError.notConnected
		}

		// reset max age
		maxAge = 0

		// use session
		try await useSession { session in
			// check endpoint existence
			self.canUpdate = try await session.query(endpoint: NAOSUpdate.endpoint)
			self.canFS = try await session.query(endpoint: NAOSFS.endpoint)
			self.canRelay = try await session.query(endpoint: NAOSRelay.endpoint)
			self.hasMetrics = try await session.query(endpoint: NAOSMetrics.endpoint)

			// list parameters
			let list = try await NAOSParams.list(session: session)

			// save parameters
			self.availableParameters = []
			for info in list {
				self.availableParameters.append(
					Parameter(
						name: info.name, type: info.type, mode: info.mode,
						ref: info.ref))
			}

			// prepare map
			let map = self.availableParameters.filter { p in p.type != .action }.map { p in p.ref }

			// refresh parameters
			for update in try await NAOSParams.collect(session: session, refs: map, since: 0) {
				if let param = self.availableParameters.first(where: { p in p.ref == update.ref }) {
					self.parameters[param] = String(data: update.value, encoding: .utf8) ?? ""
					self.maxAge = max(self.maxAge, update.age)
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
		return device.type() + ": " + (parameters[.deviceName] ?? "") + " (" + (parameters[.appType] ?? "") + ")"
	}

	/// Read will read the specified parameter. The result is placed into the parameters dictionary.
	public func read(parameter: Parameter) async throws {
		// check state
		if !active {
			throw NAOSManagedDeviceError.notConnected
		}

		// use session
		try await useSession { session in
			// read value
			let value = try await NAOSParams.read(session: session, ref: parameter.ref)

			// write parameter
			self.parameters[parameter] = String(data: value, encoding: String.Encoding.utf8)
		}

		// call delegate if present
		if let d = delegate {
			DispatchQueue.main.async {
				d.naosDeviceDidUpdate(device: self, parameter: parameter)
			}
		}
	}

	/// Write will write the specified parameter. The value is taken from the parameters dictionary.
	public func write(parameter: Parameter) async throws {
		// check state
		if !active {
			throw NAOSManagedDeviceError.notConnected
		}

		// write parameter
		try await useSession { session in
			let value = (self.parameters[parameter] ?? "").data(using: .utf8) ?? Data()
			try await NAOSParams.write(session: session, ref: parameter.ref, value: value)
		}

		// call delegate if present
		if let d = delegate {
			DispatchQueue.main.async {
				d.naosDeviceDidUpdate(device: self, parameter: parameter)
			}
		}
	}
}
