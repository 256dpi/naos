//
//  Created by Joël Gähwiler on 30.07.23.
//  Copyright © 2023 Joël Gähwiler. All rights reserved.
//

import Combine
import Foundation
import Semaphore

/// A session message.
public struct NAOSMessage {
	public var session: UInt16
	public var endpoint: UInt8
	public var data: Data?
	
	public func size() -> Int {
		return self.data?.count ?? 0
	}
	
	public static func parse(data: Data) throws -> NAOSMessage {
		// verify size and version
		if data.count < 4 || data[0] != 1 {
			throw NAOSSessionError.invalidMessage
		}

		// read session ID
		let sid = readUint16(data: Data(data[1 ... 2]))

		// read endpoint ID
		let eid = data[3]
		
		// prepare message
		let msg = NAOSMessage(session: sid, endpoint: eid, data: Data(data[4...]))

		return msg
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
	case endpointError
	
	public static func parse(value: UInt8) -> NAOSSessionError {
		switch value {
		case 2: return .invalidMessage
		case 3: return .unknownMessage
		case 4: return .endpointError
		default: return .expectedAck
		}
	}

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
			return "Message was unknown."
		case .endpointError:
			return "Endpoint errored."
		}
	}
}

/// A session to communicate with endpoints.
public class NAOSSession {
	private var id: UInt16
	private var peripheral: NAOSPeripheral
	private var stream: AsyncStream<Data>
	private var subscription: AnyCancellable
	private var mutex = AsyncSemaphore(value: 1)
	
	internal static func open(peripheral: NAOSPeripheral, timeout: TimeInterval) async throws -> NAOSSession? {
		// check characteristic
		if !peripheral.exists(char: .msg) {
			return nil
		}
		
		// open stream
		let (stream, subscription) = await peripheral.stream(char: .msg)
		
		// handle cancellaton
		var ok = false
		defer {
			if !ok {
				subscription.cancel()
			}
		}

		// generate handle
		let outHandle = randomString(length: 16)

		// prepare message
		var msg = Data([1, 0, 0, 0])
		msg.append(outHandle.data(using: .utf8)!)

		// send "begin" command
		try await peripheral.write(char: .msg, data: msg, confirm: false)

		// await response
		let sid = try? await withTimeout(seconds: timeout) {
			for await data in stream {
				// parse message
				let msg = try NAOSMessage.parse(data: data)
				
				// check endpoint
				if msg.endpoint != 0 {
					continue
				}
				
				// get handle
				let inHandle = String(data: Data(data[4...]), encoding: .utf8)!
				
				// check handle
				if inHandle != outHandle {
					continue
				}

				return msg.session
			}
			throw NAOSSessionError.closed
		}

		// handle missing session
		if sid == nil {
			return nil
		}

		// create session
		let session = NAOSSession(id: sid!, peripheral: peripheral, stream: stream, subscription: subscription)
		
		// set flag
		ok = true

		return session
	}
	
	init(id: UInt16, peripheral: NAOSPeripheral, stream: AsyncStream<Data>, subscription: AnyCancellable) {
		// setup session
		self.id = id
		self.peripheral = peripheral
		self.stream = stream
		self.subscription = subscription
	}
	
	/// Ping will check the session and keep it alive.
	public func ping(timeout: TimeInterval) async throws {
		// acquire mutex
		await mutex.wait()
		defer { mutex.signal() }
		
		// write command
		try await self.write(msg: NAOSMessage(session: self.id, endpoint: 0xFE, data: nil))
		
		// read reply
		let msg = try await self.read(timeout: timeout)
		
		// verify reply
		if msg.endpoint != 0xFE || msg.size() != 1 {
			throw NAOSSessionError.invalidMessage
		} else if msg.data![0] != 1 {
			throw NAOSSessionError.parse(value: msg.data![0])
		}
	}
	
	/// Query will check an endpoints existence.
	public func query(endpoint: UInt8, timeout: TimeInterval) async throws -> Bool {
		// acquire mutex
		await mutex.wait()
		defer { mutex.signal() }
		
		// write command
		try await self.write(msg: NAOSMessage(session: self.id, endpoint: endpoint, data: nil))
		
		// erad reply
		let msg = try await self.read(timeout: timeout)
		
		// verify message
		if msg.endpoint != 0xFE || msg.size() != 1 {
			throw NAOSSessionError.invalidMessage
		}
		
		return msg.data![0] == 1
	}
	
	/// Wait and receive the next message for the specified endpoint with reply handling.
	public func receive(endpoint: UInt8, expectAck: Bool, timeout: TimeInterval) async throws -> Data? {
		// acquire mutex
		await mutex.wait()
		defer { mutex.signal() }
		
		// await message
		let msg = try await read(timeout: timeout)
		
		// handle acks
		if msg.endpoint == 0xFE {
			// check size
			if msg.size() != 1 {
				throw NAOSSessionError.invalidMessage
			}
			
			// check if OK
			if msg.data![0] == 1 {
				if expectAck {
					return nil
				} else {
					throw NAOSSessionError.unexpectedAck
				}
			}
			
			throw NAOSSessionError.parse(value: msg.data![0])
		}
		
		// check endpoint
		if msg.endpoint != endpoint {
			throw NAOSSessionError.invalidMessage
		}
		
		return msg.data
	}
		
	/// Send a message with optionally waiting for an acknowledgement.
	public func send(endpoint: UInt8, data: Data, ackTimeout: TimeInterval) async throws {
		// acquire mutex
		await mutex.wait()
		defer { mutex.signal() }
		
		// write message
		try await self.write(msg: NAOSMessage(session: self.id, endpoint: endpoint, data: data))
		
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
			throw NAOSSessionError.parse(value: msg.data![0])
		}
	}
	
	/// End the session.
	public func end(timeout: TimeInterval) async throws {
		// acquire mutex
		await mutex.wait()
		defer { mutex.signal() }
		
		// wite command
		try await self.write(msg: NAOSMessage(session: self.id, endpoint: 0xFF, data: nil))
		
		// read reply
		let msg = try await self.read(timeout: timeout)
		
		// verify reply
		if msg.endpoint != 0xFF || msg.size() > 0 {
			throw NAOSSessionError.invalidMessage
		}
		
		// close channel
		self.subscription.cancel()
	}
	
	// Helpers
	
	private func write(msg: NAOSMessage) async throws {
		// frame message
		var data = Data(count: 4 + msg.size())
		
		// write version
		data[0] = 1
		
		// write session
		data[1] = UInt8(self.id & 0xFF)
		data[2] = UInt8(self.id >> 8)
		
		// write endpoint
		data[3] = msg.endpoint
		
		// copy data if available
		if let bytes = msg.data {
			data.replaceSubrange(4..., with: bytes)
		}
		
		// forward message
		try await self.peripheral.write(char: .msg, data: data, confirm: false)
	}

	private func read(timeout: TimeInterval) async throws -> NAOSMessage {
		// return next message from channel
		return try await withTimeout(seconds: timeout) {
			for await data in self.stream {
				// parse message
				let msg = try NAOSMessage.parse(data: data)
				
				// check session IDS
				if msg.session != self.id {
					continue
				}
				
				return msg
			}
			throw NAOSSessionError.closed
		}
	}
}
