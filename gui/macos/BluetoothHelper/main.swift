import CoreBluetooth
import Darwin
import Dispatch
import Foundation
import BridgeCore

private let serviceUUID = CBUUID(string: VoiceContract.service)
private let controlUUID = CBUUID(string: VoiceContract.control)
private let audioUUID = CBUUID(string: VoiceContract.audio)
private let infoUUID = CBUUID(string: VoiceContract.info)
private let audioMagic: UInt16 = 0x5041
private let audioEnvelopeBytes = 16
private let maxAudioFrameBytes = 166
private let maxAudioFragments = 64
private let maxPendingAudioFrames = 64
private let maxAudioSequences = 15_000
private let attOverheadBytes = 3
private let minimumAttMtu = 185
private let minimumAttPayloadBytes = minimumAttMtu - attOverheadBytes
// V1's minimum negotiated ATT MTU is 185, so the characteristic value is at
// most 182 bytes after the three-byte ATT overhead.
private let maxControlMessageBytes = minimumAttMtu - attOverheadBytes
private let maxHelperLineBytes = 16 * 1024
private let maxDiagnosticMessageBytes = 128
private let maxDeviceFieldBytes = 256
private let minimumControlWriteBytes = 27 // {"v":1,"type":"host_ready"}
private let controlWriteRetryInterval: TimeInterval = 0.25
private let controlWriteMtuTimeout: TimeInterval = 10

private let outputLock = NSLock()

private func boundedUtf8(_ value: String, maxBytes: Int) -> String {
  guard maxBytes > 0 else { return "" }
  let data = Data(value.utf8)
  guard data.count > maxBytes else { return value }
  var end = maxBytes
  while end > 0 {
    let prefix = data.prefix(end)
    if let result = String(data: prefix, encoding: .utf8) { return result }
    end -= 1
  }
  return ""
}

private func exactJsonInteger(_ value: Any?, equals expected: Int) -> Bool {
  guard let number = value as? NSNumber,
        number.doubleValue == Double(expected) else { return false }
  // JSONSerialization bridges booleans to NSNumber as well. Their numeric
  // value must not be accepted as a protocol integer.
  let objcType = String(cString: number.objCType)
  return objcType != "c" && objcType != "B"
}

private func emit(_ value: [String: Any]) {
  var sanitized = value
  if let type = sanitized["type"] as? String {
    if type == "error" || type == "diagnostic" || type == "state",
       let message = sanitized["message"] as? String {
      sanitized["message"] = boundedUtf8(message, maxBytes: maxDiagnosticMessageBytes)
    }
    if type == "connected" || type == "state",
       let deviceId = sanitized["deviceId"] as? String {
      sanitized["deviceId"] = boundedUtf8(deviceId, maxBytes: maxDeviceFieldBytes)
    }
    if type == "connected" || type == "state" || type == "discovered",
       let deviceName = sanitized["deviceName"] as? String {
      sanitized["deviceName"] = boundedUtf8(deviceName, maxBytes: maxDeviceFieldBytes)
    }
    if type == "connected" || type == "state",
       let preferredDeviceId = sanitized["preferredDeviceId"] as? String {
      sanitized["preferredDeviceId"] = boundedUtf8(preferredDeviceId, maxBytes: maxDeviceFieldBytes)
    }
  }
  guard JSONSerialization.isValidJSONObject(sanitized),
        var data = try? JSONSerialization.data(withJSONObject: sanitized),
        data.count <= maxHelperLineBytes else { return }
  data.append(10)
  outputLock.lock()
  defer { outputLock.unlock() }
  FileHandle.standardOutput.write(data)
}

private struct AudioFrameKey: Hashable {
  var session: UInt32
  var sequence: UInt32
}

private func le16(_ data: Data, _ offset: Int) -> UInt16 {
  UInt16(data[offset]) | UInt16(data[offset + 1]) << 8
}

private func le32(_ data: Data, _ offset: Int) -> UInt32 {
  UInt32(data[offset]) | UInt32(data[offset + 1]) << 8 | UInt32(data[offset + 2]) << 16
    | UInt32(data[offset + 3]) << 24
}

private final class PendingFrame {
  let sessionId: UInt32
  let sequence: UInt32
  let count: Int
  var parts: [Data?]
  var received = 0
  var totalBytes = 0

