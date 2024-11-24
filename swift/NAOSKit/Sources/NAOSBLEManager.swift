//
//  Created by Joël Gähwiler on 05.04.17.
//  Copyright © 2017 Joël Gähwiler. All rights reserved.
//

import AsyncBluetooth
import Combine
import CoreBluetooth

/// The delegate protocol to be implemented to handle NAOSManager events.
public protocol NAOSBLEManagerDelegate {
	/// The manager discovered a new device.
	func naosManagerDidDiscoverDevice(manager: NAOSBLEManager, device: NAOSManagedDevice)

	/// The manager did reset either because of a Bluetooth availability change or a manual reset().
	func naosManagerDidReset(manager: NAOSBLEManager)
}

/// The main class that handles NAOS device discovery and handling.
public class NAOSBLEManager: NSObject {
	var delegate: NAOSBLEManagerDelegate?
	var centralManager: CentralManager!
	private var devices: [NAOSManagedDevice]
	private var subscription: AnyCancellable?
	private var queue = DispatchQueue(label: "devices", attributes: .concurrent)

	/// Initializes the manager and sets the specified class as the delegate.
	public init(delegate: NAOSBLEManagerDelegate?) {
		// set delegate
		self.delegate = delegate

		// initialize devices
		devices = []

		// finish init
		super.init()

		// create central manager
		centralManager = CentralManager()

		// disable logging
		AsyncBluetoothLogging.setEnabled(false)

		// start scanning
		Task { try await self.start() }
	}

	private func start() async throws {
		// subscribe events
		subscription = await centralManager.eventPublisher.sink(receiveValue: { event in
			switch event {
			case .didUpdateState(let state):
				switch state {
				case .poweredOn:
					// ignore
					break
				case .poweredOff:
					// stop running scan
					Task {
						await self.centralManager.stopScan()
					}
				default:
					break
				}
			case .didDisconnectPeripheral(let peripheral, _, let error):
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
		})

		// scan forever
		Task {
			while true {
				do {
					try await self.scan()
				} catch {
					print("error while scanning: ", error.localizedDescription)
				}
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

	private func scan() async throws {
		// check if powered on
		while centralManager.bluetoothState != .poweredOn {
			try? await Task.sleep(nanoseconds: 1_000_000_000) // 1s
		}

		// wait until ready
		try await centralManager.waitUntilReady()

		// create scan stream
		let stream = try await centralManager.scanForPeripherals(withServices: [bleService])

		// handle discovered peripherals
		for await scanData in stream {
			// skip if device exists already
			if findDevice(peripheral: scanData.peripheral) != nil {
				continue
			}

			// otherwise, create new device
			let bleDevice = NAOSBLEDevice(manager: centralManager, peripheral: scanData.peripheral)
			let device = NAOSManagedDevice(device: bleDevice)

			// add device
			queue.sync {
				devices.append(device)
			}

			// call callback if present
			if let d = delegate {
				DispatchQueue.main.async {
					d.naosManagerDidDiscoverDevice(
						manager: self, device: device)
				}
			}
		}
	}

	// Helpers

	private func findDevice(peripheral: Peripheral) -> NAOSManagedDevice? {
		// copy list
		var list: [NAOSManagedDevice]?
		queue.sync {
			list = devices
		}

		// find device
		for device in list! {
			let bleDevice = device.device as! NAOSBLEDevice
			if bleDevice.peripheral.identifier == peripheral.identifier {
				return device
			}
		}

		return nil
	}
}
