//
//  Created by Joël Gähwiler on 18.01.17.
//  Copyright © 2017 Joël Gähwiler. All rights reserved.
//

import Cocoa

class LoadingViewController: NSViewController {
	@IBOutlet var indicator: NSProgressIndicator!
	@IBOutlet var label: NSTextField!
	@IBOutlet var button: NSButton!

	var message: String = ""
	var cancelled: (() -> Void)?

	override func viewDidLoad() {
		super.viewDidLoad()

		// set message if available
		if message != "" {
			label.stringValue = message
		}

		// hide button by default
		button.isHidden = true

		// start spinning
		indicator.startAnimation(self)
	}

	func onCancel(cancelled: @escaping () -> Void) {
		// set callback
		self.cancelled = cancelled

		// show button
		button.isHidden = false
	}

	@IBAction func cancelAction(_ sender: Any) {
		// call callback if available
		if cancelled != nil {
			cancelled!()
		}
	}
}
