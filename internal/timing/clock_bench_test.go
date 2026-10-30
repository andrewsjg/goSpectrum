package timing

import (
	"os"
	"testing"
	"time"

	"go.spectrum/internal/machine"
)

func BenchmarkMachine(b *testing.B) {
	rom := loadTestROM(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m := machine.New()
		if err := m.LoadROM(rom); err != nil {
			b.Fatal(err)
		}
		for n := 0; n < 40; n++ {
			m.RunFrame()
		}
	}
	b.StopTimer()
	elapsed := b.Elapsed().Seconds()
	if elapsed > 0 {
		b.ReportMetric(float64(b.N*40*machine.TStatesPerFrame)/elapsed, "T/s")
	}
}

// TestHeadroom fails when the interpreter cannot sustain twice real time.
// The best of three trials must reach 7,000,000 T-states per second.
func TestHeadroom(t *testing.T) {
	rom := loadTestROM(t)
	const frames = 40
	best := 0.0
	for trial := 0; trial < 3; trial++ {
		m := machine.New()
		if err := m.LoadROM(rom); err != nil {
			t.Fatal(err)
		}
		start := time.Now()
		for n := 0; n < frames; n++ {
			m.RunFrame()
		}
		elapsed := time.Since(start).Seconds()
		if elapsed <= 0 {
			continue
		}
		rate := float64(frames*machine.TStatesPerFrame) / elapsed
		if rate > best {
			best = rate
		}
	}
	if best < 7_000_000 {
		t.Fatalf("headroom %.0f T-states/s, want at least 7000000", best)
	}
	t.Logf("headroom %.0f T-states/s", best)
}

func loadTestROM(tb testing.TB) []byte {
	tb.Helper()
	b, err := os.ReadFile("../../testdata/test.rom")
	if err != nil {
		tb.Fatal(err)
	}
	return b
}
