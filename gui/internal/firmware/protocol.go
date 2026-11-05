// Package firmware owns the bounded installer protocol and in-memory install state.
package firmware

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/vfasky/my-ai-audio/gui/internal/protocol"
)

const MaxLine = 16 * 1024
const MaxDevices = 32

type Device struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type Event struct {
	Type        string   `json:"type"`
	Devices     []Device `json:"devices,omitempty"`
	Stage       string   `json:"stage,omitempty"`
	Percent     int      `json:"percent,omitempty"`
	Message     string   `json:"message,omitempty"`
	Code        string   `json:"code,omitempty"`
	Recoverable bool     `json:"recoverable,omitempty"`
}

func safeText(s string, max int) bool {
	if len(s) > max || !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == utf8.RuneError {
			return false
		}
	}
	return true
}
func ValidPort(s string) bool {
	if !safeText(s, 256) || filepath.Clean(s) != s || !(strings.HasPrefix(s, "/dev/cu.") || strings.HasPrefix(s, "/dev/tty.")) {
		return false
	}
	tail := strings.TrimPrefix(strings.TrimPrefix(s, "/dev/cu."), "/dev/tty.")
	if tail == "" || strings.Contains(tail, "..") {
		return false
	}
	for _, r := range tail {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.') {
			return false
		}
	}
	return true
}
func Parse(line []byte) (Event, error) {
	var e Event
	m, err := protocol.Object(line, MaxLine)
	if err != nil {
		return e, err
	}
	d := json.NewDecoder(bytes.NewReader(line))
	d.DisallowUnknownFields()
	if d.Decode(&e) != nil {
		return Event{}, protocol.ErrInvalid
	}
	fields := "type"
	switch e.Type {
	case "ready", "complete":
	case "ports":
		fields += " devices"
	case "progress":
		fields += " stage percent message"
	case "error":
		fields += " code message recoverable"
	default:
		return Event{}, protocol.ErrInvalid
	}
	required := strings.Fields(fields)
	if len(m) != len(required) {
		return Event{}, protocol.ErrInvalid
	}
	for _, k := range required {
		if _, ok := m[k]; !ok {
			return Event{}, protocol.ErrInvalid
		}
	}
	if !safeText(e.Message, 512) || strings.ContainsAny(e.Message, "/\\") || !safeText(e.Code, 64) {
		return Event{}, protocol.ErrInvalid
	}
	if e.Type == "error" && (e.Code == "" || e.Message == "") {
		return Event{}, protocol.ErrInvalid
	}
	if e.Type == "progress" && (stageRank(e.Stage) == 0 || e.Percent < 0 || e.Percent > 100) {
		return Event{}, protocol.ErrInvalid
	}
	if len(e.Devices) > MaxDevices {
		return Event{}, protocol.ErrInvalid
	}
	if e.Type == "ports" {
		var raw []json.RawMessage
		if json.Unmarshal(m["devices"], &raw) != nil {
			return Event{}, protocol.ErrInvalid
		}
		seen := map[string]bool{}
		for i, dev := range e.Devices {
			obj, err := protocol.Object(raw[i], MaxLine)
			if err != nil || len(obj) != 2 || obj["id"] == nil || obj["name"] == nil || !ValidPort(dev.ID) || !safeText(dev.Name, 128) || dev.Name == "" || seen[dev.ID] {
				return Event{}, protocol.ErrInvalid
			}
			seen[dev.ID] = true
		}
	}
	return e, nil
}
func stageRank(s string) int {
	switch s {
	case "preflight":
		return 1
	case "writing":
		return 2
	case "verifying":
		return 3
	case "restarting":
		return 4
	}
	return 0
}

type State struct {
	Phase       string
	HelperReady bool
	Devices     []Device
	Selected    string
	Percent     int
	Message     string
	ErrorCode   string
	FailedStage string
}

