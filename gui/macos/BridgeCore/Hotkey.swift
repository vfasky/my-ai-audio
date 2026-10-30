import Foundation

/// The modifier keys a hotkey chord is made of.
public struct Modifiers: OptionSet, Sendable {
  public let rawValue: UInt8
  public init(rawValue: UInt8) { self.rawValue = rawValue }
  public static let shift = Modifiers(rawValue: 1)
  public static let control = Modifiers(rawValue: 2)
  public static let option = Modifiers(rawValue: 4)
  public static let command = Modifiers(rawValue: 8)
  /// Fn/Globe: no chord uses it, so holding it never counts as the chord
  /// alone.
  public static let function = Modifiers(rawValue: 16)

  /// The chords go-voice offers, by the IDs of internal/hotkey; "off" and
  /// unknown names have none.
  public static func chord(named name: String) -> Modifiers? {
    switch name {
    case "shift+option": return [.shift, .option]
    case "control+option": return [.control, .option]
    case "control+shift": return [.control, .shift]
    case "command+option": return [.command, .option]
    default: return nil
    }
  }
  public static let names = [
    "shift+option", "control+option", "control+shift", "command+option", "off",
  ]
}

/// What a chord change means for push-to-talk.
public enum ChordEvent: Equatable, Sendable {
  /// The chord is held alone: call `fire(generation)` after the hold delay.
  case arm(UInt64)
  /// Held alone for the hold delay: start recording.
  case down
  /// A key of the chord was let go: stop recording and recognize.
  case up
  /// Another key or modifier was pressed while recording: this was a
  /// shortcut, so cancel.
  case abort
}

/// Recognizes a modifier chord held alone. The chord must be held for the
/// hold delay without any other key or mouse button, so that shortcuts using
/// the same modifiers never start a recording; once another key or button
/// was down with the chord, it has to be let go before it can arm again.
/// `othersDown` is whether any key other than a modifier, or a mouse button,
/// is down now, read from the system rather than from events seen, so that a
/// missed release cannot block the chord for good.
public struct ChordTracker: Sendable {
  enum State: Equatable { case idle, armed(UInt64), held, blocked }
  public private(set) var chord: Modifiers?
  var state = State.idle
  var generation: UInt64 = 0
  public init(chord: Modifiers? = nil) { self.chord = chord }

  public var isHeld: Bool { state == .held }

  /// Changes the chord; a recording in progress is aborted.
  public mutating func set(_ chord: Modifiers?) -> ChordEvent? {
    let held = state == .held
    self.chord = chord
    state = .blocked
    return held ? .abort : nil
  }

  /// The modifiers now down.
  public mutating func flags(_ m: Modifiers, othersDown: Bool = false) -> ChordEvent? {
    guard let chord else { return nil }
    switch state {
    case .idle:
      // An extra modifier with the chord is a shortcut: letting it go
      // must not leave the chord armed.
      if m.isStrictSuperset(of: chord) {
        state = .blocked
        return nil
      }
      if m == chord {
        if othersDown {
          state = .blocked
          return nil
        }
        generation &+= 1
        state = .armed(generation)
        return .arm(generation)
      }
    case .armed:
      if m != chord { state = m.isSuperset(of: chord) ? .blocked : .idle }
    case .held:
      if m.isStrictSuperset(of: chord) {
        state = .blocked
        return .abort
      }
      if m != chord {
        state = .idle
        return .up
      }
    case .blocked:
      if !m.isSuperset(of: chord) { state = .idle }
    }
    return nil
  }

  /// Another key or a mouse button went down.
  public mutating func key() -> ChordEvent? {
    switch state {
    case .armed:
      state = .blocked
    case .held:
      state = .blocked
      return .abort
    default: break
    }
    return nil
  }

  /// The hold delay of `arm(generation)` passed.
  public mutating func fire(_ generation: UInt64, othersDown: Bool = false) -> ChordEvent? {
    guard state == .armed(generation) else { return nil }
    if othersDown {
      state = .blocked
      return nil
    }
    state = .held
    return .down
  }
}

/// Cuts converted 16 kHz mono samples into 20 ms frames of 320 samples.
public struct FrameChunker: Sendable {
  public static let samples = 320
  var pending: [Int16] = []
  public init() {}
  /// The complete frames, as little-endian bytes, that samples finish.
  public mutating func push(_ samples: UnsafeBufferPointer<Int16>) -> [Data] {
    pending.append(contentsOf: samples)
    var out: [Data] = []
    var start = 0
    while pending.count - start >= Self.samples {
      out.append(Self.bytes(pending[start..<start + Self.samples]))
      start += Self.samples
    }
    pending.removeFirst(start)
    return out
  }
  /// The rest, padded with silence to a whole frame, or nil when empty.
  public mutating func flush() -> Data? {
    guard !pending.isEmpty else { return nil }
    let rest = pending + [Int16](repeating: 0, count: Self.samples - pending.count)
    pending = []
    return Self.bytes(rest[...])
  }
  static func bytes(_ s: ArraySlice<Int16>) -> Data {
    var d = Data(capacity: s.count * 2)
    for v in s {
      let le = UInt16(bitPattern: v).littleEndian
      d.append(UInt8(le & 0xff))
      d.append(UInt8(le >> 8))
    }
    return d
  }
}
