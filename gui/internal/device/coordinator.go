// Package device reduces transport and speech events; effects are owned by app.
package device

import (
	"encoding/binary"
	"github.com/vfasky/my-ai-audio/gui/internal/protocol"
	"time"
)

type Effect struct {
	Type, State, Text string
	ID                uint32
	Sequence          uint32
	PCM               []byte
}
type Coordinator struct {
	Ready                              bool
	Active                             bool
	Generation                         uint32
	Stream                             *protocol.Stream
	Phase                              string
	Partial, Original, Refined, Status string
	Meter                              protocol.Meter
	accepted                           bool
	started, processing                time.Time
	buffer                             []protocol.PCM
	seen                               map[uint32]bool
	order                              []uint32
	buttons                            map[uint32]bool
	buttonOrder                        []uint32
	wire                               uint32
	// idle is the phase between sessions while no device is ready: empty
	// before the first connection, "disconnected" after one.
	idle string
	// A session recorded by the Mac microphone: Mic marks it, micOpen
	// accepts its audio, released is when the hotkey was let go, micDone
	// asks consume to finish it, micSeq numbers its frames and lastMic is
	// when the last one came.
	Mic              bool
	micOpen, micDone bool
	released         time.Time
	lastMic          time.Time
	micSeq           uint32
	// StartTimeout bounds the wait for the recognizer's ready, 1.2 s when
	// zero; the audio meanwhile is buffered.
	StartTimeout time.Duration
}

const defaultStartTimeout = 1200 * time.Millisecond

func (c *Coordinator) startTimeout() time.Duration {
	if c.StartTimeout > 0 {
		return c.StartTimeout
	}
	return defaultStartTimeout
}

// earlyFrames bounds the audio buffered before ready: 1.5 s, or the start
// timeout when that is longer.
func (c *Coordinator) earlyFrames() int {
	return max(75, int(c.startTimeout()/(20*time.Millisecond)))
}

// states tells the device its state; a microphone session leaves the
// device alone.
func (c *Coordinator) states(states ...string) []Effect {
	if c.Mic {
		return nil
	}
	out := make([]Effect, len(states))
	for i, s := range states {
		out[i] = Effect{Type: "host_state", State: s}
	}
	return out
}

// idlePhase is the phase once a session ends.
func (c *Coordinator) idlePhase() string {
	if c.Ready {
		return "ready"
	}
	return c.idle
}

func (c *Coordinator) Cancel(reason string) []Effect {
	if !c.Active {
		return nil
	}
	id := c.Generation
	out := append([]Effect{{Type: "cancel", ID: id}}, c.states("error", "ready")...)
	c.Active = false
	c.Phase = c.idlePhase()
	c.Status = reason
	c.Stream = nil
	c.buffer = nil
	c.accepted = false
	c.Mic, c.micOpen, c.micDone = false, false, false
	return out
}

// begin resets the state of a new session.
func (c *Coordinator) begin(now time.Time) {
	c.Generation++
	c.Active = true
	c.Phase = "starting"
	c.started = now
	c.processing = time.Time{}
	c.Status = ""
	c.accepted = false
	c.buffer = nil
	c.Partial = ""
	c.Original = ""
	c.Refined = ""
	c.Meter = protocol.Meter{}
	c.Stream = nil
	c.Mic, c.micOpen, c.micDone = false, false, false
}

// MicStart begins a session that the Mac microphone records while the
// hotkey is held. It needs no device.
func (c *Coordinator) MicStart(now time.Time) []Effect {
	if c.Active {
		return nil
	}
	c.begin(now)
	c.Mic, c.micOpen = true, true
	c.released = time.Time{}
	c.lastMic = now
	c.micSeq = 0
	return []Effect{{Type: "start", ID: c.Generation}, {Type: "mic_start", ID: c.Generation}}
}

// MicHeld reports whether a microphone session records with the hotkey
// still held.
func (c *Coordinator) MicHeld() bool {
	return c.Active && c.Mic && c.micOpen && c.released.IsZero()
}

// MicRelease is the hotkey let go: the microphone stops, and MicEnd, or a
// second without it, finishes the session.
func (c *Coordinator) MicRelease(now time.Time) []Effect {
	if !c.Active || !c.Mic || !c.micOpen || !c.released.IsZero() {
		return nil
	}
	c.released = now
	c.Phase = "tail"
	return []Effect{{Type: "mic_stop", ID: c.Generation}}
}