func Initial() State {
	return State{Phase: "unavailable", Message: "固件安装助手不可用，请检查应用是否完整。"}
}
func (s State) Busy() bool    { return stageRank(s.Phase) > 0 }
func (s State) CanScan() bool { return s.HelperReady && !s.Busy() && s.Phase != "scanning" }
func (s State) CanInstall(active bool) bool {
	return s.HelperReady && !active && s.Phase == "ready" && s.Selected != ""
}
func (s *State) Scan() bool {
	if !s.CanScan() {
		return false
	}
	s.Phase = "scanning"
	s.Devices = nil
	s.Selected = ""
	s.Percent = 0
	s.ErrorCode = ""
	s.FailedStage = ""
	s.Message = "正在扫描兼容的 USB 设备…"
	return true
}
func (s *State) Select(id string) {
	if s.Phase != "ready" {
		return
	}
	for _, d := range s.Devices {
		if d.ID == id {
			s.Selected = id
			return
		}
	}
}
func (s *State) Install(active bool) bool {
	if !s.CanInstall(active) {
		return false
	}
	s.Phase = "preflight"
	s.Percent = 0
	s.ErrorCode = ""
	s.FailedStage = ""
	s.Message = "正在检查设备与随应用提供的固件…"
	return true
}
func (s *State) Unavailable() { *s = Initial() }

// Apply ignores duplicate and stale events; complete is accepted only after restart.
func (s *State) Apply(e Event) bool {
	switch e.Type {
	case "ready":
		if s.HelperReady || s.Phase != "unavailable" {
			return false
		}
		s.HelperReady = true
		s.Phase = "ready"
		s.Message = "请扫描并选择 USB 设备。"
	case "ports":
		if s.Phase != "scanning" {
			return false
		}
		s.Devices = append([]Device(nil), e.Devices...)
		s.Phase = "ready"
		s.Message = "请选择要安装固件的 USB 设备。"
	case "progress":
		current, next := stageRank(s.Phase), stageRank(e.Stage)
		if current == 0 || next < current || next > current+1 || next == current && e.Percent < s.Percent {
			return false
		}
		s.Phase = e.Stage
		s.Percent = e.Percent
		s.Message = StageLabel(e.Stage)
	case "complete":
		if s.Phase != "restarting" {
			return false
		}
		s.Phase = "complete"
		s.Percent = 100
		s.Selected = ""
		s.Devices = nil
		s.Message = "安装完成。请到设备页明确选择刚重启的 FoloToy AI Passport；USB 与蓝牙设备不会自动绑定，也不代表已认证配对。"
		return true
	case "error":
		if !s.Busy() && s.Phase != "scanning" {
			return false
		}
		s.FailedStage = s.Phase
		s.ErrorCode = e.Code
		s.Phase = "error"
		s.Selected = ""
		s.Devices = nil
		s.HelperReady = e.Recoverable
		s.Message = ErrorLabel(e.Code)
	}
	return false
}
func ErrorLabel(code string) string {
	switch code {
	case "INVALID_MANIFEST", "INVALID_IMAGE", "HASH_MISMATCH", "INVALID_FILE", "FILE_READ_FAILED":
		return "随应用提供的固件未通过完整性检查，请重新安装 伊言。"
	case "SCAN_FAILED", "PORT_NOT_ALLOWED":
		return "兼容 USB 设备已断开或不可用，请重新连接并扫描。"
	case "CONNECT_FAILED":
		return "无法让设备进入安全刷写模式，请重新上电、连接后再试。"
	case "CHIP_MISMATCH", "FLASH_SIZE_MISMATCH", "INVALID_LAYOUT", "RECOVERY_MARKER_MISSING":
		return "设备硬件或保护分区布局不兼容，已在写入前停止。"
	case "SECURITY_UNAVAILABLE", "UNSAFE_SECURITY_STATE", "FLASH_SIZE_UNAVAILABLE":
		return "无法确认设备安全状态或 Flash 容量，已在写入前停止。"
	case "PREFLIGHT_READ_UNAVAILABLE", "UNSAFE_READ":
		return "设备 ROM 不支持所需的只读安全检查，未执行写入。"
	case "FLASH_FAILED", "VERIFICATION_UNAVAILABLE":
		return "写入、设备端校验或复位未完成；请重新连接 USB 后重试，无法启动时使用设备原厂恢复方式。"
	case "OUTPUT_FAILED":
		return "安装助手通信中断，请重启应用后检查设备状态。"
	default:
		return "安装或扫描失败，请检查 USB 连接后重新扫描。"
	}
}
func StageLabel(s string) string {
	switch s {
	case "unavailable":
		return "助手不可用"
	case "scanning":
		return "扫描中"
	case "ready":
		return "等待选择"
	case "preflight":
		return "安装前检查"
	case "writing":
		return "正在写入，请勿拔线"
	case "verifying":
		return "正在验证固件"
	case "restarting":
		return "正在重启设备"
	case "complete":
		return "安装完成"
	case "error":
		return "安装失败"
	}
	return "等待助手"
}
