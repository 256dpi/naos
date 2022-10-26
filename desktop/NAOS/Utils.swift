//
//  Utils.swift
//  NAOS
//
//  Created by Joël Gähwiler on 26.10.22.
//  Copyright © 2022 Joël Gähwiler. All rights reserved.
//

import Foundation

public struct TimedOutError: Error, Equatable {}

public func withTimeout<R>(seconds: TimeInterval, operation: @escaping @Sendable () async throws -> R) async throws -> R {
	return try await withThrowingTaskGroup(of: R.self) { group in
		let deadline = Date(timeIntervalSinceNow: seconds)

		// start work
		group.addTask {
			return try await operation()
		}
		
		// start reaper
		group.addTask {
			let interval = deadline.timeIntervalSinceNow
			if interval > 0 {
				try await Task.sleep(nanoseconds: UInt64(interval * 1_000_000_000))
			}
			try Task.checkCancellation()
			print("timed out")
			throw TimedOutError()
		}
		
		// get first result, cancel the other
		let result = try await group.next()!
		group.cancelAll()
		
		return result
	}
}
