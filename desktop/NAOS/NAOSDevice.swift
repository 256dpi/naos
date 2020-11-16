//
//  Created by Joël Gähwiler on 05.04.17.
//  Copyright © 2017 Joël Gähwiler. All rights reserved.
//

import Cocoa
import CoreBluetooth

internal let NAOSPrimaryServiceUUID = CBUUID(string: "632FBA1B-4861-4E4F-8103-FFEE9D5033B5")

internal enum NAOSDeviceProperty: String {
    case wifiSSID = "802DD327-CA04-4C90-BE86-A3568275A510"
    case wifiPassword = "B3883261-F360-4CB7-9791-C3498FB2C151"
    case mqttHost = "193FFFF2-4542-4EBC-BE1F-4A355D40AC57"
    case mqttPort = "CB8A764C-546C-4787-9A6F-427382D49755"
    case mqttClientID = "08C4E543-65CC-4BF6-BBA8-C8A5BF916C3B"
    case mqttUsername = "ABB4A59A-85E9-449C-80F5-72D806A00257"
    case mqttPassword = "C5ECB1B1-0658-4FC3-8052-215521D41925"
    case deviceType = "0CEA3120-196E-4308-8883-8EA8757B9191"
    case deviceName = "25427850-28F3-40EA-926A-AB3CEB6D0B56"
    case baseTopic = "EAB7E3A8-9FF8-4938-8EA2-D09B8C63CAB4"
    case connectionStatus = "59997CE0-3A50-433C-A200-D9F6D312EB7C"
    case batteryLevel = "894060b1-30b1-4160-be65-5a438cc17c9f"
    case command = "37CF1864-5A8E-450F-A277-2981BD76D0AB"
    case paramsList = "9b89418c-4298-46b4-922d-4de826102c83"
    case paramsSelect = "a27b618a-b26c-4311-b810-272eec84e372"
    case paramsValue = "293a9e90-ed33-45a7-ae91-3031557cbfa3"
    case lockStatus = "5d8a1bd6-a033-43ef-a8f5-fb13a2603294"
    case unlock = "c7de29ff-2e98-4499-b5a3-4457f6173aff"

    func cbuuid() -> CBUUID {
        return CBUUID(string: rawValue)
    }

    func setting() -> NAOSDeviceSetting? {
        switch self {
        case .wifiSSID:
            return .wifiSSID
        case .wifiPassword:
            return .wifiPassword
        case .mqttHost:
            return .mqttHost
        case .mqttPort:
            return .mqttPort
        case .mqttClientID:
            return .mqttClientID
        case .mqttUsername:
            return .mqttUsername
        case .mqttPassword:
            return .mqttPassword
        case .deviceName:
            return .deviceName
        case .baseTopic:
            return .baseTopic
        default:
            return nil
        }
    }

    func optional() -> Bool {
        switch self {
        case .lockStatus, .unlock:
            return true
        default:
            return false
        }
    }

    // these can be read out at all times
    static let readable = [
        wifiSSID, wifiPassword, mqttHost, mqttPort, mqttClientID, mqttUsername,
        mqttPassword, deviceType, deviceName, baseTopic, connectionStatus,
        batteryLevel, paramsList, lockStatus,
    ]

    static let all = readable + [command, paramsSelect, paramsValue, unlock]
}

public enum NAOSDeviceSetting {
    case wifiSSID
    case wifiPassword
    case mqttHost
    case mqttPort
    case mqttClientID
    case mqttUsername
    case mqttPassword
    case deviceName
    case baseTopic

    internal func property() -> NAOSDeviceProperty {
        switch self {
        case .wifiSSID:
            return .wifiSSID
        case .wifiPassword:
            return .wifiPassword
        case .mqttHost:
            return .mqttHost
        case .mqttPort:
            return .mqttPort
        case .mqttClientID:
            return .mqttClientID
        case .mqttUsername:
            return .mqttUsername
        case .mqttPassword:
            return .mqttPassword
        case .deviceName:
            return .deviceName
        case .baseTopic:
            return .baseTopic
        }
    }

    public static let all = [
        wifiSSID, wifiPassword, mqttHost, mqttPort, mqttClientID,
        mqttUsername, mqttPassword, deviceName, baseTopic,
    ]
}

