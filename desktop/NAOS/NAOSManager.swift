//
//  Created by Joël Gähwiler on 05.04.17.
//  Copyright © 2017 Joël Gähwiler. All rights reserved.
//

import Cocoa
import CoreBluetooth

public protocol NAOSManagerDelegate {
    func naosManagerDidPrepareDevice(manager: NAOSManager, device: NAOSDevice)
    func naosManagerDidFailToPrepareDevice(manager: NAOSManager, error: Error)
    func naosManagerDidUpdateDevice(manager: NAOSManager, device: NAOSDevice)
    func naosManagerDidReset(manager: NAOSManager)
}

public extension NAOSManagerDelegate {
    func naosManagerDidUpdateDevice(manager _: NAOSManager, device _: NAOSDevice) {}
}

public class NAOSManager: NSObject {
    public var delegate: NAOSManagerDelegate?

    internal var centralManager: CBCentralManager!

    private var allDevices: [NAOSDevice]
    private var availableDevices: [NAOSDevice]
    private var proxy: NAOSManagerProxy!

    public init(delegate: NAOSManagerDelegate?) {
        // set delegate
        self.delegate = delegate

        // initialize arrays
        allDevices = []
        availableDevices = []

        // call superclass
        super.init()

        // create proxy and central manager
        proxy = NAOSManagerProxy(parent: self)
        centralManager = CBCentralManager(delegate: proxy, queue: nil)
    }

    // NAOSDevice

    internal func didPrepareDevice(device: NAOSDevice) {
        // add available device
        availableDevices.append(device)

        // call callback if present
        if let d = delegate {
            d.naosManagerDidPrepareDevice(manager: self, device: device)
        }
    }

    internal func failedToPrepareDevice(device _: NAOSDevice, error: Error) {
        // call callback if available
        if let d = delegate {
            d.naosManagerDidFailToPrepareDevice(manager: self, error: error)
        }
    }

    internal func didUpdateDevice(device: NAOSDevice) {
        // call callback if available
        if let d = delegate {
            d.naosManagerDidUpdateDevice(manager: self, device: device)
        }
    }

    // NAOSManagerProxy

    internal func centralManagerDidUpdateState(state: CBManagerState) {
        if state == .poweredOn {
            // start scanning
            centralManager.scanForPeripherals(withServices: [NAOSPrimaryServiceUUID], options: nil)
        } else if state == .poweredOff {
            // clear all arrays
            allDevices.removeAll()
            availableDevices.removeAll()

            // call callback if present
            if let d = delegate {
                d.naosManagerDidReset(manager: self)
            }
        }
    }

    internal func centralManagerDidDiscover(peripheral: CBPeripheral) {
        // return if peripheral is already attached to a device
        if deviceForPeripheral(peripheral: peripheral) != nil {
            return
        }

        // create new device
        let device = NAOSDevice(peripheral: peripheral, manager: self)

        // add device
        allDevices.append(device)
    }

    internal func centralManagerDidConnect(peripheral: CBPeripheral) {
        // forward connect event
        if let d = deviceForPeripheral(peripheral: peripheral) {
            d.forwardDidConnect()
        }
    }

    internal func centralManagerDidFailToConnect(peripheral: CBPeripheral, error: Error?) {
        // forward fail to connect event
        if let d = deviceForPeripheral(peripheral: peripheral) {
            d.forwardDidFailToConnect(error: error)
        }
    }

    internal func centralManagerDidDisconnect(peripheral: CBPeripheral, error: Error?) {
        // forward disconnect event
        if let d = deviceForPeripheral(peripheral: peripheral) {
            d.forwardDidDisconnect(error: error)
        }
    }

    // Helpers

    private func deviceForPeripheral(peripheral: CBPeripheral) -> NAOSDevice? {
        // search device in list
        for device in allDevices {
            if device.peripheral == peripheral {
                return device
            }
        }

        return nil
    }
}
