import BridgeCore
import Darwin
import Foundation

let outputLock = NSLock()
func emit(_ object: [String: Any]) {
  guard let data = try? JSONSerialization.data(withJSONObject: object), data.count <= BridgeLimits.line else {
    return
  }
  outputLock.lock()
  defer { outputLock.unlock() }
  FileHandle.standardOutput.write(data + Data([10]))
}
let (commands, commandSink) = AsyncStream<Command>.makeStream(
  bufferingPolicy: .bufferingOldest(128))
let parent = getppid()
let watchdog = DispatchSource.makeTimerSource(queue: .global())
watchdog.schedule(deadline: .now() + 1, repeating: 0.25)
watchdog.setEventHandler { if getppid() != parent { exit(0) } }
watchdog.resume()
let input = FileHandle.standardInput
let inputQueue = DispatchQueue(label: "yz.sh.mygo-go-voice.speech-stdin")
var inputBuffer = Data()
var inputFinished = false
input.readabilityHandler = { handle in
  inputQueue.async {
    guard !inputFinished else { return }
    let data = handle.availableData
    if data.isEmpty {
      inputFinished = true
      handle.readabilityHandler = nil
      commandSink.finish()
      return
    }
    inputBuffer.append(data)
    while let end = inputBuffer.firstIndex(of: 10) {
      let line = Data(inputBuffer[..<end])
      inputBuffer.removeSubrange(...end)
      guard let command = try? Command(data: line) else { exit(2) }
      switch commandSink.yield(command) {
      case .enqueued: break
      case .dropped, .terminated: exit(2)
      @unknown default: exit(2)
      }
    }
    if inputBuffer.count > BridgeLimits.line { exit(2) }
  }
}
Task { @MainActor in
  let service = SpeechService(emit: emit)
  for await command in commands {
    if ["warmup", "insert", "permissions", "request-permission"].contains(command.type) {
      Task { @MainActor in await service.handle(command) }
    } else {
      await service.handle(command)
    }
  }
  if let id = service.identity.active { await service.cancel(id) }
  exit(0)
}
RunLoop.main.run()
