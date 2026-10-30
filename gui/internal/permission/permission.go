// Package permission models the macOS privacy grants go-voice needs, reported
// by the native helpers, independent of TCC and of the UI toolkit.
package permission

type Name string
type State string

const (
	Bluetooth     Name = "bluetooth"
	Speech        Name = "speech"
	Accessibility Name = "accessibility"
	Automation    Name = "automation"
	// Microphone is the Mac's own microphone, used only by the hotkey.
	Microphone Name = "microphone"
)

const (
	Granted      State = "granted"
	Denied       State = "denied"
	Restricted   State = "restricted"
	Undetermined State = "undetermined"
	// Unknown is the state before a helper reports, or when macOS cannot say
	// (Automation while System Events is not running).
	Unknown State = ""
)

// Names lists the grants in the order the onboarding asks for them.
var Names = []Name{Bluetooth, Speech, Accessibility, Automation, Microphone}

// Parse validates a helper report; anything else is rejected.
func Parse(name, state string) (Name, State, bool) {
	n := Name(name)
	switch n {
	case Bluetooth, Speech, Accessibility, Automation, Microphone:
	default:
		return "", "", false
	}
	switch s := State(state); s {
	case Granted, Denied, Restricted, Undetermined:
		return n, s, true
	case "unknown":
		return n, Unknown, true
	}
	return "", "", false
}

// Required reports whether a session cannot work as designed without the
// grant. Automation is optional: without it insertion falls back to posting
// Command-V, which Accessibility covers. Microphone is optional: only the
// hotkey records with it.
func Required(n Name) bool { return n != Automation && n != Microphone }

// Set holds the latest known state of every grant.
type Set map[Name]State

// Missing returns the required grants not yet granted, in onboarding order.
func (s Set) Missing() []Name {
	var out []Name
	for _, n := range Names {
		if Required(n) && s[n] != Granted {
			out = append(out, n)
		}
	}
	return out
}

// Prompts reports whether the app can still show the system's own request;
// otherwise the user must change it in System Settings.
func Prompts(s State) bool { return s == Undetermined || s == Unknown }

// SettingsURL opens the grant's pane of Privacy & Security.
func SettingsURL(n Name) string {
	pane := map[Name]string{
		Bluetooth:     "Privacy_Bluetooth",
		Speech:        "Privacy_SpeechRecognition",
		Accessibility: "Privacy_Accessibility",
		Automation:    "Privacy_Automation",
		Microphone:    "Privacy_Microphone",
	}[n]
	return "x-apple.systempreferences:com.apple.preference.security?" + pane
}

func Title(n Name) string {
	return map[Name]string{
		Bluetooth:     "蓝牙",
		Speech:        "语音识别",
		Accessibility: "辅助功能",
		Automation:    "自动化（System Events）",
		Microphone:    "麦克风",
	}[n]
}

func Purpose(n Name) string {
	return map[Name]string{
		Bluetooth:     "连接你选择的 FoloToy AI Passport 并接收语音音频。",
		Speech:        "在本机把收到的音频转写为文字。",
		Accessibility: "把最终文字粘贴到你选择的应用；未授权时文字仅保留在剪贴板。",
		Automation:    "可选。通过 System Events 粘贴更稳妥；未授权时改用辅助功能按键粘贴。",
		Microphone:    "可选。按住热键时用 Mac 麦克风录音，松开后识别；不用热键时不需要。",
	}[n]
}

func Label(s State) string {
	switch s {
	case Granted:
		return "已授权"
	case Denied:
		return "已拒绝"
	case Restricted:
		return "受限制"
	case Undetermined:
		return "未授权"
	}
	return "未检测"
}
