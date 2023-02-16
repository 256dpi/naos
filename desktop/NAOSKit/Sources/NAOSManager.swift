//
//  Created by Joël Gähwiler on 05.04.17.
//  Copyright © 2017 Joël Gähwiler. All rights reserved.
//

import AsyncBluetooth
import Cocoa
import Combine
import CoreBluetooth

/// The delegate protocol to be implemented to handle NAOSManager events.
public protocol NAOSManagerDelegate {
	/// The manager found a new device an was able discover and load its settings.
	func naosManagerDidPrepareDevice(manager: NAOSManager, device: NAOSDevice)

	/// The manager found a new device but encountered an error when accessing it.
	func naosManagerDidFailToPrepareDevice(manager: NAOSManager, error: Error)

	/// The settings of a device have been updated because of a read(), write() or refresh() call on a device.
	func naosManagerDidRefreshDevice(manager: NAOSManager, device: NAOSDevice)

	/// The manager did reset either because of a Bluetooth availability change or a manual reset().
	func naosManagerDidReset(manager: NAOSManager)
}

/// The main class that handles NAOS device discovery and handling.
public class NAOSManager: NSObject {
	internal var delegate: NAOSManagerDelegate?
	internal var centralManager: CentralManager!
	private var allDevices: [NAOSDevice]
	private var availableDevices: [NAOSDevice]
	private var subscription: AnyCancellable?

	/// Initializes the manager and sets the specified class as the delegate.
	public init(delegate: NAOSManagerDelegate?) {
		// set delegate
		self.delegate = delegate

		// initialize arrays
		self.allDevices = []
		self.availableDevices = []

		// finish init
		super.init()

		// create central manager
		self.centralManager = CentralManager()
		
		// subscribe events
		self.subscription = self.centralManager.eventPublisher.sink { event in
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
					
					// clear all arrays
					self.allDevices.removeAll()
					self.availableDevices.removeAll()
		
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
				if error != nil {
					for device in self.allDevices {
						if device.peripheral.identifier == peripheral.identifier {
							Task {
								await device.didDisconnect(error: error!)
							}
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
		self.allDevices.removeAll()
		self.availableDevices.removeAll()
		
		// call callback if present
		if let d = self.delegate {
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
				let stream = try await centralManager.scanForPeripherals(withServices: [NAOSService])
				
				// handle discovered peripherals
				for await scanData in stream {
					// check device
					var found = false
					for device in allDevices {
						if device.peripheral.identifier == scanData.peripheral.identifier {
							found = true
						}
					}
					if found {
						continue
					}
			
					// create new device
					let device = NAOSDevice(peripheral: scanData.peripheral, manager: self)
			
					// add device
					allDevices.append(device)
					
					Task {
						// prepare device
						do {
							try await device.connect()
							try await device.refresh()
							try await device.disconnect()
						} catch {
							// disconnect
							try? await centralManager.cancelPeripheralConnection(scanData.peripheral)
							
							// handle error
							if let d = delegate {
								DispatchQueue.main.async {
									d.naosManagerDidFailToPrepareDevice(manager: self, error: error)
								}
							}
							
							return
						}
						
						// add available device
						availableDevices.append(device)
						
						// call callback if present
						if let d = delegate {
							DispatchQueue.main.async {
								d.naosManagerDidPrepareDevice(manager: self, device: device)
							}
						}
					}
				}
			}
		}
	}

	// NAOSDevice

	internal func didRefreshDevice(device: NAOSDevice) {
		// call callback if available
		if let d = delegate {
			DispatchQueue.main.async {
				d.naosManagerDidRefreshDevice(manager: self, device: device)
			}
		}
	}
}