  init(sessionId: UInt32, sequence: UInt32, count: Int) {
    self.sessionId = sessionId
    self.sequence = sequence
    self.count = count
    self.parts = Array(repeating: nil, count: count)
  }
}

final class Central: NSObject, CBCentralManagerDelegate, CBPeripheralDelegate {
  private var manager: CBCentralManager!
  private var peripheral: CBPeripheral?
  private var control: CBCharacteristic?
  private var audio: CBCharacteristic?
  private var subscribed = Set<CBUUID>()
  private var pending = [AudioFrameKey: PendingFrame]()
  // Dictionary iteration order is not a wire-level ordering guarantee. Keep
  // an explicit insertion order so bounded-frame eviction drops the oldest
  // incomplete frame deterministically.
  private var pendingOrder = [AudioFrameKey]()
  private var completed = Set<AudioFrameKey>()
  private var completedOrder = [AudioFrameKey]()
  private var reconnectAttempt = 0
  private var reconnectWorkItem: DispatchWorkItem?
  private var shuttingDown = false
  private var connectedEmitted = false
  private var setupTimer: Timer?
  private var writeQueue = [Data]()
  private var writeInFlight: Data?
  private var writeAttempts = 0
  private var writeTimer: Timer?
  private var writeRetryWorkItem: DispatchWorkItem?
  private var mtuRetryTimer: Timer?
  private var mtuWaitDeadline: Date?
  private var stdinBuffer = Data()
  private var stdinReaderInstalled = false
  private var terminationSources = [DispatchSourceSignal]()
  private var parentWatchdog: DispatchSourceTimer?
  private let launchParentPid = getppid()
  private var connectionEpoch: UInt64 = 0
  // This pin is intentionally process-local. MyGo persists the user's explicit
  // device choice and replays that peripheral UUID after a restart.
  private var preferredDeviceId: UUID?

  private func isCurrent(_ candidate: CBPeripheral) -> Bool {
    guard let current = peripheral else { return false }
    return current === candidate
  }

  func start() {
    installProcessLifecycleHandlers()
    if ProcessInfo.processInfo.environment["GO_VOICE_BRIDGE_PIPE_TEST"] != "1" {
      manager = CBCentralManager(delegate: self, queue: .main, options: [
        CBCentralManagerOptionShowPowerAlertKey: true
      ])
    }
    installStdinReader()
    RunLoop.main.run()
  }

  private func installProcessLifecycleHandlers() {
    for signalValue in [SIGTERM, SIGINT] {
      signal(signalValue, SIG_IGN)
      let source = DispatchSource.makeSignalSource(
        signal: signalValue,
        queue: .global(qos: .userInitiated)
      )
      source.setEventHandler { [weak self] in
        self?.requestShutdown()
      }
      source.resume()
      terminationSources.append(source)
    }

    let timer = DispatchSource.makeTimerSource(queue: .global(qos: .userInitiated))
    timer.schedule(
      deadline: .now() + .milliseconds(250),
      repeating: .milliseconds(250),
      leeway: .milliseconds(100)
    )
    timer.setEventHandler { [weak self] in
      guard let self, getppid() != self.launchParentPid else { return }
      self.requestShutdown()
    }
    timer.resume()
    parentWatchdog = timer
  }

  private func requestShutdown() {
    DispatchQueue.main.async { [weak self] in
      self?.shutdown()
    }
  }

  private func installStdinReader() {
    guard !stdinReaderInstalled else { return }
    stdinReaderInstalled = true
    FileHandle.standardInput.readabilityHandler = { [weak self] handle in
      let data = handle.availableData
      guard let self else { return }
      if data.isEmpty {
        DispatchQueue.main.async { self.shutdown() }
        return
      }
      DispatchQueue.main.sync { self.consumeStdin(data) }
    }
  }

  private func consumeStdin(_ data: Data) {
    guard !shuttingDown else { return }
    stdinBuffer.append(data)
    if stdinBuffer.count > maxHelperLineBytes && !stdinBuffer.contains(10) {
      shutdown()
      return
    }
    while let newline = stdinBuffer.firstIndex(of: 10) {
      let line = Data(stdinBuffer[..<newline])
      stdinBuffer.removeSubrange(...newline)
      guard line.count <= maxControlMessageBytes else {
        emit(["type": "diagnostic", "level": "warn", "message": "Helper command line too long"])
        continue
      }
      handleCommandLine(line)
    }
    if stdinBuffer.count > maxHelperLineBytes {
      shutdown()
    }
  }

