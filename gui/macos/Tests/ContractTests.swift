import XCTest

@testable import BridgeCore

final class ContractTests: XCTestCase {
  func testPCMCanonicalAndBounds() throws {
    let value = Data(repeating: 0, count: 640).base64EncodedString()
    let object: [String: Any] = [
      "type": "pcm", "sessionId": 1, "sequence": 0, "payloadBase64": value,
    ]
    let command = try Command(data: JSONSerialization.data(withJSONObject: object))
    XCTAssertEqual(try command.pcm().count, 640)
    let bad = try Command(
      data: Data(
        "{\"type\":\"pcm\",\"sessionId\":1,\"sequence\":15000,\"payloadBase64\":\"AA==\"}".utf8))
    XCTAssertThrowsError(try bad.pcm())
    XCTAssertThrowsError(try Command(data: Data(repeating: 32, count: 16385)))
  }
  func testPermissionCommandsAndStates() throws {
    let request = try Command(data: Data(#"{"type":"request-permission","text":"speech"}"#.utf8))
    XCTAssertEqual(request.text, "speech")
    XCTAssertThrowsError(
      try Command(data: Data(#"{"type":"request-permission","text":"camera"}"#.utf8)))
    XCTAssertNoThrow(try Command(data: Data(#"{"type":"permissions"}"#.utf8)))
    XCTAssertEqual(Permission.automationState(0), "granted")
    XCTAssertEqual(Permission.automationState(-1743), "denied")
    XCTAssertEqual(Permission.automationState(-1744), "undetermined")
    XCTAssertEqual(Permission.automationState(-600), "unknown")
    XCTAssertEqual(Permission.event("bluetooth", "granted")["permission"] as? String, "bluetooth")
  }
  func testBeginNeedsSession() throws {
    XCTAssertEqual(
      try Command(data: Data(#"{"type":"begin","sessionId":3}"#.utf8)).sessionId, 3)
    XCTAssertThrowsError(try Command(data: Data(#"{"type":"begin"}"#.utf8)))
  }
  func testHotkeyAndMicrophoneCommands() throws {
    XCTAssertEqual(
      try Command(data: Data(#"{"type":"hotkey","text":"shift+option"}"#.utf8)).text, "shift+option")
    XCTAssertNoThrow(try Command(data: Data(#"{"type":"hotkey","text":"off"}"#.utf8)))
    XCTAssertThrowsError(try Command(data: Data(#"{"type":"hotkey","text":"command+q"}"#.utf8)))
    XCTAssertNoThrow(try Command(data: Data(#"{"type":"mic-start","sessionId":2}"#.utf8)))
    XCTAssertThrowsError(try Command(data: Data(#"{"type":"mic-stop"}"#.utf8)))
    XCTAssertNoThrow(
      try Command(data: Data(#"{"type":"request-permission","text":"microphone"}"#.utf8)))
  }
  func testChordNeedsToBeHeldAlone() {
    var t = ChordTracker(chord: Modifiers.chord(named: "shift+option"))
    XCTAssertNil(t.flags(.shift))
    guard case .arm(let first)? = t.flags([.shift, .option]) else { return XCTFail("arm") }
    XCTAssertEqual(t.fire(first), .down)
    XCTAssertTrue(t.isHeld)
    XCTAssertEqual(t.flags(.option), .up)
    XCTAssertNil(t.flags([]))
    // A shortcut with the same modifiers never starts a recording, and the
    // chord must be let go before it arms again.
    guard case .arm(let second)? = t.flags([.shift, .option]) else { return XCTFail("rearm") }
    XCTAssertNil(t.key())
    XCTAssertNil(t.fire(second))
    XCTAssertNil(t.flags([.shift, .option]))
    XCTAssertNil(t.flags(.shift))
    guard case .arm(let third)? = t.flags([.shift, .option]) else { return XCTFail("arm again") }
    XCTAssertEqual(t.fire(third), .down)
    XCTAssertEqual(t.key(), .abort)
    XCTAssertNil(t.flags([]))
    // An extra modifier while recording is a shortcut too.
    guard case .arm(let fourth)? = t.flags([.shift, .option]) else { return XCTFail("arm 4") }
    XCTAssertEqual(t.fire(fourth), .down)
    XCTAssertEqual(t.flags([.shift, .option, .command]), .abort)
    XCTAssertNil(t.flags([.shift, .option]))
    XCTAssertNil(t.flags([]))
    // Released before the hold delay: nothing.
    guard case .arm(let fifth)? = t.flags([.shift, .option]) else { return XCTFail("arm 5") }
    XCTAssertNil(t.flags([]))
    XCTAssertNil(t.fire(fifth))
    // Turning the chord off aborts a recording.
    guard case .arm(let sixth)? = t.flags([.shift, .option]) else { return XCTFail("arm 6") }
    XCTAssertEqual(t.fire(sixth), .down)
    XCTAssertEqual(t.set(nil), .abort)
    XCTAssertNil(t.flags([.shift, .option]))
    XCTAssertNil(Modifiers.chord(named: "off"))
  }
  func testChordWithAMouseButtonOrKeyAlreadyDownDoesNotRecord() {
    var t = ChordTracker(chord: Modifiers.chord(named: "shift+option"))
    // A mouse button held before the chord: never arms.
    XCTAssertNil(t.flags(.shift, othersDown: true))
    XCTAssertNil(t.flags([.shift, .option], othersDown: true))
    // Releasing the button alone does not arm it either; the chord must be
    // pressed again.
    XCTAssertNil(t.flags([.shift, .option]))
    XCTAssertNil(t.flags(.shift))
    guard case .arm(let g)? = t.flags([.shift, .option]) else { return XCTFail("arm") }
    // Pressed during the hold delay, between events the tap saw.
    XCTAssertNil(t.fire(g, othersDown: true))
    XCTAssertFalse(t.isHeld)
    XCTAssertNil(t.flags([]))
    guard case .arm(let h)? = t.flags([.shift, .option]) else { return XCTFail("rearm") }
    XCTAssertEqual(t.fire(h), .down)
  }
  func testFnDisqualifiesTheChord() {
    var t = ChordTracker(chord: Modifiers.chord(named: "shift+option"))
    // Fn held with the chord: never arms.
    XCTAssertNil(t.flags(.function))
    XCTAssertNil(t.flags([.function, .shift]))
    XCTAssertNil(t.flags([.function, .shift, .option]))
    // Letting Fn go leaves the chord, which must be pressed again.
    XCTAssertNil(t.flags([.shift, .option]))
    XCTAssertNil(t.flags(.shift))
    guard case .arm(let g)? = t.flags([.shift, .option]) else { return XCTFail("arm") }
    XCTAssertEqual(t.fire(g), .down)
    // Fn pressed while recording cancels.
    XCTAssertEqual(t.flags([.shift, .option, .function]), .abort)
    XCTAssertFalse(t.isHeld)
  }
  func testFrameChunkerCutsTwentyMillisecondFrames() {
    var chunker = FrameChunker()
    let samples = [Int16](repeating: -2, count: 700)
    let frames = samples.withUnsafeBufferPointer { chunker.push($0) }
    XCTAssertEqual(frames.count, 2)
    XCTAssertEqual(frames[0].count, 640)
    XCTAssertEqual(frames[0][0], 0xfe)
    XCTAssertEqual(frames[0][1], 0xff)
    let rest = chunker.flush()
    XCTAssertEqual(rest?.count, 640)
    XCTAssertEqual(rest?[2 * 60], 0)
    XCTAssertNil(chunker.flush())
  }
  func testSessionIdentity() {
    var guardState = SessionGuard()
    XCTAssertTrue(guardState.start(1))
    XCTAssertFalse(guardState.start(1))
    guardState.cancel(2)
    XCTAssertTrue(guardState.matches(1))
    guardState.cancel(1)
    XCTAssertTrue(guardState.start(2))
    XCTAssertFalse(guardState.matches(1))
  }
  func testExplicitDeviceSelectionAndMTU() {
    let id = UUID()
    XCTAssertFalse(VoiceContract.mayConnect(selected: nil, candidate: id))
    XCTAssertFalse(VoiceContract.mayConnect(selected: UUID(), candidate: id))
    XCTAssertTrue(VoiceContract.mayConnect(selected: id, candidate: id))
    XCTAssertFalse(VoiceContract.supportsAudio(writeWithoutResponse: 181))
    XCTAssertTrue(VoiceContract.supportsAudio(writeWithoutResponse: 182))
    XCTAssertEqual(VoiceContract.service, "676f0001-766f-6963-652d-617564696f00")
  }
  func testClipboardOwnership() {
    XCTAssertTrue(shouldRestoreClipboard(expected: 2, current: 2, marker: "a", session: "a"))
    XCTAssertFalse(shouldRestoreClipboard(expected: 2, current: 3, marker: "a", session: "a"))
    XCTAssertFalse(shouldRestoreClipboard(expected: 2, current: 2, marker: "b", session: "a"))
  }
}
