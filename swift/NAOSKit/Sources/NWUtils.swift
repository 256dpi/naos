//
//  Created by Joël Gähwiler on 02.01.25.
//  Copyright © 2023 Joël Gähwiler. All rights reserved.
//

import Foundation
import Network

func stripInterface(from input: String) -> String {
	guard let lastPercentIndex = input.lastIndex(of: "%") else {
		return input // no "%" found, return the original string
	}

	return String(input[..<lastPercentIndex])
}

func resolveService(endpoint: NWEndpoint) async throws -> String {
	// check service type
	guard case .service(let name, let type, let domain, _) = endpoint else {
		throw NSError(domain: "NWConnectionError", code: -1, userInfo: [
			NSLocalizedDescriptionKey: "Invalid NWEndpoint type. Expected .service.",
		])
	}

	return try await withCheckedThrowingContinuation { continuation in
		// create fake UDP connection
		let connection = NWConnection(to: .service(name: name, type: type, domain: domain, interface: nil), using: .udp)

		// handle state changes
		connection.stateUpdateHandler = { state in
			switch state {
			case .ready:
				if let innerEndpoint = connection.currentPath?.remoteEndpoint, case .hostPort(let host, let port) = innerEndpoint {
					let addr = String(format: "%@:%d", arguments: [stripInterface(from: host.debugDescription), Int(port.rawValue)])
					continuation.resume(returning: (addr))
				} else {
					continuation.resume(throwing: NSError(domain: "NWConnectionError", code: -1, userInfo: [
						NSLocalizedDescriptionKey: "Unexpected endpoint type: \(endpoint)",
					]))
				}
				connection.cancel()
			case .failed(let error):
				continuation.resume(throwing: error)
				connection.cancel()
			case .cancelled:
				break
			default:
				break
			}
		}

		// run connection
		connection.start(queue: .main)

		// Timeout handling
		DispatchQueue.main.asyncAfter(deadline: .now() + 5) {
			if connection.state != .ready {
				connection.cancel()
			}
		}
	}
}