  private func handleCommandLine(_ data: Data) {
    guard let object = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
          let type = object["type"] as? String else {
      emit(["type": "diagnostic", "level": "warn", "message": "Invalid helper command"])
      return
    }
    switch type {
    case "scan":
      if peripheral == nil { stopScanIfPoweredOn(); discover() }
    case "permissions":
      emitPermission()
    case "shutdown":
      shutdown()
    case "host_ready":
      sendControl(["v": 1, "type": "host_ready"])
    case "host_state":
      guard let state = object["state"] as? String,
            ["ready", "recording", "processing", "success", "error"].contains(state) else {
        emit(["type": "diagnostic", "level": "warn", "message": "Invalid host state"])
        return
      }
      var command: [String: Any] = ["v": 1, "type": "host_state", "state": state]
      if let message = object["message"] as? String,
         message.utf8.count <= maxDiagnosticMessageBytes {
        command["message"] = message
      }
      sendControl(command)
    case "set-preferred-device":
      if object["deviceId"] is NSNull {
        setPreferredDevice(nil)
        return
      }
      guard let value = object["deviceId"] as? String,
            let identifier = UUID(uuidString: value) else {
        emit(["type": "diagnostic", "level": "warn", "message": "Invalid preferred device id"])
        return
      }
      setPreferredDevice(identifier)
    default:
      emit(["type": "diagnostic", "level": "warn", "message": "Unknown helper command"])
    }
  }

  private func emitPermission() {
    let state: String
    switch CBManager.authorization {
    case .allowedAlways: state = "granted"
    case .denied: state = "denied"
    case .restricted: state = "restricted"
    case .notDetermined: state = "undetermined"
    @unknown default: state = "unknown"
    }
    emit(Permission.event("bluetooth", state))
  }

  func centralManagerDidUpdateState(_ central: CBCentralManager) {
    emitPermission()
    guard central.state == .poweredOn else {
      reconnectWorkItem?.cancel()
      reconnectWorkItem = nil
      let hadConnection = peripheral != nil || connectedEmitted
      let candidate = peripheral
      resetConnectionState()
      // CoreBluetooth rejects scan/connection commands while the manager is
      // powered off, resetting, unauthorized, or unsupported. The adapter may
      // transition through those states while this callback is being handled,
      // so the cleanup helper intentionally skips the cancel in that case.
      cancelPeripheralIfPoweredOn(candidate)
      if hadConnection { emit(["type": "disconnected"]) }
      let stateMessage: String
      switch central.state {
      case .unauthorized: stateMessage = "Bluetooth permission denied"
      case .poweredOff: stateMessage = "Bluetooth is powered off"
      case .unsupported: stateMessage = "Bluetooth is unsupported"
      case .resetting: stateMessage = "Bluetooth adapter is resetting"
      default: stateMessage = "Bluetooth is unavailable"
      }
      emit(["type": "error", "message": stateMessage])
      return
    }
    discover()
  }

  private func discover() {
    guard !shuttingDown, peripheral == nil,
          let manager, manager.state == .poweredOn else { return }
    if let preferredDeviceId {
      if let candidate = manager.retrievePeripherals(withIdentifiers: [preferredDeviceId]).first {
        connect(candidate)
        return
      }
      manager.scanForPeripherals(withServices: [serviceUUID], options: [CBCentralManagerScanOptionAllowDuplicatesKey: false])
      return
    }
    // A peripheral is selected only by an explicit persisted user choice.
    manager.scanForPeripherals(withServices: [serviceUUID], options: [CBCentralManagerScanOptionAllowDuplicatesKey: false])
  }

  func centralManager(_ central: CBCentralManager, didDiscover candidate: CBPeripheral,
                      advertisementData: [String: Any], rssi RSSI: NSNumber) {
    guard central.state == .poweredOn else { return }
    let advertisedName = advertisementData[CBAdvertisementDataLocalNameKey] as? String
    if let preferredDeviceId, candidate.identifier != preferredDeviceId { return }
    // Without a persisted choice, discovery is intentionally non-committing:
    // the MyGo UI must let the user choose which Passport to claim.
    guard preferredDeviceId != nil else {
      emit([
        "type": "discovered",
        "deviceId": candidate.identifier.uuidString,
        "deviceName": advertisedName ?? candidate.name ?? "伊言"
      ])
      return
    }
    stopScanIfPoweredOn()
    connect(candidate)
  }

