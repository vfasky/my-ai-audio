import Foundation

/// Privacy grants the helpers report to MyGo as
/// {"type":"permission","permission":name,"state":state}.
public enum Permission {
  public static let speechNames = ["speech", "accessibility", "automation", "microphone"]
  /// Maps AEDeterminePermissionToAutomateTarget results.
  public static func automationState(_ status: Int32) -> String {
    switch status {
    case 0: return "granted"
    case -1743: return "denied"  // errAEEventNotPermitted
    case -1744: return "undetermined"  // errAEEventWouldRequireUserConsent
    default: return "unknown"  // e.g. procNotFound while System Events is not running
    }
  }
  public static func event(_ name: String, _ state: String) -> [String: Any] {
    ["type": "permission", "permission": name, "state": state]
  }
}
