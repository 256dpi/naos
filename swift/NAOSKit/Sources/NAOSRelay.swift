//
//  Created by Joël Gähwiler on 26.12.24.
//  Copyright © 2024 Joël Gähwiler. All rights reserved.
//

import Foundation

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
		let raw = try unpack(fmt: "q", data: reply)[0] as! UInt64

		// prepare map
		var map = [UInt8]()
		for i in 0 ..< 64 {
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
		// write directly for performance
		let cmd = pack(fmt: "oob", args: [UInt8(2), device, data])
		try await session.write(msg: NAOSMessage(session: session.id, endpoint: endpoint, data: cmd))
	}
	
	/// Receive a message from a downstream device.
	public static func receive(session: NAOSSession, timeout: TimeInterval = 5) async throws -> Data? {
		// read directly for performance
		try await session.read(timeout: timeout).data
	}
}

public class NAOSRelayDevice: NAOSDevice {
	var host: NAOSManagedDevice
	var device: UInt8
	
	public init(host: NAOSManagedDevice, device: UInt8) {
		self.host = host
		self.device = device
	}
	
	public func type() -> String {
		return "Relay"
	}
	
	public func id() -> String {
		return "\(host.device.id())/\(device)"
	}
	
	public func name() -> String {
		return "Relay: \(device)"
	}
	
	public func open() async throws -> NAOSChannel {
		let session = try await host.newSession()
		try await NAOSRelay.link(session: session, device: device)
		return NAOSChannel(
			transport: relayTransport(session: session, device: device),
			device: self,
			width: session.channel.width()
		)
	}
}

final class relayTransport: NAOSTransport {
	private let session: NAOSSession
	private let device: UInt8

	init(session: NAOSSession, device: UInt8) {
		self.session = session
		self.device = device
	}

	func read() async throws -> Data {
		while true {
			do {
				if let data = try await NAOSRelay.receive(session: session, timeout: 1) {
					return data
				}
			} catch is TimedOutError {
				continue
			}
		}
	}

	func write(data: Data) async throws {
		try await NAOSRelay.send(session: session, device: device, data: data)
	}

	func close() {
		session.cleanup()
	}
}
