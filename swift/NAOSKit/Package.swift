// swift-tools-version: 5.7

import PackageDescription

let package = Package(
	name: "NAOSKit",
	platforms: [
		.macOS(.v13),
		.iOS(.v16),
	],
	products: [
		.library(
			name: "NAOSKit",
			targets: ["NAOSKit"])
	],
	dependencies: [
		.package(url: "https://github.com/manolofdez/AsyncBluetooth", revision: "4.0.0"),
		.package(url: "https://github.com/groue/Semaphore", from: "0.1.0"),
	],
	targets: [
		.target(
			name: "NAOSKit",
			dependencies: ["AsyncBluetooth", "Semaphore"],
			path: "Sources")
	])
