//
//  Created by Joël Gähwiler on 30.08.18.
//  Copyright © 2017 Joël Gähwiler. All rights reserved.
//

import Cocoa
import NAOSKit

class UnlockViewController: NSViewController {
	@IBOutlet var passwordField: NSSecureTextField!

	internal var device: NAOSManagedDevice!
	internal var swc: SettingsWindowController!

	@IBAction
	func unlock(_: AnyObject) {
		// copy and reset password
		let password = passwordField.stringValue
		passwordField.stringValue = ""

		Task {
			// unlock device
			var ok = false
			do {
				ok = try await device.unlock(password: password)
			} catch {
				showError(error: error)
			}

			// yield unlock if successful
			if ok {
				DispatchQueue.main.async {
					self.swc.didUnlock()
				}
			}
		}
	}
}
