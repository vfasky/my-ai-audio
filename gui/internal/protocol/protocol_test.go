package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"testing"
	"time"
)

func encoded() []byte { b := make([]byte, 166); binary.LittleEndian.PutUint16(b[3:], 320); return b }
func fragment(session, seq uint32, index, count byte, p []byte) []byte {
	b := make([]byte, 16+len(p))
	binary.LittleEndian.PutUint16(b, 0x5041)
	b[2] = 1
	binary.LittleEndian.PutUint32(b[4:], session)
	binary.LittleEndian.PutUint32(b[8:], seq)
	b[12] = index
	b[13] = count
	binary.LittleEndian.PutUint16(b[14:], uint16(len(p)))
	copy(b[16:], p)
	return b
}
func TestControlStrict(t *testing.T) {
	valid := []string{`{"v":1,"type":"ptt_down","sessionId":0,"seq":0}`, `{"v":1,"type":"ptt_up","sessionId":4294967295,"seq":1,"finalSequence":4294967295}`, `{"v":1,"type":"hello","device":"ai-passport","fw":"1","codec":"ima-adpcm","sampleRate":16000,"channels":1,"frameMs":20}`, `{"v":1,"type":"button_event","button":"ok","event":"long","durationMs":650,"seq":3}`}
	for _, s := range valid {
		if _, e := ParseControl([]byte(s)); e != nil {
			t.Fatal(s, e)
		}
	}
	invalid := []string{`null`, `[]`, `{"v":true,"type":"host_ready"}`, `{"v":1,"v":1,"type":"host_ready"}`, `{"v":1,"type":"host_ready"} {}`, `{"v":1,"type":"ptt_down","sessionId":-1,"seq":0}`, `{"v":1,"type":"ptt_down","sessionId":1.0,"seq":0}`, `{"v":1,"type":"ptt_down","sessionId":1}`, `{"v":1,"type":"host_ready","extra":1}`, `{"v":1,"type":"host_state","state":null}`}
	for _, s := range invalid {
		if _, e := ParseControl([]byte(s)); e == nil {
			t.Fatal("accepted", s)
		}
	}
	if _, e := ParseControl(bytes.Repeat([]byte(" "), 183)); e == nil {
		t.Fatal("oversize")
	}
	if _, e := ParseControl([]byte{0xff}); e == nil {
		t.Fatal("utf8")
	}
}
func TestADPCMGolden(t *testing.T) {
	b := encoded()
	binary.LittleEndian.PutUint16(b, 1000)
	b[6] = 0x71
	b[7] = 0x8f
	out, e := Decode(b)
	if e != nil {
		t.Fatal(e)
	}
	// Independently calculated IMA first samples: +1,+11,-30,-4; index 0,8,16,15.
	want := []int16{1000, 1001, 1012, 982, 978}
	for i, w := range want {
		if out[i] != w {
			t.Fatalf("sample %d got %d want %d", i, out[i], w)
		}
	}
	for _, mutate := range []func([]byte){func(b []byte) { b[2] = 89 }, func(b []byte) { b[3] = 0 }, func(b []byte) { b[5] = 1 }, func(b []byte) { b[165] = 0xf0 }} {
		bad := bytes.Clone(b)
		mutate(bad)
		if _, e := Decode(bad); e == nil {
			t.Fatal("invalid ADPCM accepted")
		}
	}
	p, e := Decode(encoded())
	if e != nil {
		t.Fatal(e)
	}
	for _, v := range p {
		if v != 0 {
			t.Fatal("silence golden")
		}
	}
	b = encoded()
	b[2] = 88
	binary.LittleEndian.PutUint16(b, 32767)
	b[6] = 0x77
	p, _ = Decode(b)
	if p[1] != 32767 || p[2] != 32767 {
		t.Fatal("clamp")
	}
}
func TestAssemblyReorderDuplicateAndEviction(t *testing.T) {
	b := encoded()
	a := Assembler{}
	parts := [][]byte{b[:50], b[50:100], b[100:]}
	for _, idx := range []byte{2, 2, 0} {
		f, e := a.Push(fragment(9, 0, idx, 3, parts[idx]))
		if e != nil || f != nil {
			t.Fatal(f, e)
		}
	}
	f, e := a.Push(fragment(9, 0, 1, 3, parts[1]))
	if e != nil || f == nil || !bytes.Equal(f.Payload, b) {
		t.Fatal(f, e)
	}
	if f, e = a.Push(fragment(9, 0, 1, 3, parts[1])); e != nil || f != nil {
		t.Fatal("duplicate complete")
	}
	for i := uint32(1); i <= 65; i++ {
		_, e = a.Push(fragment(9, i, 0, 2, b[:80]))
		if e != nil {
			t.Fatal(e)
		}
	}
	if a.Evicted != 1 || len(a.pending) != 64 {
		t.Fatal(a.Evicted, len(a.pending))
	}
	if _, e = a.Push(fragment(9, 100, 0, 65, b)); e == nil {
		t.Fatal("65 fragments")
	}
	// Exactly 64 pieces can represent the full frame.
	a = Assembler{}
	for i := 63; i >= 0; i-- {
		lo := i * 166 / 64
		hi := (i + 1) * 166 / 64
		f, e = a.Push(fragment(1, 0, byte(i), 64, b[lo:hi]))
		if e != nil {
			t.Fatal(e)
		}
	}
	if f == nil {
		t.Fatal("64-fragment completion")
	}
}
func TestStreamReorderGapTail(t *testing.T) {
	now := time.Unix(100, 0)
	s := NewStream(1, now)
	b := encoded()
	if p, e := s.Push(Frame{1, 1, b}, now); e != nil || len(p) != 0 {
		t.Fatal(p, e)
	}
	p, e := s.Push(Frame{1, 0, b}, now.Add(79*time.Millisecond))
	if e != nil || len(p) != 2 || s.Meter.Missing != 0 {
		t.Fatal(p, e)
	}
	_, _ = s.Push(Frame{1, 3, b}, now.Add(100*time.Millisecond))
	p, e = s.Tick(now.Add(179 * time.Millisecond))
	if e != nil || len(p) != 0 {
		t.Fatal("early silence")
	}
	p, e = s.Tick(now.Add(180 * time.Millisecond))
	if e != nil || len(p) != 2 || !p[0].Silence {
		t.Fatal(p, e)
	}
	p, e = s.Stop(5, now.Add(200*time.Millisecond))
	if e != nil || len(p) != 0 {
		t.Fatal(p, e)
	}
	p, e = s.Tick(now.Add(699 * time.Millisecond))
	if e != nil || len(p) != 0 {
		t.Fatal("tail too early")
	}
	p, e = s.Tick(now.Add(700 * time.Millisecond))
	if e != nil || len(p) != 2 || !s.Done || s.Meter.Recovered != 3 {
		t.Fatal(p, e, s.Meter)
	}
}
func TestStreamTailArrivalAndLimits(t *testing.T) {
	now := time.Unix(100, 0)
	b := encoded()
	s := NewStream(1, now)
	_, _ = s.Push(Frame{1, 0, b}, now)
	_, _ = s.Stop(1, now)
	p, e := s.Push(Frame{1, 1, b}, now.Add(499*time.Millisecond))
	if e != nil || len(p) != 1 || !s.Done {
		t.Fatal(p, e)
	}
	s = NewStream(1, now)
	if _, e = s.Push(Frame{1, 101, b}, now); e == nil {
		t.Fatal("over 100 missing")
	}
	s = NewStream(1, now)
	_, _ = s.Push(Frame{1, 100, b}, now)
	p, e = s.Tick(now.Add(80 * time.Millisecond))
	if e != nil || len(p) != 101 || s.Meter.Missing != 100 {
		t.Fatal(e, len(p))
	}
	s = NewStream(1, now)
	if _, e = s.Tick(now.Add(3 * time.Second)); e == nil {
		t.Fatal("stall")
	}
	s = NewStream(1, now)
	if _, e = s.Stop(EmptyFinal, now); e != nil || !s.Done {
		t.Fatal("empty")
	}
}
func TestMeter(t *testing.T) {
	var m Meter
	var p PCM
	for i := range p.Samples {
		p.Samples[i] = -16384
	}
	m.Add(p)
	if math.Abs(m.RMS-.5) > 1e-9 || m.Peak != .5 || m.Bins[31] != .5 || m.Duration() != 20*time.Millisecond {
		t.Fatal(m)
	}
}

