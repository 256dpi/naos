//
//  Created by Joël Gähwiler on 30.07.23.
//  Copyright © 2023 Joël Gähwiler. All rights reserved.
//

import Foundation

/// Information about a file.
public struct NAOSFSInfo {
	public var name: String
	public var isDir: Bool
	public var size: UInt32
}

/// The NAOS file system endpoint.
public class NAOSFS {
	/// Get information on a file or directory.
	public static func stat(session: NAOSSession, path: String, timeout: TimeInterval = 5) async throws -> NAOSFSInfo {
		// prepare command
		var cmd = Data([0])
		cmd.append(path.data(using: .utf8)!)

		// send command
		try await send(session: session, cmd: cmd, ack: false, timeout: timeout)

		// await reply
		let reply = try await receive(session: session, expectAck: false, timeout: timeout)!

		// verify "info" reply
		if reply.count != 6 || reply[0] != 1 {
			throw NAOSSessionError.invalidMessage
		}

		// parse "info" reply
		let isDir = reply[1] == 1
		let size = readUint32(data: Data(reply[2...]))

		return NAOSFSInfo(name: "", isDir: isDir, size: size)
	}

	/// List a files and directories.
	public static func list(session: NAOSSession, dir: String, timeout: TimeInterval = 5) async throws -> [NAOSFSInfo] {
		// prepare command
		var cmd = Data([1])
		cmd.append(dir.data(using: .utf8)!)

		// send command
		try await send(session: session, cmd: cmd, ack: false, timeout: timeout)

		// prepare infos
		var infos: [NAOSFSInfo] = []

		while true {
			// await reply
			guard let reply = try await receive(session: session, expectAck: true, timeout: timeout) else {
				return infos
			}

			// verify "info" reply
			if reply.count < 7 || reply[0] != 1 {
				throw NAOSSessionError.invalidMessage
			}

			// parse "info" reply
			let isDir = reply[1] == 1
			let size = readUint32(data: Data(reply[2...]))
			let name = String(bytes: Data(reply[6...]), encoding: .utf8)!

			// add info
			infos.append(NAOSFSInfo(name: name, isDir: isDir, size: size))
		}
	}

	/// Read a full file.
	public static func read(session: NAOSSession, file: String, report: ((Int) -> Void)?, timeout: TimeInterval = 5) async throws -> Data {
		// stat file
		let info = try await stat(session: session, path: file)

		// prepare data
		var data = Data()

		// read file in chunks of 5 KB
		while data.count < info.size {
			// read data
			let chunk = try await read(session: session, file: file, offset: UInt32(data.count), length: 5000) { offset in
				if report != nil {
					report!(data.count + offset)
				}
			}

			// append chunk
			data.append(chunk)
		}

		return data
	}

	/// Read a range of a file.
	public static func read(session: NAOSSession, file: String, offset: UInt32, length: UInt32, report: ((Int) -> Void)?, timeout: TimeInterval = 5) async throws -> Data {
		// prepare "open" command
		var cmd = Data([2, 0])
		cmd.append(file.data(using: .utf8)!)

		// send "open" command
		try await send(session: session, cmd: cmd, ack: true, timeout: timeout)

		// prepare "read" command
		cmd = Data([3])
		cmd.append(writeUint32(value: offset))
		cmd.append(writeUint32(value: length))

		// send "read" command
		try await send(session: session, cmd: cmd, ack: false, timeout: timeout)

		// prepare data
		var data = Data()

		// prepare counter
		var count: UInt32 = 0

		while true {
			// await reply
			guard let reply = try await receive(session: session, expectAck: true, timeout: timeout) else {
				break
			}

			// verify "chunk" reply
			if reply.count <= 5 || reply[0] != 2 {
				throw NAOSSessionError.invalidMessage
			}

			// get offset
			let replyOffset = readUint32(data: Data(reply[1 ... 5]))

			// verify offset
			if replyOffset != offset + count {
				throw NAOSSessionError.invalidMessage
			}

			// append data
			data.append(Data(reply[5...]))

			// increment
			count += UInt32(reply.count - 5)

			// report length
			if report != nil {
				report!(data.count)
			}
		}

		// send "close" command
		try await send(session: session, cmd: Data([5]), ack: true, timeout: timeout)

		return data
	}

