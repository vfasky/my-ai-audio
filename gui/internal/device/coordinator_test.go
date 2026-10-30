package device

import (
	"encoding/binary"
	"github.com/vfasky/my-ai-audio/gui/internal/protocol"
	"testing"
	"time"
)

func start(c *Coordinator, id, seq uint32, now time.Time) []Effect {
	c.Control(protocol.Control{Type: "hello"}, now)
	return c.Control(protocol.Control{Type: "ptt_down", SessionID: id, Seq: seq}, now)
}
func TestCoordinatorEarlyFramesTailAndInsertion(t *testing.T) {
	now := time.Unix(1, 0)
	c := Coordinator{}
	effects := start(&c, 7, 0, now)
	if len(effects) != 2 {
		t.Fatal(effects)
	}
	id := c.Generation
	b := make([]byte, 166)
	binary.LittleEndian.PutUint16(b[3:], 320)
	if e := c.Audio(protocol.Frame{SessionID: 7, Sequence: 0, Payload: b}, now); len(e) != 0 {
		t.Fatal("early emitted")
	}
	if e := c.Control(protocol.Control{Type: "ptt_up", SessionID: 7, Seq: 1, FinalSequence: 0}, now); len(e) != 0 {
		t.Fatal(e)
	}
	e := c.Speech("ready", id, "", now)
	if len(e) != 3 || e[0].Type != "pcm" || e[1].Type != "finish" {
		t.Fatal(e)
	}
	e = c.Speech("final", id, "你好", now)
	if len(e) != 1 || e[0].Type != "refine" {
		t.Fatal(e)
	}
	e = c.Speech("refinement_done", id, "你好。", now)
	if len(e) != 1 || e[0].Type != "insert" {
		t.Fatal(e)
	}
	e = c.Speech("inserted", id, "inserted", now)
	if len(e) != 2 || c.Active {
		t.Fatal(e)
	}
}
func TestDuplicateDisconnectCancelAndOldResult(t *testing.T) {
	now := time.Unix(1, 0)
	c := Coordinator{}
	start(&c, 1, 0, now)
	old := c.Generation
	if e := c.Control(protocol.Control{Type: "ptt_down", SessionID: 1, Seq: 0}, now); len(e) != 0 {
		t.Fatal("duplicate")
	}
	if e := c.Control(protocol.Control{Type: "ptt_down", SessionID: 2, Seq: 1}, now); len(e) != 0 {
		t.Fatal("overlapping start")
	}
	c.Disconnect()
	if c.Active || c.Ready {
		t.Fatal("disconnect")
	}
	start(&c, 1, 0, now)
	if c.Generation == old {
		t.Fatal("generation reused across reconnect")
	}
	for _, kind := range []string{"partial", "final", "refinement_done", "inserted", "error"} {
		if e := c.Speech(kind, old, "stale", now); len(e) != 0 {
			t.Fatal(kind, e)
		}
	}
	c.Cancel("cancel")
	if e := c.Speech("refinement_done", c.Generation, "stale", now); len(e) != 0 {
		t.Fatal("insert after cancellation")
	}
}
func TestAcceptanceAndButtons(t *testing.T) {
	now := time.Unix(1, 0)
	c := Coordinator{}
	start(&c, 1, 0, now)
	if e := c.Tick(now.Add(1200 * time.Millisecond)); len(e) != 3 || c.Active {
		t.Fatal(e)
	}
	e := c.Control(protocol.Control{Type: "button_event", Button: "down", Event: "click", Seq: 1}, now)
	if len(e) != 1 || e[0].Type != "enter" {
		t.Fatal(e)
	}
	e = c.Control(protocol.Control{Type: "button_event", Button: "ok", Event: "long", DurationMS: 650, Seq: 2}, now)
	if len(e) != 1 || e[0].Type != "clear" {
		t.Fatal(e)
	}
	e = c.Control(protocol.Control{Type: "button_event", Button: "ok", Event: "click", Seq: 3}, now)
	if len(e) != 1 || e[0].Type != "delete" {
		t.Fatal(e)
	}
}

