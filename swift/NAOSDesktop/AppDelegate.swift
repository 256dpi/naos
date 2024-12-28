//
//  Created by Joël Gähwiler on 17.01.17.
//  Copyright © 2017 Joël Gähwiler. All rights reserved.
//

import Cocoa

@NSApplicationMain class AppDelegate: NSObject, NSApplicationDelegate {
	@IBOutlet var statusItemMenu: NSMenu!
	var statusBarItem: NSStatusItem!

	func applicationDidFinishLaunching(_: Notification) {
		// setup status bar item
		statusBarItem = NSStatusBar.system.statusItem(withLength: NSStatusItem.squareLength)
		statusBarItem.menu = statusItemMenu

		// add icon
		let icon = NSImage(named: "StatusIcon")
		icon?.isTemplate = true
		statusBarItem.button!.image = icon
	}

	@IBAction func quit(_: AnyObject) {
		// terminate application
		NSApplication.shared.terminate(self)
	}
}
