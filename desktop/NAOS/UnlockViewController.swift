//
//  Created by Joël Gähwiler on 30.08.18.
//  Copyright © 2017 Joël Gähwiler. All rights reserved.
//

import Cocoa

class UnlockViewController: NSViewController {
	@IBOutlet var passwordField: NSSecureTextField!

	private var device: NAOSDevice!

	func setDevice(device: NAOSDevice) {
		// save device
		self.device = device
	}

	@IBAction
	func unlock(_: AnyObject) {
		// copy and reset password
		let password = passwordField.stringValue
		passwordField.stringValue = ""

		// unlock device
		Task {
			try await device.unlock(password: password)
		}
	}
}
