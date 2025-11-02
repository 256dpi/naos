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

/// List all known serial ports on Darwin based systems.
public func NAOSSerialListPorts() -> [String] {
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

	return filtered.map { devPath + "/" + $0 }
}

/// A serial NAOS device.
public class NAOSSerialDevice: NAOSDevice {
	private let path: String
	private let displayName: String?
	private let baudRate: Int
	private let lock = NSLock()
	private weak var channel: serialChannel?

	public init(path: String, name: String? = nil, baudRate: Int = 115_200) {
		self.path = path
		self.displayName = name
		self.baudRate = baudRate
	}

	public func id() -> String {
		return "serial/" + URL(fileURLWithPath: path).lastPathComponent
	}

	public func name() -> String {
		return displayName ?? URL(fileURLWithPath: path).lastPathComponent
	}

	public func open() async throws -> NAOSChannel {
		return try lock.withLock {
			if channel != nil {
				throw NAOSSerialError.channelAlreadyOpen
			}

			let ch = try serialChannel(path: path, baudRate: baudRate, device: self)
			channel = ch
			return ch
		}
	}

	fileprivate func didClose(channel: serialChannel) {
		lock.lock()
		defer { lock.unlock() }

		if self.channel === channel {
			self.channel = nil
		}
	}
}

private final class serialChannel: NAOSChannel {
	private weak var device: NAOSSerialDevice?
	private let fd: Int32
	private let readSource: DispatchSourceRead
	private let writeQueue: DispatchQueue
	private let stateLock = NSLock()
	private var buffers = Data()
	private var queues: [NAOSQueue] = []
	private var closed = false

	init(path: String, baudRate: Int, device: NAOSSerialDevice) throws {
		self.device = device

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

		readSource.setEventHandler { [weak self] in
			self?.handleRead()
		}

		readSource.setCancelHandler { [fd] in
			Darwin.close(fd)
		}

		readSource.resume()
	}

	public func subscribe(queue: NAOSQueue) {
		stateLock.lock()
		defer { stateLock.unlock() }

		if queues.first(where: { $0 === queue }) == nil {
			queues.append(queue)
		}
	}

	public func unsubscribe(queue: NAOSQueue) {
		stateLock.lock()
		defer { stateLock.unlock() }

		queues.removeAll { $0 === queue }
	}

	public func write(data: Data) async throws {
		let payload = preparePayload(data: data)
		try await withCheckedThrowingContinuation { (continuation: CheckedContinuation<Void, Error>) in
			writeQueue.async { [weak self] in
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

	public func close() {
		var shouldCancel = false
		stateLock.lock()
		if !closed {
			closed = true
			shouldCancel = true
			queues.removeAll()
		}
		stateLock.unlock()

		if shouldCancel {
			readSource.cancel()
			device?.didClose(channel: self)
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
				deliver(data: message)
			}
		}
	}

	private func deliver(data: Data) {
		stateLock.lock()
		let targets = queues
		stateLock.unlock()

		for queue in targets {
			queue.send(value: data)
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
