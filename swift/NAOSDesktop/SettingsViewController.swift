//
//  Created by Joël Gähwiler on 18.01.17.
//  Copyright © 2017 Joël Gähwiler. All rights reserved.
//

import Cocoa
import NAOSKit

class SettingsViewController: NSViewController, NSTableViewDataSource, NSTableViewDelegate,
	SettingsParameterValueDelegate
{
	@IBOutlet var connectionStatusLabel: NSTextField!
	@IBOutlet var parameterTableView: NSTableView!

	internal var device: NAOSDevice!
	private var lvc: LoadingViewController?

	@IBAction
	func refresh(_: AnyObject) {
		// show loading view controller
		lvc = NSStoryboard(name: "Main", bundle: nil)
			.instantiateController(withIdentifier: "LoadingViewController") as? LoadingViewController
		lvc!.message = "Refreshing..."
		lvc!.preferredContentSize = CGSize(width: 200, height: 200)
		presentAsSheet(lvc!)

		// refresh device
		let task = Task {
			// perform refresh
			do {
				try await device.refresh()
			} catch {
				showError(error: error)
			}

			DispatchQueue.main.async {
				// update connection status
				self.connectionStatusLabel.stringValue =
					(self.device.parameters[.connectionStatus] ?? "")
						.capitalized

				// reload parameters
				self.parameterTableView.reloadData()

				// dismiss sheet
				self.dismiss(self.lvc!)
			}
		}

		// set cancel action
		lvc!.onCancel {
			task.cancel()
		}
	}

	@IBAction
	func flash(_: AnyObject) {
		// show loading view controller
		lvc = NSStoryboard(name: "Main", bundle: nil).instantiateController(withIdentifier: "LoadingViewController") as? LoadingViewController
		lvc!.message = "Flashing..."
		lvc!.preferredContentSize = CGSize(width: 200, height: 200)
		presentAsSheet(lvc!)

		let task = Task {
			do {
				// open file
				let (_, image) = try await openFile()

				// perform flash
				try await device.flash(data: image, progress: { (progress: NAOSProgress) in
					DispatchQueue.main.async {
						self.lvc!.label.stringValue = String(format: "Flashing...\n%.1f %% @ %.1f kB/s", progress.percent, progress.rate / 1000)
						self.lvc!.indicator.isIndeterminate = false
						self.lvc!.indicator.doubleValue = progress.percent
					}
				})
			} catch {
				showError(error: error)
			}

			// dismiss sheet
			self.dismiss(self.lvc!)
		}

		// assign cancel action
		lvc!.onCancel {
			task.cancel()
		}
	}

	@IBAction func files(_: AnyObject) {
		Task {
			do {
				// open a new session
				let sess = try await self.device.session(timeout: 5)
				defer { sess.cleanup() }

				// query endpoint existence
				let exists = try await sess.query(endpoint: 0x3, timeout: 5)

				// close session
				try await sess.end(timeout: 5)

				// check endpoint existence
				if !exists {
					throw CustomError(title: "Missing FS Endpoint.")
				}

				// load files view controller
				let fvc = NSStoryboard(name: "Main", bundle: nil).instantiateController(withIdentifier: "FilesViewController") as! FilesViewController

				// assign endpoint
				fvc.device = device

				// present view controller
				self.presentAsSheet(fvc)
			} catch {
				showError(error: error)
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
			let v =
				tableView.makeView(
					withIdentifier: NSUserInterfaceItemIdentifier("NameCell"),
					owner: nil) as! NSTableCellView
			v.textField!.stringValue = p.name
			return v
		}

		// return value cell
		if tableView.tableColumns[1] == tableColumn {
			let v =
				tableView.makeView(
					withIdentifier: NSUserInterfaceItemIdentifier("ValueCell"),
					owner: nil) as! SettingsParameterValue
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
				v.textField!.stringValue = p.format(
					value: device.parameters[p] ?? "")
				v.textField!.isEnabled = !p.mode.contains(.locked)

				// TODO: Use hex formatter for raw values?

				// set appropriate number formatters
				switch p.type {
				case .raw, .string, .bool, .action:
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
			let v =
				tableView.makeView(
					withIdentifier: NSUserInterfaceItemIdentifier("TypeCell"),
					owner: nil) as! NSTableCellView
			switch p.type {
			case .raw:
				v.textField!.stringValue = "Raw"
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
			connectionStatusLabel.stringValue =
				(device.parameters[.connectionStatus] ?? "").capitalized
		}

		// find index
		let index = device.availableParameters.firstIndex(of: parameter)!

		// reload paramter
		parameterTableView.reloadData(
			forRowIndexes: IndexSet(integer: index), columnIndexes: IndexSet(integer: 1))
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
				self.parameterTableView.noteHeightOfRows(
					withIndexesChanged: IndexSet(
						integer: self.device.availableParameters.firstIndex(
							of: parameter)!))
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
