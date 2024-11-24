// swift-tools-version: 5.7

import PackageDescription

let package = Package(
	name: "NAOSKit",
	platforms: [
		.macOS(.v12),
		.iOS(.v14),
	],
	products: [
		.library(
			name: "NAOSKit",
			targets: ["NAOSKit"])
	],
	dependencies: [
		.package(url: "https://github.com/manolofdez/AsyncBluetooth", revision: "4.0.0"),
		.package(url: "https://github.com/groue/Semaphore", from: "0.0.8"),
	],
	targets: [
		.target(
			name: "NAOSKit",
			dependencies: ["AsyncBluetooth", "Semaphore"],
			path: "Sources")
	])
