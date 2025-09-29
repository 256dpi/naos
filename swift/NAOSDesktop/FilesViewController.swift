//
//  Created by Joël Gähwiler on 02.08.23.
//  Copyright © 2023 Joël Gähwiler. All rights reserved.
//

import Cocoa
import CryptoKit
import NAOSKit

class FilesViewController: SessionViewController, NSTableViewDataSource, NSTableViewDelegate {
	@IBOutlet var pathField: NSTextField!
	@IBOutlet var listTable: NSTableView!

	var files: [NAOSFSInfo] = []

	private func root() -> String {
		return pathField.stringValue
	}

	@IBAction public func list(_: AnyObject) {
		Task {
			// list directory
			await run(title: "Listing...") { session in
				self.files = try await NAOSFS.list(session: session, dir: self.root())
			}

			// reload list
			self.listTable.reloadData()
		}
	}

	@IBAction public func upload(_: AnyObject) {
		Task {
			// open file
			let files = try await openFiles()

			// iterate files
			for file in files {
				// prepare path
				let path = self.root() + "/" + file.name

				// write file
				await process(title: "Uploading...") { session, progress in
					// get time
					let start = Date()

					// write file
					try await NAOSFS.write(
						session: session,
						file: path, data: file.data,
						report: { done in
							// calculate delta
							let delta = Date().timeIntervalSince(start)

							// report progress
							progress(
								Double(done)
									/ Double(file.data.count),
								Double(done) / delta)
						})
				}
			}

			// re-list
			self.list(self)
		}
	}

	@IBAction public func download(_: AnyObject) {
		// check selected row
		if listTable.selectedRow < 0 {
			return
		}

		// get file
		let file = files[listTable.selectedRow]

		Task {
			// read file
			var data: Data?
			await process(title: "Downloading...") { session, progress in
				// get time
				let start = Date()

				// read file
				data = try await NAOSFS.read(
					session: session,
					file: self.root() + "/" + file.name,
					report: { done in
						// calculate delta
						let delta = Date().timeIntervalSince(start)

						// report progress
						progress(
							Double(done) / Double(file.size),
							Double(done) / delta)
					})
			}

			// save file
			if data != nil {
				try await saveFile(withName: file.name, data: data!)
			}
		}
	}

	@IBAction public func rename(_: AnyObject) {
		// check selected row
		if listTable.selectedRow < 0 {
			return
		}

		// get file
		let file = files[listTable.selectedRow]

		Task {
			// request new name
			guard let name = await prompt(message: "New Name:", defaultValue: file.name)
			else {
				return
			}

			// rename file
			await run(title: "Renaming...") { session in
				// rename file
				try await NAOSFS.rename(
					session: session,
					from: self.root() + "/" + file.name,
					to: self.root() + "/" + name)
			}

			// re-list
			self.list(self)
		}
	}

	@IBAction public func remove(_: AnyObject) {
		// check selected row
		if listTable.selectedRow < 0 {
			return
		}

		// get file
		let file = files[listTable.selectedRow]

		Task {
			// remove file
			await run(title: "Removing...") { session in
				try await NAOSFS.remove(session: session, path: self.root() + "/" + file.name)
			}

			// re-list
			self.list(self)
		}
	}

	@IBAction public func verify(_: AnyObject) {
		// check selected row
		if listTable.selectedRow < 0 {
			return
		}

		// get file
		let file = files[listTable.selectedRow]

		Task {
			// hash file
			var sum: Data?
			await run(title: "Hashing...") { session in
				sum = try await NAOSFS.sha256(session: session, file: self.root() + "/" + file.name)
			}
			if sum == nil {
				return
			}

			// open file
			let (_, content) = try await openFile()

			// compare checksums
			if sum!.elementsEqual(SHA256.hash(data: content)) {
				showError(error: CustomError(title: "Files are equal."))
			} else {
				showError(error: CustomError(title: "Files are not equal."))
			}
		}
	}
	
	@IBAction public func make(_: AnyObject) {
		Task {
			// request new name
			guard let name = await prompt(message: "Path:", defaultValue: self.root())
			else {
				return
			}

			// make path
			await run(title: "Making...") { session in
				try await NAOSFS.make(session: session, path: name)
			}

			// re-list
			self.list(self)
		}
	}

	@IBAction public func close(_: AnyObject) {
		// dismiss sheet
		dismiss(self)
	}

	// NSTableView

	func numberOfRows(in _: NSTableView) -> Int {
		return files.count
	}

	func tableView(_ tableView: NSTableView, viewFor tableColumn: NSTableColumn?, row: Int)
		-> NSView?
	{
		// get file
		let f = files[row]

		// return name cell
		if tableView.tableColumns[0] == tableColumn {
			let v =
				tableView.makeView(
					withIdentifier: NSUserInterfaceItemIdentifier("NameCell"),
					owner: nil) as! NSTableCellView
			v.textField!.stringValue = f.name
			return v
		}

		// return type cell
		if tableView.tableColumns[1] == tableColumn {
			let v =
				tableView.makeView(
					withIdentifier: NSUserInterfaceItemIdentifier("TypeCell"),
					owner: nil) as! NSTableCellView
			v.textField!.stringValue = f.isDir ? "D" : "F"
			return v
		}

		// return size cell
		if tableView.tableColumns[2] == tableColumn {
			let v =
				tableView.makeView(
					withIdentifier: NSUserInterfaceItemIdentifier("SizeCell"),
					owner: nil) as! NSTableCellView
			v.textField!.stringValue = String(f.size)
			return v
		}

		return nil
	}
}