  private func connect(_ candidate: CBPeripheral) {
    guard !shuttingDown, manager?.state == .poweredOn,
          VoiceContract.mayConnect(selected: preferredDeviceId, candidate: candidate.identifier) else { return }
    connectionEpoch &+= 1
    peripheral = candidate
    candidate.delegate = self
    stopScanIfPoweredOn()
    let epoch = connectionEpoch
    setupTimer?.invalidate()
    setupTimer = Timer.scheduledTimer(withTimeInterval: 10, repeats: false) { [weak self, weak candidate] _ in
      guard let self, let candidate,
            self.connectionEpoch == epoch, self.isCurrent(candidate) else { return }
      self.setupFailed("BLE service setup timed out")
    }
    manager.connect(candidate, options: nil)
  }

  func centralManager(_ central: CBCentralManager, didConnect candidate: CBPeripheral) {
    guard !shuttingDown, central.state == .poweredOn, isCurrent(candidate) else { return }
    reconnectWorkItem?.cancel()
    reconnectWorkItem = nil
    reconnectAttempt = 0
    candidate.delegate = self
    candidate.discoverServices([serviceUUID])
  }

  func centralManager(_ central: CBCentralManager, didFailToConnect candidate: CBPeripheral, error: Error?) {
    guard isCurrent(candidate) else { return }
    resetConnectionState()
    emit(["type": "diagnostic", "level": "warn", "message": error?.localizedDescription ?? "BLE connection failed"])
    scheduleReconnect()
  }

  func centralManager(_ central: CBCentralManager, didDisconnectPeripheral candidate: CBPeripheral, error: Error?) {
    guard isCurrent(candidate) else { return }
    let wasConnected = connectedEmitted
    resetConnectionState()
    emit(["type": "disconnected", "deviceId": candidate.identifier.uuidString])
    if !wasConnected, let error { emit(["type": "diagnostic", "level": "warn", "message": error.localizedDescription]) }
    scheduleReconnect()
  }

  func peripheral(_ peripheral: CBPeripheral, didDiscoverServices error: Error?) {
    guard isCurrent(peripheral), manager?.state == .poweredOn else { return }
    guard error == nil, let service = peripheral.services?.first(where: { $0.uuid == serviceUUID }) else {
      setupFailed(error?.localizedDescription ?? "Voice service not found")
      return
    }
    peripheral.discoverCharacteristics([controlUUID, audioUUID, infoUUID], for: service)
  }

  func peripheral(_ peripheral: CBPeripheral, didDiscoverCharacteristicsFor service: CBService, error: Error?) {
    guard isCurrent(peripheral), manager?.state == .poweredOn, service.uuid == serviceUUID else { return }
    guard error == nil else { setupFailed(error!.localizedDescription); return }
    for characteristic in service.characteristics ?? [] {
      if characteristic.uuid == controlUUID { control = characteristic; peripheral.setNotifyValue(true, for: characteristic) }
      if characteristic.uuid == audioUUID { audio = characteristic; peripheral.setNotifyValue(true, for: characteristic) }
      if characteristic.uuid == infoUUID { peripheral.readValue(for: characteristic) }
    }
    guard control != nil, audio != nil else { setupFailed("Control or Audio characteristic not found"); return }
  }

  func peripheral(_ peripheral: CBPeripheral, didUpdateNotificationStateFor characteristic: CBCharacteristic, error: Error?) {
    guard isCurrent(peripheral), manager?.state == .poweredOn,
          (characteristic.uuid == controlUUID || characteristic.uuid == audioUUID) else { return }
    guard error == nil, characteristic.isNotifying else {
      setupFailed(error?.localizedDescription ?? "BLE notification subscription failed")
      return
    }
    subscribed.insert(characteristic.uuid)
    completeSubscriptions(peripheral)
  }

