//
//  Created by Joël Gähwiler on 24.04.26.
//  Copyright © 2026 Joël Gähwiler. All rights reserved.
//

import Foundation

/// The NAOS time endpoint.
public class NAOSTime {
	/// The endpoint number.
	public static let endpoint: UInt8 = 0x9

	/// Get the device's current wall-clock time in UTC at millisecond resolution.
	public static func get(session: NAOSSession, timeout: TimeInterval = 5) async throws -> Date {
		// send command
		try await session.send(endpoint: self.endpoint, data: Data([0]), ackTimeout: 0)

		// receive reply
		let reply = try await session.receive(
			endpoint: self.endpoint, expectAck: false, timeout: timeout)!

		// verify reply
		if reply.count != 8 {
			throw NAOSSessionError.invalidMessage
		}

		// parse epoch milliseconds
		let ms = Int64(bitPattern: readUint64(data: reply))

		return Date(timeIntervalSince1970: TimeInterval(ms) / 1000)
	}

	/// Set the device's wall-clock time in UTC at millisecond resolution.
	public static func set(session: NAOSSession, date: Date, timeout: TimeInterval = 5) async throws {
		// build command
		let ms = Int64((date.timeIntervalSince1970 * 1000).rounded())
		var cmd = Data([1])
		cmd.append(writeUint64(value: UInt64(bitPattern: ms)))

		// send command
		try await session.send(endpoint: self.endpoint, data: cmd, ackTimeout: timeout)
	}

	/// Get the device's current timezone offset from UTC in seconds.
	public static func info(session: NAOSSession, timeout: TimeInterval = 5) async throws -> Int32 {
		// send command
		try await session.send(endpoint: self.endpoint, data: Data([2]), ackTimeout: 0)

		// receive reply
		let reply = try await session.receive(
			endpoint: self.endpoint, expectAck: false, timeout: timeout)!

		// verify reply
		if reply.count != 4 {
			throw NAOSSessionError.invalidMessage
		}

		// parse offset in seconds
		return readInt32(data: reply)
	}
}
