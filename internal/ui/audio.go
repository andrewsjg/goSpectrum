package ui

import (
	"fmt"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2/audio"

	"go.spectrum/internal/machine"
)

// Audio feeds one 48 kHz stereo player. A disabled device or a failed open
// stays silent for the session and records a title notice. There is no retry.
type Audio struct {
	notice string
	on     bool
	player *audio.Player
	q      *pcmQueue
}

// OpenAudio starts the player when enabled is true.
// Tests must not call this: NewContext panics if a context already exists.
func OpenAudio(enabled bool) *Audio {
	a := &Audio{q: &pcmQueue{}}
	if !enabled {
		a.notice = "sound off"
		return a
	}
	ctx, err := newAudioContext()
	if err != nil {
		a.notice = "sound unavailable"
		return a
	}
	p, err := ctx.NewPlayer(a.q)
	if err != nil {
		a.notice = "sound unavailable"
		return a
	}
	p.SetBufferSize(50 * time.Millisecond)
	p.Play()
	a.player = p
	a.on = true
	return a
}

func newAudioContext() (ctx *audio.Context, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("%v", rec)
		}
	}()
	return audio.NewContext(machine.AudioHz), nil
}

// Notice is empty when sound is playing.
func (a *Audio) Notice() string {
	if a == nil {
		return ""
	}
	return a.notice
}

// Push queues mono samples as stereo frames. Turbo passes a nil slice.
func (a *Audio) Push(samples []int16) {
	if a == nil || !a.on || len(samples) == 0 {
		return
	}
	a.q.write(StereoLE(samples))
}

// Close releases the player.
func (a *Audio) Close() {
	if a == nil || a.player == nil {
		return
	}
	_ = a.player.Close()
	a.player = nil
	a.on = false
}

// StereoLE duplicates mono int16 samples into little-endian stereo.
func StereoLE(mono []int16) []byte {
	out := make([]byte, len(mono)*4)
	for i, s := range mono {
		lo := byte(s)
		hi := byte(s >> 8)
		o := i * 4
		out[o] = lo
		out[o+1] = hi
		out[o+2] = lo
		out[o+3] = hi
	}
	return out
}

// pcmQueue is the player source. An empty queue reads as silence, not EOF.
type pcmQueue struct {
	mu  sync.Mutex
	buf []byte
}

func (q *pcmQueue) write(p []byte) {
	if len(p) == 0 {
		return
	}
	q.mu.Lock()
	q.buf = append(q.buf, p...)
	const max = 48000 * 4
	if len(q.buf) > max {
		q.buf = append([]byte(nil), q.buf[len(q.buf)-max:]...)
	}
	q.mu.Unlock()
}

func (q *pcmQueue) Read(p []byte) (int, error) {
	q.mu.Lock()
	n := copy(p, q.buf)
	q.buf = q.buf[n:]
	q.mu.Unlock()
	for i := n; i < len(p); i++ {
		p[i] = 0
	}
	if len(p) == 0 {
		return 0, nil
	}
	return len(p), nil
}
