//
//  Created by Joël Gähwiler on 18.01.17.
//  Copyright © 2017 Joël Gähwiler. All rights reserved.
//

import Cocoa
import CoreBluetooth

class SettingsViewController: NSViewController, NSTableViewDataSource, NSTableViewDelegate, SettingsParameterValueDelegate {
	@IBOutlet var connectionStatusLabel: NSTextField!
	@IBOutlet var wifiSSIDTextField: NSTextField!
	@IBOutlet var wifiPasswordTextField: NSTextField!
	@IBOutlet var wifiSSIDLabel: NSTextField!
	@IBOutlet var mqttHostTextField: NSTextField!
	@IBOutlet var mqttPortTextField: NSTextField!
	@IBOutlet var mqttClientIDTextField: NSTextField!
	@IBOutlet var mqttUsernameTextField: NSTextField!
	@IBOutlet var mqttPasswordTextField: NSTextField!
	@IBOutlet var deviceNameTextField: NSTextField!
	@IBOutlet var baseTopicTextField: NSTextField!
	@IBOutlet var parameterTableView: NSTableView!
	@IBOutlet var descriptionLabel: NSTextField!

	private var device: NAOSDevice!

	private var loadingViewController: LoadingViewController?

	override func viewDidLoad() {
		super.viewDidLoad()

		// hide wifi ssid label
		wifiSSIDLabel.isHidden = true

		// clear description
		descriptionLabel.stringValue = ""
	}

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

	@IBAction
	func reboot(_: AnyObject) {
		// send reboot command
		device.execute(cmd: .reboot)
	}

	@IBAction
	func ping(_: AnyObject) {
		// send ping command
		device.execute(cmd: .ping)
	}

	@IBAction
	func configureWiFi(_: AnyObject) {
		// write settings
		writeSettings(settings: [.wifiSSID, .wifiPassword])

		// send restart command
		device.execute(cmd: .restartWifi)
	}

	@IBAction
	func useShiftrIO(_: AnyObject) {
		// set settings
		mqttHostTextField.stringValue = "public.cloud.shiftr.io"
		mqttPortTextField.stringValue = "1883"
		mqttUsernameTextField.stringValue = "public"
		mqttPasswordTextField.stringValue = "public"
	}

	@IBAction
	func useLocalBroker(_: AnyObject) {
		// set settings
		mqttHostTextField.stringValue = NetworkUtilities.getWiFiAddress() ?? ""
		mqttPortTextField.stringValue = UserDefaults.standard.string(forKey: "mqttPort") ?? ""
	}

	@IBAction
	func configureMQTT(_: AnyObject) {
		// write settings
		writeSettings(settings: [
			.mqttHost, .mqttPort, .mqttClientID, .mqttUsername,
			.mqttPassword,
		])

		// send restart command
		device.execute(cmd: .restartMQTT)
	}

	@IBAction
	func configureDevice(_: AnyObject) {
		// write settings
		writeSettings(settings: [.deviceName, .baseTopic])

		// send restart command
		device.execute(cmd: .restartMQTT)
	}

	// SettingsWindowController

	func didRefresh() {
		// update text fields
		for (s, v) in device.settings {
			if let textField = textFieldForSetting(setting: s) {
				textField.stringValue = v
			}
		}

		// update connection status
		connectionStatusLabel.stringValue = (device.descriptors[.conenctionStatus] ?? "").capitalized

		// update description
		var info = [String]()
		for (descriptor, value) in device.descriptors {
			info.append(descriptor.title() + ": " + descriptor.format(value: value))
		}
		descriptionLabel.stringValue = info.joined(separator: "\n")

		// reload parameters
		parameterTableView.reloadData()

		// set wifi ssid
		if let ssid = NetworkUtilities.getSSID() {
			wifiSSIDLabel.stringValue = String(format: "Your computer uses \"%@\".", ssid)
			wifiSSIDLabel.isHidden = false
		} else {
			wifiSSIDLabel.isHidden = true
		}

		// dismiss sheet
		dismiss(loadingViewController!)
	}

	func didUpdateConnectionStatus() {
		// update connection status
		connectionStatusLabel.stringValue = (device.descriptors[.conenctionStatus] ?? "").capitalized
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

			// setup controls
			if p.type == .bool {
				v.checkbox!.isHidden = false
				v.button!.isHidden = true
				v.textField!.isHidden = true
				v.checkbox.state = device.parameters[p]! == "1" ? .on : .off
			} else if p.type == .action {
				v.checkbox!.isHidden = true
				v.button!.isHidden = false
				v.textField!.isHidden = true
			} else {
				v.checkbox!.isHidden = true
				v.button!.isHidden = true
				v.textField!.isHidden = false
				v.textField!.stringValue = device.parameters[p]!

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

	// Helpers

	func writeSettings(settings: [NAOSDeviceSetting]) {
		for s in settings {
			if let textField = textFieldForSetting(setting: s) {
				device.settings[s] = textField.stringValue
				device.write(setting: s)
			}
		}
	}

	func textFieldForSetting(setting: NAOSDeviceSetting) -> NSTextField? {
		switch setting {
		case .wifiSSID:
			return wifiSSIDTextField
		case .wifiPassword:
			return wifiPasswordTextField
		case .mqttHost:
			return mqttHostTextField
		case .mqttPort:
			return mqttPortTextField
		case .mqttClientID:
			return mqttClientIDTextField
		case .mqttUsername:
			return mqttUsernameTextField
		case .mqttPassword:
			return mqttPasswordTextField
		case .deviceName:
			return deviceNameTextField
		case .baseTopic:
			return baseTopicTextField
		default:
			return nil
		}
	}
}