	/// Write a full file.
	public static func write(session: NAOSSession, file: String, data: Data, report: ((Int) -> Void)?, timeout: TimeInterval = 5) async throws {
		// prepare "open" command (create & truncate)
		var cmd = Data([2, 1 << 0 | 1 << 2])
		cmd.append(file.data(using: .utf8)!)

		// send "create" command
		try await send(session: session, cmd: cmd, ack: true, timeout: timeout)

		// TODO: Dynamically determine channel MTU?

		// write data in 500-byte chunks
		var num = 0
		var offset = 0
		while offset < data.count {
			// determine chunk size and chunk data
			let chunkSize = min(500, data.count - offset)
			let chunkData = data.subdata(in: offset ..< offset + chunkSize)

			// determine mode
			let acked = num % 10 == 0

			// prepare "write" command (acked or silent & sequential)
			cmd = Data([4, acked ? 0 : 1 << 0 | 1 << 1])
			cmd.append(writeUint32(value: UInt32(offset)))
			cmd.append(chunkData)

			// send "write" command
			try await send(session: session, cmd: cmd, ack: false, timeout: timeout)

			// receive ack or "error" replies
			if acked {
				_ = try await receive(session: session, expectAck: true, timeout: timeout)
			}

			// increment offset
			offset += chunkSize

			// report offset
			if report != nil {
				report!(offset)
			}

			// increment count
			num += 1
		}

		// send "close" command
		try await send(session: session, cmd: Data([5]), ack: true, timeout: timeout)
	}

	/// Rename a file.
	public static func rename(session: NAOSSession, from: String, to: String, timeout: TimeInterval = 5) async throws {
		// prepare command
		var cmd = Data([6])
		cmd.append(from.data(using: .utf8)!)
		cmd.append(Data([0]))
		cmd.append(to.data(using: .utf8)!)

		// send command
		try await send(session: session, cmd: cmd, ack: true, timeout: timeout)
	}

	/// Remove a file.
	public static func remove(session: NAOSSession, path: String, timeout: TimeInterval = 5) async throws {
		// prepare command
		var cmd = Data([7])
		cmd.append(path.data(using: .utf8)!)

		// send command
		try await send(session: session, cmd: cmd, ack: true, timeout: timeout)
	}

	/// Calculate the SHA256 checksum of a file.
	public static func sha256(session: NAOSSession, file: String, timeout: TimeInterval = 5) async throws -> Data {
		// prepare command
		var cmd = Data([8])
		cmd.append(file.data(using: .utf8)!)

		// send command
		try await send(session: session, cmd: cmd, ack: false, timeout: timeout)

		// await reply
		let reply = try await receive(session: session, expectAck: false, timeout: timeout)!

		// verify "chunk" reply
		if reply.count != 33 || reply[0] != 3 {
			throw NAOSSessionError.invalidMessage
		}

		// get sum
		let sum = Data(reply[1...])

		return sum
	}

	// - Helpers

	static func receive(session: NAOSSession, expectAck: Bool, timeout: TimeInterval) async throws -> Data? {
		// receive reply
		guard let data = try await session.receive(endpoint: 0x3, expectAck: expectAck, timeout: timeout) else {
			return nil
		}

		// handle errors
		if data[0] == 0 {
			throw POSIXError(POSIXErrorCode(rawValue: Int32(data[1]))!)
		}

		return data
	}

	static func send(session: NAOSSession, cmd: Data, ack: Bool, timeout: TimeInterval) async throws {
		// send command
		try await session.send(endpoint: 0x3, data: cmd, ackTimeout: ack ? timeout : 0)
	}
}