public enum NAOSDeviceSystemCommand: String {
    case ping
    case bootFactory = "boot-factory"
    case restartWifi = "restart-wifi"
    case restartMQTT = "restart-mqtt"
}

public enum NAOSDeviceParameterType: String {
    case string = "s"
    case bool = "b"
    case long = "l"
    case double = "d"
}

public struct NAOSDeviceParameter: Hashable {
    public var name: String
    public var type: NAOSDeviceParameterType

    public func hash(into hasher: inout Hasher) {
        hasher.combine(name)
    }

    public static func == (lhs: NAOSDeviceParameter, rhs: NAOSDeviceParameter) -> Bool {
        return lhs.name == rhs.name && lhs.type == rhs.type
    }
}

public enum NAOSDeviceError: Error {
    case primaryServiceNotFound
    case characteristicNotFound
}

public protocol NAOSDeviceDelegate {
    func naosDeviceDidConnect(device: NAOSDevice)
    func naosDeviceDidUpdateConnectionStatus(device: NAOSDevice)
    func naosDeviceDidUnlock(device: NAOSDevice)
    func naosDeviceDidRefresh(device: NAOSDevice)
    func naosDeviceDidError(device: NAOSDevice, error: Error)
    func naosDeviceDidDisconnect(device: NAOSDevice, error: Error?)
}

public class NAOSDevice: NSObject, CBPeripheralDelegate {
    public private(set) var deviceType: String = ""
    public private(set) var connectionStatus: String = ""
    public private(set) var batteryLevel: Float = -1
    public private(set) var protected: Bool = false
    public private(set) var locked: Bool = false
    public var settings: [NAOSDeviceSetting: String] = [:]
    public private(set) var availableParameters: [NAOSDeviceParameter] = []
    public var parameters: [NAOSDeviceParameter: String] = [:]
    public var delegate: NAOSDeviceDelegate?

    internal var peripheral: CBPeripheral

    private var proxy: NAOSDeviceProxy!
    private var manager: NAOSManager
    private var primaryService: CBService?
    private var initialRefresh: Bool = true
    private var refreshing: Bool = false
    private var tracker: [NAOSDeviceProperty: Bool] = [:]
    private var currentParameter: Int = -1
    private var errorOccurred: Bool = false

    init(peripheral: CBPeripheral, manager: NAOSManager) {
        // initialize instance
        self.peripheral = peripheral
        self.manager = manager

        // initialize settings
        for s in NAOSDeviceSetting.all {
            settings[s] = ""
        }

        // initialize tracker
        for c in NAOSDeviceProperty.all {
            tracker[c] = false
        }

        super.init()

        // create proxy and set delegate
        proxy = NAOSDeviceProxy(parent: self)
        peripheral.delegate = proxy

        // start initial refresh
        connect()
    }

    public func connect() {
        // connect to device
        manager.centralManager.connect(peripheral, options: nil)
    }

    public func refresh() {
        // immediately return if already refreshing
        if refreshing {
            return
        }

        // set flag
        refreshing = true

        // iterate over all readable properties
        for property in NAOSDeviceProperty.readable {
            // get characteristic
            guard let c = characteristicForProperty(property: property) else {
                // raise error if not optional
                if !property.optional() {
                    raiseError(error: NAOSDeviceError.characteristicNotFound)
                }

                return
            }

            // track request
            tracker[property] = true

            // issue read request
            peripheral.readValue(for: c)
        }
    }

    private func finishRefresh() {
        // set flag
        refreshing = false

        // check for initial refresh
        if initialRefresh {
            // disconnect from device
            disconnect()

            // update state
            initialRefresh = false

            // notify manager
            manager.didPrepareDevice(device: self)

            return
        }

        // notify manager
        manager.didUpdateDevice(device: self)

        // call delegate if available
        if let d = delegate {
            d.naosDeviceDidRefresh(device: self)
        }
    }

    public func name() -> String {
        return deviceType + " (" + settings[.deviceName]! + ")"
    }

