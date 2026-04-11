import Foundation
import XCTest
@testable import NAOSKit

private actor MockTransportState {
	var writes: [Data] = []
	var onWrite: (@Sendable (Data) async -> Void)?

	func appendWrite(_ data: Data) {
		writes.append(data)
	}

	func currentOnWrite() -> (@Sendable (Data) async -> Void)? {
		onWrite
	}

	func setOnWrite(_ onWrite: @escaping @Sendable (Data) async -> Void) {
		self.onWrite = onWrite
	}
}

private final class MockTransport: NAOSTransport {
	private let reads = Channel<Data>()
	private let state = MockTransportState()

	func read() async throws -> Data {
		try await reads.receive(timeout: 1)
	}

	func write(data: Data) async throws {
		await state.appendWrite(data)
		if let onWrite = await state.currentOnWrite() {
			await onWrite(data)
		}
	}

	func close() {}

	func push(_ data: Data) {
		reads.send(value: data)
	}

	func setOnWrite(_ onWrite: @escaping @Sendable (Data) async -> Void) async {
		await state.setOnWrite(onWrite)
	}
}

final class DefaultNAOSChannelTests: XCTestCase {
	func testRoutesOwnedSessionTraffic() async throws {
		let transport = MockTransport()
		let channel = NAOSChannel(transport: transport, width: 10)
		let queue = NAOSQueue()
		channel.subscribe(queue: queue)

		let handle = Data("open-owned".utf8)
		try await channel.write(queue: queue, msg: NAOSMessage(session: 0, endpoint: 0x0, data: handle))

		transport.push(NAOSMessage(session: 21, endpoint: 0x0, data: handle).build())
		let openReply = try await queue.receive(timeout: 1)
		XCTAssertEqual(openReply.session, 21)

		transport.push(NAOSMessage(session: 21, endpoint: 0x42, data: Data("payload".utf8)).build())
		let payload = try await queue.receive(timeout: 1)
		XCTAssertEqual(payload.endpoint, 0x42)
		XCTAssertEqual(payload.data, Data("payload".utf8))

		transport.push(NAOSMessage(session: 21, endpoint: 0xFF, data: Data()).build())
		let close = try await queue.receive(timeout: 1)
		XCTAssertEqual(close.endpoint, 0xFF)

		transport.push(NAOSMessage(session: 21, endpoint: 0x42, data: Data("later".utf8)).build())
		do {
			_ = try await queue.receive(timeout: 0.05)
			XCTFail("unexpected routed message after close")
		} catch {
		}
	}

	func testRejectsWrongOwnerWrites() async throws {
		let transport = MockTransport()
		let channel = NAOSChannel(transport: transport, width: 10)
		let owner = NAOSQueue()
		let other = NAOSQueue()
		channel.subscribe(queue: owner)
		channel.subscribe(queue: other)

		let handle = Data("open-owner".utf8)
		try await channel.write(queue: owner, msg: NAOSMessage(session: 0, endpoint: 0x0, data: handle))
		transport.push(NAOSMessage(session: 9, endpoint: 0x0, data: handle).build())
		_ = try await owner.receive(timeout: 1)

		do {
			try await channel.write(
				queue: other,
				msg: NAOSMessage(session: 9, endpoint: 0x42, data: Data("payload".utf8))
			)
			XCTFail("expected wrong-owner error")
		} catch let error as NAOSChannelError {
			XCTAssertEqual(error, .wrongOwner)
		}
	}

	func testRegistersPendingOpenBeforeWriteReturns() async throws {
		let transport = MockTransport()
		let channel = NAOSChannel(transport: transport, width: 10)
		let queue = NAOSQueue()
		channel.subscribe(queue: queue)

		let handle = Data("open-race".utf8)
		await transport.setOnWrite { _ in
			transport.push(NAOSMessage(session: 7, endpoint: 0x0, data: handle).build())
			transport.push(NAOSMessage(session: 7, endpoint: 0x42, data: Data("payload".utf8)).build())
		}

		try await channel.write(queue: queue, msg: NAOSMessage(session: 0, endpoint: 0x0, data: handle))

		let openReply = try await queue.receive(timeout: 1)
		let payload = try await queue.receive(timeout: 1)
		XCTAssertEqual(openReply.session, 7)
		XCTAssertEqual(payload.endpoint, 0x42)
	}
}
