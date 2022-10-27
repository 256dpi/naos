//
//  Utils.swift
//  NAOS
//
//  Created by Joël Gähwiler on 26.10.22.
//  Copyright © 2022 Joël Gähwiler. All rights reserved.
//

import Cocoa

public struct TimedOutError: LocalizedError, Equatable {
	public var errorDescription: String? {
		return "Operation timed out."
	}
}

public func withTimeout<R>(seconds: TimeInterval, operation: @escaping @Sendable () async throws -> R) async throws -> R {
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

		// get first result, cancel the other
		let result = try await group.next()!
		group.cancelAll()

		return result
	}
}

public func showError(error: Error) {
	// show error
	DispatchQueue.main.async {
		let alert = NSAlert()
		alert.messageText = error.localizedDescription
		alert.runModal()
	}
}
