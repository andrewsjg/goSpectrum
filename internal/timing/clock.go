// Package timing paces the 48K frame against the wall clock.
package timing

// One frame is 69,888 T-states at 3,500,000 Hz, which is exactly 19,968,000 ns.
const (
	cpuHz  = 3500000
	frameT = 69888
	denom  = int64(frameT) * 1_000_000_000
)

// Clock turns elapsed nanoseconds into a frame count.
// The fraction is kept. While turbo is false, a tick runs at most 3 frames
// and drops any further whole frames. Pause adds nothing. Leaving turbo
// clears the accumulator so the skipped wall time is not replayed.
type Clock struct {
	acc   int64
	turbo bool
}

// SetTurbo records the tape fast-path. The transition from on to off clears
// the accumulator.
func (c *Clock) SetTurbo(on bool) {
	if c.turbo && !on {
		c.acc = 0
	}
	c.turbo = on
}

// Turbo reports the fast-path flag.
func (c *Clock) Turbo() bool { return c.turbo }

// Tick returns how many frames this slice of wall time should present.
// Turbo returns 0: the caller runs as many frames as the host can spare,
// and this clock does not store that gap.
func (c *Clock) Tick(wallNs int64, paused bool) int {
	if paused || c.turbo || wallNs <= 0 {
		return 0
	}
	const maxInt64 = int64(^uint64(0) >> 1)
	room := maxInt64 - c.acc
	if wallNs > room/cpuHz {
		c.acc = maxInt64
	} else {
		c.acc += wallNs * cpuHz
	}
	frames := c.acc / denom
	c.acc %= denom
	if frames > 3 {
		frames = 3
	}
	return int(frames)
}