    public func write(setting: NAOSDeviceSetting) {
        // get characteristic
        guard let c = characteristicForProperty(property: setting.property()) else {
            // raise error if not optional
            if !setting.property().optional() {
                raiseError(error: NAOSDeviceError.characteristicNotFound)
            }

            return
        }

        // write setting
        peripheral.writeValue(settings[setting]!.data(using: .utf8)!, for: c, type: .withResponse)

        // notify manager
        manager.didUpdateDevice(device: self)
    }

    public func command(cmd: NAOSDeviceSystemCommand) {
        // get characteristic
        guard let c = characteristicForProperty(property: .command) else {
            raiseError(error: NAOSDeviceError.characteristicNotFound)
            return
        }

        // write system command
        peripheral.writeValue(cmd.rawValue.data(using: .utf8)!, for: c, type: .withResponse)
    }

    public func unlock(password: String) {
        // get characteristic
        guard let c = characteristicForProperty(property: .unlock) else {
            raiseError(error: NAOSDeviceError.characteristicNotFound)
            return
        }

        // write unlock command
        peripheral.writeValue(password.data(using: .utf8)!, for: c, type: .withResponse)
    }

    public func write(parameter: NAOSDeviceParameter) {
        // return if not available
        if availableParameters.firstIndex(of: parameter) == nil {
            return
        }

        // get characteristic
        guard let s = characteristicForProperty(property: .paramsSelect) else {
            raiseError(error: NAOSDeviceError.characteristicNotFound)
            return
        }

        // get characteristic
        guard let p = characteristicForProperty(property: .paramsValue) else {
            raiseError(error: NAOSDeviceError.characteristicNotFound)
            return
        }

        // select parameter
        peripheral.writeValue(parameter.name.data(using: .utf8)!, for: s, type: .withResponse)

        // write parameter
        peripheral.writeValue(parameters[parameter]!.data(using: .utf8)!, for: p, type: .withResponse)

        // notify manager
        manager.didUpdateDevice(device: self)
    }

    public func disconnect() {
        // disconnect from device
        manager.centralManager.cancelPeripheralConnection(peripheral)
    }

    // NAOSManager

    internal func forwardDidConnect() {
        // discover primary service
        peripheral.discoverServices([NAOSPrimaryServiceUUID])
    }

    internal func forwardDidFailToConnect(error: Error?) {
        print("forwardDidFailToConnect", error?.localizedDescription ?? "")
    }

    internal func forwardDidDisconnect(error: Error?) {
        // lock again if protected
        if protected {
            locked = true
        }

        // return immediately if an error occurred beforehand
        if errorOccurred {
            return
        }

        // call delegate if present
        if let d = delegate {
            d.naosDeviceDidDisconnect(device: self, error: error)
        }
    }

    // NAOSDeviceProxy

    internal func peripheralDidDiscoverServices(error: Error?) {
        // check error
        if let e = error {
            raiseError(error: e)
            return
        }

        // save service reference
        for svc in peripheral.services ?? [] {
            if svc.uuid == NAOSPrimaryServiceUUID {
                primaryService = svc
            }
        }

        // check existence of service
        guard let ps = primaryService else {
            raiseError(error: NAOSDeviceError.primaryServiceNotFound)
            return
        }

        // discover characteristics
        peripheral.discoverCharacteristics(nil, for: ps)
    }

    internal func peripheralDidDiscoverCharacteristicsFor(service: CBService, error: Error?) {
        // check error
        if let e = error {
            raiseError(error: e)
            return
        }

        // go through all characteristics
        for chr in service.characteristics ?? [] {
            // enable notifications for characteristics that support indication
            if chr.properties.contains(.indicate) {
                peripheral.setNotifyValue(true, for: chr)
            }
        }

        // perform initial refresh
        if initialRefresh {
            refresh()
            return
        }

        // call delegate if available
        if let d = delegate {
            d.naosDeviceDidConnect(device: self)
        }
    }

