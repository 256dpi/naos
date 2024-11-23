//
//  Created by Joël Gähwiler on 05.04.17.
//  Copyright © 2017 Joël Gähwiler. All rights reserved.
//

import Foundation
import Semaphore

struct TimedOutError: LocalizedError, Equatable {
	public var errorDescription: String? {
		return "Operation timed out."
	}
}

func withTimeout<R>(
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

func readUint16(data: Data) -> UInt16 {
	return UInt16(data[0]) | (UInt16(data[1]) << 8)
}

func readUint32(data: Data) -> UInt32 {
	return UInt32(data[0]) | (UInt32(data[1]) << 8) | (UInt32(data[2]) << 16)
		| (UInt32(data[3]) << 24)
}

func readUint64(data: Data) -> UInt64 {
	let byte0 = UInt64(data[0])
	let byte1 = UInt64(data[1]) << 8
	let byte2 = UInt64(data[2]) << 16
	let byte3 = UInt64(data[3]) << 24
	let byte4 = UInt64(data[4]) << 32
	let byte5 = UInt64(data[5]) << 40
	let byte6 = UInt64(data[6]) << 48
	let byte7 = UInt64(data[7]) << 56

	return byte0 | byte1 | byte2 | byte3 | byte4 | byte5 | byte6 | byte7
}

func writeUint16(value: UInt16) -> Data {
	return Data([
		UInt8(value & 0xFF),
		UInt8((value >> 8) & 0xFF),
	])
}

func writeUint32(value: UInt32) -> Data {
	return Data([
		UInt8(value & 0xFF),
		UInt8((value >> 8) & 0xFF),
		UInt8((value >> 16) & 0xFF),
		UInt8((value >> 24) & 0xFF),
	])
}

func writeUint64(value: UInt64) -> Data {
	return Data([
		UInt8(value & 0xFF),
		UInt8((value >> 8) & 0xFF),
		UInt8((value >> 16) & 0xFF),
		UInt8((value >> 24) & 0xFF),
		UInt8((value >> 32) & 0xFF),
		UInt8((value >> 40) & 0xFF),
		UInt8((value >> 48) & 0xFF),
		UInt8((value >> 56) & 0xFF),
	])
}

func randomString(length: Int) -> String {
	let letters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	return String((0..<length).map { _ in letters.randomElement()! })
}

class Channel<T> {
	private var queue = DispatchQueue(label: "Channel")
	private var buffer: [T] = []
	private var semaphore = AsyncSemaphore(value: 0)

	func send(value: T) {
		queue.sync {
			self.buffer.append(value)
			semaphore.signal()
		}
	}

	func receive(timeout: TimeInterval) async throws -> T {
		return try await withTimeout(seconds: timeout) {
			try await self.semaphore.waitUnlessCancelled()
			return self.queue.sync {
				self.buffer.removeFirst()
			}
		}
	}
}

func pack(fmt: String, args: [Any]) -> Data {
	var buffer = Data()

	for (index, code) in fmt.enumerated() {
		switch code {
		case "s":
			let str = args[index] as! String
			buffer.append(Data(str.utf8))
		case "b":
			let bytes = args[index] as! Data
			buffer.append(bytes)
		case "o":
			let byte = args[index] as! UInt8
			buffer.append(byte)
		case "h":
			let value = args[index] as! UInt16
			buffer.append(writeUint16(value: value))
		case "i":
			let value = args[index] as! UInt32
			buffer.append(writeUint32(value: value))
		case "q":
			let value = args[index] as! UInt64
			buffer.append(writeUint64(value: value))
		default:
			fatalError("Invalid format code: \(code)")
		}
	}

	return buffer
}

func unpack(format: String, buffer: Data, start: Int = 0) -> [Any] {
	var offset = start
	var results: [Any] = []

	for code in format {
		switch code {
		case "s":
			if let end = buffer[offset...].firstIndex(of: 0) {
				let stringData = buffer[offset..<end]
				let value = String(data: stringData, encoding: .utf8) ?? ""
				results.append(value)
				offset = end + 1
			} else {
				let stringData = buffer[offset...]
				let value = String(data: stringData, encoding: .utf8) ?? ""
				results.append(value)
				offset += stringData.count
			}
		case "b":
			let value = Data(buffer[offset...])
			results.append(value)
			offset += value.count
		case "o":
			results.append(buffer[offset])
			offset += 1
		case "h":
			let value = readUint16(data: Data(buffer[offset..<offset + 2]))
			results.append(value)
			offset += 2
		case "i":
			let value = readUint32(data: Data(buffer[offset..<offset + 4]))
			results.append(value)
			offset += 4
		case "q":
			let value = readUint64(data: Data(buffer[offset..<offset + 8]))
			results.append(value)
			offset += 8
		default:
			fatalError("Invalid format code: \(code)")
		}
	}

	return results
}
