//
//  Created by Joël Gähwiler on 18.01.17.
//  Copyright © 2017 Joël Gähwiler. All rights reserved.
//

import Cocoa
import Combine
import NAOSKit

class DeviceManager: NSObject {
	@IBOutlet private var devicesMenuItem: NSMenuItem!
	@IBOutlet private var devicesMenu: NSMenu!

	private var bleManager: NAOSBLEManager!
	private var httpDiscover: Cancellable!
	private var serialTask: Task<Void, Never>?
	private var serialDevices: [NAOSSerialDescriptor: DesktopDevice] = [:]
	
	private var devices: [DesktopDevice: NSMenuItem] = [:]
	private var controllers: [DesktopDevice: SettingsWindowController] = [:]

	static var shared: DeviceManager!

	override init() {
		// call superclass
		super.init()

		// hide dock icon
		NSApp.setActivationPolicy(.accessory)

		// create BLE manager
		bleManager = NAOSBLEManager(
			onDiscover: { descriptor in
				self.addDevice(device: DesktopDevice(device: NAOSBLEDevice(descriptor: descriptor)))
			},
			onReset: {
				let bleDevices = self.devices.keys.filter { $0.device.type() == "BLE" }
				for device in bleDevices {
					self.removeDevice(device: device)
				}
			}
		)
		
		// run HTTP discovery
		httpDiscover = NAOSHTTPDiscover{ descriptor in
			// add device
			Task { @MainActor in
				self.addDevice(device: DesktopDevice(device: NAOSHTTPDevice(host: descriptor.host)))
			}
		}

		// run serial discovery
		serialTask = Task {
			// prepare state
			var known = Set<NAOSSerialDescriptor>()

			while !Task.isCancelled {
				let ports = Set(NAOSSerialList())

				// handle added ports
				for descriptor in ports.subtracting(known) {
					let device = DesktopDevice(device: NAOSSerialDevice(path: descriptor.path))
					self.serialDevices[descriptor] = device
					await MainActor.run {
						self.addDevice(device: device)
					}
				}

				// handle removed ports
				for descriptor in known.subtracting(ports) {
					if let device = self.serialDevices.removeValue(forKey: descriptor) {
						await MainActor.run {
							self.removeDevice(device: device)
						}
					}
				}

				// update known ports
				known = ports

				// wait two seconds
				try? await Task.sleep(for: .seconds(2))
			}
		}

		// run updater
		Task { @MainActor in
			while true {
				// wait a second
				try await Task.sleep(for: .seconds(1))

				// update menu items
				for (d, i) in devices {
					i.title = d.title()
				}

				// update window titles
				for (d, c) in controllers {
					c.window?.title = d.title()
				}
			}
		}

		// set shared instance
		DeviceManager.shared = self
	}

	deinit {
		serialTask?.cancel()
	}
	
	func addDevice(device: DesktopDevice) {
		// add menu item for new device
		let item = NSMenuItem()
		item.title = device.title()
		item.representedObject = device
		item.target = self
		item.action = #selector(open(_:))
		devicesMenu.addItem(item)

		// save device
		devices[device] = item

		// update menu item title
		if devices.count == 1 {
			devicesMenuItem.title = "1 Device"
		} else {
			devicesMenuItem.title = String(format: "%d Devices", devices.count)
		}
	}

	func removeDevice(device: DesktopDevice) {
		// drop menu item
		if let item = devices.removeValue(forKey: device) {
			devicesMenu.removeItem(item)
		}

		// update menu item title
		if devices.count == 1 {
			devicesMenuItem.title = "1 Device"
		} else {
			devicesMenuItem.title = String(format: "%d Devices", devices.count)
		}

		// close controller if needed
		closeDevice(device: device)
	}

	func openDevice(device: DesktopDevice) {
		Task { @MainActor in
			// check if a controller already exists
			for (d, c) in controllers {
				if d == device {
					// bring window to front
					NSApp.activate(ignoringOtherApps: true)
					c.window!.orderFrontRegardless()
					c.window!.makeKey()
					return
				}
			}

			// create window controller
			let controller = loadVC("SettingsWindowController") as! SettingsWindowController

			// store controller for device
			controllers[device] = controller

			// show and configure window
			controller.showWindow(self)
			controller.window!.title = device.title()

			// show dock icon again on first window
			if controllers.count == 1 {
				NSApp.setActivationPolicy(.regular)
			}

			// bring window to front
			NSApp.activate(ignoringOtherApps: true)
			controller.window!.orderFrontRegardless()
			controller.window!.makeKey()

			// configure device and manager
			controller.configure(device: device)
		}
	}

	func closeDevice(device: DesktopDevice) {
		Task { @MainActor in
			// close window
			for (d, c) in controllers {
				if d == device {
					c.close()
				}
			}

			// remove controller for device
			controllers.removeValue(forKey: device)

			// hide dock icon on last window
			if controllers.count == 0 {
				NSApp.setActivationPolicy(.accessory)
			}
		}
	}

	// UI Actions

	@objc func open(_ menuItem: NSMenuItem) {
		// get associated device
		let device = menuItem.representedObject as! DesktopDevice

		// open device
		openDevice(device: device)
	}

	@IBAction func reset(_: AnyObject) {
		// reset manager
		bleManager.reset()
	}

}
