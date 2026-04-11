//
//  Created by Joël Gähwiler on 18.01.17.
//  Copyright © 2017 Joël Gähwiler. All rights reserved.
//

import Cocoa
import NAOSKit

class SettingsWindowController: NSWindowController, NSWindowDelegate, DesktopDeviceDelegate {
	private var device: DesktopDevice!
	private var lvc: LoadingViewController!
	private var uvc: UnlockViewController?
	private var svc: SettingsViewController?
	private var eventsTask: Task<Void, Never>?

	override func windowDidLoad() {
		super.windowDidLoad()

		// set delegates
		window!.delegate = self
	}

	func configure(device: DesktopDevice) {
		// save device
		self.device = device
		device.delegate = self

		// grab connecting view controller
		lvc = (contentViewController as! LoadingViewController)

		// set cancel handler
		lvc.onCancel {
			self.close()
		}

		// watch for lifecycle events
		eventsTask = Task { @MainActor in
			for await event in device.events() {
				switch event {
				case .disconnected:
					// show connecting view
					contentViewController = lvc
					svc = nil
					// reconnect silently
					reconnect()
				case .connected:
					break
				}
			}
		}

		// activate
		connect()
	}

	func reconnect() {
		Task {
			// retry until connected
			while !Task.isCancelled {
				do {
					try await device.activate()
					break
				} catch {
					try? await Task.sleep(for: .seconds(1))
				}
			}

			// show device
			showDevice()
		}
	}

	func connect() {
		// activate device
		Task {
			// perform activate
			do {
				try await device.activate()
			} catch {
				showError(error: error)
				return
			}

			// show device
			showDevice()
		}
	}

	private func showDevice() {
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

	// DesktopDeviceDelegate

	func naosDeviceDidUpdate(device _: DesktopDevice, parameter: Parameter) {
		// forward parameter update
		if let svc = svc {
			svc.didUpdateParameter(parameter: parameter)
		}
	}

	// NSWindowDelegate

	func windowShouldClose(_: NSWindow) -> Bool {
		// let manager close window
		DeviceManager.shared.closeDevice(device: device)

		// cancel events task
		eventsTask?.cancel()
		eventsTask = nil

		// deactivate device
		Task {
			do {
				try await device.deactivate()
			} catch {
				showError(error: error)
			}
		}

		return false
	}
}
