// Package protocol implements the wire contract in root docs/ai-passport-ble-protocol.md.
package protocol

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	ServiceUUID = "676f0001-766f-6963-652d-617564696f00"
	ControlUUID = "676f0002-766f-6963-652d-617564696f00"
	AudioUUID   = "676f0003-766f-6963-652d-617564696f00"
	InfoUUID    = "676f0004-766f-6963-652d-617564696f00"
	FrameBytes  = 166
	Samples     = 320
	// MaxFrames bounds the audio sequence of one session: MaxDuration of
	// 20 ms frames.
	MaxFrames  = uint32(MaxDuration / (20 * time.Millisecond))
	EmptyFinal = uint32(0xffffffff)
)

// MaxDuration is the longest session the host records; an engine may set a
// shorter one. At the bound the host ends the recording and recognizes it.
const MaxDuration = 30 * time.Minute

var ErrInvalid = errors.New("invalid protocol data")

// Object rejects duplicate keys, trailing values, null and invalid UTF-8.
func Object(data []byte, limit int) (map[string]json.RawMessage, error) {
	if len(data) == 0 || len(data) > limit || !utf8.Valid(data) {
		return nil, ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(data))
	t, e := d.Token()
	if e != nil || t != json.Delim('{') {
		return nil, ErrInvalid
	}
	m := map[string]json.RawMessage{}
	for d.More() {
		t, e = d.Token()
		if e != nil {
			return nil, ErrInvalid
		}
		k, ok := t.(string)
		if !ok {
			return nil, ErrInvalid
		}
		if _, ok = m[k]; ok {
			return nil, ErrInvalid
		}
		var v json.RawMessage
		if d.Decode(&v) != nil || bytes.Equal(v, []byte("null")) {
			return nil, ErrInvalid
		}
		m[k] = v
	}
	if _, e = d.Token(); e != nil {
		return nil, ErrInvalid
	}
	if _, e = d.Token(); e != io.EOF {
		return nil, ErrInvalid
	}
	return m, nil
}
func uintField(m map[string]json.RawMessage, k string) (uint32, error) {
	var v uint32
	b, ok := m[k]
	if !ok {
		return 0, ErrInvalid
	}
	if json.Unmarshal(b, &v) != nil {
		return 0, ErrInvalid
	}
	return v, nil
}
func str(m map[string]json.RawMessage, k string) string {
	var v string
	_ = json.Unmarshal(m[k], &v)
	return v
}

type Control struct {
	Type, Firmware, State, Button, Event, Message string
	SessionID, Seq, FinalSequence, DurationMS     uint32
}

func ParseControl(data []byte) (Control, error) {
	c := Control{}
	m, e := Object(data, 182)
	if e != nil {
		return c, e
	}
	v, e := uintField(m, "v")
	if e != nil || v != 1 {
		return c, ErrInvalid
	}
	c.Type = str(m, "type")
	allowed := "v type"
	required := "v type"
	switch c.Type {
	case "hello":
		allowed += " device fw codec sampleRate channels frameMs"
		required = allowed
		c.Firmware = str(m, "fw")
		if str(m, "device") != "ai-passport" || strings.TrimSpace(c.Firmware) == "" || str(m, "codec") != "ima-adpcm" {
			return c, ErrInvalid
		}
		for k, w := range map[string]uint32{"sampleRate": 16000, "channels": 1, "frameMs": 20} {
			n, e := uintField(m, k)
			if e != nil || n != w {
				return c, ErrInvalid
			}
		}
	case "ptt_down":
		allowed += " sessionId seq"
		required = allowed
	case "ptt_up":
		allowed += " sessionId seq finalSequence"
		required = allowed
	case "device_error":
		allowed += " sessionId message"
		required = "v type sessionId"
		c.Message = str(m, "message")
		if len(c.Message) > 128 {
			return c, ErrInvalid
		}
		if _, ok := m["message"]; ok && c.Message == "" {
			return c, ErrInvalid
		}
	case "button_event":
		allowed += " button event durationMs seq"
		required = allowed
		c.Button = str(m, "button")
		c.Event = str(m, "event")
		if c.Button != "down" && c.Button != "ok" {
			return c, ErrInvalid
		}
		if c.Event != "click" && c.Event != "long" {
			return c, ErrInvalid
		}
		if c.Button == "down" && c.Event != "click" {
			return c, ErrInvalid
		}
	case "host_ready":
	case "host_state":
		allowed += " state"
		required = allowed
		c.State = str(m, "state")
		if !strings.Contains(" ready recording processing success error ", " "+c.State+" ") || c.State == "" {
			return c, ErrInvalid
		}
	default:
		return c, ErrInvalid
	}
	for k := range m {
		if !strings.Contains(" "+allowed+" ", " "+k+" ") {
			return c, ErrInvalid
		}
	}
	for _, k := range strings.Fields(required) {
		if _, ok := m[k]; !ok {
			return c, ErrInvalid
		}
	}
	for k, p := range map[string]*uint32{"sessionId": &c.SessionID, "seq": &c.Seq, "finalSequence": &c.FinalSequence, "durationMs": &c.DurationMS} {
		if _, ok := m[k]; ok {
			n, e := uintField(m, k)
			if e != nil {
				return c, e
			}
			*p = n
		}
	}
	if c.DurationMS > 60000 || c.Event == "long" && c.DurationMS < 650 {
		return c, ErrInvalid
	}
	return c, nil
}
