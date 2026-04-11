import Dispatch
import Foundation
import Darwin

private let serialLinePrefix = Data("NAOS!".utf8)
private let serialFramingPrefix = Data("\nNAOS!".utf8)
private let serialNewline = Data([0x0A])
private let knownSerialPrefixes = ["cu.SLAB", "cu.usbserial", "cu.usbmodem", "ttyUSB"]

/// Serial specific errors.
public enum NAOSSerialError: LocalizedError {
	case channelAlreadyOpen
	case closed

	public var errorDescription: String? {
		switch self {
		case .channelAlreadyOpen:
			return "Serial channel already open."
		case .closed:
			return "Serial channel has been closed."
		}
	}
}

/// A discovered serial device descriptor.
public struct NAOSSerialDescriptor: Hashable, Sendable {
	public let path: String
	public let name: String

	init(path: String) {
		self.path = path
		self.name = URL(fileURLWithPath: path).lastPathComponent
	}

	public func hash(into hasher: inout Hasher) {
		hasher.combine(path)
	}

	public static func == (lhs: NAOSSerialDescriptor, rhs: NAOSSerialDescriptor) -> Bool {
		lhs.path == rhs.path
	}
}

/// List all known serial ports on Darwin based systems.
public func NAOSSerialListPorts() -> [NAOSSerialDescriptor] {
	let devPath = "/dev"
	guard let entries = try? FileManager.default.contentsOfDirectory(atPath: devPath) else {
		return []
	}

	let filtered = entries.filter { entry in
		for prefix in knownSerialPrefixes {
			if entry.contains(prefix) {
				return true
			}
		}
		return false
	}.sorted(by: >)

	return filtered.map { NAOSSerialDescriptor(path: devPath + "/" + $0) }
}

public class NAOSSerialDevice: NAOSDevice {
	private let path: String
	private let baudRate: Int
	private let lock = NSLock()
	private weak var channel: NAOSChannel?

	public init(path: String, baudRate: Int = 115_200) {
		self.path = path
		self.baudRate = baudRate
	}
	
	public func type() -> String {
		return "Serial"
	}

	public func id() -> String {
		return "serial/" + URL(fileURLWithPath: path).lastPathComponent
	}

	public func name() -> String {
		return URL(fileURLWithPath: path).lastPathComponent
	}

	public func open() async throws -> NAOSChannel {
		return try lock.withLock {
			if channel != nil {
				throw NAOSSerialError.channelAlreadyOpen
			}

			let transport = try serialTransport(path: path, baudRate: baudRate)
			let ch = NAOSChannel(transport: transport, device: self, width: 1) { [weak self] in
				self?.didClose()
			}
			channel = ch
			return ch
		}
	}

	fileprivate func didClose() {
		lock.lock()
		defer { lock.unlock() }

		self.channel = nil
	}
}

private actor serialStreamReader {
	private var iterator: AsyncThrowingStream<Data, Error>.Iterator

	init(stream: AsyncThrowingStream<Data, Error>) {
		self.iterator = stream.makeAsyncIterator()
	}

	func next() async throws -> Data? {
		var iterator = self.iterator
		let value = try await iterator.next()
		self.iterator = iterator
		return value
	}
}

private final class serialTransport: NAOSTransport {
	private let fd: Int32
	private let readSource: DispatchSourceRead
	private let writeQueue: DispatchQueue
	private let streamContinuation: AsyncThrowingStream<Data, Error>.Continuation
	private let reader: serialStreamReader
	private let stateLock = NSLock()
	private var buffers = Data()
	private var closed = false

	init(path: String, baudRate: Int) throws {
		let descriptor = path.withCString { ptr -> Int32 in
			return Darwin.open(ptr, O_RDWR | O_NOCTTY | O_NONBLOCK)
		}

		if descriptor < 0 {
			throw POSIXError(POSIXErrorCode(rawValue: errno) ?? .EIO)
		}

		// configure serial port
		do {
			try configureSerial(fd: descriptor, baudRate: baudRate)
		} catch {
			Darwin.close(descriptor)
			throw error
		}

		fd = descriptor

		writeQueue = DispatchQueue(label: "naos.serial.write.\(UUID().uuidString)")
		readSource = DispatchSource.makeReadSource(fileDescriptor: fd, queue: DispatchQueue(label: "naos.serial.read.\(UUID().uuidString)"))
		var continuation: AsyncThrowingStream<Data, Error>.Continuation?
		let stream = AsyncThrowingStream<Data, Error> { continuation = $0 }
		self.streamContinuation = continuation!
		self.reader = serialStreamReader(stream: stream)

		readSource.setEventHandler { [weak self] in
			self?.handleRead()
		}

		readSource.setCancelHandler { [fd] in
			Darwin.close(fd)
		}

		readSource.resume()
	}