// MicEnd is the microphone of session id stopped, after its last frame.
func (c *Coordinator) MicEnd(id uint32, now time.Time) []Effect {
	if !c.Active || !c.Mic || !c.micOpen || id != c.Generation {
		return nil
	}
	return c.endMic(now)
}

// MicAudio takes a 20 ms frame of 16 kHz signed 16-bit little-endian mono
// PCM from the microphone of session id. At the frame limit of a session
// the recording stops as if the hotkey were let go.
func (c *Coordinator) MicAudio(id uint32, pcm []byte, now time.Time) []Effect {
	if !c.Active || !c.Mic || !c.micOpen || id != c.Generation || len(pcm) != protocol.Samples*2 {
		return nil
	}
	p := protocol.PCM{Sequence: c.micSeq}
	for i := range p.Samples {
		p.Samples[i] = int16(binary.LittleEndian.Uint16(pcm[i*2:]))
	}
	c.micSeq++
	c.lastMic = now
	c.Meter.Add(p)
	out := c.consume([]protocol.PCM{p}, now)
	if c.Active && c.micOpen && c.micSeq >= protocol.MaxFrames {
		stop := c.MicRelease(now)
		out = append(append(out, stop...), c.endMic(now)...)
	}
	return out
}

// endMic closes the recording; consume finishes the session once the
// recognizer has taken the buffered audio.
func (c *Coordinator) endMic(now time.Time) []Effect {
	c.micOpen = false
	c.micDone = true
	c.Phase = "tail"
	return c.consume(nil, now)
}

// CanCommit reports whether Commit would insert now: while refinement runs,
// the final ASR original is already available.
func (c *Coordinator) CanCommit() bool {
	return c.Active && c.Phase == "refining" && c.Original != ""
}

// Commit skips refinement and inserts the final ASR original. A late
// refinement_done is then ignored, as the phase is no longer refining.
func (c *Coordinator) Commit() []Effect {
	if !c.CanCommit() {
		return nil
	}
	c.Refined = c.Original
	c.Phase = "inserting"
	return []Effect{{Type: "insert", ID: c.Generation, Text: c.Original}}
}

