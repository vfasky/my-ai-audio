package bridge

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestHelperValidation(t *testing.T) {
	b := make([]byte, 166)
	b[0] = 7
	valid := Event{Type: "audio", SessionID: 4, Sequence: 2, PayloadBase64: base64.StdEncoding.EncodeToString(b)}
	line, _ := json.Marshal(valid)
	e, err := Parse(line)
	if err != nil || len(e.Payload) != 166 || e.Payload[0] != 7 || e.SessionID != 4 || e.Sequence != 2 {
		t.Fatal(err, e)
	}
	if _, err = Parse([]byte(`{"type":"audio","sessionId":1,"sequence":0,"payloadBase64":"AA==","extra":1}`)); err == nil {
		t.Fatal("audio extra field")
	}
	cases := []string{`{"type":"audio","payloadBase64":"AA=="}`, `{"type":"discovered","deviceName":"` + strings.Repeat("x", 257) + `"}`, `{"type":"audio","sessionId":1,"sequence":15000,"payloadBase64":"AA=="}`, `{"type":"control","payloadBase64":"AB=="}`, `{"type":"control","payloadBase64":"AA==\n"}`, `{"type":"connected","unexpected":1}`, `{"type":"connected","type":"connected"}`, `{"type":"error","message":"` + strings.Repeat("x", 129) + `"}`, strings.Repeat(" ", MaxLine+1)}
	for _, line := range cases {
		if _, err := Parse([]byte(line)); err == nil {
			t.Fatal("accepted", line)
		}
	}
}

func TestPermissionEvents(t *testing.T) {
	e, err := Parse([]byte(`{"type":"permission","permission":"speech","state":"undetermined"}`))
	if err != nil || e.Permission != "speech" || e.State != "undetermined" {
		t.Fatal(err)
	}
	for _, line := range []string{
		`{"type":"permission","permission":"camera","state":"granted"}`,
		`{"type":"permission","permission":"speech","state":"maybe"}`,
		`{"type":"permission","permission":"speech"}`,
		`{"type":"permission","permission":"speech","state":"granted","message":"x"}`,
		`{"type":"connected","permission":"speech","state":"granted"}`,
	} {
		if _, err := Parse([]byte(line)); err == nil {
			t.Fatal("accepted", line)
		}
	}
}

func TestMicrophoneAndHotkeyEvents(t *testing.T) {
	frame := base64.StdEncoding.EncodeToString(make([]byte, MicFrame))
	e, err := Parse([]byte(`{"type":"mic","sessionId":3,"sequence":0,"payloadBase64":"` + frame + `"}`))
	if err != nil || len(e.Payload) != MicFrame || e.SessionID != 3 {
		t.Fatal(err, e)
	}
	for _, text := range []string{"down", "up", "abort"} {
		if e, err := Parse([]byte(`{"type":"hotkey","text":"` + text + `"}`)); err != nil || e.Text != text {
			t.Fatal(text, err)
		}
	}
	if e, err := Parse([]byte(`{"type":"mic-end","sessionId":3}`)); err != nil || e.SessionID != 3 {
		t.Fatal(err)
	}
	short := base64.StdEncoding.EncodeToString(make([]byte, MicFrame-2))
	for _, line := range []string{
		`{"type":"mic","sessionId":3,"sequence":0,"payloadBase64":"` + short + `"}`,
		`{"type":"mic","sequence":0,"payloadBase64":"` + frame + `"}`,
		`{"type":"mic","sessionId":3,"sequence":15000,"payloadBase64":"` + frame + `"}`,
		`{"type":"mic","sessionId":3,"sequence":0,"text":"x","payloadBase64":"` + frame + `"}`,
		`{"type":"hotkey","text":"left"}`,
		`{"type":"hotkey"}`,
		`{"type":"mic-end"}`,
		`{"type":"mic-error","message":"x"}`,
	} {
		if _, err := Parse([]byte(line)); err == nil {
			t.Fatal("accepted", line)
		}
	}
}
