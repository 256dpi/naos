//
//  Created by Joël Gähwiler on 17.01.17.
//  Copyright © 2017 Joël Gähwiler. All rights reserved.
//

import Cocoa

extension Notification.Name {
	static let open = Notification.Name("open")
	static let close = Notification.Name("close")
}

@NSApplicationMain
class AppDelegate: NSObject, NSApplicationDelegate {
	@IBOutlet var statusItemMenu: NSMenu!
	var statusBarItem: NSStatusItem!

	private var openWindows = 0

	func applicationDidFinishLaunching(_: Notification) {
		// setup status bar item
		statusBarItem = NSStatusBar.system.statusItem(withLength: NSStatusItem.squareLength)
		statusBarItem.menu = statusItemMenu

		// add icon
		let icon = NSImage(named: "StatusIcon")
		icon?.isTemplate = true
		statusBarItem.button!.image = icon

		// hide dock icon
		NSApp.setActivationPolicy(.accessory)

		// register observers
		NotificationCenter.default.addObserver(
			self, selector: #selector(AppDelegate.open), name: .open, object: nil)
		NotificationCenter.default.addObserver(
			self, selector: #selector(AppDelegate.close), name: .close, object: nil)
	}

	@objc func open() {
		// increment counter
		openWindows += 1

		// show dock icon on first window
		if openWindows == 1 {
			NSApp.setActivationPolicy(.regular)
			NSApp.activate(ignoringOtherApps: true)
			NSApp.windows[0].orderFrontRegardless()
			NSApp.windows[0].makeKey()
		}
	}

	@objc func close() {
		// decrement counter
		openWindows -= 1

		// hide dock icon on last window
		if openWindows == 0 {
			NSApp.setActivationPolicy(.accessory)
		}
	}

	@IBAction func quit(_: AnyObject) {
		// terminate application
		NSApplication.shared.terminate(self)
	}

	func applicationWillTerminate(_: Notification) {}
}