  private func completeSubscriptions(_ peripheral: CBPeripheral) {
    guard subscribed.contains(controlUUID), subscribed.contains(audioUUID), !connectedEmitted else { return }
    guard validateNegotiatedMtu() else { return }
    if preferredDeviceId == nil { preferredDeviceId = peripheral.identifier }
    connectedEmitted = true
    setupTimer?.invalidate()
    setupTimer = nil
    emit([
      "type": "connected",
      "deviceId": peripheral.identifier.uuidString,
      "deviceName": peripheral.name ?? "伊言",
      "preferredDeviceId": preferredDeviceId?.uuidString ?? peripheral.identifier.uuidString
    ])
    emit(["type": "subscribed"])
    // Flush commands that may have arrived while CoreBluetooth was still
    // enabling the two notifications. This keeps host_ready after both CCCD
    // subscriptions, as required by the wire protocol.
    writeNextControl()
  }

  func peripheral(_ peripheral: CBPeripheral, didUpdateValueFor characteristic: CBCharacteristic, error: Error?) {
    guard isCurrent(peripheral) else { return }
    guard error == nil, let value = characteristic.value else {
      if let error { emit(["type": "diagnostic", "level": "warn", "message": error.localizedDescription]) }
      return
    }
    if characteristic.uuid == controlUUID {
      guard value.count <= maxControlMessageBytes else {
        emit(["type": "diagnostic", "level": "warn", "message": "Control notification too large"])
        return
      }
      emit(["type": "control", "payloadBase64": value.base64EncodedString()])
    } else if characteristic.uuid == audioUUID {
      ingestAudio(value)
    } else if characteristic.uuid == infoUUID {
      emitInfoHello(value)
    }
  }

  func peripheral(_ peripheral: CBPeripheral, didWriteValueFor characteristic: CBCharacteristic, error: Error?) {
    guard isCurrent(peripheral), manager?.state == .poweredOn,
          characteristic.uuid == controlUUID, writeInFlight != nil else { return }
    writeTimer?.invalidate()
    writeTimer = nil
    if let error, writeAttempts < 3 {
      emit(["type": "diagnostic", "level": "warn", "message": "Control write failed; retrying: \(error.localizedDescription)"])
      scheduleWriteRetryAfterFailure()
      return
    }
    if let error {
      setupFailed("BLE control write failed: \(error.localizedDescription)")
      return
    }
    writeRetryWorkItem?.cancel()
    writeRetryWorkItem = nil
    writeInFlight = nil
    writeAttempts = 0
    writeNextControl()
  }

  private func ingestAudio(_ data: Data) {
    guard data.count >= audioEnvelopeBytes else { emit(["type": "diagnostic", "level": "warn", "message": "Audio fragment too short"]); return }
    guard le16(data, 0) == audioMagic, data[2] == 1, data[3] == 0 else {
      emit(["type": "diagnostic", "level": "warn", "message": "Invalid audio envelope"]); return
    }
    let sessionId = le32(data, 4)
    let sequence = le32(data, 8)
    let index = Int(data[12]); let count = Int(data[13])
    let payloadLength = Int(le16(data, 14))
    guard count > 0, count <= maxAudioFragments, index < count, payloadLength > 0,
          sequence < UInt32(maxAudioSequences),
          payloadLength <= maxAudioFrameBytes, data.count == audioEnvelopeBytes + payloadLength else {
      emit(["type": "diagnostic", "level": "warn", "message": "Invalid audio fragment length"]); return
    }
    let key = AudioFrameKey(session: sessionId, sequence: sequence)
    if completed.contains(key) { return }
    let payload = data[audioEnvelopeBytes..<(audioEnvelopeBytes + payloadLength)]
    if count == 1 {
      guard index == 0, payloadLength == maxAudioFrameBytes else {
        emit(["type": "diagnostic", "level": "warn", "message": "Incomplete ADPCM frame payload"])
        return
      }
      rememberCompleted(key)
      emit(["type": "audio", "sessionId": Int(sessionId), "sequence": Int(sequence), "payloadBase64": payload.base64EncodedString()])
      return
    }
    let frame: PendingFrame
    if let existing = pending[key] {
      frame = existing
    } else {
      if pending.count >= maxPendingAudioFrames,
         let oldest = pendingOrder.first {
        removePending(oldest)
        emit(["type": "diagnostic", "level": "warn", "message": "Audio frame evicted before completion"])
      }
      frame = PendingFrame(sessionId: sessionId, sequence: sequence, count: count)
      pending[key] = frame
      pendingOrder.append(key)
    }
    guard frame.count == count else { emit(["type": "diagnostic", "level": "warn", "message": "Audio fragment count changed"]); return }
    if frame.parts[index] == nil {
      guard frame.totalBytes + payloadLength <= maxAudioFrameBytes else {
        removePending(key)
        emit(["type": "diagnostic", "level": "warn", "message": "Audio frame exceeds ADPCM payload limit"])
        return
      }
      frame.parts[index] = Data(payload)
      frame.totalBytes += payloadLength
      frame.received += 1
    }
    guard frame.received == count else { return }
    removePending(key)
    guard frame.totalBytes == maxAudioFrameBytes else {
      emit(["type": "diagnostic", "level": "warn", "message": "Incomplete ADPCM frame payload"])
      return
    }
    rememberCompleted(key)
    let assembled = frame.parts.compactMap { $0 }.reduce(into: Data()) { $0.append($1) }
    emit(["type": "audio", "sessionId": Int(sessionId), "sequence": Int(sequence), "payloadBase64": assembled.base64EncodedString()])
  }

