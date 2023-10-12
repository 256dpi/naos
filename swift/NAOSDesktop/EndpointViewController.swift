//
//  Created by Joël Gähwiler on 15.08.23.
//  Copyright © 2023 Joël Gähwiler. All rights reserved.
//

import Cocoa

internal class EndpointViewController: NSViewController {
	internal func run(title: String, operation: () async throws -> Void) async {
		// show loading view controller
		let lvc = NSStoryboard(name: "Main", bundle: nil).instantiateController(withIdentifier: "LoadingViewController") as! LoadingViewController
		lvc.message = title
		lvc.preferredContentSize = CGSize(width: 200, height: 200)

		// present view controller
		presentAsSheet(lvc)

		// run operation and dismiss controller
		do {
			try await operation()
			lvc.dismiss(lvc)
		} catch {
			lvc.dismiss(lvc)
			showError(error: error)
		}
	}
	
	internal func process(title: String, operation: (@escaping (Double, Double) -> Void) async throws -> Void) async {
		// show loading view controller
		let lvc = NSStoryboard(name: "Main", bundle: nil).instantiateController(withIdentifier: "LoadingViewController") as! LoadingViewController
		lvc.message = title
		lvc.preferredContentSize = CGSize(width: 200, height: 200)
		DispatchQueue.main.async {
			lvc.progressIndicator.isIndeterminate = false
		}

		// present view controller
		presentAsSheet(lvc)

		// run operation and dismiss controller
		do {
			try await operation({ (progress, rate) in
				DispatchQueue.main.async {
					lvc.progressIndicator.doubleValue = progress * 100
					if rate > 0 {
						lvc.label.stringValue = String(format: title + "\n%.1f %% @ %.1f kB/s", progress * 100, rate / 1000)
					}
				}
			})
			lvc.dismiss(lvc)
		} catch {
			lvc.dismiss(lvc)
			showError(error: error)
		}
	}
}
