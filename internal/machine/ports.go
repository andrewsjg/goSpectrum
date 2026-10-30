package machine

// Beeper is the Bresenham resampler: add 48,000 per T-state and emit one
// int16 each time the accumulator reaches 3,500,000.
type Beeper struct {
	Speaker bool
	Acc     int
	Hz      int // 0 selects CPUHz
}

func (b *Beeper) Tick(n int) []int16 {
	if n <= 0 {
		return nil
	}
	out := make([]int16, 0, n*AudioHz/CPUHz+2)
	for i := 0; i < n; i++ {
		if s, ok := b.tickOne(); ok {
			out = append(out, s)
		}
	}
	return out
}

func (b *Beeper) hz() int {
	if b.Hz == 0 {
		return CPUHz
	}
	return b.Hz
}

func (b *Beeper) tickOne() (int16, bool) {
	b.Acc += AudioHz
	if b.Acc < b.hz() {
		return 0, false
	}
	b.Acc -= b.hz()
	if b.Speaker {
		return AudioAmp, true
	}
	return -AudioAmp, true
}

// WriteULA stores border, MIC, and speaker when port bit 0 is clear.
// It reports whether the port was the ULA. Other ports do not change the latch.
func WriteULA(border *byte, mic, speaker *bool, port uint16, val byte) bool {
	if port&1 != 0 {
		return false
	}
	*border = val & 7
	*mic = val&0x08 != 0
	*speaker = val&0x10 != 0
	return true
}