  private func rememberCompleted(_ key: AudioFrameKey) {
    completed.insert(key)
    completedOrder.append(key)
    if completedOrder.count > 128 {
      completed.remove(completedOrder.removeFirst())
    }
  }

  private func removePending(_ key: AudioFrameKey) {
    guard pending.removeValue(forKey: key) != nil else { return }
    if let index = pendingOrder.firstIndex(of: key) {
      pendingOrder.remove(at: index)
    }
  }

  /// The Info characteristic is readable and CoreBluetooth reassembles long
  /// reads for us. It is therefore a useful fallback when a peripheral sends
  /// the notification-sized hello before ATT MTU exchange has completed.
  private func emitInfoHello(_ data: Data) {
    guard data.count > 0, data.count <= maxControlMessageBytes,
          let object = (try? JSONSerialization.jsonObject(with: data)) as? [String: Any],
          exactJsonInteger(object["v"], equals: 1),
          object["device"] as? String == "ai-passport",
          let firmware = object["fw"] as? String,
          !firmware.isEmpty,
          firmware.utf8.count <= maxDeviceFieldBytes,
          object["codec"] as? String == "ima-adpcm",
          exactJsonInteger(object["sampleRate"], equals: 16000),
          exactJsonInteger(object["channels"], equals: 1),
          exactJsonInteger(object["frameMs"], equals: 20) else {
      emit(["type": "diagnostic", "level": "warn", "message": "Invalid AI Passport info value"])
      return
    }
    // Rebuild the object from the fields V1 defines. This prevents arbitrary
    // extra values in the readable Info characteristic from crossing into the
    // Worker and keeps the notification/line size deterministic.
    let helloObject: [String: Any] = [
      "v": 1,
      "type": "hello",
      "device": "ai-passport",
      "fw": firmware,
      "codec": "ima-adpcm",
      "sampleRate": 16000,
      "channels": 1,
      "frameMs": 20
    ]
    guard JSONSerialization.isValidJSONObject(helloObject),
          let hello = try? JSONSerialization.data(withJSONObject: helloObject),
          hello.count <= maxControlMessageBytes else {
      emit(["type": "diagnostic", "level": "warn", "message": "AI Passport info value is too large"])
      return
    }
    emit(["type": "control", "payloadBase64": hello.base64EncodedString()])
  }

  private func sendControl(_ object: [String: Any]) {
    guard control != nil,
          let data = try? JSONSerialization.data(withJSONObject: object) else { return }
    guard data.count <= maxControlMessageBytes else {
      emit(["type": "error", "message": "Control message exceeds helper size limit"])
      return
    }
    // Avoid replaying a queued ready command when the Worker reports both the
    // subscription and the validated hello in quick succession.
    if object["type"] as? String == "host_ready",
       writeInFlight == data || writeQueue.contains(where: { $0 == data }) {
      return
    }
    // Queueing is intentional: CoreBluetooth may report a 20-byte write
    // limit briefly before ATT exchange. writeNextControl waits for the
    // negotiated limit instead of losing host_ready during that window.
    if writeQueue.count >= 64 {
      setupFailed("BLE control write queue overflow")
      return
    }
    writeQueue.append(data)
    writeNextControl()
  }

