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
	/// The endpoint number.
	public static let endpoint: UInt8 = 0x3
	
	/// Get information on a file or directory.
	public static func stat(session: NAOSSession, path: String, timeout: TimeInterval = 5) async throws -> NAOSFSInfo {
		// send command
		let cmd = pack(fmt: "os", args: [UInt8(0), path])
		try await send(session: session, cmd: cmd, ack: false, timeout: timeout)

		// await reply
		let reply = try await receive(session: session, expectAck: false, timeout: timeout)!

		// verify "info" reply
		if reply.count != 6 || reply[0] != 1 {
			throw NAOSSessionError.invalidMessage
		}

		// unpack "info" reply
		let args = unpack(fmt: "oi", data: reply, start: 1)
		let isDir = args[0] as! UInt8 == 1
		let size = args[1] as! UInt32

		return NAOSFSInfo(name: "", isDir: isDir, size: size)
	}

	/// List a files and directories.
	public static func list(session: NAOSSession, dir: String, timeout: TimeInterval = 5) async throws -> [NAOSFSInfo] {
		// send command
		let cmd = pack(fmt: "os", args: [UInt8(1), dir])
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

			// unpack "info" reply
			let args = unpack(fmt: "ois", data: reply, start: 1)
			let isDir = args[0] as! UInt8 == 1
			let size = args[1] as! UInt32
			let name = args[2] as! String

			// add info
			infos.append(NAOSFSInfo(name: name, isDir: isDir, size: size))
		}
	}

	/// Read a full file.
	public static func read(session: NAOSSession, file: String, report: ((Int) -> Void)?, timeout: TimeInterval = 5) async throws -> Data {
		// stat file
		let info = try await stat(session: session, path: file)

		// prepare buffer
		var data = Data()

		// read file in chunks of 5 KB
		while data.count < info.size {
			// determine length
			let length = min(5000, info.size - UInt32(data.count))

			// read range
			let range = try await read(session: session, file: file, offset: UInt32(data.count), length: length) { offset in
				if report != nil {
					report!(data.count + offset)
				}
			}

			// append range
			data.append(range)
		}

		return data
	}

	/// Read a range of a file.
	public static func read(session: NAOSSession, file: String, offset: UInt32, length: UInt32, report: ((Int) -> Void)?, timeout: TimeInterval = 5) async throws -> Data {
		// send "open" command
		var cmd = pack(fmt: "oos", args: [UInt8(2), UInt8(0), file])
		try await send(session: session, cmd: cmd, ack: true, timeout: timeout)

		// send "read" command
		cmd = pack(fmt: "oii", args: [UInt8(3), offset, length])
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

			// unpack offset
			let args = unpack(fmt: "i", data: reply, start: 1)
			let replyOffset = args[0] as! UInt32

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
		cmd = pack(fmt: "o", args: [UInt8(5)])
		try await send(session: session, cmd: cmd, ack: true, timeout: timeout)

		return data
	}

	/// Write a full file.
	public static func write(session: NAOSSession, file: String, data: Data, report: ((Int) -> Void)?, timeout: TimeInterval = 5) async throws {
		// send "create" command
		var cmd = pack(fmt: "oos", args: [UInt8(2), UInt8(1 << 0 | 1 << 2), file])
		try await send(session: session, cmd: cmd, ack: true, timeout: timeout)
		
		// determine MTU
		var mtu = Int(try await session.getMTU())
		
		// subtract overhead
		mtu -= 6

		// write data in chunks
		var num = 0
		var offset = 0
		while offset < data.count {
			// determine chunk size and chunk data
			let chunkSize = min(mtu, data.count - offset)
			let chunkData = data.subdata(in: offset ..< offset + chunkSize)

			// determine mode
			let acked = num % 10 == 0

			// prepare "write" command (acked or silent & sequential)
			cmd = pack(fmt: "ooib", args: [UInt8(4), UInt8(acked ? 0 : 1 << 0 | 1 << 1), UInt32(offset), chunkData])

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
		cmd = pack(fmt: "o", args: [UInt8(5)])
		try await send(session: session, cmd: cmd, ack: true, timeout: timeout)
	}

	/// Rename a file.
	public static func rename(session: NAOSSession, from: String, to: String, timeout: TimeInterval = 5) async throws {
		// send command
		let cmd = pack(fmt: "osos", args: [UInt8(6), from, UInt8(0), to])
		try await send(session: session, cmd: cmd, ack: true, timeout: timeout)
	}

	/// Remove a file.
	public static func remove(session: NAOSSession, path: String, timeout: TimeInterval = 5) async throws {
		// send command
		let cmd = pack(fmt: "os", args: [UInt8(7), path])
		try await send(session: session, cmd: cmd, ack: true, timeout: timeout)
	}

	/// Calculate the SHA256 checksum of a file.
	public static func sha256(session: NAOSSession, file: String, timeout: TimeInterval = 5) async throws -> Data {
		// send command
		let cmd = pack(fmt: "os", args: [UInt8(8), file])
		try await send(session: session, cmd: cmd, ack: false, timeout: timeout)

		// await reply
		let reply = try await receive(session: session, expectAck: false, timeout: timeout)!

		// verify "chunk" reply
		if reply.count != 33 || reply[0] != 3 {
			throw NAOSSessionError.invalidMessage
		}

		// get hash
		let sum = Data(reply[1...])

		return sum
	}

	// - Helpers

	static func receive(session: NAOSSession, expectAck: Bool, timeout: TimeInterval) async throws -> Data? {
		// receive reply
		guard let data = try await session.receive(endpoint: self.endpoint, expectAck: expectAck, timeout: timeout) else {
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
		try await session.send(endpoint: self.endpoint, data: cmd, ackTimeout: ack ? timeout : 0)
	}
}
