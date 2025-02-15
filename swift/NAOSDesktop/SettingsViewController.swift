//
//  Created by Joël Gähwiler on 18.01.17.
//  Copyright © 2017 Joël Gähwiler. All rights reserved.
//

import Cocoa
import NAOSKit

class SettingsViewController: SessionViewController, NSTableViewDataSource, NSTableViewDelegate,
	SettingsParameterValueDelegate
{
	@IBOutlet var statusLabel: NSTextField!
	@IBOutlet var flashButton: NSButton!
	@IBOutlet var filesButton: NSButton!
	@IBOutlet var relayButton: NSComboButton!
	@IBOutlet var metricsButton: NSButton!
	@IBOutlet var parameterTableView: NSTableView!
	@IBOutlet var infoLabel: NSTextField!

	private var lvc: LoadingViewController?

	@IBAction func refresh(_: AnyObject) {
		// show loading view controller
		lvc = loadVC("LoadingViewController") as? LoadingViewController
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
				self.statusLabel.stringValue =
					(self.device.parameters[.connectionStatus] ?? "")
						.capitalized

				// update buttons
				self.flashButton.isEnabled = self.device.canUpdate
				self.filesButton.isEnabled = self.device.canFS
				self.relayButton.isEnabled = self.device.canRelay
				self.metricsButton.isEnabled = self.device.hasMetrics

				// reload parameters
				self.parameterTableView.reloadData()

				// update relay menu
				self.relayButton.menu.items = self.device.relayDevices.map { device in
					let item = NSMenuItem(title: device.name(), action: #selector(SettingsViewController.relay), keyEquivalent: "")
					item.representedObject = device
					return item
				}
				
				// update info label
				self.infoLabel.stringValue = "ID: \(self.device.device.id()) • MTU: \(self.device.mtu)"

				// dismiss sheet
				self.dismiss(self.lvc!)
			}
		}

		// set cancel action
		lvc!.onCancel {
			task.cancel()
		}
	}

	@IBAction func flash(_: AnyObject) {
		Task {
			// open file
			let (_, image) = try await openFile()

			await process(title: "Flashing...") { session, progress in
				// get time
				let start = Date()

				// perform flash
				try await NAOSUpdate.run(session: session, image: image) { done in
					// calculate delta
					let delta = Date().timeIntervalSince(start)

					// report progress
					progress(
						Double(done) / Double(image.count),
						Double(done) / delta)
				}
			}
		}
	}

	@IBAction func files(_: AnyObject) {
		// load files view controller
		let fvc = loadVC("FilesViewController") as! FilesViewController

		// assign endpoint
		fvc.device = device

		// present view controller
		presentAsSheet(fvc)
	}

	@objc func relay(a: NSMenuItem) {
		// get device
		let device = a.representedObject as! NAOSDevice

		Task {
			// prepare device
			let managedDevice = NAOSManagedDevice(device: device)

			// let manager open device
			DeviceManager.shared.openDevice(device: managedDevice)
		}
	}

	@IBAction func metrics(_: AnyObject) {
		// prepare metrics view controller
		let container = MetricsContainer(series: [])
		let view = MetricsView(data: container)
		let hvc = MetricsViewController(rootView: view)

		// present view controller
		presentAsModalWindow(hvc)

		// collect metrics
		hvc.collect(device: device)
	}

	// NSTableView

	func numberOfRows(in _: NSTableView) -> Int {
		// return parameter count if available
		return device != nil ? device.availableParameters.count : 0
	}

	func tableView(_ tableView: NSTableView, viewFor tableColumn: NSTableColumn?, row: Int)
		-> NSView?
	{
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
			let v = tableView.makeView(
				withIdentifier: NSUserInterfaceItemIdentifier("TypeCell"),
				owner: nil) as! NSTableCellView

			v.textField!.stringValue = p.type.string()

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
			statusLabel.stringValue =
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
