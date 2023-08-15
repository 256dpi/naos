//
//  Created by Joël Gähwiler on 15.08.23.
//  Copyright © 2023 Joël Gähwiler. All rights reserved.
//

import Cocoa

internal class EndpointViewController: NSViewController {
	internal func run(title: String, operation: @escaping () async throws -> Void) async {
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
}
