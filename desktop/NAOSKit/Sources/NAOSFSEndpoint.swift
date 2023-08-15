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
public class NAOSFSEndpoint {
	private let session: NAOSSession
	private let timeout: TimeInterval
	
	public init(session: NAOSSession, timeout: TimeInterval) {
		self.session = session
		self.timeout = timeout
	}
	
	/// Get information on a file or directory.
	public func stat(path: String) async throws -> NAOSFSInfo {
		// prepare command
		var cmd = Data([0])
		cmd.append(path.data(using: .utf8)!)
		
		// send comamnd
		try await send(data: cmd, ack: false)
		
		// await reply
		let reply = try await receive(ack: false)!
		
		// verify "info" reply
		if reply.count != 6 || reply[0] != 1 {
			throw NAOSError.invalidMessage
		}
		
		// parse "info" reply
		let isDir = reply[1] == 1
		let size = readUint32(data: Data(reply[2...]))
		
		return NAOSFSInfo(name: "", isDir: isDir, size: size)
	}
	
	/// List a files and directories.
	public func list(path: String) async throws -> [NAOSFSInfo] {
		// prepare command
		var cmd = Data([1])
		cmd.append(path.data(using: .utf8)!)
		
		// send comamnd
		try await send(data: cmd, ack: false)
		
		// prepare infos
		var infos: [NAOSFSInfo] = []
		
		while true {
			// await reply
			guard let reply = try await receive(ack: true) else {
				break
			}
			
			// verify "info" reply
			if reply.count < 7 || reply[0] != 1 {
				throw NAOSError.invalidMessage
			}
			
			// parse "info" reply
			let isDir = reply[1] == 1
			let size = readUint32(data: Data(reply[2...]))
			let name = String(bytes: Data(reply[6...]), encoding: .utf8)!
			
			// add info
			infos.append(NAOSFSInfo(name: name, isDir: isDir, size: size))
		}
		
		return infos
	}
	
	/// Read a full file.
	public func read(path: String) async throws -> Data {
		// prepare command
		var cmd = Data([2, 0, 0, 0, 0, 0, 0, 0, 0])
		cmd.append(path.data(using: .utf8)!)
		
		// send comamnd
		try await send(data: cmd, ack: false)
		
		// prepare data
		var data = Data()
		
		while true {
			// await reply
			guard let reply = try await receive(ack: true) else {
				break
			}
			
			// verify "chunk" reply
			if reply.count == 0 || reply[0] != 2 {
				throw NAOSError.invalidMessage
			}
			
			// append data
			data.append(Data(reply[1...]))
		}
		
		return data
	}
	
	/// Write a full file.
	public func write(path: String, data: Data) async throws {
		// prepare "create" command
		var cmd = Data([3])
		cmd.append(path.data(using: .utf8)!)
		
		// send "create" comamnd
		try await send(data: cmd, ack: true)
		
		// TODO: Dynamically determine channel MTU?
		
		// write data in 500-byte chunks
		var offset = 0
		while offset < data.count {
			// determine chunk size and chunk data
			let chunkSize = min(500, data.count - offset)
			let chunkData = data.subdata(in: offset ..< offset + chunkSize)
			
			// prepare "write" command
			cmd = Data([4])
			cmd.append(writeUint32(value: UInt32(offset)))
			cmd.append(chunkData)
			
			// send "write" command
			try await send(data: cmd, ack: false)
			
			// receive ack or "error" replies
			let _ = try await receive(ack: true)
			
			// increment offset
			offset += chunkSize
		}
		
		// send final "write" comamnd
		try await send(data: Data([4, 0, 0, 0, 0]), ack: true)
	}
	
	/// Rename a file.
	public func rename(from: String, to: String) async throws {
		// prepare command
		var cmd = Data([5])
		cmd.append(from.data(using: .utf8)!)
		cmd.append(Data([0]))
		cmd.append(to.data(using: .utf8)!)
		
		// send comamnd
		try await send(data: cmd, ack: true)
	}
	
	/// Remove a file.
	public func remove(path: String) async throws {
		// prepare command
		var cmd = Data([6])
		cmd.append(path.data(using: .utf8)!)
		
		// send comamnd
		try await send(data: cmd, ack: true)
	}
	
	/// End the underlying session.
	public func end() async throws {
		// end session
		try await session.end(timeout: timeout)
	}
	
	// - Helpers
	
	internal func receive(ack: Bool) async throws -> Data? {
		// TODO: Move parts to session?
		
		// await message
		let msg = try await session.receive(timeout: timeout)
		
		// handle acks
		if ack, msg.endpoint == 0xFE, msg.size() == 1, msg.data![0] == 1 {
			return nil
		}
		
		// check message
		if msg.endpoint != 0x03 {
			throw NAOSError.invalidMessage
		}
		
		// check reply
		guard let data = msg.data else {
			throw NAOSError.invalidMessage
		}
		
		// handle errors
		if data[0] == 0 {
			throw POSIXError(POSIXErrorCode(rawValue: Int32(data[1]))!)
		}
		
		return data
	}
	
	internal func send(data: Data, ack: Bool) async throws {
		// TODO: Move parts to session?
		
		// send message
		try await session.send(msg: NAOSMessage(endpoint: 0x03, data: data))
		
		// return if ack is false
		if !ack {
			return
		}
		
		// await message
		let msg = try await session.receive(timeout: timeout)
		
		// check message
		if msg.size() != 1 || msg.endpoint != 0xFE {
			throw NAOSError.invalidMessage
		}
		
		// check ack
		if msg.data![0] != 1 {
			throw NAOSError.expectedAck
		}
	}
}
