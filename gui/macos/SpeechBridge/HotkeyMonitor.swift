import ApplicationServices
import BridgeCore
import CoreGraphics
import Foundation

/// Watches the keyboard system-wide for the push-to-talk chord with an
/// event tap that passes every event through unchanged. A filtering tap is
/// covered by Accessibility, which insertion already requires (a listen-only
/// tap would need Input Monitoring as well); until it is granted the monitor
/// retries every two seconds.
@MainActor final class HotkeyMonitor {
  /// How long the chord must be held alone before recording starts.
  static let holdDelay: Duration = .milliseconds(250)
  private var tracker = ChordTracker()
  private var tap: CFMachPort?
  private var source: CFRunLoopSource?
  private var retry: Task<Void, Never>?
  private let emit: ([String: Any]) -> Void
  init(emit: @escaping ([String: Any]) -> Void) { self.emit = emit }

  func configure(_ name: String) {
    let chord = Modifiers.chord(named: name)
    report(tracker.set(chord))
    if chord == nil {
      uninstall()
    } else {
      install()
    }
  }

  private func install() {
    guard tap == nil else { return }
    let mask: CGEventMask =
      (1 << CGEventType.flagsChanged.rawValue) | (1 << CGEventType.keyDown.rawValue)
      | (1 << CGEventType.leftMouseDown.rawValue) | (1 << CGEventType.rightMouseDown.rawValue)
      | (1 << CGEventType.otherMouseDown.rawValue)
    let me = Unmanaged.passUnretained(self).toOpaque()
    if let port = CGEvent.tapCreate(
      tap: .cgSessionEventTap, place: .headInsertEventTap, options: .defaultTap,
      eventsOfInterest: mask, callback: hotkeyCallback, userInfo: me)
    {
      tap = port
      source = CFMachPortCreateRunLoopSource(nil, port, 0)
      CFRunLoopAddSource(CFRunLoopGetMain(), source, .commonModes)
      CGEvent.tapEnable(tap: port, enable: true)
      retry?.cancel()
      retry = nil
      return
    }
    guard retry == nil else { return }
    retry = Task { @MainActor [weak self] in
      while !Task.isCancelled {
        try? await Task.sleep(for: .seconds(2))
        guard let self, self.tracker.chord != nil else { return }
        self.retry = nil
        self.install()
        if self.tap != nil || self.retry != nil { return }
      }
    }
  }

  private func uninstall() {
    retry?.cancel()
    retry = nil
    if let tap { CGEvent.tapEnable(tap: tap, enable: false) }
    if let source { CFRunLoopRemoveSource(CFRunLoopGetMain(), source, .commonModes) }
    tap = nil
    source = nil
  }

  fileprivate func handle(_ type: CGEventType, _ event: CGEvent) {
    switch type {
    case .tapDisabledByTimeout, .tapDisabledByUserInput:
      if let tap { CGEvent.tapEnable(tap: tap, enable: true) }
      // Events may have been missed: start over from the current keys.
      report(tracker.set(tracker.chord))
    case .flagsChanged:
      report(tracker.flags(Self.modifiers(event.flags), othersDown: Self.othersDown()))
    case .keyDown, .leftMouseDown, .rightMouseDown, .otherMouseDown:
      report(tracker.key())
    default: break
    }
  }

  private func report(_ e: ChordEvent?) {
    switch e {
    case .arm(let generation)?:
      Task { @MainActor [weak self] in
        try? await Task.sleep(for: Self.holdDelay)
        guard let self else { return }
        self.report(self.tracker.fire(generation, othersDown: Self.othersDown()))
      }
    case .down?: emit(["type": "hotkey", "text": "down"])
    case .up?: emit(["type": "hotkey", "text": "up"])
    case .abort?: emit(["type": "hotkey", "text": "abort"])
    case nil: break
    }
  }

  /// Virtual key codes of the modifier keys, Caps Lock and Fn; Fn is
  /// reported through the event flags instead.
  static let modifierKeys: Set<CGKeyCode> = [54, 55, 56, 57, 58, 59, 60, 61, 62, 63]

  /// Whether a key other than a modifier, or a mouse button, is down now.
  static func othersDown() -> Bool {
    for button in 0..<5 {
      if let b = CGMouseButton(rawValue: UInt32(button)),
        CGEventSource.buttonState(.combinedSessionState, button: b)
      {
        return true
      }
    }
    for code in CGKeyCode(0)..<CGKeyCode(128) where !modifierKeys.contains(code) {
      if CGEventSource.keyState(.combinedSessionState, key: code) { return true }
    }
    return false
  }

  static func modifiers(_ f: CGEventFlags) -> Modifiers {
    var m: Modifiers = []
    if f.contains(.maskShift) { m.insert(.shift) }
    if f.contains(.maskControl) { m.insert(.control) }
    if f.contains(.maskAlternate) { m.insert(.option) }
    if f.contains(.maskCommand) { m.insert(.command) }
    if f.contains(.maskSecondaryFn) { m.insert(.function) }
    return m
  }
}

private let hotkeyCallback: CGEventTapCallBack = { _, type, event, info in
  if let info {
    let monitor = Unmanaged<HotkeyMonitor>.fromOpaque(info).takeUnretainedValue()
    MainActor.assumeIsolated { monitor.handle(type, event) }
  }
  return Unmanaged.passUnretained(event)
}
