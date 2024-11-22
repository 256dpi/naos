//
//  Created by Joël Gähwiler on 30.08.23.
//  Copyright © 2023 Joël Gähwiler. All rights reserved.
//

import Foundation

/// The update endpoint number.
public let NAOSUpdateEndpoint: UInt8 = 0x02

public class NAOSUpdate {
	/// Perform a firmware update.
	public static func run(session: NAOSSession, image: Data, report: ((Int) -> Void)?, timeout: TimeInterval = 30) async throws {
		// prepare "begin" command
		var cmd = Data([0])
		cmd.append(writeUint32(value: UInt32(image.count)))

		// write "begin" command
		try await session.send(endpoint: NAOSUpdateEndpoint, data: cmd, ackTimeout: 0)

		// receive value
		var reply = try await session.receive(endpoint: NAOSUpdateEndpoint, expectAck: false, timeout: timeout)!

		// verify reply
		if reply.count != 1 || reply[0] != 0 {
			throw NAOSSessionError.invalidMessage
		}

		// TODO: Dynamically determine channel MTU?

		// write data in 500-byte chunks
		var num = 0
		var offset = 0
		while offset < image.count {
			// determine chunk size and chunk data
			let chunkSize = min(500, image.count - offset)
			let chunkData = image.subdata(in: offset ..< offset + chunkSize)

			// determine acked
			let acked = num % 10 == 0

			// prepare "write" command
			cmd = Data([1, acked ? 1 : 0])
			cmd.append(chunkData)

			// send "write" command
			try await session.send(endpoint: NAOSUpdateEndpoint, data: cmd, ackTimeout: acked ? timeout : 0)

			// increment offset
			offset += chunkSize

			// report offset
			if report != nil {
				report!(offset)
			}

			// increment count
			num += 1
		}

		// prepare "finish" command
		cmd = Data([3])

		// write "finish" command
		try await session.send(endpoint: NAOSUpdateEndpoint, data: cmd, ackTimeout: 0)

		// receive value
		reply = try await session.receive(endpoint: NAOSUpdateEndpoint, expectAck: false, timeout: timeout)!

		// verify reply
		if reply.count != 1 || reply[0] != 1 {
			throw NAOSSessionError.invalidMessage
		}
	}
}
