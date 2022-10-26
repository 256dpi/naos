//
// Created by Joël Gähwiler on 05.04.17.
// Copyright (c) 2017 Joël Gähwiler. All rights reserved.
//

import Cocoa
import CoreBluetooth

class NAOSManagerProxy: NSObject, CBCentralManagerDelegate {
	var parent: NAOSManager

	init(parent: NAOSManager) {
		self.parent = parent
		super.init()
	}

	public func centralManagerDidUpdateState(_ centralManager: CBCentralManager) {
		parent.centralManagerDidUpdateState(state: centralManager.state)
	}

	public func centralManager(_: CBCentralManager, didDiscover peripheral: CBPeripheral, advertisementData _: [String: Any], rssi _: NSNumber) {
		parent.centralManagerDidDiscover(peripheral: peripheral)
	}

	public func centralManager(_: CBCentralManager, didConnect peripheral: CBPeripheral) {
		parent.centralManagerDidConnect(peripheral: peripheral)
	}

	public func centralManager(_: CBCentralManager, didFailToConnect peripheral: CBPeripheral, error: Error?) {
		parent.centralManagerDidFailToConnect(peripheral: peripheral, error: error)
	}

	public func centralManager(_: CBCentralManager, didDisconnectPeripheral peripheral: CBPeripheral, error: Error?) {
		parent.centralManagerDidDisconnect(peripheral: peripheral, error: error)
	}
}
