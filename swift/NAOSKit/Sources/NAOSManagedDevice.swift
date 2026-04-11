//
//  Created by Joël Gähwiler on 05.04.17.
//  Copyright © 2017 Joël Gähwiler. All rights reserved.
//

import Foundation
import Semaphore

/// A lifecycle event emitted by a managed device.
public enum NAOSManagedDeviceEvent {
	case connected
	case disconnected
}

public enum NAOSManagedDeviceError: LocalizedError {
	case notConnected
	case stopped

	public var errorDescription: String? {
		switch self {
		case .notConnected:
			return "Device not connected."
		case .stopped:
			return "Device has been stopped."
		}
	}
}

open class NAOSManagedDevice: NSObject {
	private var mutex = AsyncSemaphore(value: 1)
	private var session: NAOSSession?
	private var channel: NAOSChannel?
	private var pingerTask: Task<Void, Never>?
	private var eventSubs: [UUID: AsyncStream<NAOSManagedDeviceEvent>.Continuation] = [:]
	private var eventSubsLock = NSLock()
	private var stopped = false
	private var password: String = ""

	public private(set) var device: NAOSDevice
	public private(set) var active: Bool = false
	public private(set) var locked: Bool = false

	public init(device: NAOSDevice) {
		// set device
		self.device = device

		// finish init
		super.init()

		// run pinger
		pingerTask = Task { [weak self] in
			while !Task.isCancelled {
				try? await Task.sleep(for: .seconds(1))
				if Task.isCancelled { break }
				guard let self else { break }

				await self.mutex.wait()
				if self.session != nil {
					do {
						try await self.session!.ping(timeout: 5)
					} catch {
						self.session?.cleanup()
						self.session = nil
					}
				}
				self.mutex.signal()
			}
		}
	}

	deinit {
		pingerTask?.cancel()
		stopped = true
	}

	/// Returns a new subscription stream for lifecycle events. The stream is
	/// buffered (4) and drops events if the consumer is too slow. It ends when
	/// the managed device is deallocated.
	public func events() -> AsyncStream<NAOSManagedDeviceEvent> {
		let id = UUID()
		return AsyncStream(bufferingPolicy: .bufferingNewest(4)) { continuation in
			self.eventSubsLock.lock()
			self.eventSubs[id] = continuation
			self.eventSubsLock.unlock()

			continuation.onTermination = { @Sendable _ in
				self.eventSubsLock.lock()
				self.eventSubs.removeValue(forKey: id)
				self.eventSubsLock.unlock()
			}
		}
	}

	/// Activate will initiate a connection to a device.
	public func activate() async throws {
		// acquire mutex
		await mutex.wait()
		defer { mutex.signal() }

		// check state
		if stopped {
			throw NAOSManagedDeviceError.stopped
		}
		if active {
			return
		}

		// open channel
		let ch = try await device.open()
		channel = ch

		// set flag
		active = true

		// read lock status
		try await withSession { session in
			self.locked = try await session.status().contains(.locked)
		}

		// emit connected
		emitEvent(.connected)

		// watch for transport loss
		Task { [weak self] in
			await ch.done
			guard let self else { return }
			await self.didDisconnect()
		}
	}

	/// Unlock will attempt to unlock the device and returns its success.
	public func unlock(password: String) async throws -> Bool {
		// acquire mutex
		await mutex.wait()
		defer { mutex.signal() }

		// check state
		if !active {
			throw NAOSManagedDeviceError.notConnected
		}

		// unlock
		try await withSession { session in
			if try await session.unlock(password: password) {
				self.password = password
				self.locked = false
			}
		}

		return !locked
	}

	/// NewSession will create a new session and return it.
	public func newSession(timeout: TimeInterval = 5) async throws -> NAOSSession {
		// acquire mutex
		await mutex.wait()
		defer { mutex.signal() }

		// check state
		if stopped {
			throw NAOSManagedDeviceError.stopped
		}
		if !active {
			throw NAOSManagedDeviceError.notConnected
		}

		// open new session
		return try await openSession(timeout: timeout)
	}

	/// UseSession will yield the managed session.
	public func useSession(callback: (NAOSSession) async throws -> Void) async throws {
		// acquire mutex
		await mutex.wait()
		defer { mutex.signal() }

		// check state
		if stopped {
			throw NAOSManagedDeviceError.stopped
		}
		if !active {
			throw NAOSManagedDeviceError.notConnected
		}

		// yield session
		try await withSession(callback: callback)
	}

	/// Deactivate will close the connection to the device.
	public func deactivate() async throws {
		// acquire mutex
		await mutex.wait()
		defer { mutex.signal() }

		// check state
		if !active {
			return
		}

		// cleanup session
		session?.cleanup()
		session = nil

		// close channel
		channel?.close()
		channel = nil

		// set flag
		active = false
	}

	/// Stop will deactivate and permanently disable the device.
	public func stop() async {
		// deactivate
		try? await deactivate()

		// cancel pinger
		pingerTask?.cancel()
		pingerTask = nil

		// finish all event streams
		eventSubsLock.lock()
		let subs = eventSubs
		eventSubs = [:]
		eventSubsLock.unlock()
		for (_, cont) in subs {
			cont.finish()
		}

		// set flag
		stopped = true
	}

	// Helpers

	private func openSession(timeout: TimeInterval = 5) async throws -> NAOSSession {
		// open session
		let session = try await NAOSSession.open(channel: channel!, timeout: timeout)

		// try to unlock if locked
		if !password.isEmpty {
			if try (await session.status()).contains(.locked) {
				if try await session.unlock(password: password) {
					locked = false
				}
			}
		}

		return session
	}

	private func withSession(callback: (NAOSSession) async throws -> Void) async throws {
		// ensure session
		if session == nil {
			session = try await openSession()
		}

		// yield session
		do {
			try await callback(session!)
		} catch {
			session?.cleanup()
			session = nil
			throw error
		}
	}

	private func didDisconnect() async {
		// acquire mutex
		await mutex.wait()
		defer { mutex.signal() }

		// check state
		if !active {
			return
		}

		// clear session
		session?.cleanup()
		session = nil

		// close channel
		channel?.close()
		channel = nil

		// set flag
		active = false

		// emit disconnected
		emitEvent(.disconnected)
	}

	private func emitEvent(_ event: NAOSManagedDeviceEvent) {
		eventSubsLock.lock()
		let subs = Array(eventSubs.values)
		eventSubsLock.unlock()
		for cont in subs {
			cont.yield(event)
		}
	}
}