    internal func peripheralDidUpdateValueFor(characteristic: CBCharacteristic, error: Error?) {
        // check error
        if let e = error {
            raiseError(error: e)
            return
        }

        // get string from value
        var value: String = ""
        if let v = characteristic.value {
            if let s = String(data: v, encoding: .utf8) {
                value = s
            }
        }

        // get property
        guard let property = propertyForCharacteristic(characteristic: characteristic) else {
            raiseError(error: NAOSDeviceError.characteristicNotFound)
            return
        }

        // check if got an updated connection status
        if characteristic.uuid == NAOSDeviceProperty.connectionStatus.cbuuid() {
            // set new connection status
            connectionStatus = value

            // notify delegate and return immediately if not refreshing
            if !refreshing {
                if let d = delegate {
                    d.naosDeviceDidUpdateConnectionStatus(device: self)
                }
            }
        } else if characteristic.uuid == NAOSDeviceProperty.deviceType.cbuuid() {
            // save device type
            deviceType = value
        } else if characteristic.uuid == NAOSDeviceProperty.batteryLevel.cbuuid() {
            // save battery level
            batteryLevel = Float(value) ?? -1
        } else if characteristic.uuid == NAOSDeviceProperty.paramsList.cbuuid() {
            // reset list
            availableParameters = []

            // save parameters
            let segments = value.split(separator: ",")
            for s in segments {
                let subSegments = s.split(separator: ":")
                let name = String(subSegments[0])
                let type = NAOSDeviceParameterType(rawValue: String(subSegments[1])) ?? .string
                availableParameters.append(NAOSDeviceParameter(name: name, type: type))
            }

            // queue first parameter if refreshing
            if refreshing && availableParameters.count > 0 {
                // set index
                currentParameter = 0

                // select parameter
                peripheral.writeValue(availableParameters[currentParameter].name.data(using: .utf8)!, for: characteristicForProperty(property: .paramsSelect)!, type: .withResponse)

                // read parameter
                peripheral.readValue(for: characteristicForProperty(property: .paramsValue)!)
            }
        } else if characteristic.uuid == NAOSDeviceProperty.paramsValue.cbuuid() {
            // update parameter
            parameters[availableParameters[currentParameter]] = value

            // increment parameter
            currentParameter += 1

            // check overlfow
            if currentParameter == availableParameters.count {
                currentParameter = -1
            } else {
                // select parameter
                peripheral.writeValue(availableParameters[currentParameter].name.data(using: .utf8)!, for: characteristicForProperty(property: .paramsSelect)!, type: .withResponse)

                // read parameter
                peripheral.readValue(for: characteristicForProperty(property: .paramsValue)!)
            }
        } else if characteristic.uuid == NAOSDeviceProperty.lockStatus.cbuuid() {
            // save lock status
            locked = value == "locked"

            // save if this device is protected
            if locked {
                protected = true
            }

            // notify delegate and return immediately if not refreshing
            if !refreshing && !locked {
                if let d = delegate {
                    d.naosDeviceDidUnlock(device: self)
                }
            }
        } else {
            // it must be a setting
            let s = property.setting()!

            // save updated value
            settings[s] = value
        }

        // check if refreshing and property is marked to be refreshed
        if refreshing {
            // unmark property if marked
            if tracker[property]! {
                tracker[property] = false
            }

            // return if one property is still flagged to be refreshed
            for (_, v) in tracker {
                if v {
                    return
                }
            }

            // return if parameters are not finished
            if currentParameter >= 0 {
                return
            }

            // finish refresh
            finishRefresh()
        }
    }

    // Helpers

    private func raiseError(error: Error) {
        // check for initial refresh
        if initialRefresh {
            // notify manager
            manager.failedToPrepareDevice(device: self, error: error)
        } else {
            // call delegate if available
            if let d = delegate {
                d.naosDeviceDidError(device: self, error: error)
            }
        }

        // set flag
        errorOccurred = true

        // disconnect device
        disconnect()
    }

    private func propertyForCharacteristic(characteristic: CBCharacteristic) -> NAOSDeviceProperty? {
        for property in NAOSDeviceProperty.all {
            if property.cbuuid() == characteristic.uuid {
                return property
            }
        }

        return nil
    }

    private func characteristicForProperty(property: NAOSDeviceProperty) -> CBCharacteristic? {
        if let s = primaryService {
            if let cs = s.characteristics {
                for c in cs {
                    if c.uuid == property.cbuuid() {
                        return c
                    }
                }
            }
        }

        return nil
    }
}
