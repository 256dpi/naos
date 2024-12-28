//
//  Created by Joël Gähwiler on 18.01.17.
//  Copyright © 2017 Joël Gähwiler. All rights reserved.
//

import Cocoa
import NAOSKit

class SettingsWindowController: NSWindowController, NSWindowDelegate, NAOSManagedDeviceDelegate {
	private var device: NAOSManagedDevice!
	private var manager: DeviceManager!
	private var lvc: LoadingViewController!
	private var uvc: UnlockViewController?
	private var svc: SettingsViewController?

	override func windowDidLoad() {
		super.windowDidLoad()

		// set delegates
		window!.delegate = self
	}

	func configure(device: NAOSManagedDevice, manager: DeviceManager) {
		// save device
		self.device = device
		device.delegate = self

		// save manager
		self.manager = manager

		// grab connecting view controller
		lvc = (contentViewController as! LoadingViewController)

		// set cancel handler
		lvc.onCancel {
			self.close()
		}

		// connect
		connect()
	}

	func connect() {
		// connect to device
		Task {
			// perform connect
			do {
				try await device.connect()
			} catch {
				showError(error: error)
				return
			}

			// check if locked
			if device.locked {
				// show unlock view
				uvc = loadVC("UnlockViewController") as? UnlockViewController
				uvc!.device = device
				uvc!.swc = self
				contentViewController = uvc
			} else {
				// show settings view
				svc = loadVC("SettingsViewController") as? SettingsViewController
				svc!.device = device
				contentViewController = svc

				// trigger refresh
				svc?.refresh(self)
			}
		}
	}

	func didUnlock() {
		// show settings view
		svc = loadVC("SettingsViewController") as? SettingsViewController
		svc!.device = device
		contentViewController = svc

		// remove old controller
		uvc = nil

		// trigger refresh
		svc!.refresh(self)
	}

	// NAOSManagedDeviceDelegate

	func naosDeviceDidUpdate(device _: NAOSManagedDevice, parameter: NAOSParameter) {
		// forward parameter update
		if let svc = svc {
			svc.didUpdateParameter(parameter: parameter)
		}
	}

	func naosDeviceDidDisconnect(device _: NAOSManagedDevice, error _: Error) {
		// show connecting view
		contentViewController = lvc
		svc = nil

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