// Disconnect forgets the device. A microphone session goes on without it.
func (c *Coordinator) Disconnect() []Effect {
	var effects []Effect
	if !c.Mic {
		effects = c.Cancel("Disconnected; transcript not inserted")
	}
	c.Ready = false
	c.seen = nil
	c.order = nil
	c.buttons = nil
	c.buttonOrder = nil
	c.idle = "disconnected"
	if !c.Active {
		c.Phase = c.idle
	}
	return effects
}
func (c *Coordinator) Control(v protocol.Control, now time.Time) []Effect {
	if v.Type == "hello" {
		c.Ready = true
		return []Effect{{Type: "host_ready"}}
	}
	if !c.Ready {
		return nil
	}
	if v.Type == "button_event" {
		if c.buttons == nil {
			c.buttons = map[uint32]bool{}
		}
		if c.buttons[v.Seq] {
			return nil
		}
		c.buttons[v.Seq] = true
		c.buttonOrder = append(c.buttonOrder, v.Seq)
		if len(c.buttonOrder) > 128 {
			delete(c.buttons, c.buttonOrder[0])
			c.buttonOrder = c.buttonOrder[1:]
		}
	}
	switch v.Type {
	case "ptt_down":
		if c.Active {
			return nil
		}
		if c.seen == nil {
			c.seen = map[uint32]bool{}
		}
		if c.seen[v.SessionID] {
			return nil
		}
		c.seen[v.SessionID] = true
		c.order = append(c.order, v.SessionID)
		if len(c.order) > 128 {
			delete(c.seen, c.order[0])
			c.order = c.order[1:]
		}
		c.begin(now)
		c.wire = v.SessionID
		c.Stream = protocol.NewStream(v.SessionID, now)
		return append([]Effect{{Type: "start", ID: c.Generation}}, c.states("recording")...)
	case "ptt_up":
		if !c.Active || c.Stream == nil || v.SessionID != c.wire {
			return nil
		}
		frames, e := c.Stream.Stop(v.FinalSequence, now)
		if e != nil {
			return c.Cancel(e.Error())
		}
		c.Phase = "tail"
		return c.consume(frames, now)
	case "device_error":
		if c.Active && v.SessionID == c.wire {
			return c.Cancel("Device error")
		}
	case "button_event":
		if v.Button == "down" && v.Event == "click" {
			return []Effect{{Type: "enter"}}
		}
		if v.Button == "ok" {
			if c.Active {
				return c.Cancel("Cancelled")
			}
			if v.Event == "long" && v.DurationMS >= 650 {
				return []Effect{{Type: "clear"}}
			}
			if v.Event == "click" {
				return []Effect{{Type: "delete"}}
			}
		}
	}
	return nil
}
func (c *Coordinator) Audio(f protocol.Frame, now time.Time) []Effect {
	if !c.Active || c.Stream == nil {
		return nil
	}
	frames, e := c.Stream.Push(f, now)
	if e != nil {
		return c.Cancel(e.Error())
	}
	return c.consume(frames, now)
}
func (c *Coordinator) Tick(now time.Time) []Effect {
	if !c.Active {
		return nil
	}
	if !c.accepted && now.Sub(c.started) >= c.startTimeout() {
		return c.Cancel("Speech start timeout")
	}
	if !c.processing.IsZero() && now.Sub(c.processing) >= 60*time.Second {
		return c.Cancel("Processing timeout")
	}
	if c.Mic && c.micOpen {
		if !c.released.IsZero() && now.Sub(c.released) >= time.Second {
			return c.endMic(now)
		}
		if now.Sub(c.lastMic) >= 3*time.Second {
			return c.Cancel("麦克风连续 3 秒没有音频：请检查输入设备")
		}
	}
	if c.Stream == nil {
		return nil
	}
	frames, e := c.Stream.Tick(now)
	if e != nil {
		return c.Cancel(e.Error())
	}
	return c.consume(frames, now)
}
func (c *Coordinator) consume(frames []protocol.PCM, now time.Time) []Effect {
	if c.Stream != nil {
		c.Meter = c.Stream.Meter
	}
	if !c.accepted {
		c.buffer = append(c.buffer, frames...)
		if len(c.buffer) > c.earlyFrames() {
			return c.Cancel("Early audio buffer overflow")
		}
		return nil
	}
	var out []Effect
	for _, p := range frames {
		out = append(out, Effect{Type: "pcm", ID: c.Generation, Sequence: p.Sequence, PCM: protocol.PCMBytes(p.Samples)})
	}
	done := c.micDone
	c.micDone = false
	if c.Stream != nil && c.Stream.Done {
		c.Stream = nil
		done = true
	}
	if done {
		c.Phase = "finalizing"
		c.processing = now
		out = append(out, Effect{Type: "finish", ID: c.Generation})
		out = append(out, c.states("processing")...)
	}
	return out
}
func (c *Coordinator) Speech(kind string, id uint32, text string, now time.Time) []Effect {
	if !c.Active || id != c.Generation {
		return nil
	}
	switch kind {
	case "ready":
		if c.accepted {
			return nil
		}
		c.accepted = true
		c.processing = time.Time{}
		if c.Phase == "starting" {
			c.Phase = "recording"
		}
		frames := c.buffer
		c.buffer = nil
		return c.consume(frames, now)
	case "partial":
		c.Partial = text
	case "final":
		if c.Phase != "finalizing" {
			return nil
		}
		if text == "" {
			return c.Cancel("Empty transcription")
		}
		c.Original = text
		c.Phase = "refining"
		return []Effect{{Type: "refine", ID: id, Text: text}}
	case "refinement_done":
		if c.Phase != "refining" {
			return nil
		}
		c.Refined = text
		c.Phase = "inserting"
		return []Effect{{Type: "insert", ID: id, Text: text}}
	case "inserted":
		if c.Phase != "inserting" {
			return nil
		}
		out := c.states("success", "ready")
		c.Active = false
		c.Mic = false
		c.Phase = c.idlePhase()
		c.Status = text
		return out
	case "error", "cancelled":
		return c.Cancel("Speech cancelled or unavailable")
	}
	return nil
}
