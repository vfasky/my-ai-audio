import AVFoundation
import BridgeCore
import Foundation
import os

/// Records the default Mac input only between mic-start and mic-stop, as
/// 20 ms frames of 16 kHz signed 16-bit mono PCM. Nothing is stored.
///
/// The audio tap only copies each buffer and hands it to a serial worker,
/// which converts, frames and emits it, so a slow output pipe never stalls
/// capture or the main actor. At most `backlog` buffers (about a second)
/// may wait in all, counting those of an earlier recording still queued;
/// beyond that the recording fails with mic-error rather than silently
/// dropping speech.
final class Microphone: @unchecked Sendable {
  static let backlog = 48
  private let output = AVAudioFormat(
    commonFormat: .pcmFormatInt16, sampleRate: 16000, channels: 1, interleaved: true)!
  private let emit: ([String: Any]) -> Void
  init(emit: @escaping ([String: Any]) -> Void) { self.emit = emit }

  // Main actor: the engine, created on the first authorized start, so that
  // no other path ever touches the input hardware.
  private var engine: AVAudioEngine?
  private var tapInstalled = false
  private var token: UInt64 = 0

  // Audio thread and worker: the buffers waiting, of every recording until
  // the worker took them, and the recording whose backlog overflowed.
  private struct Queue {
    var pending = 0
    var overflowed: UInt64?
  }
  private let queueState = OSAllocatedUnfairLock(initialState: Queue())

  // Worker only.
  private let worker = DispatchQueue(label: "yz.sh.mygo-go-voice.microphone")
  private struct Recording {
    let id: UInt32
    let token: UInt64
    let converter: AVAudioConverter
    var sequence: UInt32 = 0
    var chunker = FrameChunker()
  }
  private var recording: Recording?

  static func state() -> String {
    switch AVCaptureDevice.authorizationStatus(for: .audio) {
    case .authorized: return "granted"
    case .denied: return "denied"
    case .restricted: return "restricted"
    case .notDetermined: return "undetermined"
    @unknown default: return "unknown"
    }
  }
  static func request() async -> String {
    _ = await AVCaptureDevice.requestAccess(for: .audio)
    return state()
  }

  /// Starts recording session id; false when the input cannot start.
  @MainActor func start(_ id: UInt32) -> Bool {
    stop(notify: false)
    guard AVCaptureDevice.authorizationStatus(for: .audio) == .authorized else { return false }
    let engine = self.engine ?? AVAudioEngine()
    self.engine = engine
    let input = engine.inputNode
    let format = input.outputFormat(forBus: 0)
    guard format.sampleRate > 0, format.channelCount > 0,
      let converter = AVAudioConverter(from: format, to: output)
    else { return false }
    token &+= 1
    let token = self.token
    // Enqueued before the tap exists, so the worker knows the recording
    // before its first buffer.
    worker.async { self.recording = Recording(id: id, token: token, converter: converter) }
    input.installTap(onBus: 0, bufferSize: 1024, format: format) { [weak self] buffer, _ in
      self?.handOff(buffer, token)
    }
    tapInstalled = true
    engine.prepare()
    do {
      try engine.start()
    } catch {
      stop(notify: false)
      return false
    }
    return true
  }

  /// Stops the input; with notify, the worker sends the rest of the audio
  /// and mic-end after every buffer already handed off.
  @MainActor func stop(notify: Bool = true) {
    guard tapInstalled, let engine else { return }
    engine.inputNode.removeTap(onBus: 0)
    engine.stop()
    tapInstalled = false
    let token = self.token
    worker.async { self.finish(token, notify: notify) }
  }

  /// Audio thread: copy and hand off, nothing else.
  private func handOff(_ buffer: AVAudioPCMBuffer, _ token: UInt64) {
    let admitted = queueState.withLock { q -> Bool? in
      if q.overflowed == token { return nil }
      if q.pending >= Self.backlog {
        q.overflowed = token
        return false
      }
      q.pending += 1
      return true
    }
    switch admitted {
    case true?:
      guard let copy = Self.copy(buffer) else {
        queueState.withLock { $0.pending -= 1 }
        return
      }
      worker.async {
        self.process(copy, token)
        self.queueState.withLock { $0.pending -= 1 }
      }
    case false?:
      worker.async { self.fail(token) }
    case nil:
      break
    }
  }

  private static func copy(_ buffer: AVAudioPCMBuffer) -> AVAudioPCMBuffer? {
    guard buffer.frameLength > 0,
      let copy = AVAudioPCMBuffer(pcmFormat: buffer.format, frameCapacity: buffer.frameLength)
    else { return nil }
    copy.frameLength = buffer.frameLength
    let source = UnsafeMutableAudioBufferListPointer(
      UnsafeMutablePointer(mutating: buffer.audioBufferList))
    let target = UnsafeMutableAudioBufferListPointer(copy.mutableAudioBufferList)
    for (s, var t) in zip(source, target) {
      guard let from = s.mData, let to = t.mData else { return nil }
      let size = min(s.mDataByteSize, t.mDataByteSize)
      memcpy(to, from, Int(size))
      t.mDataByteSize = size
    }
    return copy
  }

  /// Worker: converts a buffer to 16 kHz frames and emits them.
  private func process(_ buffer: AVAudioPCMBuffer, _ token: UInt64) {
    guard var r = recording, r.token == token else { return }
    let ratio = output.sampleRate / buffer.format.sampleRate
    let capacity = AVAudioFrameCount(Double(buffer.frameLength) * ratio) + 64
    guard let out = AVAudioPCMBuffer(pcmFormat: output, frameCapacity: capacity) else { return }
    var supplied = false
    var error: NSError?
    r.converter.convert(to: out, error: &error) { _, status in
      if supplied {
        status.pointee = .noDataNow
        return nil
      }
      supplied = true
      status.pointee = .haveData
      return buffer
    }
    guard error == nil, let samples = out.int16ChannelData, out.frameLength > 0 else { return }
    let frames = r.chunker.push(UnsafeBufferPointer(start: samples[0], count: Int(out.frameLength)))
    for frame in frames { send(&r, frame) }
    recording = r
  }

  /// Worker: the recording ended; with notify, flush and send mic-end.
  private func finish(_ token: UInt64, notify: Bool) {
    guard var r = recording, r.token == token else { return }
    recording = nil
    guard notify else { return }
    if let rest = r.chunker.flush() { send(&r, rest) }
    emit(["type": "mic-end", "sessionId": r.id])
  }

  /// Worker: the backlog overflowed; the app cancels the session.
  private func fail(_ token: UInt64) {
    guard let r = recording, r.token == token else { return }
    recording = nil
    emit(["type": "mic-error", "sessionId": r.id])
  }

  private func send(_ r: inout Recording, _ frame: Data) {
    guard r.sequence < 15000 else { return }
    emit([
      "type": "mic", "sessionId": r.id, "sequence": r.sequence,
      "payloadBase64": frame.base64EncodedString(),
    ])
    r.sequence += 1
  }
}
