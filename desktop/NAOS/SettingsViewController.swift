//
//  Created by Joël Gähwiler on 18.01.17.
//  Copyright © 2017 Joël Gähwiler. All rights reserved.
//

import Cocoa
import NAOSKit

class SettingsViewController: NSViewController, NSTableViewDataSource, NSTableViewDelegate, SettingsParameterValueDelegate {
	@IBOutlet var connectionStatusLabel: NSTextField!
	@IBOutlet var parameterTableView: NSTableView!

	internal var device: NAOSDevice!
	private var loadingViewController: LoadingViewController?

	@IBAction
	func refresh(_: AnyObject) {
		// show loading view controller
		loadingViewController = NSStoryboard(name: "Main", bundle: nil).instantiateController(withIdentifier: "LoadingViewController") as? LoadingViewController
		loadingViewController!.message = "Refreshing..."
		loadingViewController!.preferredContentSize = CGSize(width: 200, height: 200)
		presentAsSheet(loadingViewController!)

		// refresh device
		Task {
			// perform refresh
			do {
				try await device.refresh()
			} catch {
				showError(error: error)
			}

			DispatchQueue.main.async {
				// update connection status
				self.connectionStatusLabel.stringValue = (self.device.parameters[.connectionStatus] ?? "").capitalized

				// reload parameters
				self.parameterTableView.reloadData()

				// dismiss sheet
				self.dismiss(self.loadingViewController!)
			}
		}
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
				v.checkbox.state = device.parameters[p] == "1" ? .on : .off
				v.checkbox.isEnabled = !p.mode.contains(.locked)
			} else if p.type == .action {
				v.button!.isHidden = false
				v.button.isEnabled = !p.mode.contains(.locked)
			} else {
				v.textField!.isHidden = false
				v.textField!.formatter = nil
				v.textField!.stringValue = p.format(value: device.parameters[p] ?? "") 
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
					f.maximumFractionDigits = 32
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

	// SettingsWindowController

	func didUpdateParameter(parameter: NAOSParameter) {
		// update connection status
		if parameter == .connectionStatus {
			connectionStatusLabel.stringValue = (device.parameters[.connectionStatus] ?? "").capitalized
		}

		// find index
		let index = device.availableParameters.firstIndex(of: parameter)!

		// reload paramter
		parameterTableView.reloadData(forRowIndexes: IndexSet(integer: index), columnIndexes: IndexSet(integer: 1))
	}

	// SettingsParameterValueDelegate

	func didChangeTextField(parameter: NAOSParameter, value: String) {
		// update parameter
		device.parameters[parameter] = value

		// write parameter
		Task {
			// perform write
			do {
				try await device.write(parameter: parameter)
			} catch {
				showError(error: error)
				return
			}

			// recalculate row height
			DispatchQueue.main.async {
				self.parameterTableView.noteHeightOfRows(withIndexesChanged: IndexSet(integer: self.device.availableParameters.firstIndex(of: parameter)!))
			}
		}
	}

	func didClickCheckbox(parameter: NAOSParameter, value: Bool) {
		// update parameter
		device.parameters[parameter] = value ? "1" : "0"

		// write parameter
		Task {
			do {
				try await device.write(parameter: parameter)
			} catch {
				showError(error: error)
			}
		}
	}

	func didClickButton(parameter: NAOSParameter) {
		// update parameter
		device.parameters[parameter] = ""

		// write parameter
		Task {
			do {
				try await device.write(parameter: parameter)
			} catch {
				showError(error: error)
			}
		}
	}
}
