//
//  NAOSDevice.swift
//
//
//  Created by Joël Gähwiler on 24.11.2024.
//

import Foundation

/// A generic message device.
public protocol NAOSDevice {
	func id() -> String
	func open() async throws -> NAOSChannel
}

/// A generic message queue.
public typealias NAOSQueue = Channel<Data>

/// A generic message channel.
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
	public var data: Data

	public func size() -> Int {
		return self.data.count
	}
}

/// Read a message from a queue.
func NAOSRead(queue: NAOSQueue, timeout: TimeInterval) async throws -> NAOSMessage {
	// read data
	let data = try await queue.receive(timeout: timeout)

	// check length and version
	if data.count < 4 || data[0] != 1 {
		throw NAOSSessionError.invalidMessage
	}

	// unpack message
	let args = unpack(fmt: "hob", data: data, start: 1)

	return NAOSMessage(
		session: args[0] as! UInt16,
		endpoint: args[1] as! UInt8,
		data: args[2] as! Data
	)
}

/// Write a message to a channel.
func NAOSWrite(channel: NAOSChannel, msg: NAOSMessage) async throws {
	// pack message
	let data = pack(fmt: "ohob", args: [UInt8(1), msg.session, msg.endpoint, msg.data])

	// write data
	try await channel.write(data: data)
}