func TestConsecutiveSessionsReusePTTSequenceNumbers(t *testing.T) {
	now := time.Unix(1, 0)
	c := Coordinator{}
	first := start(&c, 10, 0, now)
	if len(first) != 2 {
		t.Fatal(first)
	}
	firstGeneration := c.Generation
	if e := c.Speech("ready", firstGeneration, "", now); len(e) != 0 {
		t.Fatal(e)
	}
	if e := c.Control(protocol.Control{Type: "ptt_up", SessionID: 10, Seq: 1, FinalSequence: protocol.EmptyFinal}, now); len(e) != 2 || e[0].Type != "finish" {
		t.Fatal(e)
	}
	if e := c.Speech("final", firstGeneration, "第一段", now); len(e) != 1 || e[0].Type != "refine" {
		t.Fatal(e)
	}
	if e := c.Speech("refinement_done", firstGeneration, "第一段。", now); len(e) != 1 || e[0].Type != "insert" {
		t.Fatal(e)
	}
	if e := c.Speech("inserted", firstGeneration, "inserted", now); len(e) != 2 || c.Active {
		t.Fatal(e)
	}

	// Firmware intentionally restarts each PTT session at control seq 0/1.
	second := c.Control(protocol.Control{Type: "ptt_down", SessionID: 11, Seq: 0}, now.Add(time.Second))
	if len(second) != 2 || !c.Active || c.Generation == firstGeneration {
		t.Fatal(second, c.Generation)
	}
	// Button events own an independent sequence space and must not collide
	// with PTT sequence numbers.
	button := c.Control(protocol.Control{Type: "button_event", Button: "down", Event: "click", Seq: 0}, now)
	if len(button) != 1 || button[0].Type != "enter" {
		t.Fatal(button)
	}
	if duplicate := c.Control(protocol.Control{Type: "button_event", Button: "down", Event: "click", Seq: 0}, now); len(duplicate) != 0 {
		t.Fatal(duplicate)
	}
}

func TestCommitSkipsRefinementOnlyWhileRefining(t *testing.T) {
	now := time.Unix(1, 0)
	c := Coordinator{}
	start(&c, 3, 0, now)
	id := c.Generation
	if c.CanCommit() || len(c.Commit()) != 0 {
		t.Fatal("commit while recording")
	}
	b := make([]byte, 166)
	binary.LittleEndian.PutUint16(b[3:], 320)
	c.Audio(protocol.Frame{SessionID: 3, Sequence: 0, Payload: b}, now)
	c.Control(protocol.Control{Type: "ptt_up", SessionID: 3, Seq: 1, FinalSequence: 0}, now)
	c.Speech("ready", id, "", now)
	c.Speech("final", id, "原文", now)
	e := c.Commit()
	if len(e) != 1 || e[0].Type != "insert" || e[0].Text != "原文" || c.Phase != "inserting" {
		t.Fatal(e)
	}
	if late := c.Speech("refinement_done", id, "润色", now); len(late) != 0 || c.Refined != "原文" {
		t.Fatal("late refinement replaced the committed original")
	}
}

func TestStartTimeoutIsConfigurable(t *testing.T) {
	now := time.Unix(1, 0)
	c := Coordinator{}
	start(&c, 1, 0, now)
	if c.Tick(now.Add(1200 * time.Millisecond)); c.Active || c.Status != "Speech start timeout" {
		t.Fatalf("default timeout: %v %q", c.Active, c.Status)
	}
	// An online recognizer connects first: it gets longer, and the audio of
	// that time is buffered.
	c = Coordinator{StartTimeout: 4 * time.Second}
	start(&c, 2, 0, now)
	if c.Tick(now.Add(2500 * time.Millisecond)); !c.Active {
		t.Fatalf("online start timed out early: %q", c.Status)
	}
	if c.earlyFrames() != 200 {
		t.Fatalf("early frames %d", c.earlyFrames())
	}
}

func types(effects []Effect) []string {
	out := make([]string, len(effects))
	for i, e := range effects {
		out[i] = e.Type
	}
	return out
}

