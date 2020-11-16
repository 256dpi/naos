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
        // unlock device
        device.unlock(password: passwordField.stringValue)

        // reset
        passwordField.stringValue = ""
    }
}