func TestStreamLimitsSayWhy(t *testing.T) {
	start := time.Unix(1000, 0)
	s := NewStream(1, start)
	if _, e := s.Tick(start.Add(3 * time.Second)); !errors.Is(e, ErrSilence) || !errors.Is(e, ErrLimit) {
		t.Fatalf("silence: %v", e)
	}
	if _, e := NewStream(1, start).Push(Frame{SessionID: 1, Sequence: MaxFrames}, start); !errors.Is(e, ErrSequence) || !errors.Is(e, ErrLimit) {
		t.Fatalf("sequence: %v", e)
	}
	for _, e := range []error{ErrSilence, ErrSequence, ErrBacklog, ErrMissing} {
		if e.Error() == ErrLimit.Error() {
			t.Fatalf("%v says no reason", e)
		}
	}
}

func TestMaxDurationIsThirtyMinutes(t *testing.T) {
	if MaxDuration != 30*time.Minute || MaxFrames != 90000 {
		t.Fatal(MaxDuration, MaxFrames)
	}
	// A device stop past the bound is the stream's to judge, not a parse error.
	if c, e := ParseControl([]byte(`{"v":1,"type":"ptt_up","sessionId":1,"seq":1,"finalSequence":90500}`)); e != nil || c.FinalSequence != 90500 {
		t.Fatal(c, e)
	}
}

