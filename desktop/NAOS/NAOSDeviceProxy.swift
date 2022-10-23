//
//  Created by Joël Gähwiler on 06.04.17.
//  Copyright © 2017 Joël Gähwiler. All rights reserved.
//

import Cocoa
import CoreBluetooth

public class NAOSDeviceProxy: NSObject, CBPeripheralDelegate {
	var parent: NAOSDevice

	init(parent: NAOSDevice) {
		self.parent = parent
		super.init()
	}

	public func peripheral(_: CBPeripheral, didDiscoverServices error: Error?) {
		parent.peripheralDidDiscoverServices(error: error)
	}

	public func peripheral(_: CBPeripheral, didDiscoverCharacteristicsFor service: CBService, error: Error?) {
		parent.peripheralDidDiscoverCharacteristicsFor(service: service, error: error)
	}

	public func peripheral(_: CBPeripheral, didUpdateValueFor characteristic: CBCharacteristic, error: Error?) {
		parent.peripheralDidUpdateValueFor(characteristic: characteristic, error: error)
	}

	public func peripheral(_: CBPeripheral, didWriteValueFor characteristic: CBCharacteristic, error: Error?) {
		parent.peripheralDidWriteValueFor(characteristic: characteristic, error: error)
	}
}
