//
//  NAOSDevice.swift
//
//
//  Created by Joël Gähwiler on 24.11.2024.
//

import Foundation

public protocol NAOSDevice {
	func id() -> String
	func open() async throws -> NAOSChannel
}

public typealias NAOSQueue = Channel<Data>

public protocol NAOSChannel {
	// func device() -> NAOSDevice
	func subscribe(queue: NAOSQueue)
	func unsubscribe(queue: NAOSQueue)
	func write(data: Data) async throws
	func close()
}

/// A session message.
public struct NAOSMessage {
	public var session: UInt16
	public var endpoint: UInt8
	public var data: Data?

	public func size() -> Int {
		return self.data?.count ?? 0
	}

	// TODO: Remove.
	public static func parse(data: Data) throws -> NAOSMessage {
		// verify size and version
		if data.count < 4 || data[0] != 1 {
			throw NAOSSessionError.invalidMessage
		}

		// unpack message
		let args = unpack(fmt: "hob", data: data, start: 1)
		let sid = args[0] as! UInt16
		let eid = args[1] as! UInt8
		let data = args[2] as! Data

		// prepare message
		let msg = NAOSMessage(session: sid, endpoint: eid, data: data)

		return msg
	}
}

func NAOSRead(queue: NAOSQueue, timeout: TimeInterval) async throws -> NAOSMessage {
	// read data
	let data = try await queue.receive(timeout: timeout)

	// check length and version
	if data.count < 4 || data[0] != 1 {
		throw NAOSSessionError.invalidMessage
	}

	// unpack message
	let args = unpack(fmt: "ohob", data: data, start: 1)

	return NAOSMessage(
		session: args[0] as! UInt16,
		endpoint: args[1] as! UInt8,
		data: args[2] as? Data
	)
}

func NAOSWrite(channel: NAOSChannel, msg: NAOSMessage) async throws {
	// pack message
	let data = pack(fmt: "ohob", args: [UInt8(0), msg.session, msg.endpoint, msg.data ?? Data()])

	// write data
	try await channel.write(data: data)
}
