import AVFoundation
import Foundation
import Speech

/// Turns speech into text with Apple's recognizer, on the device when it can.
/// Only the recognised words leave this class.
@MainActor
final class SpeechListener {
    enum Failure: LocalizedError {
        case denied, unavailable
        var errorDescription: String? {
            switch self {
            case .denied: "AudioPulse needs microphone and speech recognition access. Turn them on in Settings."
            case .unavailable: "Speech recognition isn’t available for your language right now."
            }
        }
    }

    private let recognizer = SFSpeechRecognizer(locale: Locale.current) ?? SFSpeechRecognizer(locale: Locale(identifier: "en-US"))
    private let engine = AVAudioEngine()
    private var request: SFSpeechAudioBufferRecognitionRequest?
    private var task: SFSpeechRecognitionTask?

    var onDevice: Bool { recognizer?.supportsOnDeviceRecognition ?? false }

    func start(onText: @escaping (String) -> Void, onLevel: @escaping (Double) -> Void) async throws {
        guard await Self.authorize() else { throw Failure.denied }
        guard let recognizer, recognizer.isAvailable else { throw Failure.unavailable }
        stop()

        #if os(iOS)
        let session = AVAudioSession.sharedInstance()
        // Duck Spotify while listening rather than stopping it.
        try session.setCategory(.playAndRecord, mode: .measurement, options: [.duckOthers, .defaultToSpeaker, .allowBluetoothHFP])
        try session.setActive(true, options: .notifyOthersOnDeactivation)
        #endif

        let request = SFSpeechAudioBufferRecognitionRequest()
        request.shouldReportPartialResults = true
        if recognizer.supportsOnDeviceRecognition { request.requiresOnDeviceRecognition = true }
        self.request = request

        let input = engine.inputNode
        let format = input.outputFormat(forBus: 0)
        input.removeTap(onBus: 0)
        input.installTap(onBus: 0, bufferSize: 1024, format: format) { buffer, _ in
            request.append(buffer)
            guard let data = buffer.floatChannelData?[0] else { return }
            let n = Int(buffer.frameLength)
            var sum: Float = 0
            for i in 0..<n { sum += data[i] * data[i] }
            let rms = n > 0 ? sqrt(sum / Float(n)) : 0
            let level = Double(min(1, rms * 12))
            Task { @MainActor in onLevel(level) }
        }
        engine.prepare()
        try engine.start()

        task = recognizer.recognitionTask(with: request) { result, _ in
            guard let text = result?.bestTranscription.formattedString else { return }
            Task { @MainActor in onText(text) }
        }
    }

    func stop() {
        if engine.isRunning {
            engine.stop()
            engine.inputNode.removeTap(onBus: 0)
        }
        request?.endAudio()
        task?.finish()
        request = nil
        task = nil
        #if os(iOS)
        try? AVAudioSession.sharedInstance().setActive(false, options: .notifyOthersOnDeactivation)
        #endif
    }

    private static func authorize() async -> Bool {
        let speech = await withCheckedContinuation { cont in
            SFSpeechRecognizer.requestAuthorization { cont.resume(returning: $0 == .authorized) }
        }
        guard speech else { return false }
        #if os(iOS)
        return await AVAudioApplication.requestRecordPermission()
        #else
        return await AVCaptureDevice.requestAccess(for: .audio)
        #endif
    }
}
