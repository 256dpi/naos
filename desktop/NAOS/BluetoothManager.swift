//
//  Created by Joël Gähwiler on 18.01.17.
//  Copyright © 2017 Joël Gähwiler. All rights reserved.
//

import Cocoa

// TODO: Add multiple menus per device type.

class BluetoothManager: NSObject, NAOSManagerDelegate {
	@IBOutlet private var devicesMenuItem: NSMenuItem!
	@IBOutlet private var devicesMenu: NSMenu!

	private var manager: NAOSManager!
	private var devices: [NAOSDevice: NSMenuItem] = [:]
	private var controllers: [NAOSDevice: SettingsWindowController] = [:]

	override init() {
		// call superclass
		super.init()

		// create naos manager
		manager = NAOSManager(delegate: self)
	}

	@objc func open(_ menuItem: NSMenuItem) {
		// get associated device
		let device = menuItem.representedObject as! NAOSDevice

		// check if a controller already exists
		for (d, wc) in controllers {
			if d == device {
				// bring window to front
				wc.window?.orderFrontRegardless()
				return
			}
		}

		// instantiate window controller
		let controller = NSStoryboard(name: "Main", bundle: nil).instantiateController(withIdentifier: "SettingsWindowController") as! SettingsWindowController

		// store and show window
		controllers[device] = controller
		controller.showWindow(self)

		// configure window
		controller.window!.title = device.title()
		controller.window!.orderFrontRegardless()
		controller.window!.makeKey()

		// send notification
		NotificationCenter.default.post(name: .open, object: nil)

		// configure device and manager
		controller.configure(device: device, manager: self)
	}
	
	@IBAction func reset(_: AnyObject) {
		// reset manager
		self.manager.reset();
	}

	// SettingsWindowController

	func close(_ wc: SettingsWindowController) {
		// remove controller
		for (d, c) in controllers {
			if c == wc {
				controllers.removeValue(forKey: d)
			}
		}

		// close window
		wc.close()

		// send notification
		NotificationCenter.default.post(name: .close, object: nil)
	}

	// NAOSManagerDelegate

	func naosManagerDidPrepareDevice(manager _: NAOSManager, device: NAOSDevice) {
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

	func naosManagerDidFailToPrepareDevice(manager _: NAOSManager, error: Error) {
		// show error
		showError(error: error)
	}

	func naosManagerDidRefreshDevice(manager _: NAOSManager, device: NAOSDevice) {
		// update menu item title
		devices[device]?.title = device.title()

		// update eventual window titles
		controllers[device]?.window!.title = device.title()
	}

	func naosManagerDidReset(manager _: NAOSManager) {
		// close all controllers
		for (_, c) in controllers {
			close(c)
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
