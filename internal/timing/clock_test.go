package timing

import "testing"

const frameNs = int64(19_968_000)

func TestClockOneThousandFrames(t *testing.T) {
	var c Clock
	var n int
	for i := 0; i < 1000; i++ {
		n += c.Tick(frameNs, false)
	}
	if n != 1000 || c.acc != 0 {
		t.Fatalf("presented %d acc %d", n, c.acc)
	}
}

func TestClockCatchUpDropsExtraFrames(t *testing.T) {
	var c Clock
	got := c.Tick(5*frameNs, false)
	if got != 3 || c.acc != 0 {
		t.Fatalf("catch-up %d acc %d", got, c.acc)
	}
	if c.Tick(0, false) != 0 {
		t.Fatal("dropped frames were replayed")
	}
}

func TestClockPauseHoldsTheFraction(t *testing.T) {
	var c Clock
	if c.Tick(1<<62, true) != 0 || c.acc != 0 {
		t.Fatal("pause accumulated")
	}
	if c.Tick(frameNs/2, false) != 0 {
		t.Fatal("half frame")
	}
	saved := c.acc
	if c.Tick(1<<62, true) != 0 || c.acc != saved {
		t.Fatal("pause moved the accumulator")
	}
	if c.Tick(frameNs/2, false) != 1 || c.acc != 0 {
		t.Fatalf("two halves want 1 frame, acc %d", c.acc)
	}
}

func TestClockTurboClearsTheGap(t *testing.T) {
	var c Clock
	if c.Tick(frameNs/2, false) != 0 || c.acc == 0 {
		t.Fatal("expected a stored fraction")
	}
	c.SetTurbo(true)
	if c.Tick(5*frameNs, false) != 0 {
		t.Fatal("turbo presented frames")
	}
	c.SetTurbo(false)
	if c.acc != 0 {
		t.Fatal("leaving turbo kept the gap")
	}
	if c.Tick(frameNs/2, false) != 0 {
		t.Fatal("turbo gap was replayed")
	}
}
