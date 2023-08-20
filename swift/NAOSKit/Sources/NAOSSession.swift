//
//  Created by Joël Gähwiler on 30.07.23.
//  Copyright © 2023 Joël Gähwiler. All rights reserved.
//

import Foundation

/// A session message.
public struct NAOSMessage {
	public var endpoint: UInt8
	public var data: Data?
	
	public func size() -> Int {
		return self.data?.count ?? 0
	}
}

/// An session specific error..
public enum NAOSSessionError: LocalizedError {
	case timeout
	case closed
	case expectedAck
	case unexpectedAck
	case invalidMessage
	case unknownMessage
	case failedMessage

	public var errorDescription: String? {
		switch self {
		case .timeout:
			return "Session timed out."
		case .closed:
			return "Session has been closed."
		case .expectedAck:
			return "Expected acknowledgemnt."
		case .unexpectedAck:
			return "Unexpected acknowledgement."
		case .invalidMessage:
			return "Message was invalid."
		case .unknownMessage:
			return "Unknown message."
		case .failedMessage:
			return "Message failed."
		}
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
		// write command
		try await self.write(msg: NAOSMessage(endpoint: 0xFE, data: nil))
		
		// read reply
		let msg = try await self.read(timeout: timeout)
		
		// verify reply
		if msg.endpoint != 0xFE || msg.size() != 1 {
			throw NAOSSessionError.invalidMessage
		} else if msg.data![0] != 1 {
			throw NAOSSessionError.expectedAck
		}
	}
	
	/// Query will check an endpoints existence.
	public func query(endpoint: UInt8, timeout: TimeInterval) async throws -> Bool {
		// write command
		try await self.write(msg: NAOSMessage(endpoint: endpoint, data: nil))
		
		// erad reply
		let msg = try await self.read(timeout: timeout)
		
		// verify message
		if msg.endpoint != 0xFE || msg.size() != 1 {
			throw NAOSSessionError.invalidMessage
		}
		
		return msg.data![0] == 1
	}
	
	/// Wait and read the next message.
	public func read(timeout: TimeInterval) async throws -> NAOSMessage {
		// return next message from channel
		return try await withTimeout(seconds: timeout) {
			for await msg in self.stream! {
				return msg
			}
			throw NAOSSessionError.closed
		}
	}
	
	/// Wait and receive the next message for the specified endpoint with optionally handling acks.
	public func receive(endpoint: UInt8, ack: Bool, timeout: TimeInterval) async throws -> Data? {
		// await message
		let msg = try await read(timeout: timeout)
		
		// handle acks
		if ack && msg.endpoint == 0xFE {
			// check size
			if msg.size() != 1 {
				throw NAOSSessionError.invalidMessage
			}
			
			// check if OK
			if msg.data![0] == 1 {
				return nil
			}
			
			throw NAOSSessionError.expectedAck
		}
		
		// check endpoint
		if msg.endpoint != endpoint {
			throw NAOSSessionError.invalidMessage
		}
		
		return msg.data
	}
	
	/// Write a message.
	public func write(msg: NAOSMessage) async throws {
		// get device
		guard let device = self.device else {
			throw NAOSSessionError.closed
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
		
	/// Send a message with optionally waiting for an acknowledgement.
	public func send(endpoint: UInt8, data: Data, ackTimeout: TimeInterval) async throws {
		// write message
		try await write(msg: NAOSMessage(endpoint: endpoint, data: data))
		
		// return if timeout is zero
		if ackTimeout == 0 {
			return
		}
		
		// await reply
		let msg = try await read(timeout: ackTimeout)
		
		// check reply
		if msg.size() != 1 || msg.endpoint != 0xFE {
			throw NAOSSessionError.invalidMessage
		} else if msg.data![0] != 1 {
			throw NAOSSessionError.expectedAck
		}
	}
	
	/// End the session.
	public func end(timeout: TimeInterval) async throws {
		// get device
		guard let device = self.device else {
			throw NAOSSessionError.closed
		}
		
		// wite command
		try await self.write(msg: NAOSMessage(endpoint: 0xFF, data: nil))
		
		// read reply
		let msg = try await self.read(timeout: timeout)
		
		// verify reply
		if msg.endpoint != 0xFF || msg.size() > 0 {
			throw NAOSSessionError.invalidMessage
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
