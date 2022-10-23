//
//  Created by Joël Gähwiler on 18.01.17.
//  Copyright © 2017 Joël Gähwiler. All rights reserved.
//

import Cocoa
import CoreBluetooth

class SettingsWindowController: NSWindowController, NSWindowDelegate, NAOSDeviceDelegate {
	private var device: NAOSDevice!
	private var manager: BluetoothManager!
	private var connectingViewController: LoadingViewController!
	private var unlockViewController: UnlockViewController?
	private var settingsViewController: SettingsViewController?

	override func windowDidLoad() {
		super.windowDidLoad()

		// set delegates
		window!.delegate = self
	}

	func configure(device: NAOSDevice, manager: BluetoothManager) {
		// save device
		self.device = device
		device.delegate = self

		// save manager
		self.manager = manager

		// grab default connecting view
		connectingViewController = (contentViewController as! LoadingViewController)

		// connect to device
		device.connect()
	}

	// NAOSDeviceDelegate

	func naosDeviceDidConnect(device _: NAOSDevice) {
		// check if locked
		if device.locked {
			// show unlock view
			unlockViewController = NSStoryboard(name: "Main", bundle: nil).instantiateController(withIdentifier: "UnlockViewController") as? UnlockViewController
			contentViewController = unlockViewController

			// set device
			unlockViewController!.setDevice(device: device)

			return
		}

		// show settings view
		settingsViewController = NSStoryboard(name: "Main", bundle: nil).instantiateController(withIdentifier: "SettingsViewController") as? SettingsViewController
		contentViewController = settingsViewController

		// set device
		settingsViewController!.setDevice(device: device)
	}

	func naosDeviceDidError(device _: NAOSDevice, error: Error) {
		// show error
		let alert = NSAlert()
		alert.messageText = error.localizedDescription
		alert.runModal()

		// let manager close window
		manager.close(self)
	}

	func naosDeviceDidUpdateConnectionStatus(device _: NAOSDevice) {
		if let wc = settingsViewController {
			wc.didUpdateConnectionStatus()
		}
	}

	func naosDeviceDidUnlock(device: NAOSDevice) {
		// return if unlock screen is not active
		if unlockViewController == nil {
			return
		}

		// show settings view
		settingsViewController = NSStoryboard(name: "Main", bundle: nil).instantiateController(withIdentifier: "SettingsViewController") as? SettingsViewController
		contentViewController = settingsViewController

		// set device
		settingsViewController!.setDevice(device: device)

		// remove old controller
		unlockViewController = nil
	}

	func naosDeviceDidRefresh(device _: NAOSDevice) {
		if let wc = settingsViewController {
			wc.didRefresh()
		}
	}

	func naosDeviceDidDisconnect(device _: NAOSDevice, error: Error?) {
		// show connecting view
		contentViewController = connectingViewController
		settingsViewController = nil

		// reconnect on error
		if error != nil {
			device.connect()
		}
	}

	// NSWindowDelegate

	func windowShouldClose(_: NSWindow) -> Bool {
		// disconnect device
		device.disconnect()

		// let manager close window
		manager.close(self)

		return false
	}
}
