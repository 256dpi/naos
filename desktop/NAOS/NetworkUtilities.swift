//
//  Created by Joël Gähwiler on 12.04.19.
//  Copyright © 2019 Joël Gähwiler. All rights reserved.
//

import Cocoa
import CoreWLAN

class NetworkUtilities {
	static func getWiFiAddress() -> String? {
		// prepare address
		var address: String?

		// get list of all interfaces on the local machine
		var ifaddr: UnsafeMutablePointer<ifaddrs>?
		guard getifaddrs(&ifaddr) == 0 else { return nil }
		guard let firstAddr = ifaddr else { return nil }

		// check each interface
		for ifptr in sequence(first: firstAddr, next: { $0.pointee.ifa_next }) {
			// get interface
			let interface = ifptr.pointee

			// check for ipv4 or ipv6 interface
			let addrFamily = interface.ifa_addr.pointee.sa_family
			if addrFamily == UInt8(AF_INET) || addrFamily == UInt8(AF_INET6) {
				// check interface name
				let name = String(cString: interface.ifa_name)
				if name == "en0" {
					// convert interface address to a human readable string
					var hostname = [CChar](repeating: 0, count: Int(NI_MAXHOST))
					getnameinfo(interface.ifa_addr, socklen_t(interface.ifa_addr.pointee.sa_len),
					            &hostname, socklen_t(hostname.count),
					            nil, socklen_t(0), NI_NUMERICHOST)
					address = String(cString: hostname)
				}
			}
		}

		// free list
		freeifaddrs(ifaddr)

		return address
	}

	static func getSSID() -> String? {
		return CWWiFiClient.shared().interface()?.ssid()
	}
}
