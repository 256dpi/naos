//
//  NAOSDevice.swift
//
//
//  Created by Joël Gähwiler on 24.11.2024.
//

import Foundation

/// A generic message device.
public protocol NAOSDevice: AnyObject {
	func type() -> String
	func id() -> String
	func name() -> String
	func open() async throws -> NAOSChannel
}

/// A session message.
public struct NAOSMessage {
	public var session: UInt16
	public var endpoint: UInt8
	public var data: Data

	public func size() -> Int {
		return self.data.count
	}

	static func parse(data: Data) -> NAOSMessage? {
		if data.count < 4 || data[0] != 1 {
			return nil
		}

		guard let args = try? unpack(fmt: "hob", data: data, start: 1) else {
			return nil
		}

		return NAOSMessage(
			session: args[0] as! UInt16,
			endpoint: args[1] as! UInt8,
			data: args[2] as! Data
		)
	}

	func build() -> Data {
		pack(fmt: "ohob", args: [UInt8(1), session, endpoint, data])
	}
}

// TODO: Make `Channel<T>` bounded and handle backpressure.

/// A generic message queue.
public typealias NAOSQueue = Channel<NAOSMessage>

/// A generic message transport.
public protocol NAOSTransport {
	func read() async throws -> Data
	func write(data: Data) async throws
	func close()
}

enum NAOSTransportError: LocalizedError {
	case closed

	var errorDescription: String? {
		switch self {
		case .closed:
			return "Transport has been closed."
		}
	}
}

enum NAOSChannelError: LocalizedError {
	case wrongOwner

	var errorDescription: String? {
		switch self {
		case .wrongOwner:
			return "Message belongs to another session owner."
		}
	}
}

private struct NAOSDataKey: Hashable {
	let data: Data
}

/// A generic message channel.
///
/// The channel owns the transport reader task and routes messages to
/// subscribers. Session ownership is tracked per subscribed queue so session
/// traffic is only delivered to the queue that opened that session.
public final class NAOSChannel {
	private let transport: NAOSTransport
	private let deviceValue: NAOSDevice?
	private let onClose: (@Sendable () -> Void)?
	private let widthValue: Int
	private let lock = NSLock()
	private var closed = false
	private var doneContinuation: CheckedContinuation<Void, Never>?

	/// Resolves when the channel is closed, whether by transport loss or
	/// explicit close. Analogous to the web `ch.done` promise.
	public var done: Void {
		get async {
			let alreadyClosed = lock.withLock { closed }
			if alreadyClosed { return }
			await withCheckedContinuation { continuation in
				let finish = lock.withLock { () -> Bool in
					if closed { return true }
					doneContinuation = continuation
					return false
				}
				if finish {
					continuation.resume()
				}
			}
		}
	}
	private var readerTask: Task<Void, Never>?
	private var queues: [ObjectIdentifier: NAOSQueue] = [:]
	private var opening: [NAOSDataKey: ObjectIdentifier] = [:]
	private var sessions: [UInt16: ObjectIdentifier] = [:]
	private var closing: [UInt16: ObjectIdentifier] = [:]

	init(
		transport: NAOSTransport,
		device: NAOSDevice? = nil,
		width: Int,
		onClose: (@Sendable () -> Void)? = nil
	) {
		self.transport = transport
		self.deviceValue = device
		self.onClose = onClose
		self.widthValue = width
		self.readerTask = Task { [weak self] in
			guard let self else { return }

			while !Task.isCancelled {
				do {
					let data = try await self.transport.read()
					guard let msg = NAOSMessage.parse(data: data) else {
						continue
					}
					for queue in self.route(msg: msg) {
						queue.send(value: msg)
					}
				} catch {
					self.close()
					return
				}
			}
		}
	}

	/// Returns the maximum number of inflight messages supported by the
	/// underlying transport.
	public func width() -> Int {
		widthValue
	}

	/// Returns the originating device when the channel was created by a device.
	public func device() -> NAOSDevice? {
		deviceValue
	}

	/// Registers a queue to receive incoming messages.
	public func subscribe(queue: NAOSQueue) {
		lock.withLock {
			queues[ObjectIdentifier(queue)] = queue
		}
	}

	/// Removes a queue and clears any session ownership associated with it.
	public func unsubscribe(queue: NAOSQueue) {
		lock.withLock {
			let key = ObjectIdentifier(queue)
			queues.removeValue(forKey: key)

			for (handle, owner) in opening where owner == key {
				opening.removeValue(forKey: handle)
			}
			for (session, owner) in sessions where owner == key {
				sessions.removeValue(forKey: session)
				closing.removeValue(forKey: session)
			}
			for (session, owner) in closing where owner == key {
				closing.removeValue(forKey: session)
			}
		}
	}

	/// Sends a single message on behalf of the specified queue.
	///
	/// A `nil` queue means the write has no subscriber ownership context.
	public func write(queue: NAOSQueue?, msg: NAOSMessage) async throws {
		guard let queue else {
			try await transport.write(data: msg.build())
			return
		}

		let key = ObjectIdentifier(queue)

		try lock.withLock {
			if msg.session != 0, let owner = sessions[msg.session], owner != key {
				throw NAOSChannelError.wrongOwner
			}

			if msg.session == 0 && msg.endpoint == 0x0 {
				opening[NAOSDataKey(data: msg.data)] = key
			}

			if msg.session != 0 && msg.endpoint == 0xFF {
				closing[msg.session] = key
			}
		}

		do {
			try await transport.write(data: msg.build())
		} catch {
			lock.withLock {
				if msg.session == 0 && msg.endpoint == 0x0 && opening[NAOSDataKey(data: msg.data)] == key {
					opening.removeValue(forKey: NAOSDataKey(data: msg.data))
				}
				if msg.session != 0 && msg.endpoint == 0xFF && closing[msg.session] == key {
					closing.removeValue(forKey: msg.session)
				}
			}

			throw error
		}
	}

	/// Closes the channel and the underlying transport.
	///
	/// The close callback is invoked once when the reader task exits or the
	/// channel is explicitly closed.
	public func close() {
		let shouldClose = lock.withLock {
			if closed {
				return false
			}
			closed = true
			return true
		}

		guard shouldClose else {
			return
		}

		readerTask?.cancel()
		readerTask = nil
		transport.close()
		onClose?()
		let cont = lock.withLock { () -> CheckedContinuation<Void, Never>? in
			let c = doneContinuation
			doneContinuation = nil
			return c
		}
		cont?.resume()
	}

	deinit {
		close()
	}

	private func route(msg: NAOSMessage) -> [NAOSQueue] {
		lock.withLock {
			if msg.endpoint == 0x0 {
				let handle = NAOSDataKey(data: msg.data)
				if let owner = opening.removeValue(forKey: handle), let queue = queues[owner] {
					sessions[msg.session] = owner
					return [queue]
				}
			}

			if msg.session != 0 {
				if let owner = sessions[msg.session], let queue = queues[owner] {
					if msg.endpoint == 0xFF && msg.data.isEmpty {
						sessions.removeValue(forKey: msg.session)
						closing.removeValue(forKey: msg.session)
					}
					return [queue]
				}

				sessions.removeValue(forKey: msg.session)
				closing.removeValue(forKey: msg.session)
				return []
			}

			return Array(queues.values)
		}
	}
}
