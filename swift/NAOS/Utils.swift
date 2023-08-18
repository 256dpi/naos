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

func openFile() async throws -> (String, Data) {
	return try await MainActor.run {
		// prepare panel
		let openPanel = NSOpenPanel()
		openPanel.canChooseFiles = true
		openPanel.canChooseDirectories = false
		openPanel.allowsMultipleSelection = false
		
		// run panel
		let result = openPanel.runModal()
		if result != .OK {
			throw NSError(domain: NSCocoaErrorDomain, code: NSUserCancelledError, userInfo: nil)
		}

		// get URL
		guard let url = openPanel.urls.first else {
			throw NSError(domain: NSCocoaErrorDomain, code: NSFileNoSuchFileError, userInfo: nil)
		}
		
		// get name and content
		let name = url.lastPathComponent
		let content = try Data(contentsOf: url)

		return (name, content)
	}
}

func saveFile(withName name: String, data: Data) async throws {
	try await MainActor.run {
		// prepare panel
		let savePanel = NSSavePanel()
		savePanel.nameFieldStringValue = name
		
		// run panel
		let result = savePanel.runModal()
		if result != .OK {
			throw NSError(domain: NSCocoaErrorDomain, code: NSUserCancelledError, userInfo: nil)
		}
		
		// get URL
		guard let url = savePanel.url else {
			throw NSError(domain: NSCocoaErrorDomain, code: NSFileWriteUnknownError, userInfo: nil)
		}
		
		// write data to file
		try data.write(to: url)
	}
}

func prompt(message: String, defaultValue: String? = nil) async -> String? {
	return await MainActor.run {
		// prepare aleert
		let alert = NSAlert()
		alert.messageText = message
		alert.addButton(withTitle: "OK")
		alert.addButton(withTitle: "Cancel")
		
		// prepare text field
		let textField = NSTextField(frame: NSRect(x: 0, y: 0, width: 200, height: 24))
		textField.stringValue = defaultValue ?? ""
		alert.accessoryView = textField
		
		// handle resposne
		let response = alert.runModal()
		if response == .alertFirstButtonReturn {
			return textField.stringValue
		} else {
			return nil
		}
	}
}
