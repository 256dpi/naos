//
//  Created by Joël Gähwiler on 18.01.17.
//  Copyright © 2017 Joël Gähwiler. All rights reserved.
//

import Cocoa

class LoadingViewController: NSViewController {
	@IBOutlet var progressIndicator: NSProgressIndicator!
	@IBOutlet var label: NSTextField!

	var message: String = ""

	override func viewDidLoad() {
		super.viewDidLoad()

		// set message if available
		if message != "" {
			label.stringValue = message
		}

		// start spinning
		progressIndicator.startAnimation(self)
	}
}
