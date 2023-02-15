// swift-tools-version: 5.7

import PackageDescription

let package = Package(
	name: "NAOSKit",
	platforms: [
		.macOS(.v11),
	],
	products: [
		.library(
			name: "NAOSKit",
			targets: ["NAOSKit"]),
	],
	dependencies: [
		.package(url: "https://github.com/manolofdez/AsyncBluetooth", from: "1.4.1"),
	],
	targets: [
		.target(
			name: "NAOSKit",
			dependencies: ["AsyncBluetooth"],
			path: "Sources"),
	])
