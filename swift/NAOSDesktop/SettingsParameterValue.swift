//
//  Created by Joël Gähwiler on 18.04.18.
//  Copyright © 2018 Joël Gähwiler. All rights reserved.
//

import Cocoa
import NAOSKit

public protocol SettingsParameterValueDelegate {
	func didChangeTextField(parameter: Parameter, value: String)
	func didClickCheckbox(parameter: Parameter, value: Bool)
	func didClickButton(parameter: Parameter)
}

class SettingsParameterValue: NSTableCellView {
	@IBOutlet var checkbox: NSButton!
	@IBOutlet var button: NSButton!
	public var parameter: Parameter?
	public var delegate: SettingsParameterValueDelegate?

	@IBAction func didChange(sender: NSTextField) {
		if let d = delegate {
			d.didChangeTextField(parameter: parameter!, value: sender.stringValue)
		}
	}

	@IBAction func didCheck(sender: NSButton) {
		if let d = delegate {
			d.didClickCheckbox(parameter: parameter!, value: sender.state == .on)
		}
	}

	@IBAction func didClick(sender: NSButton) {
		if let d = delegate {
			d.didClickButton(parameter: parameter!)
		}
	}
}
