//
//  Created by Joël Gähwiler on 20.08.23.
//  Copyright © 2017 Joël Gähwiler. All rights reserved.
//

import Combine
import Foundation
import Network

enum NAOSHTTPError: LocalizedError {
	case missingProtocol
	case invalidProtocol(String)

	var errorDescription: String? {
		switch self {
		case .missingProtocol:
			return "WebSocket did not negotiate the naos subprotocol."
		case .invalidProtocol(let proto):
			return "WebSocket negotiated unexpected subprotocol \(proto)."
		}
	}
}

/// A discovered HTTP device descriptor.
public struct NAOSHTTPDescriptor: Hashable, Sendable {
	public let host: String

	init(host: String) {
		self.host = host
	}

	public func hash(into hasher: inout Hasher) {
		hasher.combine(host)
	}

	public static func == (lhs: NAOSHTTPDescriptor, rhs: NAOSHTTPDescriptor) -> Bool {
		lhs.host == rhs.host
	}
}

public func NAOSHTTPDiscover(_ callback: @escaping @Sendable (_ descriptor: NAOSHTTPDescriptor) -> Void) -> Cancellable {
	// prepare parameters
	let params = NWParameters()
	params.includePeerToPeer = true

	// create browser
	let browser = NWBrowser(for: .bonjour(type: "_naos_http._tcp", domain: nil), using: params)

	// set callback
	browser.browseResultsChangedHandler = { _, changes in
		for change in changes {
			switch change {
			case .added(let result):
				Task {
					let addr = try await resolveService(endpoint: result.endpoint)
					callback(NAOSHTTPDescriptor(host: addr))
				}
			default:
				break
			}
		}
	}

	// run browser
	browser.start(queue: .main)

	return AnyCancellable {
		browser.cancel()
	}
}

public class NAOSHTTPDevice: NAOSDevice {
	private let host: String

	public init(host: String) {
		self.host = host
	}
	
	public func type() -> String {
		return "HTTP"
	}

	public func id() -> String {
		return "http/" + host
	}

	public func name() -> String {
		return "Unnamed"
	}

	public func open() async throws -> NAOSChannel {
		// create session and task
		let delegate = HTTPTransportDelegate()
		let urlSession = URLSession(configuration: .default, delegate: delegate, delegateQueue: nil)
		guard let url = URL(string: "ws://" + host) else {
			throw URLError(.badURL)
		}
		var request = URLRequest(url: url)
		request.setValue("naos", forHTTPHeaderField: "Sec-WebSocket-Protocol")
		let webSocketTask = urlSession.webSocketTask(with: request)
		delegate.prepare(task: webSocketTask)

		// wait until the websocket upgrade completed before returning the channel
		webSocketTask.resume()
		do {
			try await delegate.awaitOpen(timeout: 5)
		} catch {
			webSocketTask.cancel()
			urlSession.invalidateAndCancel()
			throw error
		}

		return NAOSChannel(
			transport: HTTPTransport(session: urlSession, delegate: delegate, task: webSocketTask),
			device: self,
			width: 10
		)
	}
}

final class HTTPTransport: NAOSTransport {
	private let session: URLSession
	private let delegate: HTTPTransportDelegate
	private let task: URLSessionWebSocketTask

	init(session: URLSession, delegate: HTTPTransportDelegate, task: URLSessionWebSocketTask) {
		self.session = session
		self.delegate = delegate
		self.task = task
	}

	func read() async throws -> Data {
		let msg = try await task.receive()
		switch msg {
		case .data(let data):
			return data
		case .string(let text):
			return Data(text.utf8)
		@unknown default:
			throw NAOSTransportError.closed
		}
	}

	func write(data: Data) async throws {
		try await task.send(.data(data))
	}

	func close() {
		task.cancel()
		session.invalidateAndCancel()
	}
}

final class HTTPTransportDelegate: NSObject, URLSessionWebSocketDelegate, URLSessionTaskDelegate {
	private let lock = NSLock()
	private weak var task: URLSessionWebSocketTask?
	private var result: Result<Void, Error>?
	private var continuation: CheckedContinuation<Void, Error>?

	func prepare(task: URLSessionWebSocketTask) {
		lock.withLock {
			self.task = task
			self.result = nil
			self.continuation = nil
		}
	}

	func awaitOpen(timeout: TimeInterval) async throws {
		try await withTimeout(seconds: timeout) {
			try await withCheckedThrowingContinuation { continuation in
				let result = self.lock.withLock { () -> Result<Void, Error>? in
					if let result = self.result {
						return result
					}

					self.continuation = continuation
					return nil
				}

				if let result {
					continuation.resume(with: result)
				}
			}
		}
	}

	func urlSession(
		_ session: URLSession,
		webSocketTask: URLSessionWebSocketTask,
		didOpenWithProtocol protocol: String?
	) {
		if let proto = `protocol`, proto != "naos" {
			complete(task: webSocketTask, result: .failure(NAOSHTTPError.invalidProtocol(proto)))
			webSocketTask.cancel()
			return
		}
		if `protocol` == nil {
			complete(task: webSocketTask, result: .failure(NAOSHTTPError.missingProtocol))
			webSocketTask.cancel()
			return
		}

		complete(task: webSocketTask, result: .success(()))
	}

	func urlSession(
		_ session: URLSession,
		task: URLSessionTask,
		didCompleteWithError error: Error?
	) {
		guard let wsTask = task as? URLSessionWebSocketTask, let error else {
			return
		}

		complete(task: wsTask, result: .failure(error))
	}

	func urlSession(
		_ session: URLSession,
		webSocketTask: URLSessionWebSocketTask,
		didCloseWith closeCode: URLSessionWebSocketTask.CloseCode,
		reason: Data?
	) {
		let error = URLError(.networkConnectionLost)
		complete(task: webSocketTask, result: .failure(error))
	}

	private func complete(task: URLSessionWebSocketTask, result: Result<Void, Error>) {
		let continuation = lock.withLock { () -> CheckedContinuation<Void, Error>? in
			guard self.task === task else {
				return nil
			}
			guard self.result == nil else {
				return nil
			}

			self.result = result
			let continuation = self.continuation
			self.continuation = nil
			return continuation
		}

		continuation?.resume(with: result)
	}
}
