import Foundation

public struct Command: Decodable {
  public let type: String
  public let sessionId: UInt32?
  public let sequence: UInt32?
  public let payloadBase64: String?
  public let text: String?
  public let terms: [String]?
  public init(data: Data) throws {
    guard data.count <= 16384, String(data: data, encoding: .utf8) != nil else {
      throw BridgeError.invalid
    }
    self = try JSONDecoder().decode(Command.self, from: data)
    guard
      [
        "warmup", "begin", "start", "pcm", "finish", "cancel", "refine", "insert", "enter",
        "delete", "clear", "ping", "shutdown", "permissions", "request-permission", "hotkey",
        "mic-start", "mic-stop",
      ].contains(type), (text?.utf8.count ?? 0) <= 12000, (terms?.count ?? 0) <= 100,
      terms?.allSatisfy({ $0.utf8.count <= 256 }) ?? true
    else { throw BridgeError.invalid }
    if ["begin", "start", "pcm", "finish", "cancel", "refine", "insert", "mic-start", "mic-stop"]
      .contains(type),
      sessionId == nil
    {
      throw BridgeError.invalid
    }
    if type == "request-permission", !Permission.speechNames.contains(text ?? "") {
      throw BridgeError.invalid
    }
    if type == "hotkey", !Modifiers.names.contains(text ?? "") { throw BridgeError.invalid }
  }
  public func pcm() throws -> Data {
    guard let value = payloadBase64, value.count == 856, let data = Data(base64Encoded: value),
      data.count == 640, let sequence, sequence < 15000
    else { throw BridgeError.invalid }
    return data
  }
}
public enum BridgeError: Error { case invalid, unavailable, stale }
public struct SessionGuard {
  public private(set) var active: UInt32?
  public init() {}
  public mutating func start(_ id: UInt32) -> Bool {
    guard active == nil else { return false }
    active = id
    return true
  }
  public func matches(_ id: UInt32) -> Bool { active == id }
  public mutating func cancel(_ id: UInt32) { if active == id { active = nil } }
}
public func shouldRestoreClipboard(expected: Int, current: Int, marker: String?, session: String)
  -> Bool
{ expected == current && marker == session }
