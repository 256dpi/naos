//
//  Created by Joël Gähwiler on 05.04.17.
//  Copyright © 2017 Joël Gähwiler. All rights reserved.
//

import Foundation

public struct TimedOutError: LocalizedError, Equatable {
	public var errorDescription: String? {
		return "Operation timed out."
	}
}

public func withTimeout<R>(
	seconds: TimeInterval, operation: @escaping @Sendable () async throws -> R
) async throws -> R {
	return try await withThrowingTaskGroup(of: R.self) { group in
		let deadline = Date(timeIntervalSinceNow: seconds)

		// start work
		group.addTask {
			try await operation()
		}

		// start reaper
		group.addTask {
			let interval = deadline.timeIntervalSinceNow
			if interval > 0 {
				try await Task.sleep(nanoseconds: UInt64(interval * 1_000_000_000))
			}
			try Task.checkCancellation()
			throw TimedOutError()
		}

		// get first result
		let result = await group.nextResult()!

		// cancel other tasks
		group.cancelAll()

		// get value
		let value = try result.get()

		return value
	}
}
