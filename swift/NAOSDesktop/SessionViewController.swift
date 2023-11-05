//
//  Created by Joël Gähwiler on 15.08.23.
//  Copyright © 2023 Joël Gähwiler. All rights reserved.
//

import Cocoa
import NAOSKit

internal class SessionViewController: NSViewController {
	internal var device: NAOSDevice?
	
	internal func run(title: String, operation: @escaping (NAOSSession) async throws -> Void) async {
		// show loading view controller
		let lvc = NSStoryboard(name: "Main", bundle: nil).instantiateController(withIdentifier: "LoadingViewController") as! LoadingViewController
		lvc.message = title
		lvc.preferredContentSize = CGSize(width: 200, height: 200)

		// present view controller
		presentAsSheet(lvc)
		
		// run task
		let task = Task {
			// open session
			let session = try await device!.session(timeout: 5)
			defer { session.cleanup() }
			
			// run operation
			try await operation(session)
			
			// end session
			try await session.end(timeout: 5)
		}
		
		// set cancel action
		lvc.onCancel {
			task.cancel()
		}

		// run operation and dismiss controller
		do {
			try await task.value
			lvc.dismiss(lvc)
		} catch {
			lvc.dismiss(lvc)
			showError(error: error)
		}
	}
	
	internal func process(title: String, operation: @escaping (NAOSSession, @escaping (Double, Double) -> Void) async throws -> Void) async {
		// show loading view controller
		let lvc = NSStoryboard(name: "Main", bundle: nil).instantiateController(withIdentifier: "LoadingViewController") as! LoadingViewController
		lvc.message = title
		lvc.preferredContentSize = CGSize(width: 200, height: 200)
		DispatchQueue.main.async {
			lvc.indicator.isIndeterminate = false
		}

		// present view controller
		presentAsSheet(lvc)
		
		// run task
		let task = Task.detached {
			// open session
			let session = try await self.device!.session(timeout: 5)
			defer { session.cleanup() }
			
			// run operation
			try await operation(session) { progress, rate in
				DispatchQueue.main.async {
					lvc.indicator.doubleValue = progress * 100
					if rate > 0 {
						lvc.label.stringValue = String(format: title + "\n%.1f %% @ %.1f kB/s", progress * 100, rate / 1000)
					}
				}
			}
			
			// end session
			try await session.end(timeout: 5)
		}
		
		// set cancel action
		lvc.onCancel {
			task.cancel()
		}
		
		// run operation and dismiss controller
		do {
			try await task.value
			lvc.dismiss(lvc)
		} catch {
			lvc.dismiss(lvc)
			showError(error: error)
		}
	}
}
