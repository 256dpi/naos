//
//  Created by Joël Gähwiler on 20.08.23.
//  Copyright © 2017 Joël Gähwiler. All rights reserved.
//

import Combine
import Foundation
import Network

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
		// create task
		let urlSession = URLSession(configuration: .default)
		guard let url = URL(string: "ws://" + host) else {
			throw URLError(.badURL)
		}
		let webSocketTask = urlSession.webSocketTask(with: url)
		webSocketTask.resume()
		return NAOSChannel(transport: httpTransport(task: webSocketTask), device: self, width: 10)
	}
}

final class httpTransport: NAOSTransport {
	private let task: URLSessionWebSocketTask

	init(task: URLSessionWebSocketTask) {
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
	}
}