	func read() async throws -> Data {
		guard let data = try await reader.next() else {
			throw NAOSTransportError.closed
		}
		return data
	}

	func write(data: Data) async throws {
		let payload = preparePayload(data: data)
		try await withCheckedThrowingContinuation { (continuation: CheckedContinuation<Void, Error>) in
			self.writeQueue.async { [weak self] in
				guard let self else {
					continuation.resume(throwing: NAOSSerialError.closed)
					return
				}

				do {
					try self.performWrite(data: payload)
					continuation.resume(returning: ())
				} catch {
					continuation.resume(throwing: error)
				}
			}
		}
	}

	func close() {
		var shouldCancel = false
		stateLock.lock()
		if !closed {
			closed = true
			shouldCancel = true
		}
		stateLock.unlock()

		if shouldCancel {
			streamContinuation.finish()
			readSource.cancel()
		}
	}

	private func preparePayload(data: Data) -> Data {
		var payload = Data()
		payload.append(serialFramingPrefix)
		payload.append(data.base64EncodedData())
		payload.append(serialNewline)
		return payload
	}

	private func performWrite(data: Data) throws {
		stateLock.lock()
		let isClosed = closed
		stateLock.unlock()
		if isClosed {
			throw NAOSSerialError.closed
		}

		try data.withUnsafeBytes { ptr in
			guard let base = ptr.baseAddress else {
				return
			}

			var written = 0
			while written < data.count {
				let result = Darwin.write(fd, base.advanced(by: written), data.count - written)
				if result > 0 {
					written += result
					continue
				}

				if result == -1 && (errno == EINTR || errno == EAGAIN) {
					continue
				}

				throw POSIXError(POSIXErrorCode(rawValue: errno) ?? .EIO)
			}
		}
	}

	private func handleRead() {
		var tempBuffer = [UInt8](repeating: 0, count: 4096)

		while true {
			let count = tempBuffer.withUnsafeMutableBytes { ptr -> Int in
				guard let base = ptr.baseAddress else {
					return 0
				}
				return Darwin.read(fd, base, ptr.count)
			}

			if count > 0 {
				buffers.append(contentsOf: tempBuffer[0..<count])
				processBuffer()
			} else if count == 0 {
				close()
				break
			} else {
				if errno == EAGAIN || errno == EINTR {
					break
				}
				close()
				break
			}
		}
	}

	private func processBuffer() {
		while let newlineIndex = buffers.firstIndex(of: 0x0A) {
			let lineData = buffers.prefix(upTo: newlineIndex)
			buffers.removeSubrange(..<buffers.index(after: newlineIndex))

			if lineData.isEmpty {
				continue
			}

			var trimmed = lineData
			if let last = trimmed.last, last == 0x0D {
				trimmed = trimmed.dropLast()
			}

			var content = trimmed
			if content.first == 0x0A {
				content = content.dropFirst()
			}

			guard content.count >= serialLinePrefix.count else {
				continue
			}
			if !content.starts(with: serialLinePrefix) {
				continue
			}

			let base64Data = content.dropFirst(serialLinePrefix.count)
			if let message = Data(base64Encoded: Data(base64Data), options: .ignoreUnknownCharacters) {
				streamContinuation.yield(message)
			}
		}
	}

	deinit {
		close()
	}
}

private func configureSerial(fd: Int32, baudRate: Int) throws {
	var config = termios()
	if tcgetattr(fd, &config) != 0 {
		throw POSIXError(POSIXErrorCode(rawValue: errno) ?? .EIO)
	}

	cfmakeraw(&config)
	if cfsetspeed(&config, speed_t(baudRate)) != 0 {
		throw POSIXError(POSIXErrorCode(rawValue: errno) ?? .EIO)
	}

	config.c_cflag |= tcflag_t(CLOCAL | CREAD)
	config.c_cflag &= ~tcflag_t(PARENB | CSTOPB | CSIZE)
	config.c_cflag |= tcflag_t(CS8)

	if tcsetattr(fd, TCSANOW, &config) != 0 {
		throw POSIXError(POSIXErrorCode(rawValue: errno) ?? .EIO)
	}
}