func TestMicSessionBuffersUntilReadyAndFinishesAfterMicEnd(t *testing.T) {
	now := time.Unix(1, 0)
	c := Coordinator{}
	e := c.MicStart(now)
	if len(e) != 2 || e[0].Type != "start" || e[1].Type != "mic_start" || !c.Active || !c.Mic {
		t.Fatal(types(e))
	}
	id := c.Generation
	frame := make([]byte, 640)
	binary.LittleEndian.PutUint16(frame, 0x4000)
	if e := c.MicAudio(id, frame, now); len(e) != 0 {
		t.Fatal("early frame emitted", types(e))
	}
	if e := c.MicAudio(id, frame[:320], now); len(e) != 0 || c.micSeq != 1 {
		t.Fatal("short frame accepted")
	}
	if e := c.MicAudio(id+1, frame, now); len(e) != 0 || c.micSeq != 1 {
		t.Fatal("frame of another session accepted")
	}
	if e := c.MicRelease(now); len(e) != 1 || e[0].Type != "mic_stop" || c.Phase != "tail" {
		t.Fatal(types(e))
	}
	if e := c.MicRelease(now); len(e) != 0 {
		t.Fatal("second release")
	}
	// The last frames arrive before the microphone reports its end.
	c.MicAudio(id, frame, now)
	if e := c.MicEnd(id, now); len(e) != 0 {
		t.Fatal("finished before ready", types(e))
	}
	if e := c.MicAudio(id, frame, now); len(e) != 0 || c.micSeq != 2 {
		t.Fatal("frame after mic end accepted")
	}
	e = c.Speech("ready", id, "", now)
	if got := types(e); len(got) != 3 || got[0] != "pcm" || got[1] != "pcm" || got[2] != "finish" {
		t.Fatal(got)
	}
	if e[0].Sequence != 0 || e[1].Sequence != 1 || c.Phase != "finalizing" || c.Meter.Frames != 2 {
		t.Fatal(e[0].Sequence, e[1].Sequence, c.Phase, c.Meter.Frames)
	}
	c.Speech("final", id, "你好", now)
	c.Speech("refinement_done", id, "你好。", now)
	// No device state is reported for a microphone session.
	if e := c.Speech("inserted", id, "inserted", now); len(e) != 0 || c.Active || c.Mic || c.Phase != "" {
		t.Fatal(types(e), c.Phase)
	}
}

func TestMicSessionReleaseWithoutMicEndFinishesAfterASecond(t *testing.T) {
	now := time.Unix(1, 0)
	c := Coordinator{}
	c.MicStart(now)
	id := c.Generation
	c.Speech("ready", id, "", now)
	c.MicAudio(id, make([]byte, 640), now)
	c.MicRelease(now)
	if e := c.Tick(now.Add(900 * time.Millisecond)); len(e) != 0 {
		t.Fatal(types(e))
	}
	if e := c.Tick(now.Add(time.Second)); len(e) != 1 || e[0].Type != "finish" {
		t.Fatal(types(e))
	}
}

func TestMicSessionSurvivesDeviceDisconnectAndBlocksPTT(t *testing.T) {
	now := time.Unix(1, 0)
	c := Coordinator{}
	c.Control(protocol.Control{Type: "hello"}, now)
	c.MicStart(now)
	if e := c.Control(protocol.Control{Type: "ptt_down", SessionID: 1}, now); len(e) != 0 {
		t.Fatal("device session started over a mic session")
	}
	if e := c.Disconnect(); len(e) != 0 || !c.Active || c.Phase != "starting" {
		t.Fatal(types(e), c.Phase)
	}
	if e := c.Cancel("已取消"); len(e) != 1 || e[0].Type != "cancel" || c.Phase != "disconnected" {
		t.Fatal(types(e), c.Phase)
	}
	if e := c.MicStart(now); len(e) != 2 {
		t.Fatal(types(e))
	}
	if e := c.Tick(now.Add(3 * time.Second)); c.Active {
		t.Fatal("silent microphone kept the session", types(e))
	}
}

func TestMicSessionStopsAtFrameLimit(t *testing.T) {
	now := time.Unix(1, 0)
	c := Coordinator{}
	c.MicStart(now)
	id := c.Generation
	c.Speech("ready", id, "", now)
	c.micSeq = protocol.MaxFrames - 1
	e := c.MicAudio(id, make([]byte, 640), now)
	if got := types(e); len(got) != 3 || got[0] != "pcm" || got[1] != "mic_stop" || got[2] != "finish" {
		t.Fatal(got)
	}
}
