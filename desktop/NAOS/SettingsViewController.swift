//
//  Created by Joël Gähwiler on 18.01.17.
//  Copyright © 2017 Joël Gähwiler. All rights reserved.
//

import Cocoa
import CoreBluetooth

class SettingsViewController: NSViewController, NSTableViewDataSource, NSTableViewDelegate, SettingsParameterValueDelegate {
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
    @IBOutlet var connectionStatusLabel: NSTextField!
    @IBOutlet var batteryLevelLabel: NSTextField!
    @IBOutlet var batteryLevelIndicator: NSProgressIndicator!
    @IBOutlet var tableView: NSTableView!

    private var device: NAOSDevice!

    private var loadingViewController: LoadingViewController?

    override func viewDidLoad() {
        super.viewDidLoad()

        // hide battery level label and indicator
        batteryLevelLabel.isHidden = true
        batteryLevelIndicator.isHidden = true

        // hide wifi ssid label
        wifiSSIDLabel.isHidden = true
    }

    func setDevice(device: NAOSDevice) {
        // save device
        self.device = device

        // read all values
        refresh(self)

        // reload table
        tableView.reloadData()
    }

    @IBAction
    func refresh(_: AnyObject) {
        // create refresh view controller
        loadingViewController = NSStoryboard(name: "Main", bundle: nil).instantiateController(withIdentifier: "LoadingViewController") as? LoadingViewController

        // set label
        loadingViewController!.message = "Refreshing..."

        // present refresh controller as sheet
        presentAsSheet(loadingViewController!)

        // read all settings
        device.refresh()
    }

    @IBAction
    func bootFactory(_: AnyObject) {
        // send select factory command
        device.command(cmd: .bootFactory)
    }

    @IBAction
    func ping(_: AnyObject) {
        // send ping command
        device.command(cmd: .ping)
    }

    @IBAction
    func configureWiFi(_: AnyObject) {
        // write settings
        writeSettings(settings: [.wifiSSID, .wifiPassword])

        // send restart command
        device.command(cmd: .restartWifi)
    }

    @IBAction
    func useShiftrIO(_: AnyObject) {
        // set settings
        mqttHostTextField.stringValue = "broker.shiftr.io"
        mqttPortTextField.stringValue = "1883"
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
        device.command(cmd: .restartMQTT)
    }

    @IBAction
    func configureDevice(_: AnyObject) {
        // write settings
        writeSettings(settings: [.deviceName, .baseTopic])

        // send restart command
        device.command(cmd: .restartMQTT)
    }

    // SettingsWindowController

    func didRefresh() {
        // update text fields
        for (s, v) in device.settings {
            textFieldForSetting(setting: s).stringValue = v
        }

        // update connection status
        connectionStatusLabel.stringValue = device.connectionStatus

        // update battery level
        if device.batteryLevel >= 0 {
            batteryLevelLabel.isHidden = false
            batteryLevelIndicator.isHidden = false
            batteryLevelIndicator.doubleValue = Double(device.batteryLevel * 100)
        } else {
            batteryLevelLabel.isHidden = true
            batteryLevelIndicator.isHidden = true
        }

        // reload parameters
        tableView.reloadData()

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
        connectionStatusLabel.stringValue = device.connectionStatus
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
                v.textField!.isHidden = true
                v.checkbox.state = device.parameters[p]! == "1" ? .on : .off
            } else {
                v.checkbox!.isHidden = true
                v.textField!.stringValue = device.parameters[p]!

                // set appropriate number formatters
                switch p.type {
                case .string, .bool:
                    break
                case .long:
                    let f = NumberFormatter()
                    f.numberStyle = .decimal
                    f.usesGroupingSeparator = false
                    f.maximumFractionDigits = 0
                    v.textField!.formatter = f
                    break
                case .double:
                    let f = NumberFormatter()
                    f.numberStyle = .decimal
                    f.usesGroupingSeparator = false
                    f.maximumFractionDigits = 8
                    v.textField!.formatter = f
                    break
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
                break
            case .bool:
                v.textField!.stringValue = "Bool"
                break
            case .long:
                v.textField!.stringValue = "Long"
                break
            case .double:
                v.textField!.stringValue = "Double"
                break
            }

            return v
        }

        return nil
    }

    // SettingsParameterValueDelegate

    func didChangeTextField(parameter: NAOSDeviceParameter, value: String) {
        // update parameter
        device.parameters[parameter] = value

        // write parameter
        device.write(parameter: parameter)
    }

    func didClickCheckbox(parameter: NAOSDeviceParameter, value: Bool) {
        // update parameter
        device.parameters[parameter] = value ? "1" : "0"

        // write parameter
        device.write(parameter: parameter)
    }

    // Helpers

    func writeSettings(settings: [NAOSDeviceSetting]) {
        for s in settings {
            device.settings[s] = textFieldForSetting(setting: s).stringValue
            device.write(setting: s)
        }
    }

    func textFieldForSetting(setting: NAOSDeviceSetting) -> NSTextField {
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
        }
    }
}
