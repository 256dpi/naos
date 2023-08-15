//
//  Created by Joël Gähwiler on 02.08.23.
//  Copyright © 2023 Joël Gähwiler. All rights reserved.
//

import Cocoa
import NAOSKit

class FilesViewController: NSViewController, NSTableViewDataSource, NSTableViewDelegate {
	internal var endpoint: NAOSFSEndpoint!
	@IBOutlet var pathField: NSTextField!
	@IBOutlet var listTable: NSTableView!
	
	internal var files: [NAOSFSInfo] = []
	
	private func root() -> String {
		return pathField.stringValue
	}
	
	@IBAction public func list(_: AnyObject) {
		Task {
			do {
				// list directory
				files = try await endpoint.list(path: root())
				
				// reload list
				listTable.reloadData()
			} catch {
				showError(error: error)
			}
		}
	}
	
	@IBAction public func upload(_: AnyObject) {
		Task {
			do {
				// open file
				let file = try await openFile()
				
				// prepare path
				let path = root() + "/" + file.0
				
				// write file
				try await endpoint.write(path: path, data: file.1)
				
				// re-list
				list(self)
			} catch {
				showError(error: error)
			}
		}
	}
	
	@IBAction public func download(_: AnyObject) {
		Task {
			do {
				// check selected row
				if listTable.selectedRow < 0 {
					return
				}
				
				// get file
				let file = files[listTable.selectedRow]
				
				// read file
				let data = try await endpoint.read(path: root() + "/" + file.name)
				
				// save file
				try await saveFile(withName: file.name, data: data)
			} catch {
				showError(error: error)
			}
		}
	}
	
	@IBAction public func rename(_: AnyObject) {
		Task {
			do {
				// check selected row
				if listTable.selectedRow < 0 {
					return
				}
				
				// get file
				let file = files[listTable.selectedRow]
				
				// request new name
				guard let name = await prompt(message: "New Name:", defaultValue: file.name) else {
					return
				}
				
				// rename file
				try await endpoint.rename(from: root() + "/" + file.name, to: root() + "/" + name)
				
				// re-list
				list(self)
			} catch {
				showError(error: error)
			}
		}
	}
	
	@IBAction public func remove(_: AnyObject) {
		Task {
			do {
				// check selected row
				if listTable.selectedRow < 0 {
					return
				}
				
				// get file
				let file = files[listTable.selectedRow]
				
				// rename file
				try await endpoint.remove(path: root() + "/" + file.name)
				
				// re-list
				list(self)
			} catch {
				showError(error: error)
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