  private func writeNextControl() {
    guard connectedEmitted, writeInFlight == nil, let data = writeQueue.first, let characteristic = control,
          let peripheral, manager?.state == .poweredOn, peripheral.state == .connected else { return }
    let maximumLength = peripheral.maximumWriteValueLength(for: .withResponse)
    guard data.count <= maximumLength else {
      if mtuWaitDeadline == nil { mtuWaitDeadline = Date().addingTimeInterval(controlWriteMtuTimeout) }
      if Date() >= mtuWaitDeadline! {
        setupFailed("BLE control MTU is too small for host_ready")
        return
      }
      scheduleMtuRetry()
      return
    }
    mtuWaitDeadline = nil
    mtuRetryTimer?.invalidate()
    mtuRetryTimer = nil
    writeQueue.removeFirst()
    writeInFlight = data
    writeAttempts = 1
    peripheral.writeValue(data, for: characteristic, type: .withResponse)
    armWriteTimeout()
  }

  private func writeCurrentControl() {
    guard connectedEmitted, let data = writeInFlight, let characteristic = control, let peripheral,
          manager?.state == .poweredOn, peripheral.state == .connected else { return }
    let maximumLength = peripheral.maximumWriteValueLength(for: .withResponse)
    guard data.count <= maximumLength else {
      if mtuWaitDeadline == nil { mtuWaitDeadline = Date().addingTimeInterval(controlWriteMtuTimeout) }
      if Date() >= mtuWaitDeadline! {
        writeTimer?.invalidate()
        writeTimer = nil
        setupFailed("BLE control MTU is too small for host_ready")
        return
      }
      scheduleMtuRetry()
      return
    }
    writeRetryWorkItem?.cancel()
    writeRetryWorkItem = nil
    writeAttempts += 1
    peripheral.writeValue(data, for: characteristic, type: .withResponse)
    armWriteTimeout()
  }

  private func scheduleWriteRetryAfterFailure() {
    writeRetryWorkItem?.cancel()
    guard writeInFlight != nil, !shuttingDown else { return }
    let expected = writeInFlight
    let work = DispatchWorkItem { [weak self] in
      guard let self, self.writeInFlight == expected else { return }
      self.writeRetryWorkItem = nil
      self.writeCurrentControl()
    }
    writeRetryWorkItem = work
    DispatchQueue.main.asyncAfter(deadline: .now() + 0.1, execute: work)
  }

  private func scheduleMtuRetry() {
    guard mtuRetryTimer == nil, !shuttingDown else { return }
    mtuRetryTimer = Timer.scheduledTimer(withTimeInterval: controlWriteRetryInterval, repeats: false) { [weak self] _ in
      guard let self else { return }
      self.mtuRetryTimer = nil
      if self.connectedEmitted {
        self.writeNextControl()
      } else if let peripheral = self.peripheral {
        self.completeSubscriptions(peripheral)
      }
    }
  }

  /// CoreBluetooth exposes the negotiated ATT payload through the maximum
  /// write length. Use `.withoutResponse` as the probe: unlike `.withResponse`,
  /// it is the ATT_MTU - 3 payload limit and cannot report the 512-byte
  /// prepare/execute long-write limit on a small-MTU link. V1 requires MTU 185
  /// so a complete 166-byte ADPCM frame and its 16-byte envelope fit in one
  /// notification; lower links are rejected before the host announces the
  /// device as connected.
  @discardableResult
  private func validateNegotiatedMtu() -> Bool {
    guard let peripheral else { return false }
    let writePayload = peripheral.maximumWriteValueLength(for: .withoutResponse)
    let mtu = writePayload + attOverheadBytes
    guard writePayload >= minimumControlWriteBytes,
          VoiceContract.supportsAudio(writeWithoutResponse: writePayload),
          mtu >= minimumAttMtu else {
      if mtuWaitDeadline == nil {
        mtuWaitDeadline = Date().addingTimeInterval(controlWriteMtuTimeout)
      }
      if Date() >= mtuWaitDeadline! {
        setupFailed("BLE ATT MTU \(mtu) is below required minimum \(minimumAttMtu)")
      } else {
        scheduleMtuRetry()
      }
      return false
    }
    mtuWaitDeadline = nil
    mtuRetryTimer?.invalidate()
    mtuRetryTimer = nil
    return true
  }

