//
//  Utils.swift
//  NAOS
//
//  Created by Joël Gähwiler on 26.10.22.
//  Copyright © 2022 Joël Gähwiler. All rights reserved.
//

import Cocoa

struct CustomError: LocalizedError {
	var title: String?
	var errorDescription: String?

	init(title: String) {
		self.title = title
		self.errorDescription = title
	}
}

public func showError(error: Error) {
	// show error
	DispatchQueue.main.async {
		let alert = NSAlert()
		alert.messageText = error.localizedDescription
		alert.runModal()
	}
}
