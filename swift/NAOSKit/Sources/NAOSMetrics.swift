//
//  Created by Joël Gähwiler on 05.01.25.
//  Copyright © 2025 Joël Gähwiler. All rights reserved.
//

import Foundation

/// The available metric kinds.
public enum NAOSMetricKind: UInt8 {
	case gauge
	case counter
}

/// The available metric types.
public enum NAOSMetricType: UInt8 {
	case long
	case float
	case double
}

/// A basic metric description.
public struct NAOSMetricInfo {
	public var ref: UInt8
	public var kind: NAOSMetricKind
	public var type: NAOSMetricType
	public var name: String
	public var size: UInt8
}

/// A metric layout description.
public struct NAOSMetricLayout {
	public var keys: [String]
	public var values: [[String]]
}

/// The NAOS metrics endpoint.
public class NAOSMetrics {
	/// The endpoint number
	public static let endpoint: UInt8 = 0x5

	/// Obtain a list of all known metrics.
	public static func list(session: NAOSSession, timeout: TimeInterval = 5) async throws -> [NAOSMetricInfo] {
		// send command
		try await session.send(endpoint: self.endpoint, data: Data([0]), ackTimeout: 0)

		// prepare list
		var list = [NAOSMetricInfo]()

		while true {
			// receive reply or return list on ack
			guard
				let reply = try await session.receive(
					endpoint: self.endpoint, expectAck: true, timeout: timeout)
			else {
				return list
			}

			// verify reply
			if reply.count < 5 {
				throw NAOSSessionError.invalidMessage
			}

			// parse reply
			let args = unpack(fmt: "oooos", data: reply)

			// parse reply
			let ref = args[0] as! UInt8
			let kind = NAOSMetricKind(rawValue: args[1] as! UInt8)!
			let type = NAOSMetricType(rawValue: args[2] as! UInt8)!
			let size = args[3] as! UInt8
			let name = args[4] as! String

			// append info
			list.append(NAOSMetricInfo(ref: ref, kind: kind, type: type, name: name, size: size))
		}
	}

	/// Describe a metric's keys and values,
	public static func describe(session: NAOSSession, ref: UInt8, timeout: TimeInterval = 5) async throws -> NAOSMetricLayout {
		// send command
		let cmd = Data([UInt8(1), ref])
		try await session.send(endpoint: self.endpoint, data: cmd, ackTimeout: 0)

		// prepare lists
		var keys = [String]()
		var values = [[String]]()

		while true {
			// receive reply or return layout on ack
			guard
				let reply = try await session.receive(
					endpoint: self.endpoint, expectAck: true, timeout: timeout)
			else {
				return NAOSMetricLayout(keys: keys, values: values)
			}

			// verify reply
			if reply.count < 1 {
				throw NAOSSessionError.invalidMessage
			}

			// handle key
			if reply[0] == 0 {
				// verify reply
				if reply.count < 3 {
					throw NAOSSessionError.invalidMessage
				}

				// parse reply
				let args = unpack(fmt: "os", data: reply, start: 1)

				// parse reply
				let num = args[0] as! UInt8
				let name = args[1] as! String

				// add key
				keys.insert(name, at: Int(num))
				values.insert([String](), at: Int(num))

				continue
			}

			// handle value
			if reply[0] == 1 {
				// verify reply
				if reply.count < 4 {
					throw NAOSSessionError.invalidMessage
				}

				// parse reply
				let args = unpack(fmt: "oos", data: reply, start: 1)

				// parse reply
				let numKey = args[0] as! UInt8
				let numValue = args[1] as! UInt8
				let name = args[2] as! String

				// add key
				values[Int(numKey)].insert(name, at: Int(numValue))

				continue
			}

			throw NAOSSessionError.invalidMessage
		}
	}

	/// Read metrics as  raw data.
	public static func read(session: NAOSSession, ref: UInt8, timeout: TimeInterval = 5) async throws -> Data {
		// prepare command
		let cmd = Data([2, ref])

		// write command
		try await session.send(endpoint: self.endpoint, data: cmd, ackTimeout: 0)

		// receive value
		let reply = try await session.receive(endpoint: self.endpoint, expectAck: false, timeout: timeout)!

		return reply
	}

	/// Read metrics as  long data.
	public static func readLong(session: NAOSSession, ref: UInt8, timeout: TimeInterval = 5) async throws -> [Int32] {
		// receive value
		let reply = try await read(session: session, ref: ref, timeout: timeout)

		// convert reply
		var list = [Int32]()
		for i in 0..<(reply.count/4) {
			let n = readUint32(data: reply.subdata(in: i*4..<i*4+4))
			list.append(Int32(n))
		}

		return list
	}

	/// Read metrics as  float data.
	public static func readFloat(session: NAOSSession, ref: UInt8, timeout: TimeInterval = 5) async throws -> [Float] {
		// receive value
		let reply = try await read(session: session, ref: ref, timeout: timeout)

		// convert reply
		var list = [Float]()
		for i in 0..<(reply.count/4) {
			let n = readUint32(data: reply.subdata(in: i*4..<i*4+4))
			list.append(Float(bitPattern: n))
		}

		return list
	}

	/// Read metrics as double data.
	public static func readDouble(session: NAOSSession, ref: UInt8, timeout: TimeInterval = 5) async throws -> [Double] {
		// receive value
		let reply = try await read(session: session, ref: ref, timeout: timeout)

		// convert reply
		var list = [Double]()
		for i in 0..<(reply.count/8) {
			let n = readUint64(data: reply.subdata(in: i*8..<i*8+8))
			list.append(Double(bitPattern: n))
		}

		return list
	}
}
