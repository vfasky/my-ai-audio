package protocol

import (
	"errors"
	"math"
	"time"
)

// ErrLimit is a session that the stream ends because its audio broke off
// or ran past a bound. Each reason is an error of its own, which
// errors.Is(err, ErrLimit) matches, so that the user learns which.
var ErrLimit = errors.New("audio recovery or session limit exceeded")

var (
	ErrSilence  = limit("连续 3 秒没有收到音频，本次语音已结束：请检查蓝牙连接，或按键松开时设备是否发送了结束信号")
	ErrSequence = limit("音频帧序号超出单次语音上限")
	ErrBacklog  = limit("乱序等待的音频帧过多，蓝牙传输可能不稳定")
	ErrMissing  = limit("丢失的音频帧过多（超过 100 帧），蓝牙传输可能不稳定")
)

// limitError is one reason of ErrLimit.
type limitError struct{ msg string }

func limit(msg string) error               { return &limitError{msg} }
func (e *limitError) Error() string        { return e.msg }
func (e *limitError) Is(target error) bool { return target == ErrLimit }

type PCM struct {
	Sequence uint32
	Samples  [Samples]int16
	Silence  bool
}
type Meter struct {
	RMS, Peak                                float64
	Bins                                     [32]float64
	Received, Missing, Recovered, Duplicates uint32
	Frames                                   uint32
}

func (m *Meter) Add(p PCM) {
	var sum float64
	m.Peak = 0
	for _, s := range p.Samples {
		x := float64(s) / 32768
		sum += x * x
		m.Peak = max(m.Peak, math.Abs(x))
	}
	m.RMS = math.Sqrt(sum / Samples)
	copy(m.Bins[:], m.Bins[1:])
	m.Bins[31] = m.Peak
	m.Frames++
	if p.Silence {
		m.Missing++
		m.Recovered++
	}
}
func (m Meter) Duration() time.Duration { return time.Duration(m.Frames) * 20 * time.Millisecond }

// Stream is advanced by one owner using an injected monotonic clock.
type Stream struct {
	ID    uint32
	Next  uint32
	Meter Meter
	// Limit is the frames the session may record, MaxFrames when zero.
	// Full: the stream reached it and ended the recording itself; later
	// frames and the device's own stop are ignored.
	Limit                    uint32
	Full                     bool
	started, last, gap, tail time.Time
	final                    uint32
	stopped                  bool
	Done                     bool
	pending                  map[uint32][Samples]int16
}

func NewStream(id uint32, now time.Time) *Stream {
	return &Stream{ID: id, started: now, last: now, pending: map[uint32][Samples]int16{}}
}
func (s *Stream) Push(f Frame, now time.Time) ([]PCM, error) {
	if s.Done || f.SessionID != s.ID {
		return nil, nil
	}
	if s.Full && (s.final == EmptyFinal || f.Sequence > s.final) {
		return nil, nil
	}
	if f.Sequence >= MaxFrames {
		return nil, ErrSequence
	}
	if !s.stopped && f.Sequence >= s.limit() {
		return s.fill(now)
	}
	if s.stopped && (s.final == EmptyFinal || f.Sequence > s.final) {
		return nil, ErrInvalid
	}
	if f.Sequence < s.Next {
		s.Meter.Duplicates++
		return nil, nil
	}
	if _, ok := s.pending[f.Sequence]; ok {
		s.Meter.Duplicates++
		return nil, nil
	}
	p, e := Decode(f.Payload)
	if e != nil {
		return nil, e
	}
	if len(s.pending) >= 75 {
		return nil, ErrBacklog
	}
	s.pending[f.Sequence] = p
	s.last = now
	s.Meter.Received++
	return s.Tick(now)
}

// limit is the frames the session may record.
func (s *Stream) limit() uint32 {
	if s.Limit == 0 || s.Limit > MaxFrames {
		return MaxFrames
	}
	return s.Limit
}

// LimitDuration is how long the session may record.
func (s *Stream) LimitDuration() time.Duration {
	return time.Duration(s.limit()) * 20 * time.Millisecond
}

// fill ends the recording at the limit, keeping the audio received so far,
// as if the device had stopped there.
func (s *Stream) fill(now time.Time) ([]PCM, error) {
	final := EmptyFinal
	if s.Next > 0 {
		final = s.Next - 1
	}
	for seq := range s.pending {
		if final == EmptyFinal || seq > final {
			final = seq
		}
	}
	if final != EmptyFinal && final >= s.limit() {
		final = s.limit() - 1
		for seq := range s.pending {
			if seq > final {
				delete(s.pending, seq)
			}
		}
	}
	s.Full = true
	s.stopped = true
	s.final = final
	s.tail = now
	return s.Tick(now)
}
func (s *Stream) Stop(final uint32, now time.Time) ([]PCM, error) {
	if s.Full {
		return nil, nil
	}
	if s.stopped {
		if final != s.final {
			return nil, ErrInvalid
		}
		return nil, nil
	}
	if final == EmptyFinal {
		if s.Next > 0 || len(s.pending) > 0 {
			return nil, ErrInvalid
		}
	} else {
		if final >= MaxFrames || final+1 < s.Next {
			return nil, ErrInvalid
		}
		for seq := range s.pending {
			if seq > final {
				return nil, ErrInvalid
			}
		}
	}
	s.stopped = true
	s.final = final
	s.tail = now
	return s.Tick(now)
}
func (s *Stream) Tick(now time.Time) ([]PCM, error) {
	if s.Done {
		return nil, nil
	}
	// Frames arrive in real time, so the sequence reaches the limit first;
	// the clock is a backstop for audio that stopped counting up.
	if !s.stopped && now.Sub(s.started) >= s.LimitDuration()+5*time.Second {
		return s.fill(now)
	}
	if !s.stopped && now.Sub(s.last) >= 3*time.Second {
		return nil, ErrSilence
	}
	var out []PCM
	emit := func(p PCM) { out = append(out, p); s.Meter.Add(p); s.Next++; s.gap = time.Time{} }
	for {
		if p, ok := s.pending[s.Next]; ok {
			delete(s.pending, s.Next)
			emit(PCM{Sequence: s.Next, Samples: p})
			continue
		}
		break
	}
	if s.stopped && (s.final == EmptyFinal || s.Next > s.final) {
		s.Done = true
		return out, nil
	}
	target := s.Next
	hasGap := false
	for n := range s.pending {
		if !hasGap || n > target {
			target = n
		}
		hasGap = true
	}
	force := s.stopped && now.Sub(s.tail) >= 500*time.Millisecond
	if force {
		target = s.final
		hasGap = true
	}
	if hasGap {
		missing := uint32(0)
		for i := s.Next; i <= target; i++ {
			if _, ok := s.pending[i]; !ok {
				missing++
			}
		}
		if missing+s.Meter.Missing > 100 {
			return nil, ErrMissing
		}
		if s.gap.IsZero() {
			s.gap = now
		}
		if force || !s.stopped && now.Sub(s.gap) >= 80*time.Millisecond {
			for s.Next <= target {
				p, ok := s.pending[s.Next]
				delete(s.pending, s.Next)
				emit(PCM{Sequence: s.Next, Samples: p, Silence: !ok})
			}
		}
	}
	if s.stopped && s.Next > s.final {
		s.Done = true
	}
	return out, nil
}
