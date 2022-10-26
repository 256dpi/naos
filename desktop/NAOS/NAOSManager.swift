//
//  Created by Joël Gähwiler on 05.04.17.
//  Copyright © 2017 Joël Gähwiler. All rights reserved.
//

import AsyncBluetooth
import Cocoa
import Combine
import CoreBluetooth

public protocol NAOSManagerDelegate {
	func naosManagerDidPrepareDevice(manager: NAOSManager, device: NAOSDevice)
	func naosManagerDidFailToPrepareDevice(manager: NAOSManager, error: Error)
	func naosManagerDidRefreshDevice(manager: NAOSManager, device: NAOSDevice)
	func naosManagerDidReset(manager: NAOSManager)
}

public class NAOSManager: NSObject {
	public var delegate: NAOSManagerDelegate?
	internal var centralManager: CentralManager!
	private var allDevices: [NAOSDevice]
	private var availableDevices: [NAOSDevice]
	private var subscription: AnyCancellable?

	public init(delegate: NAOSManagerDelegate?) {
		// set delegate
		self.delegate = delegate

		// initialize arrays
		allDevices = []
		availableDevices = []

		// call superclass
		super.init()

		// central manager
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
							device.didDisconnect(error: error!)
						}
					}
				}
			default:
				break
			}
		}
	}
	
	func scan() {
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
