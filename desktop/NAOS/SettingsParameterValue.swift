//
//  Created by Joël Gähwiler on 18.04.18.
//  Copyright © 2018 Joël Gähwiler. All rights reserved.
//

import Cocoa

public protocol SettingsParameterValueDelegate {
	func didChangeTextField(parameter: NAOSDeviceParameter, value: String)
	func didClickCheckbox(parameter: NAOSDeviceParameter, value: Bool)
}

class SettingsParameterValue: NSTableCellView {
	@IBOutlet var checkbox: NSButton!
	public var parameter: NAOSDeviceParameter?
	public var delegate: SettingsParameterValueDelegate?

	@IBAction func didChangeTextField(sender: NSTextField) {
		if let d = delegate {
			d.didChangeTextField(parameter: parameter!, value: sender.stringValue)
		}
	}

	@IBAction func didClickCheckbox(sender: NSButton) {
		if let d = delegate {
			d.didClickCheckbox(parameter: parameter!, value: sender.state == .on)
		}
	}
}
