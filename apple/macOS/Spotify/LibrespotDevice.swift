import AppKit
import Foundation

enum LibrespotError: LocalizedError {
    case notInstalled, authorizationTimedOut, exited(String)

    var errorDescription: String? {
        switch self {
        case .notInstalled: "librespot isn’t installed. Run `make librespot` in the AudioPulse repo, then try again."
        case .authorizationTimedOut: "Timed out waiting for the AudioPulse device to be approved."
        case .exited(let tail): "The playback device stopped: \(tail)"
        }
    }
}

/// Runs librespot as the "AudioPulse" Spotify Connect device, mirroring
/// `internal/librespot`: same arguments (rodio backend on macOS) and the same
/// credential cache, so a device authorized by the TUI works here too.
@MainActor
final class LibrespotDevice {
    private var process: Process?
    private var keepAlive = false
    private var backoff: Duration = .seconds(1)

    var hasCredentials: Bool {
        FileManager.default.fileExists(atPath: SpotifyConfig.librespotCache.appendingPathComponent("credentials.json").path)
    }

    private func baseArgs() -> [String] {
        [
            "--name", SpotifyConfig.deviceName,
            "--backend", "rodio",
            "--bitrate", "320",
            "--cache", SpotifyConfig.librespotCache.path,
            "--disable-audio-cache",
            "--autoplay", "off",
        ]
    }

    /// Called when a running device rejects its cached login (librespot logs
    /// `INVALID_CREDENTIALS`), so the session can re-authorize it.
    var onInvalidCredentials: (() -> Void)?

    var credentialsFile: URL { SpotifyConfig.librespotCache.appendingPathComponent("credentials.json") }

    func discardCredentials() {
        try? FileManager.default.removeItem(at: credentialsFile)
    }

    /// One-time device login through librespot's own OAuth (the same step the
    /// TUI runs). Its URL is opened in the browser. A login derived from the
    /// app's Web API token is NOT used: librespot accepts it for the session
    /// but Spotify Connect rejects it with INVALID_CREDENTIALS.
    func ensureCredentials() async throws {
        if hasCredentials { return }
        guard let bin = SpotifyConfig.librespotBinary else { throw LibrespotError.notInstalled }
        if try await runUntilCredentials(bin: bin, extra: ["--enable-oauth"], timeout: 180, openURLs: true) {
            return
        }
        throw LibrespotError.authorizationTimedOut
    }

    private func runUntilCredentials(bin: URL, extra: [String], timeout: TimeInterval, openURLs: Bool) async throws -> Bool {
        let proc = Process()
        proc.executableURL = bin
        proc.arguments = baseArgs() + extra
        let pipe = Pipe()
        proc.standardOutput = pipe
        proc.standardError = pipe
        let log = LogSink(url: SpotifyConfig.librespotLog)
        let scanner = LinkScanner(open: openURLs)
        pipe.fileHandleForReading.readabilityHandler = { handle in
            let data = handle.availableData
            guard !data.isEmpty else { return }
            log.write(data)
            scanner.feed(data)
        }
        try proc.run()
        defer {
            pipe.fileHandleForReading.readabilityHandler = nil
            if proc.isRunning { proc.terminate() }
        }
        let deadline = Date().addingTimeInterval(timeout)
        while Date() < deadline {
            if hasCredentials { return true }
            if !proc.isRunning { return hasCredentials }
            try await Task.sleep(for: .milliseconds(400))
        }
        return hasCredentials
    }

    /// Starts the device and keeps it running, restarting with back-off.
    func start() throws {
        guard let bin = SpotifyConfig.librespotBinary else { throw LibrespotError.notInstalled }
        keepAlive = true
        launch(bin)
    }

    private func launch(_ bin: URL) {
        let proc = Process()
        proc.executableURL = bin
        proc.arguments = baseArgs()
        let log = LogSink(url: SpotifyConfig.librespotLog)
        let pipe = Pipe()
        proc.standardOutput = pipe
        proc.standardError = pipe
        var reported = false
        pipe.fileHandleForReading.readabilityHandler = { [weak self] handle in
            let data = handle.availableData
            log.write(data)
            guard !reported, let text = String(data: data, encoding: .utf8),
                  text.contains("INVALID_CREDENTIALS") else { return }
            reported = true
            DispatchQueue.main.async {
                guard let self else { return }
                self.keepAlive = false
                proc.terminate()
                self.onInvalidCredentials?()
            }
        }
        let started = Date()
        proc.terminationHandler = { [weak self] _ in
            pipe.fileHandleForReading.readabilityHandler = nil
            Task { @MainActor in
                guard let self, self.keepAlive else { return }
                if Date().timeIntervalSince(started) > 10 { self.backoff = .seconds(1) }
                try? await Task.sleep(for: self.backoff)
                self.backoff = min(self.backoff * 2, .seconds(30))
                if self.keepAlive { self.launch(bin) }
            }
        }
        do {
            try proc.run()
            process = proc
        } catch {
            log.write(Data("could not start librespot: \(error)\n".utf8))
        }
    }

    func stop() {
        keepAlive = false
        process?.terminate()
        process = nil
    }

    var isRunning: Bool { process?.isRunning ?? false }

    /// The last few lines of librespot's log, for error messages.
    static func logTail(lines: Int = 3) -> String {
        guard let text = try? String(contentsOf: SpotifyConfig.librespotLog, encoding: .utf8) else { return "" }
        return text.split(separator: "\n").suffix(lines).joined(separator: "\n")
    }
}

/// Finds librespot's "Browse to: <url>" line and opens it once. Output is
/// buffered until a full line arrives, because pipe reads can split the long
/// authorize URL, and opening a truncated one makes Spotify show an error.
private final class LinkScanner: @unchecked Sendable {
    private let enabled: Bool
    private var buffer = ""
    private var opened = false
    private let lock = NSLock()

    init(open: Bool) { enabled = open }

    func feed(_ data: Data) {
        guard enabled, let text = String(data: data, encoding: .utf8) else { return }
        lock.lock(); defer { lock.unlock() }
        guard !opened else { return }
        buffer += text
        while let newline = buffer.firstIndex(of: "\n") {
            let line = String(buffer[..<newline])
            buffer.removeSubrange(...newline)
            if let range = line.range(of: #"https://accounts\.spotify\.com/\S+"#, options: .regularExpression),
               let url = URL(string: String(line[range])) {
                opened = true
                DispatchQueue.main.async { NSWorkspace.shared.open(url) }
                return
            }
        }
    }
}

/// Appends child-process output to a log file.
private final class LogSink: @unchecked Sendable {
    private let handle: FileHandle?

    init(url: URL) {
        if !FileManager.default.fileExists(atPath: url.path) {
            FileManager.default.createFile(atPath: url.path, contents: nil)
        }
        handle = try? FileHandle(forWritingTo: url)
        _ = try? handle?.seekToEnd()
    }

    func write(_ data: Data) {
        guard !data.isEmpty else { return }
        try? handle?.write(contentsOf: data)
    }
}
