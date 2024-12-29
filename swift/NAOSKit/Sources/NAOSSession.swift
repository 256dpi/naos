//
//  Created by Joël Gähwiler on 30.07.23.
//  Copyright © 2023 Joël Gähwiler. All rights reserved.
//

import Foundation
import Semaphore

/// The available status flags.
public struct NAOSSessionStatus: OptionSet {
	public let rawValue: UInt8

	public init(rawValue: UInt8) { self.rawValue = rawValue }

	public static let locked = NAOSSessionStatus(rawValue: 1 << 0)
}

/// An session specific error..
public enum NAOSSessionError: LocalizedError {
	case unavailable
	case timeout
	case closed
	case expectedAck
	case unexpectedAck
	case invalidMessage
	case unknownMessage
	case endpointError
	case sessionLocked

	public static func parse(value: UInt8) -> NAOSSessionError {
		switch value {
		case 2: return .invalidMessage
		case 3: return .unknownMessage
		case 4: return .endpointError
		case 5: return .sessionLocked
		default: return .expectedAck
		}
	}

	public var errorDescription: String? {
		switch self {
		case .unavailable:
			return "Session not available."
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
		case .sessionLocked:
			return "Session locked."
		}
	}
}

/// A session to communicate with endpoints.
public class NAOSSession {
	public private(set) var id: UInt16
	public private(set) var channel: NAOSChannel
	
	private var queue: NAOSQueue
	private var mutex = AsyncSemaphore(value: 1)

	public static func open(channel: NAOSChannel, timeout: TimeInterval) async throws -> NAOSSession {
		// create queue
		let queue = NAOSQueue()

		// subscribe queue
		channel.subscribe(queue: queue)

		// handle cancellaton
		var ok = false
		defer {
			if !ok {
				channel.unsubscribe(queue: queue)
			}
		}

		// generate handle
		let outHandle = randomString(length: 16).data(using: .utf8)!

		// send "begin" command
		try await NAOSWrite(channel: channel, msg: NAOSMessage(session: 0, endpoint: 0, data: outHandle))

		// await response
		var sid: UInt16 = 0
		while true {
			let msg = try await NAOSRead(queue: queue, timeout: timeout)
			if msg.endpoint == 0 && msg.data == outHandle {
				sid = msg.session
				break
			}
		}

		// create session
		let session = NAOSSession(id: sid, channel: channel, queue: queue)

		// set flag
		ok = true

		return session
	}

	init(id: UInt16, channel: NAOSChannel, queue: NAOSQueue) {
		// setup session
		self.id = id
		self.channel = channel
		self.queue = queue
	}

	/// Ping will check the session and keep it alive.
	public func ping(timeout: TimeInterval = 5) async throws {
		// acquire mutex
		await mutex.wait()
		defer { mutex.signal() }

		// write command
		try await self.write(msg: NAOSMessage(session: self.id, endpoint: 0xFE, data: Data()))

		// read reply
		let msg = try await self.read(timeout: timeout)

		// verify reply
		if msg.endpoint != 0xFE || msg.size() != 1 {
			throw NAOSSessionError.invalidMessage
		} else if msg.data[0] != 1 {
			throw NAOSSessionError.parse(value: msg.data[0])
		}
	}

	/// Query will check an endpoints existence.
	public func query(endpoint: UInt8, timeout: TimeInterval = 5) async throws -> Bool {
		// acquire mutex
		await mutex.wait()
		defer { mutex.signal() }

		// write command
		try await self.write(
			msg: NAOSMessage(session: self.id, endpoint: endpoint, data: Data()))

		// reaad reply
		let msg = try await self.read(timeout: timeout)

		// verify message
		if msg.endpoint != 0xFE || msg.size() != 1 {
			throw NAOSSessionError.invalidMessage
		}

		return msg.data[0] == 1
	}

	/// Wait and receive the next message for the specified endpoint with reply handling.
	public func receive(endpoint: UInt8, expectAck: Bool, timeout: TimeInterval = 5) async throws
		-> Data?
	{
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
			if msg.data[0] == 1 {
				if expectAck {
					return nil
				} else {
					throw NAOSSessionError.unexpectedAck
				}
			}

			throw NAOSSessionError.parse(value: msg.data[0])
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
		try await write(msg: NAOSMessage(session: self.id, endpoint: endpoint, data: data))

		// return if timeout is zero
		if ackTimeout == 0 {
			return
		}

		// await reply
		let msg = try await read(timeout: ackTimeout)

		// check reply
		if msg.size() != 1 || msg.endpoint != 0xFE {
			throw NAOSSessionError.invalidMessage
		} else if msg.data[0] != 1 {
			throw NAOSSessionError.parse(value: msg.data[0])
		}
	}

	/// Request the session status.
	public func status(timeout: TimeInterval = 5) async throws -> NAOSSessionStatus {
		// send command
		try? await send(endpoint: 0xFD, data: Data([0]), ackTimeout: 0)

		// await reply
		let reply = try await receive(endpoint: 0xFD, expectAck: false, timeout: timeout)!

		// verify reply
		if reply.count != 1 {
			throw NAOSSessionError.invalidMessage
		}

		// get status
		let status = NAOSSessionStatus(rawValue: reply[0])

		return status
	}

	/// Unlock  a locked session with the password.
	public func unlock(password: String, timeout: TimeInterval = 5) async throws -> Bool {
		// prepare command
		var cmd = Data([1])
		cmd.append(password.data(using: .utf8)!)

		// send command
		try? await send(endpoint: 0xFD, data: cmd, ackTimeout: 0)

		// await reply
		let reply = try await receive(endpoint: 0xFD, expectAck: false, timeout: timeout)!

		// verify reply
		if reply.count != 1 {
			throw NAOSSessionError.invalidMessage
		}

		return reply[0] == 1
	}

	/// End the session.
	public func end(timeout: TimeInterval = 5) async throws {
		// acquire mutex
		await mutex.wait()
		defer { mutex.signal() }

		// ensure unsubscribe
		defer { channel.unsubscribe(queue: queue) }

		// write command
		try await write(msg: NAOSMessage(session: self.id, endpoint: 0xFF, data: Data()))

		// stop if timeout is zero
		if timeout == 0 {
			return
		}

		// read reply
		let msg = try await self.read(timeout: timeout)

		// verify reply
		if msg.endpoint != 0xFF || msg.size() > 0 {
			throw NAOSSessionError.invalidMessage
		}
	}

	/// Clean  up the session.
	public func cleanup() {
		// end session in background
		Task{ try? await end(timeout: 0) }
	}

	// Helpers

	/// Write a message directly to the sessions underyling channel.
	public func write(msg: NAOSMessage) async throws {
		try await NAOSWrite(channel: channel, msg: msg)
	}

	/// Read a message directly from the sessions underlying queue.
	public func read(timeout: TimeInterval) async throws -> NAOSMessage {
		while true {
			let msg = try await NAOSRead(queue: queue, timeout: timeout)
			if msg.session == id {
				return msg
			}
		}
	}
}
