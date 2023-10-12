//
//  Created by Joël Gähwiler on 02.08.23.
//  Copyright © 2023 Joël Gähwiler. All rights reserved.
//

import Cocoa
import CryptoKit
import NAOSKit

class FilesViewController: EndpointViewController, NSTableViewDataSource, NSTableViewDelegate {
	internal var endpoint: NAOSFSEndpoint!
	@IBOutlet var pathField: NSTextField!
	@IBOutlet var listTable: NSTableView!
	
	internal var files: [NAOSFSInfo] = []
	
	private func root() -> String {
		return pathField.stringValue
	}
	
	@IBAction public func list(_: AnyObject) {
		Task {
			// list directory
			await run(title: "Listing...") {
				self.files = try await self.endpoint.list(path: self.root())
			}
			
			// reload list
			self.listTable.reloadData()
		}
	}
	
	@IBAction public func upload(_: AnyObject) {
		Task {
			// open file
			let file = try await openFile()
			
			// prepare path
			let path = self.root() + "/" + file.0
			
			// write file
			await process(title: "Uploading...") { progress in
				let start = Date()
				try await self.endpoint.write(path: path, data: file.1, report: { done in
					let diff = Date().timeIntervalSince(start)
					progress(Double(done) / Double(file.1.count), Double(done) / diff)
				})
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
			await process(title: "Downloading...") { progress in
				let start = Date()
				data = try await self.endpoint.read(path: self.root() + "/" + file.name, report: { done in
					let diff = Date().timeIntervalSince(start)
					progress(Double(done) / Double(file.size), Double(done) / diff)
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
			guard let name = await prompt(message: "New Name:", defaultValue: file.name) else {
				return
			}
			
			// rename file
			await run(title: "Renaming...") {
				try await self.endpoint.rename(from: self.root() + "/" + file.name, to: self.root() + "/" + name)
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
			// rename file
			await run(title: "Removing...") {
				try await self.endpoint.remove(path: self.root() + "/" + file.name)
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
			await run(title: "Hashing...") {
				sum = try await self.endpoint.sha256(path: self.root() + "/" + file.name)
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
	
	@IBAction public func close(_: AnyObject) {
		// dismiss sheet
		dismiss(self)
		
		Task {
			// close endpoint
			try await endpoint.end()
		}
	}

	// NSTableView
	
	func numberOfRows(in _: NSTableView) -> Int {
		return files.count
	}
	
	func tableView(_ tableView: NSTableView, viewFor tableColumn: NSTableColumn?, row: Int) -> NSView? {
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
