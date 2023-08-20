//
//  Created by Joël Gähwiler on 05.04.17.
//  Copyright © 2017 Joël Gähwiler. All rights reserved.
//

import AsyncBluetooth
import Combine
import CoreBluetooth

/// The delegate protocol to be implemented to handle NAOSManager events.
public protocol NAOSManagerDelegate {
	/// The manager discovered a new device.
	func naosManagerDidDiscoverDevice(manager: NAOSManager, device: NAOSDevice)

	/// The settings of a device have been updated because of a read(), write() or refresh() call on a device.
	func naosManagerDidUpdateDevice(manager: NAOSManager, device: NAOSDevice)

	/// The manager did reset either because of a Bluetooth availability change or a manual reset().
	func naosManagerDidReset(manager: NAOSManager)
}

/// The main class that handles NAOS device discovery and handling.
public class NAOSManager: NSObject {
	internal var delegate: NAOSManagerDelegate?
	internal var centralManager: CentralManager!
	private var devices: [NAOSDevice]
	private var subscription: AnyCancellable?
	private var queue = DispatchQueue(label: "devices", attributes: .concurrent)

	/// Initializes the manager and sets the specified class as the delegate.
	public init(delegate: NAOSManagerDelegate?) {
		// set delegate
		self.delegate = delegate

		// initialize devices
		devices = []

		// finish init
		super.init()

		// create central manager
		centralManager = CentralManager()

		// subscribe events
		subscription = centralManager.eventPublisher.sink { event in
			switch event {
			case .didUpdateState(let state):
				switch state {
				case .poweredOn:
					// start scan
					self.scan()
				case .poweredOff:
					// stop scan
					Task {
						await self.centralManager.stopScan()
					}

					// clear devices
					self.queue.sync {
						self.devices.removeAll()
					}

					// call callback if present
					if let d = self.delegate {
						DispatchQueue.main.async {
							d.naosManagerDidReset(manager: self)
						}
					}
				default:
					break
				}
			case .didDisconnectPeripheral(let peripheral, let error):
				// forward disconnect error to device
				if error != nil {
					if let device = self.findDevice(peripheral: peripheral) {
						Task {
							await device.didDisconnect(error: error!)
						}
					}
				}
			default:
				break
			}
		}
	}

	/// Reset discovered devices.
	public func reset() {
		// clear devices
		queue.sync {
			devices.removeAll()
		}

		// call callback if present
		if let d = delegate {
			DispatchQueue.main.async {
				d.naosManagerDidReset(manager: self)
			}
		}
	}

	private func scan() {
		Task {
			// run until cancelled
			while !Task.isCancelled {
				// wait until ready
				try await centralManager.waitUntilReady()

				// create scan stream
				let stream = try await centralManager.scanForPeripherals(
					withServices: [NAOSService])

				// handle discovered peripherals
				for await scanData in stream {
					// skip if device exists already
					if findDevice(peripheral: scanData.peripheral) != nil {
						continue
					}

					// prepare peripheral
					let peripheral = NAOSPeripheral(man: centralManager, raw: scanData.peripheral)

					// otherwise, create new device
					let device = NAOSDevice(peripheral: peripheral, manager: self)

					// add device
					self.queue.sync {
						devices.append(device)
					}

					// call callback if present
					if let d = delegate {
						DispatchQueue.main.async {
							d.naosManagerDidDiscoverDevice(manager: self, device: device)
						}
					}
				}
			}
		}
	}

	// NAOSDevice

	internal func didUpdateDevice(device: NAOSDevice) {
		// call callback if available
		if let d = delegate {
			DispatchQueue.main.async {
				d.naosManagerDidUpdateDevice(manager: self, device: device)
			}
		}
	}

	// Helpers

	private func findDevice(peripheral: Peripheral) -> NAOSDevice? {
		// copy list
		var list: [NAOSDevice]?
		queue.sync {
			list = devices
		}

		// find device
		for device in list! {
			if device.peripheral.identifier() == peripheral.identifier {
				return device
			}
		}

		return nil
	}
}