  private func armWriteTimeout() {
    writeTimer?.invalidate()
    writeTimer = Timer.scheduledTimer(withTimeInterval: 1.0, repeats: false) { [weak self] _ in
      self?.controlWriteTimedOut()
    }
  }

  private func controlWriteTimedOut() {
    guard writeInFlight != nil else { return }
    writeTimer = nil
    if writeAttempts < 3 {
      emit(["type": "diagnostic", "level": "warn", "message": "Control write timed out; retrying"])
      writeCurrentControl()
      return
    }
    setupFailed("BLE control write timed out")
  }

  private func setupFailed(_ message: String) {
    emit(["type": "error", "message": message])
    let candidate = peripheral
    resetConnectionState()
    cancelPeripheralIfPoweredOn(candidate)
    scheduleReconnect()
  }

  private func setPreferredDevice(_ identifier: UUID?) {
    preferredDeviceId = identifier
    reconnectWorkItem?.cancel()
    reconnectWorkItem = nil
    reconnectAttempt = 0
    guard !shuttingDown, manager?.state == .poweredOn else { return }

    if let current = peripheral,
       identifier == nil || current.identifier != identifier {
      let wasConnected = connectedEmitted
      resetConnectionState()
      if wasConnected {
        emit(["type": "disconnected", "deviceId": current.identifier.uuidString])
      }
      cancelPeripheralIfPoweredOn(current)
      discover()
      return
    }

    // A changed target while scanning must restart discovery so stale scan
    // callbacks cannot select the old target.
    if peripheral == nil {
      stopScanIfPoweredOn()
      discover()
    }
  }

  private func resetConnectionState() {
    connectionEpoch &+= 1
    stopScanIfPoweredOn()
    setupTimer?.invalidate()
    setupTimer = nil
    peripheral = nil
    control = nil
    audio = nil
    subscribed.removeAll()
    pending.removeAll()
    pendingOrder.removeAll()
    completed.removeAll()
    completedOrder.removeAll()
    writeQueue.removeAll()
    writeInFlight = nil
    writeAttempts = 0
    writeTimer?.invalidate()
    writeTimer = nil
    writeRetryWorkItem?.cancel()
    writeRetryWorkItem = nil
    mtuRetryTimer?.invalidate()
    mtuRetryTimer = nil
    mtuWaitDeadline = nil
    connectedEmitted = false
  }

  private func scheduleReconnect() {
    guard !shuttingDown, reconnectWorkItem == nil, manager?.state == .poweredOn else { return }
    let delay = min(pow(2.0, Double(reconnectAttempt)), 15.0)
    reconnectAttempt += 1
    let workItem = DispatchWorkItem { [weak self] in
      guard let self else { return }
      self.reconnectWorkItem = nil
      self.discover()
    }
    reconnectWorkItem = workItem
    DispatchQueue.main.asyncAfter(deadline: .now() + delay, execute: workItem)
  }

  private func shutdown() {
    guard !shuttingDown else { return }
    shuttingDown = true
    parentWatchdog?.cancel()
    parentWatchdog = nil
    terminationSources.forEach { $0.cancel() }
    terminationSources.removeAll()
    reconnectWorkItem?.cancel()
    reconnectWorkItem = nil
    FileHandle.standardInput.readabilityHandler = nil
    stdinReaderInstalled = false
    stdinBuffer.removeAll(keepingCapacity: false)
    let candidate = peripheral
    resetConnectionState()
    cancelPeripheralIfPoweredOn(candidate)
    DispatchQueue.main.asyncAfter(deadline: .now() + .milliseconds(100)) {
      Darwin.exit(EXIT_SUCCESS)
    }
  }

  /// CoreBluetooth only accepts scan/connection commands while powered on.
  /// Keep all teardown call sites behind the same state check because state
  /// transitions can race disconnect/error/shutdown callbacks.
  private func stopScanIfPoweredOn() {
    guard let manager, manager.state == .poweredOn else { return }
    manager.stopScan()
  }

  private func cancelPeripheralIfPoweredOn(_ candidate: CBPeripheral?) {
    guard let candidate, let manager, manager.state == .poweredOn else { return }
    manager.cancelPeripheralConnection(candidate)
  }
}

let central = Central()
central.start()
