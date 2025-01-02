//
//  Created by Joël Gähwiler on 20.08.23.
//  Copyright © 2017 Joël Gähwiler. All rights reserved.
//

import Combine
import Foundation
import Network

public func NAOSHTTPDiscover(_ callback: @escaping @Sendable (_ device: NAOSDevice) -> Void) -> Cancellable {
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
					callback(NAOSHTTPDevice(host: addr))
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

class NAOSHTTPDevice: NAOSDevice {
	let host: String

	init(host: String) {
		self.host = host
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
		let webSocketTask = urlSession.webSocketTask(with: URL(string: "ws://" + host)!)

		return await httpChannel.create(task: webSocketTask)
	}
}

class httpChannel: NAOSChannel {
	private var task: URLSessionWebSocketTask
	private var queues: [NAOSQueue] = []

	static func create(task: URLSessionWebSocketTask) async -> httpChannel {
		// resume task
		task.resume()

		// create channel
		let ch = httpChannel(task: task)

		// run forwarder
		Task {
			while true {
				let msg = try await task.receive()
				switch msg {
				case .data(var data):
					// check prefix
					if !data.starts(with: "msg#".data(using: .utf8)!) {
						continue
					}
					
					// unprefix message
					data = data.subdata(in: 4 ..< data.count)
					
					// forward message
					for queue in ch.queues {
						queue.send(value: data)
					}
				case .string(let text):
					print("got text frame", text)
				@unknown default:
					fatalError("unhandled type")
				}
			}
		}

		return ch
	}

	init(task: URLSessionWebSocketTask) {
		// set fields
		self.task = task
	}

	public func subscribe(queue: NAOSQueue) {
		// add queue
		if queues.first(where: { $0 === queue }) == nil {
			queues.append(queue)
		}
	}

	public func unsubscribe(queue: NAOSQueue) {
		// remove queue
		queues.removeAll { $0 === queue }
	}

	public func write(data: Data) async throws {
		// prefix message
		var msg = "msg#".data(using: .utf8)!
		msg.append(data)
		
		// write message
		try await task.send(.data(msg))
	}

	public func getMTU() -> Int {
		return 4096
	}

	public func close() {
		// cancel task
		task.cancel()
	}
}
