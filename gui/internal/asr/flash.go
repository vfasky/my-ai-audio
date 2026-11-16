package asr

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// The whole-recording model: the session keeps the audio in memory and,
// once it ends, sends it as one WAV in a multimodal generation request.

// maxWhole bounds the kept audio: ten seconds past WholeMaxDuration, at
// which the coordinator ends the recording.
const maxWhole = int((WholeMaxDuration+10*time.Second)/time.Second) * 32000

// wholeTimeout bounds the request, under the coordinator's 60-second
// processing timeout so that its reason ends the session.
const wholeTimeout = 45 * time.Second

// recognizeWhole is ready at once, keeps the audio until Finish, then
// recognizes all of it.
func (s *Session) recognizeWhole(key string) (string, error) {
	s.emit(Event{Type: "ready"})
	var pcm []byte
	for {
		select {
		case <-s.ctx.Done():
			return "", s.ctx.Err()
		case b := <-s.out:
			if len(b) == 0 {
				ctx, cancel := context.WithTimeout(s.ctx, wholeTimeout)
				defer cancel()
				return flash(ctx, s.opts.Config.Model, key, pcm)
			}
			if len(pcm)+len(b) > maxWhole {
				return "", errors.New("asr: recording too long")
			}
			pcm = append(pcm, b...)
		}
	}
}

func flash(ctx context.Context, model, key string, pcm []byte) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"model": model,
		"input": map[string]any{"messages": []any{map[string]any{
			"role":    "user",
			"content": []any{map[string]any{"audio": "data:audio/wav;base64," + base64.StdEncoding.EncodeToString(wav(pcm))}},
		}}},
		"parameters": map[string]any{
			"result_format": "message",
			"asr_options":   map[string]any{"enable_lid": true, "enable_itn": false},
		},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, flashEndpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirect refused") }}
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", &StatusError{Code: resp.StatusCode, Reason: errorCode(io.LimitReader(resp.Body, 4096))}
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxMessage))
	if err != nil {
		return "", err
	}
	return flashText(b)
}

// tags are the markers, such as <|zh|>, the model may write into its text.
var tags = regexp.MustCompile(`<\|[^|]*\|>`)

// flashText is the transcript of a reply; empty when nothing was heard.
func flashText(b []byte) (string, error) {
	var r struct {
		Output struct {
			Choices []struct {
				Message struct {
					Content []struct {
						Text string `json:"text"`
					} `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		} `json:"output"`
	}
	if json.Unmarshal(b, &r) != nil || len(r.Output.Choices) == 0 {
		return "", ErrProtocol
	}
	var text strings.Builder
	for _, c := range r.Output.Choices[0].Message.Content {
		text.WriteString(c.Text)
	}
	return strings.TrimSpace(tags.ReplaceAllString(text.String(), "")), nil
}

// wav wraps 16 kHz 16-bit mono PCM in a WAV header.
func wav(pcm []byte) []byte {
	h := make([]byte, 0, 44+len(pcm))
	h = append(h, "RIFF"...)
	h = binary.LittleEndian.AppendUint32(h, uint32(36+len(pcm)))
	h = append(h, "WAVEfmt "...)
	h = binary.LittleEndian.AppendUint32(h, 16)
	h = binary.LittleEndian.AppendUint16(h, 1)     // PCM
	h = binary.LittleEndian.AppendUint16(h, 1)     // mono
	h = binary.LittleEndian.AppendUint32(h, 16000) // sample rate
	h = binary.LittleEndian.AppendUint32(h, 32000) // byte rate
	h = binary.LittleEndian.AppendUint16(h, 2)     // block align
	h = binary.LittleEndian.AppendUint16(h, 16)    // bits per sample
	h = append(h, "data"...)
	h = binary.LittleEndian.AppendUint32(h, uint32(len(pcm)))
	return append(h, pcm...)
}
