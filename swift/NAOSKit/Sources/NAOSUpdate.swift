//
//  Created by Joël Gähwiler on 30.08.23.
//  Copyright © 2023 Joël Gähwiler. All rights reserved.
//

import Foundation

/// The NAOS update endpoint.
public class NAOSUpdate {
	/// The endpoint number
	public static let endpoint: UInt8 = 0x2
	
	/// Perform a firmware update.
	public static func run(session: NAOSSession, image: Data, report: ((Int) -> Void)?, timeout: TimeInterval = 30) async throws {
		// send "begin" command
		var cmd = pack(fmt: "oi", args: [UInt8(0), UInt32(image.count)])
		try await session.send(endpoint: self.endpoint, data: cmd, ackTimeout: 0)

		// receive reply
		var reply = try await session.receive(endpoint: self.endpoint, expectAck: false, timeout: timeout)!

		// verify reply
		if reply.count != 1 || reply[0] != 0 {
			throw NAOSSessionError.invalidMessage
		}
		
		// determine MTU
		let mtu = Int(try await session.getMTU() - 2)

		// write data in chunks
		var num = 0
		var offset = 0
		while offset < image.count {
			// determine chunk size and chunk data
			let chunkSize = min(mtu, image.count - offset)
			let chunkData = image.subdata(in: offset ..< offset + chunkSize)

			// determine acked
			let acked = num % 10 == 0

			// send "write" command
			cmd = pack(fmt: "oob", args: [UInt8(1), UInt8(acked ? 1 : 0), chunkData])
			try await session.send(endpoint: self.endpoint, data: cmd, ackTimeout: acked ? timeout : 0)

			// increment offset
			offset += chunkSize

			// report offset
			if report != nil {
				report!(offset)
			}

			// increment count
			num += 1
		}

		// send "finish" command
		cmd = pack(fmt: "o", args: [UInt8(3)])
		try await session.send(endpoint: self.endpoint, data: cmd, ackTimeout: 0)

		// receive reply
		reply = try await session.receive(endpoint: self.endpoint, expectAck: false, timeout: timeout)!

		// verify reply
		if reply.count != 1 || reply[0] != 1 {
			throw NAOSSessionError.invalidMessage
		}
	}
}
