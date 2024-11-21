//
//  Created by Joël Gähwiler on 30.08.23.
//  Copyright © 2023 Joël Gähwiler. All rights reserved.
//

import Foundation
import Semaphore

public class NAOSUpdateEndpoint {
	private let session: NAOSSession
	private let mutex = AsyncSemaphore(value: 1)

	public var beginTimeout: TimeInterval = 30
	public var writeTimeout: TimeInterval = 5
	public var finishTimeout: TimeInterval = 10

	public init(session: NAOSSession) {
		self.session = session
	}

	public func run(image: Data, report: ((Int) -> Void)?) async throws {
		// acquire mutex
		await mutex.wait()
		defer { mutex.signal() }

		// prepare "begin" command
		var cmd = Data([0])
		cmd.append(writeUint32(value: UInt32(image.count)))

		// write "begin" command
		try await session.send(endpoint: 0x2, data: cmd, ackTimeout: 0)

		// receive value
		var reply = try await session.receive(
			endpoint: 0x2, expectAck: false, timeout: beginTimeout)!

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
			let chunkData = image.subdata(in: offset..<offset + chunkSize)

			// determine acked
			let acked = num % 10 == 0

			// prepare "write" command
			cmd = Data([1, acked ? 1 : 0])
			cmd.append(chunkData)

			// send "write" command
			try await session.send(
				endpoint: 0x2, data: cmd, ackTimeout: acked ? writeTimeout : 0)

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
		try await session.send(endpoint: 0x2, data: cmd, ackTimeout: 0)

		// receive value
		reply = try await session.receive(
			endpoint: 0x2, expectAck: false, timeout: finishTimeout)!

		// verify reply
		if reply.count != 1 || reply[0] != 1 {
			throw NAOSSessionError.invalidMessage
		}
	}
}
