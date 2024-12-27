//
//  Created by Joël Gähwiler on 20.08.23.
//  Copyright © 2023 Joël Gähwiler. All rights reserved.
//

import Foundation

/// The available parameter types.
public enum NAOSParamType: UInt8 {
	case raw
	case string
	case bool
	case long
	case double
	case action

	public func string() -> String {
		switch self {
		case .raw:
			return "Raw"
		case .string:
			return "String"
		case .bool:
			return "Bool"
		case .long:
			return "Long"
		case .double:
			return "Double"
		case .action:
			return "Action"
		}
	}
}

/// The available parameter modes.
public struct NAOSParamMode: OptionSet {
	public let rawValue: UInt8

	public init(rawValue: UInt8) {
		self.rawValue = rawValue
	}

	public static let volatile = NAOSParamMode(rawValue: 1 << 0)
	public static let system = NAOSParamMode(rawValue: 1 << 1)
	public static let application = NAOSParamMode(rawValue: 1 << 2)
	public static let locked = NAOSParamMode(rawValue: 1 << 4)
}

/// A parameter description.
public struct NAOSParamInfo {
	public var ref: UInt8
	public var type: NAOSParamType
	public var mode: NAOSParamMode
	public var name: String
}

/// A parameter update.
public struct NAOSParamUpdate {
	public var ref: UInt8
	public var age: UInt64
	public var value: Data
}

/// The NAOS parameter endpoint.
public class NAOSParams {
	/// The endpoint number
	public static let endpoint: UInt8 = 0x1
	
	/// Get a parameter value by name.
	public static func get(session: NAOSSession, name: String, timeout: TimeInterval = 5) async throws -> Data {
		// prepare command
		var cmd = Data([0])
		cmd.append(name.data(using: .utf8)!)

		// write command
		try await session.send(endpoint: self.endpoint, data: cmd, ackTimeout: 0)

		// receive value
		return try await session.receive(endpoint: self.endpoint, expectAck: false, timeout: timeout)!
	}

	/// Set a parameter value by name.
	public static func set(session: NAOSSession, name: String, value: Data, timeout: TimeInterval = 5)
		async throws
	{
		// prepare command
		var cmd = Data([1])
		cmd.append(name.data(using: .utf8)!)
		cmd.append(Data([0]))
		cmd.append(value)

		// write command
		try await session.send(endpoint: self.endpoint, data: cmd, ackTimeout: timeout)
	}

	/// Obtain a list of all known parametersession.
	public static func list(session: NAOSSession, timeout: TimeInterval = 5) async throws -> [NAOSParamInfo] {
		// send command
		try await session.send(endpoint: self.endpoint, data: Data([2]), ackTimeout: 0)

		// prepare list
		var list = [NAOSParamInfo]()

		while true {
			// receive reply or return list on ack
			guard
				let reply = try await session.receive(
					endpoint: self.endpoint, expectAck: true, timeout: timeout)
			else {
				return list
			}

			// verify reply
			if reply.count < 4 {
				throw NAOSSessionError.invalidMessage
			}

			// parse reply
			let ref = reply[0]
			let type = NAOSParamType(rawValue: reply[1])!
			let mode = NAOSParamMode(rawValue: reply[2])
			let name = String(data: Data(reply[3...]), encoding: .utf8)!

			// append info
			list.append(NAOSParamInfo(ref: ref, type: type, mode: mode, name: name))
		}
	}

	/// Read a parameter by reference.
	public static func read(session: NAOSSession, ref: UInt8, timeout: TimeInterval = 5) async throws -> Data {
		// prepare command
		let cmd = Data([3, ref])

		// write command
		try await session.send(endpoint: self.endpoint, data: cmd, ackTimeout: 0)

		// receive value
		return try await session.receive(endpoint: self.endpoint, expectAck: false, timeout: timeout)!
	}

	/// Write a parameter by reference.
	public static func write(session: NAOSSession, ref: UInt8, value: Data, timeout: TimeInterval = 5)
		async throws
	{
		// prepare command
		var cmd = Data([4, ref])
		cmd.append(value)

		// write command
		try await session.send(endpoint: self.endpoint, data: cmd, ackTimeout: timeout)
	}

	/// Collect parameter values by providing a list of refrences, a since timestamp or both.
	public static func collect(session: NAOSSession, refs: [UInt8]?, since: UInt64, timeout: TimeInterval = 5)
		async throws -> [NAOSParamUpdate]
	{
		// prepare map
		var map = UINT64_MAX
		if refs != nil {
			map = UInt64(0)
			for ref in refs! {
				map |= (1 << ref)
			}
		}

		// send command
		let cmd = pack(fmt: "oqq", args: [UInt8(5), map, since])
		try await session.send(endpoint: self.endpoint, data: cmd, ackTimeout: 0)

		// prepare list
		var list = [NAOSParamUpdate]()

		while true {
			// receive reply or return list on ack
			guard
				let reply = try await session.receive(
					endpoint: self.endpoint, expectAck: true, timeout: timeout)
			else {
				return list
			}

			// verify reply
			if reply.count < 9 {
				throw NAOSSessionError.invalidMessage
			}

			// unpack reply
			let args = unpack(fmt: "oqb", data: reply)
			let ref = args[0] as! UInt8
			let age = args[1] as! UInt64
			let value = args[2] as! Data

			// append info
			list.append(NAOSParamUpdate(ref: ref, age: age, value: value))
		}
	}

	/// Clear a parameter by reference.
	public static func clear(session: NAOSSession, ref: UInt8, timeout: TimeInterval = 5) async throws {
		// prepare command
		let cmd = Data([6, ref])

		// write command
		try await session.send(endpoint: self.endpoint, data: cmd, ackTimeout: timeout)
	}
}
