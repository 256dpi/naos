//
//  Created by Joël Gähwiler on 18.01.17.
//  Copyright © 2017 Joël Gähwiler. All rights reserved.
//

import Cocoa
import NAOSKit

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

		// connect
		connect()
	}

	func connect() {
		// connect to device
		Task {
			do {
				// perform connect
				try await device.connect()

				// refresh device
				try await device.refresh()
			} catch {
				showError(error: error)
				return
			}

			// check if locked
			if device.locked {
				// show unlock view
				unlockViewController =
					NSStoryboard(name: "Main", bundle: nil)
					.instantiateController(
						withIdentifier: "UnlockViewController")
					as? UnlockViewController
				unlockViewController!.device = device
				unlockViewController!.swc = self
				contentViewController = unlockViewController
			} else {
				// show settings view
				settingsViewController =
					NSStoryboard(name: "Main", bundle: nil)
					.instantiateController(
						withIdentifier: "SettingsViewController")
					as? SettingsViewController
				settingsViewController!.device = device
				contentViewController = settingsViewController

				// trigger refresh
				settingsViewController?.refresh(self)
			}
		}
	}

	func didUnlock() {
		// show settings view
		settingsViewController =
			NSStoryboard(name: "Main", bundle: nil).instantiateController(
				withIdentifier: "SettingsViewController") as? SettingsViewController
		settingsViewController!.device = device
		contentViewController = settingsViewController

		// remove old controller
		unlockViewController = nil

		// trigger refresh
		settingsViewController!.refresh(self)
	}

	// NAOSDeviceDelegate

	func naosDeviceDidUpdate(device _: NAOSDevice, parameter: NAOSParameter) {
		// forward parameter update
		if let svc = settingsViewController {
			svc.didUpdateParameter(parameter: parameter)
		}
	}

	func naosDeviceDidDisconnect(device _: NAOSDevice, error _: Error) {
		// show connecting view
		contentViewController = connectingViewController
		settingsViewController = nil

		// reconnect
		connect()
	}

	// NSWindowDelegate

	func windowShouldClose(_: NSWindow) -> Bool {
		// let manager close window
		manager.close(self)

		// disconnect device
		Task {
			do {
				try await device.disconnect()
			} catch {
				showError(error: error)
			}
		}

		return false
	}
}
