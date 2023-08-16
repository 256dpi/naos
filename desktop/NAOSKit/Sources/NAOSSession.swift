//
//  Created by Joël Gähwiler on 30.07.23.
//  Copyright © 2023 Joël Gähwiler. All rights reserved.
//

import Foundation

/// A message.
public struct NAOSMessage {
	public var endpoint: UInt8
	public var data: Data?
	
	public func size() -> Int {
		return self.data?.count ?? 0
	}
}

/// A session to communicate with endpoints.
public class NAOSSession {
	public var device: NAOSDevice?
	private var id: UInt16
	private var stream: AsyncStream<NAOSMessage>?
	private var continuation: AsyncStream<NAOSMessage>.Continuation?
	
	init(device: NAOSDevice, id: UInt16) {
		// set device and id
		self.device = device
		self.id = id
		
		// create channel
		self.stream = AsyncStream<NAOSMessage> { continuation in
			self.continuation = continuation
		}
	}
	
	/// Ping will check the session and keep it alive.
	public func ping(timeout: TimeInterval) async throws {
		// send command
		try await self.send(msg: NAOSMessage(endpoint: 0xFE, data: nil))
		
		// await message
		let msg = try await self.receive(timeout: timeout)
		
		// verify message
		if msg.endpoint != 0xFE || msg.size() != 1 || msg.data![0] != 1 {
			throw NAOSError.invalidMessage
		}
	}
	
	/// Query will check an endpoints existence.
	public func query(endpoint: UInt8, timeout: TimeInterval) async throws -> Bool {
		// send command
		try await self.send(msg: NAOSMessage(endpoint: endpoint, data: nil))
		
		// await message
		let msg = try await self.receive(timeout: timeout)
		
		// verify message
		if msg.endpoint != 0xFE || msg.size() != 1 {
			throw NAOSError.invalidMessage
		}
		
		return msg.data![0] == 1
	}
	
	/// Wait and receive a message.
	public func receive(timeout: TimeInterval) async throws -> NAOSMessage {
		// return next message from channel
		return try await withTimeout(seconds: timeout) {
			for await msg in self.stream! {
				return msg
			}
			throw NAOSError.sessionClosed
		}
	}
	
	/// Send a message.
	public func send(msg: NAOSMessage) async throws {
		// get device
		guard let device = self.device else {
			throw NAOSError.sessionClosed
		}
		
		// frame message
		var data = Data(count: 4 + msg.size())
		
		// write version
		data[0] = 1
		
		// write session
		data[1] = UInt8(self.id)
		data[2] = UInt8(self.id >> 8)
		
		// write endpoint
		data[3] = msg.endpoint
		
		// copy data if available
		if let bytes = msg.data {
			data.replaceSubrange(4..., with: bytes)
		}
		
		// forward message
		try await device.write(char: .msg, data: data, confirm: false)
	}
	
	/// End the session.
	public func end(timeout: TimeInterval) async throws {
		// get device
		guard let device = self.device else {
			throw NAOSError.sessionClosed
		}
		
		// send end command
		try await self.send(msg: NAOSMessage(endpoint: 0xFF, data: nil))
		
		// await message
		let msg = try await self.receive(timeout: timeout)
		
		// verify message
		if msg.endpoint != 0xFF || msg.size() > 0 {
			throw NAOSError.invalidMessage
		}
		
		// close channel
		self.continuation?.finish()
		
		// remove session
		device.sessions[self.id] = nil
		
		// unset device
		self.device = nil
	}
	
	internal func dispatch(msg: NAOSMessage) {
		// send to channel
		self.continuation?.yield(msg)
	}
}