// At its limit the stream ends the recording itself and keeps the audio;
// later frames and the device's own stop change nothing.
func TestStreamEndsAtLimit(t *testing.T) {
	now := time.Unix(100, 0)
	b := encoded()
	s := NewStream(1, now)
	s.Limit = 50
	var got int
	for i := uint32(0); i < 50; i++ {
		p, e := s.Push(Frame{1, i, b}, now.Add(time.Duration(i)*20*time.Millisecond))
		if e != nil {
			t.Fatal(i, e)
		}
		got += len(p)
	}
	if s.Full || s.Done {
		t.Fatal("ended before the limit")
	}
	p, e := s.Push(Frame{1, 50, b}, now.Add(time.Second))
	got += len(p)
	if e != nil || !s.Full || !s.Done || got != 50 || s.Meter.Duration() != time.Second {
		t.Fatal(e, s.Full, s.Done, got)
	}
	if p, e = s.Push(Frame{1, 51, b}, now.Add(time.Second)); e != nil || len(p) != 0 {
		t.Fatal("late frame", e)
	}
	if p, e = s.Stop(90010, now.Add(time.Second)); e != nil || len(p) != 0 {
		t.Fatal("device stop", e)
	}
	if s.LimitDuration() != time.Second {
		t.Fatal(s.LimitDuration())
	}
}

// Frames that stop counting up still end at the clock's backstop, with
// what arrived.
func TestStreamClockBackstop(t *testing.T) {
	now := time.Unix(100, 0)
	b := encoded()
	s := NewStream(1, now)
	s.Limit = 50
	_, _ = s.Push(Frame{1, 0, b}, now)
	s.last = now.Add(6 * time.Second)
	p, e := s.Tick(now.Add(6 * time.Second))
	if e != nil || !s.Full || !s.Done || len(p) != 0 || s.Meter.Received != 1 {
		t.Fatal(e, s.Full, s.Done, len(p))
	}
}
