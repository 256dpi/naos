//
//  Created by Joël Gähwiler on 20.08.23.
//  Copyright © 2023 Joël Gähwiler. All rights reserved.
//

import Foundation
import Semaphore

public struct NAOSParamInfo {
	public var ref: UInt8
	public var type: NAOSType
	public var mode: NAOSMode
	public var name: String
}

public struct NAOSParamUpdate {
	public var ref: UInt8
	public var age: UInt64
	public var value: Data
}

/// The NAOS paramter endpoint.
public class NAOSParamsEndpoint {
	private let session: NAOSSession
	private let mutex = AsyncSemaphore(value: 1)
	
	public var timeout: TimeInterval = 5
	
	public init(session: NAOSSession) {
		self.session = session
	}
	
	/// Get a parameter value by name.
	public func get(name: String) async throws -> Data {
		// acquire mutex
		await mutex.wait()
		defer { mutex.signal() }
		
		// prepare command
		var cmd = Data([0])
		cmd.append(name.data(using: .utf8)!)
		
		// write command
		try await session.send(endpoint: 0x1, data: cmd, ackTimeout: 0)
		
		// receive value
		return try await session.receive(endpoint: 0x1, expectAck: false, timeout: timeout)!
	}
	
	/// Set a parameter value by name.
	public func set(name: String, value: Data) async throws {
		// acquire mutex
		await mutex.wait()
		defer { mutex.signal() }
		
		// prepare command
		var cmd = Data([1])
		cmd.append(name.data(using: .utf8)!)
		cmd.append(Data([0]))
		cmd.append(value)
		
		// write command
		try await session.send(endpoint: 0x1, data: cmd, ackTimeout: timeout)
	}
	
	/// Obtain a list of all known parameters.
	public func list() async throws -> [NAOSParamInfo] {
		// acquire mutex
		await mutex.wait()
		defer { mutex.signal() }
		
		// send command
		try await session.send(endpoint: 0x1, data: Data([2]), ackTimeout: 0)
		
		// prepare list
		var list = [NAOSParamInfo]()
		
		while true {
			// receive reply or return list on ack
			guard let reply = try await session.receive(endpoint: 0x1, expectAck: true, timeout: timeout) else {
				return list
			}
			
			// verify reply
			if reply.count < 4 {
				throw NAOSSessionError.invalidMessage
			}
			
			// parse reply
			let ref = reply[0]
			let type = NAOSType(rawValue: reply[1])!
			let mode = NAOSMode(rawValue: reply[2])
			let name = String(data: Data(reply[3...]), encoding: .utf8)!
			
			// append info
			list.append(NAOSParamInfo(ref: ref, type: type, mode: mode, name: name))
		}
	}
	
	/// Read a parameter by reference.
	public func read(ref: UInt8) async throws -> Data {
		// acquire mutex
		await mutex.wait()
		defer { mutex.signal() }
		
		// prepare command
		let cmd = Data([3, ref])
		
		// write command
		try await session.send(endpoint: 0x1, data: cmd, ackTimeout: 0)
		
		// receive value
		return try await session.receive(endpoint: 0x1, expectAck: false, timeout: timeout)!
	}
	
	/// Write a parameter by reference.
	public func write(ref: UInt8, value: Data) async throws {
		// acquire mutex
		await mutex.wait()
		defer { mutex.signal() }
		
		// prepare command
		var cmd = Data([4, ref])
		cmd.append(value)
		
		// write command
		try await session.send(endpoint: 0x1, data: cmd, ackTimeout: timeout)
	}
	
	/// Collect parameter values by providing a list of refrence, a since timestamp or both.
	public func collect(refs: [UInt8]?, since: UInt64) async throws -> [NAOSParamUpdate] {
		// acquire mutex
		await mutex.wait()
		defer { mutex.signal() }
		
		// prepare map
		var map = UINT64_MAX
		if refs != nil {
			map = UInt64(0)
			for ref in refs! {
				map |= (1 << ref)
			}
		}
		
		// prepare command
		var cmd = Data([5])
		cmd.append(writeUint64(value: map))
		cmd.append(writeUint64(value: since))
		
		// send command
		try await session.send(endpoint: 0x1, data: cmd, ackTimeout: 0)
		
		// prepare list
		var list = [NAOSParamUpdate]()
		
		while true {
			// receive reply or return list on ack
			guard let reply = try await session.receive(endpoint: 0x1, expectAck: true, timeout: timeout) else {
				return list
			}
			
			// verify reply
			if reply.count < 9 {
				throw NAOSSessionError.invalidMessage
			}
			
			// parse reply
			let ref = reply[0]
			let age = readUint64(data: Data(reply[1 ... 8]))
			let value = Data(reply[9...])
			
			// append info
			list.append(NAOSParamUpdate(ref: ref, age: age, value: value))
		}
	}
}
