//
//  Created by Joël Gähwiler on 20.08.23.
//  Copyright © 2023 Joël Gähwiler. All rights reserved.
//

import Foundation
import Semaphore

/// The parameter endpoint number.
public let NAOSParamsEndpoint: UInt8 = 0x01

/// The available parameter types.
public enum NAOSParamType: UInt8 {
	case raw
	case string
	case bool
	case long
	case double
	case action
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

/// The parameter endpoint methods.
public class NAOSParams {
	/// Get a parameter value by name.
	static public func get(session: NAOSSession, name: String, timeout: TimeInterval = 5000) async throws -> Data {
		// prepare command
		var cmd = Data([0])
		cmd.append(name.data(using: .utf8)!)

		// write command
		try await session.send(endpoint: NAOSParamsEndpoint, data: cmd, ackTimeout: 0)

		// receive value
		return try await session.receive(endpoint: NAOSParamsEndpoint, expectAck: false, timeout: timeout)!
	}

	/// Set a parameter value by name.
	static public func set(session: NAOSSession, name: String, value: Data, timeout: TimeInterval = 5000)
		async throws
	{
		// prepare command
		var cmd = Data([1])
		cmd.append(name.data(using: .utf8)!)
		cmd.append(Data([0]))
		cmd.append(value)

		// write command
		try await session.send(endpoint: NAOSParamsEndpoint, data: cmd, ackTimeout: timeout)
	}

	/// Obtain a list of all known parametersession.
	static public func list(session: NAOSSession, timeout: TimeInterval = 5000) async throws -> [NAOSParamInfo] {
		// send command
		try await session.send(endpoint: NAOSParamsEndpoint, data: Data([2]), ackTimeout: 0)

		// prepare list
		var list = [NAOSParamInfo]()

		while true {
			// receive reply or return list on ack
			guard
				let reply = try await session.receive(
					endpoint: NAOSParamsEndpoint, expectAck: true, timeout: timeout)
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
	static public func read(session: NAOSSession, ref: UInt8, timeout: TimeInterval = 5000) async throws -> Data {
		// prepare command
		let cmd = Data([3, ref])

		// write command
		try await session.send(endpoint: NAOSParamsEndpoint, data: cmd, ackTimeout: 0)

		// receive value
		return try await session.receive(endpoint: NAOSParamsEndpoint, expectAck: false, timeout: timeout)!
	}

	/// Write a parameter by reference.
	static public func write(session: NAOSSession, ref: UInt8, value: Data, timeout: TimeInterval = 5000)
		async throws
	{
		// prepare command
		var cmd = Data([4, ref])
		cmd.append(value)

		// write command
		try await session.send(endpoint: NAOSParamsEndpoint, data: cmd, ackTimeout: timeout)
	}

	/// Collect parameter values by providing a list of refrences, a since timestamp or both.
	static public func collect(session: NAOSSession, refs: [UInt8]?, since: UInt64, timeout: TimeInterval = 5000)
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

		// prepare command
		var cmd = Data([5])
		cmd.append(writeUint64(value: map))
		cmd.append(writeUint64(value: since))

		// send command
		try await session.send(endpoint: NAOSParamsEndpoint, data: cmd, ackTimeout: 0)

		// prepare list
		var list = [NAOSParamUpdate]()

		while true {
			// receive reply or return list on ack
			guard
				let reply = try await session.receive(
					endpoint: NAOSParamsEndpoint, expectAck: true, timeout: timeout)
			else {
				return list
			}

			// verify reply
			if reply.count < 9 {
				throw NAOSSessionError.invalidMessage
			}

			// parse reply
			let ref = reply[0]
			let age = readUint64(data: Data(reply[1...8]))
			let value = Data(reply[9...])

			// append info
			list.append(NAOSParamUpdate(ref: ref, age: age, value: value))
		}
	}

	/// Clear a parameter by reference.
	static public func clear(session: NAOSSession, ref: UInt8, timeout: TimeInterval = 5000) async throws {
		// prepare command
		let cmd = Data([6, ref])

		// write command
		try await session.send(endpoint: NAOSParamsEndpoint, data: cmd, ackTimeout: timeout)
	}
}
