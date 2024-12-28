//
//  Created by Joël Gähwiler on 18.01.17.
//  Copyright © 2017 Joël Gähwiler. All rights reserved.
//

import Cocoa
import NAOSKit

// TODO: Add multiple menus per device type.

class DeviceManager: NSObject, NAOSBLEManagerDelegate {
	@IBOutlet private var devicesMenuItem: NSMenuItem!
	@IBOutlet private var devicesMenu: NSMenu!

	private var manager: NAOSBLEManager!
	private var devices: [NAOSManagedDevice: NSMenuItem] = [:]
	private var controllers: [NAOSManagedDevice: SettingsWindowController] = [:]

	static var shared: DeviceManager!

	override init() {
		// call superclass
		super.init()

		// hide dock icon
		NSApp.setActivationPolicy(.accessory)

		// create BLE manager
		manager = NAOSBLEManager(delegate: self)

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
					c.window!.title = d.title()
				}
			}
		}

		// set shared instance
		DeviceManager.shared = self
	}

	func openDevice(device: NAOSManagedDevice) {
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

	func closeDevice(device: NAOSManagedDevice) {
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
		let device = menuItem.representedObject as! NAOSManagedDevice

		// open device
		openDevice(device: device)
	}

	@IBAction func reset(_: AnyObject) {
		// reset manager
		manager.reset()
	}

	// NAOSManagerDelegate

	func naosManagerDidDiscoverDevice(manager _: NAOSBLEManager, device: NAOSManagedDevice) {
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

	func naosManagerDidReset(manager _: NAOSBLEManager) {
		// close all devices
		for (device, _) in controllers {
			closeDevice(device: device)
		}

		// remove all controllers
		controllers.removeAll()

		// remove all devices
		devices.removeAll()

		// remove all items
		devicesMenu.removeAllItems()

		// update menu item title
		devicesMenuItem.title = "0 Devices"
	}
}
