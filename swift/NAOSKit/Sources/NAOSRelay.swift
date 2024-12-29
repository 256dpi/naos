//
//  Created by Joël Gähwiler on 26.12.24.
//  Copyright © 2024 Joël Gähwiler. All rights reserved.
//

import Foundation
import Semaphore

/// The NAOS relay endpoint.
public class NAOSRelay {
	/// The endpoint number
	public static let endpoint: UInt8 = 0x4
	
	/// Scan for downstream devices.
	public static func scan(session: NAOSSession, timeout: TimeInterval = 5) async throws -> [UInt8] {
		// send command
		let cmd = Data([0])
		try await session.send(endpoint: endpoint, data: cmd, ackTimeout: 0)

		// receive reply
		let reply = try await session.receive(endpoint: endpoint, expectAck: false, timeout: timeout)!

		// check size
		if reply.count != 8 {
			throw NAOSSessionError.invalidMessage
		}

		// unpack reply
		let raw = unpack(fmt: "q", data: reply)[0] as! UInt64

		// prepare map
		var map = [UInt8]()
		for i in 0 ... 64 {
			if (raw & (1 << i)) != 0 {
				map.append(UInt8(i))
			}
		}

		return map
	}
	
	/// Link the current session with the specified device.
	public static func link(session: NAOSSession, device: UInt8, timeout: TimeInterval = 5) async throws {
		// send command
		let cmd = pack(fmt: "oo", args: [UInt8(1), device])
		try await session.send(endpoint: endpoint, data: cmd, ackTimeout: timeout)
	}
	
	/// Send a message to a downstream device.
	public static func send(session: NAOSSession, device: UInt8, data: Data) async throws {
		// send command
		let cmd = pack(fmt: "oob", args: [UInt8(2), device, data])
		try await session.send(endpoint: endpoint, data: cmd, ackTimeout: 0)
	}
	
	/// Receive a message from a downstream device.
	public static func receive(session: NAOSSession, timeout: TimeInterval = 5) async throws -> Data? {
		// receive reply
		return try await session.receive(endpoint: endpoint, expectAck: false, timeout: timeout)
	}
}

public class NAOSRelayDevice: NAOSDevice {
	var host: NAOSManagedDevice
	var device: UInt8
	
	public init(host: NAOSManagedDevice, device: UInt8) {
		self.host = host
		self.device = device
	}
	
	public func id() -> String {
		return String(format: "%s/%d", arguments: [host.device.id(), device])
	}
	
	public func name() -> String {
		return String(format: "Relay %d", arguments: [device])
	}
	
	public func open() async throws -> any NAOSChannel {
		return try await NAOSRelayChannel.open(host: host, device: device)
	}
}

public class NAOSRelayChannel: NAOSChannel {
	private var session: NAOSSession
	private var device: UInt8
	private var queues: [NAOSQueue] = []
	
	// TODO: Figure out a way to not make receive and send block each other.
	
	public static func open(host: NAOSManagedDevice, device: UInt8) async throws -> NAOSRelayChannel {
		// open session
		let session = try await host.newSession()
		
		// link device
		try await NAOSRelay.link(session: session, device: device)
		
		// create channel
		return NAOSRelayChannel(session: session, device: device)
	}
	
	init(session: NAOSSession, device: UInt8) {
		// set state
		self.session = session
		self.device = device
		
		// run forwarder
		Task {
			while true {
				// receive message
				let data = try? await NAOSRelay.receive(session: session, timeout: 0.1) // 100ms
				
				// forward message, if present
				if let data = data {
					for queue in queues {
						queue.send(value: data)
					}
				}
			}
		}
	}
	
	public func subscribe(queue: NAOSQueue) {
		// add queue
		if queues.first(where: { $0 === queue }) == nil {
			queues.append(queue)
		}
	}

	public func unsubscribe(queue: NAOSQueue) {
		// remove queue
		queues.removeAll { $0 === queue }
	}
	
	public func write(data: Data) async throws {
		// forward message
		try await NAOSRelay.send(session: session, device: device, data: data)
	}
	
	public func close() {
		// clean up session
		session.cleanup()
	}
}
