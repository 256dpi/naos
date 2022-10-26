//
//  Created by Joël Gähwiler on 18.01.17.
//  Copyright © 2017 Joël Gähwiler. All rights reserved.
//

import Cocoa

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
		Task {
			// perform connect
			try await device.connect()

			// refresh device
			try await device.refresh()

			// check if locked
			if device.locked {
				// show unlock view
				unlockViewController = NSStoryboard(name: "Main", bundle: nil).instantiateController(withIdentifier: "UnlockViewController") as? UnlockViewController
				unlockViewController!.device = device
				unlockViewController!.swc = self
				contentViewController = unlockViewController
			} else {
				// show settings view
				settingsViewController = NSStoryboard(name: "Main", bundle: nil).instantiateController(withIdentifier: "SettingsViewController") as? SettingsViewController
				settingsViewController!.device = device
				contentViewController = settingsViewController

				// trigger refresh
				settingsViewController?.refresh(self)
			}
		}
	}

	func didUnlock() {
		// show settings view
		settingsViewController = NSStoryboard(name: "Main", bundle: nil).instantiateController(withIdentifier: "SettingsViewController") as? SettingsViewController
		settingsViewController!.device = device
		contentViewController = settingsViewController

		// remove old controller
		unlockViewController = nil

		// trigger refresh
		settingsViewController!.refresh(self)
	}

	// NAOSDeviceDelegate

//	func naosDeviceDidError(device _: NAOSDevice, error: Error) {
//		// show error
//		let alert = NSAlert()
//		alert.messageText = error.localizedDescription
//		alert.runModal()
//
//		// let manager close window
//		manager.close(self)
//	}

	func naosDeviceDidUpdate(device _: NAOSDevice, parameter: NAOSParameter) {
		// forward parameter update
		if let wc = settingsViewController {
			wc.didUpdateParameter(parameter: parameter)
		}
	}

	func naosDeviceDidDisconnect(device _: NAOSDevice, error: Error?) {
		// show connecting view
		contentViewController = connectingViewController
		settingsViewController = nil

		// reconnect on error
		if error != nil {
			Task {
				try await device.connect()
			}
		}
	}

	// NSWindowDelegate

	func windowShouldClose(_: NSWindow) -> Bool {
		// disconnect device
		Task {
			try await device.disconnect()
		}

		// let manager close window
		manager.close(self)

		return false
	}
}
