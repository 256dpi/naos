//
//  Created by Joël Gähwiler on 18.01.17.
//  Copyright © 2017 Joël Gähwiler. All rights reserved.
//

import Cocoa

class SettingsViewController: NSViewController, NSTableViewDataSource, NSTableViewDelegate, SettingsParameterValueDelegate {
	@IBOutlet var connectionStatusLabel: NSTextField!
	@IBOutlet var parameterTableView: NSTableView!

	private var device: NAOSDevice!

	private var loadingViewController: LoadingViewController?

	func setDevice(device: NAOSDevice) {
		// save device
		self.device = device

		// read all values
		refresh(self)

		// reload table
		parameterTableView.reloadData()
	}

	@IBAction
	func refresh(_: AnyObject) {
		// create refresh view controller
		loadingViewController = NSStoryboard(name: "Main", bundle: nil).instantiateController(withIdentifier: "LoadingViewController") as? LoadingViewController

		// set label
		loadingViewController!.message = "Refreshing..."

		// set size
		loadingViewController!.preferredContentSize = CGSize(width: 200, height: 200)

		// present refresh controller as sheet
		presentAsSheet(loadingViewController!)

		// read all settings
		device.refresh()
	}

	// SettingsWindowController

	func didRefresh() {
		// update connection status
		connectionStatusLabel.stringValue = (device.parameters[.connectionStatus] ?? "").capitalized

		// reload parameters
		parameterTableView.reloadData()

		// dismiss sheet
		dismiss(loadingViewController!)
	}

	func didUpdateConnectionStatus() {
		// update connection status
		connectionStatusLabel.stringValue = (device.parameters[.connectionStatus] ?? "").capitalized
	}

	// NSTableView

	func numberOfRows(in _: NSTableView) -> Int {
		// return parameter count if available
		return device != nil ? device.availableParameters.count : 0
	}

	func tableView(_ tableView: NSTableView, viewFor tableColumn: NSTableColumn?, row: Int) -> NSView? {
		// get parameter
		let p = device.availableParameters[row]

		// return name cell
		if tableView.tableColumns[0] == tableColumn {
			let v = tableView.makeView(withIdentifier: NSUserInterfaceItemIdentifier("NameCell"), owner: nil) as! NSTableCellView
			v.textField!.stringValue = p.name
			return v
		}

		// return value cell
		if tableView.tableColumns[1] == tableColumn {
			let v = tableView.makeView(withIdentifier: NSUserInterfaceItemIdentifier("ValueCell"), owner: nil) as! SettingsParameterValue
			v.parameter = p
			v.delegate = self

			v.checkbox!.isHidden = true
			v.button!.isHidden = true
			v.textField!.isHidden = true

			// setup controls
			if p.type == .bool {
				v.checkbox!.isHidden = false
				v.checkbox.state = device.parameters[p]! == "1" ? .on : .off
				v.checkbox.isEnabled = !p.mode.contains(.locked)
			} else if p.type == .action {
				v.button!.isHidden = false
				v.button.isEnabled = !p.mode.contains(.locked)
			} else {
				v.textField!.isHidden = false
				v.textField!.formatter = nil
				v.textField!.stringValue = device.parameters[p]!
				v.textField!.isEnabled = !p.mode.contains(.locked)

				// set appropriate number formatters
				switch p.type {
				case .string, .bool, .action:
					break
				case .long:
					let f = NumberFormatter()
					f.numberStyle = .decimal
					f.usesGroupingSeparator = false
					f.maximumFractionDigits = 0
					v.textField!.formatter = f
				case .double:
					let f = NumberFormatter()
					f.numberStyle = .decimal
					f.usesGroupingSeparator = false
					f.maximumFractionDigits = 8
					v.textField!.formatter = f
				}
			}

			return v
		}

		// return type cell
		if tableView.tableColumns[2] == tableColumn {
			let v = tableView.makeView(withIdentifier: NSUserInterfaceItemIdentifier("TypeCell"), owner: nil) as! NSTableCellView
			switch p.type {
			case .string:
				v.textField!.stringValue = "String"
			case .bool:
				v.textField!.stringValue = "Bool"
			case .long:
				v.textField!.stringValue = "Long"
			case .double:
				v.textField!.stringValue = "Double"
			case .action:
				v.textField!.stringValue = "Action"
			}

			return v
		}

		return nil
	}

	func tableView(_: NSTableView, heightOfRow row: Int) -> CGFloat {
		// get parameter
		let p = device.availableParameters[row]

		// handle non strings
		if p.type != .string {
			return 17
		}

		// get string
		let str = device.parameters[p] ?? ""

		// count lines
		let lines = str.reduce(into: 1) { count, letter in
			if letter == "\n" {
				count += 1
			}
		}

		return CGFloat(lines * 17)
	}

	// SettingsParameterValueDelegate

	func didChangeTextField(parameter: NAOSDeviceParameter, value: String) {
		// update parameter
		device.parameters[parameter] = value

		// write parameter
		device.write(parameter: parameter)

		// recalcualte row height
		parameterTableView.noteHeightOfRows(withIndexesChanged: IndexSet(integer: device.availableParameters.firstIndex(of: parameter)!))
	}

	func didClickCheckbox(parameter: NAOSDeviceParameter, value: Bool) {
		// update parameter
		device.parameters[parameter] = value ? "1" : "0"

		// write parameter
		device.write(parameter: parameter)
	}

	func didClickButton(parameter: NAOSDeviceParameter) {
		// update parameter
		device.parameters[parameter] = ""

		// write parameter
		device.write(parameter: parameter)
	}
}
