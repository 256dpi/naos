//
//  Created by Joël Gähwiler on 05.01.2025.
//  Copyright © 2025 Joël Gähwiler. All rights reserved.
//

import Charts
import NAOSKit
import SwiftUI

struct MetricsSample: Identifiable {
	var id = UUID()
	var name: String
	var time: Date
	var value: Double

	init(name: String, time: Date, value: Double) {
		self.name = name
		self.time = time
		self.value = value
	}
}

class MetricsSeries: Identifiable {
	var id = UUID()
	var name: String
	var samples: [MetricsSample]

	init(name: String, samples: [MetricsSample]) {
		self.name = name
		self.samples = samples
	}

	func info(time: Date?) -> String {
		let samples = time != nil ? atTime(time: time!) : last()
		return samples.map { sample in
			"\(sample.name): \(sample.value)"
		}.joined(separator: ", ")
	}

	func atTime(time: Date) -> [MetricsSample] {
		let needle = samples.first { s in
			s.time > time
		}
		return samples.filter { s in
			s.time == needle?.time
		}
	}

	func last() -> [MetricsSample] {
		return samples.filter { s in
			s.time == samples.last?.time
		}
	}
}

class MetricsContainer: ObservableObject {
	@Published var series: [MetricsSeries]

	init(series: [MetricsSeries]) {
		self.series = series
	}
}

struct MetricsView: View {
	@ObservedObject var data: MetricsContainer

	@State private var hoverDate: Date?

	var body: some View {
		ScrollView {
			ForEach(data.series) { series in
				VStack {
					HStack {
						Label(title: { Text(series.name) }, icon: {})
						Spacer()
						Label(title: {
							Text(series.info(time: hoverDate))
						}, icon: {}).font(.footnote)
					}
					Chart(series.samples) {
						LineMark(
							x: .value("Date", $0.time),
							y: .value("Value", $0.value)
						)
						.foregroundStyle(by: .value("Name", $0.name))
						if let hoverDate {
							RectangleMark(x: .value("Date", hoverDate))
								.foregroundStyle(.primary.opacity(0.1))
						}
					}
					.chartOverlay { (chartProxy: ChartProxy) in
						Color.clear
							.onContinuousHover { hoverPhase in
								switch hoverPhase {
								case .active(let hoverLocation):
									hoverDate = chartProxy.value(atX: hoverLocation.x, as: Date.self)
								case .ended:
									hoverDate = nil
								}
							}
					}
				}.padding()
			}
		}.frame(minWidth: 400, idealWidth: 600, minHeight: 450, idealHeight: 800)
	}
}

class MetricsViewController: NSHostingController<MetricsView> {
	public var device: NAOSManagedDevice?
	private var task: Task<Void, Error>?

	func collect(device: NAOSManagedDevice) {
		// get data
		let container = rootView.data
		var metrics = [NAOSMetricInfo]()
		var layouts = [NAOSMetricLayout]()

		// create task
		task = Task {
			// ensure close on error
			defer { self.dismiss(nil) }

			// get sessions
			let session = try await device.newSession()

			// gather metrics
			metrics = try await NAOSMetrics.list(session: session)
			for m in metrics {
				let layout = try await NAOSMetrics.describe(session: session, ref: m.ref)
				layouts.insert(layout, at: Int(m.ref))
			}

			// create series
			for m in metrics {
				container.series.append(MetricsSeries(name: m.name, samples: []))
			}

			// get data
			while !Task.isCancelled {
				// load metrics data
				for m in metrics {
					// read data
					var data: [Double]
					switch m.type {
					case .long:
						data = try await NAOSMetrics.readLong(session: session, ref: m.ref).map { n in
							Double(n)
						}
					case .float:
						data = try await NAOSMetrics.readFloat(session: session, ref: m.ref).map { n in
							Double(n)
						}
					case .double:
						data = try await NAOSMetrics.readDouble(session: session, ref: m.ref)
					}

					// get time
					let now = Date.now

					// add samples
					container.series[Int(m.ref)].samples.append(contentsOf: data.enumerated().map { i, n in
						// find keys and values
						var name = "scalar"
						if m.size > 1 {
							name = ""
							var offset = i
							let layout = layouts[Int(m.ref)]
							for key in layout.keys.enumerated().reversed() {
								let vn = offset % layout.values[key.offset].count
								offset /= layout.values[key.offset].count
								let vs = layout.values[key.offset][vn]
								name += "\(key.element)=\(vs) "
							}
							name = name.trimmingCharacters(in: CharacterSet(charactersIn: " "))
						}

						return MetricsSample(name: name, time: now, value: n)
					})

					// trim samples
					container.series[Int(m.ref)].samples = container.series[Int(m.ref)].samples.filter { sample in
						Date.now.timeIntervalSince(sample.time) < 30
					}
				}

				// trigger update
				container.series = container.series

				// wait a second
				try await Task.sleep(for: .seconds(0.1))
			}
		}
	}

	override func viewDidAppear() {
		super.viewDidAppear()

		// set window title
		view.window?.title = "Metrics"
	}

	override func viewWillDisappear() {
		super.viewWillDisappear()

		// cancel task
		task?.cancel()
	}
}

#Preview {
	MetricsView(data: MetricsContainer(series: [
		MetricsSeries(name: "Series 1", samples: [
			MetricsSample(name: "foo", time: Date(timeIntervalSinceNow: 1), value: 7),
			MetricsSample(name: "foo", time: Date(timeIntervalSinceNow: 2), value: 6),
			MetricsSample(name: "foo", time: Date(timeIntervalSinceNow: 3), value: 8),
			MetricsSample(name: "foo", time: Date(timeIntervalSinceNow: 4), value: 7),
			MetricsSample(name: "foo", time: Date(timeIntervalSinceNow: 5), value: 6),
			MetricsSample(name: "bar", time: Date(timeIntervalSinceNow: 1), value: 3),
			MetricsSample(name: "bar", time: Date(timeIntervalSinceNow: 5), value: 4),
		]), MetricsSeries(name: "Series 2", samples: [
			MetricsSample(name: "foo", time: Date(timeIntervalSinceNow: 1), value: 7),
			MetricsSample(name: "foo", time: Date(timeIntervalSinceNow: 2), value: 6),
			MetricsSample(name: "foo", time: Date(timeIntervalSinceNow: 3), value: 8),
			MetricsSample(name: "foo", time: Date(timeIntervalSinceNow: 4), value: 7),
			MetricsSample(name: "foo", time: Date(timeIntervalSinceNow: 5), value: 6),
		]),
	]))
}
