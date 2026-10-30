package machine

import (
	"os"
	"strings"
	"testing"
)

func TestSpectrum128Menu(t *testing.T) {
	m, font := load128ROM(t)
	want := []string{"Tape Loader", "128 BASIC", "Calculator", "48 BASIC", "Tape Tester", "1986"}
	menuAt := -1
	var text string
	for i := 0; i < 150; i++ {
		m.RunFrame()
		text = ocrScreen(m, font)
		ok := true
		for _, s := range want {
			if !strings.Contains(text, s) {
				ok = false
				break
			}
		}
		if ok {
			menuAt = i
			break
		}
	}
	if menuAt < 0 {
		t.Fatalf("menu incomplete by frame 150\nPC %04X\n%s", m.cpu.PC, text)
	}
	frames := m.MemRead(0x5C78)
	m.RunFrame()
	if m.MemRead(0x5C78) == frames {
		t.Fatal("the 50 Hz interrupt did not advance FRAMES")
	}
	if menuAt >= 250 {
		t.Fatalf("menu at frame %d, after the autoload Enter", menuAt)
	}
}

func TestSpectrum128AutoloadEnter(t *testing.T) {
	m, font := load128ROM(t)
	m.ArmTape([]Pulse{{Level: false, Dur: 50_000_000}}, false, true)
	var text string
	for i := 0; i < 500; i++ {
		m.RunFrame()
		if m.romSel == 1 && m.cpu.PC >= 0x0556 && m.cpu.PC < 0x0606 && !m.tapeHold {
			text = ocrScreen(m, font)
			if strings.Contains(text, "To cancel - press BREAK twice") {
				return
			}
		}
	}
	t.Fatalf("Enter did not start the tape loader\nPC %04X rom %d hold %v\n%s",
		m.cpu.PC, m.romSel, m.tapeHold, text)
}

func load128ROM(t *testing.T) (*Machine, []byte) {
	t.Helper()
	rom, err := os.ReadFile("../../rom/spec128uk.rom")
	if err != nil {
		t.Skip(err)
	}
	if len(rom) != 32768 {
		t.Fatalf("rom %d", len(rom))
	}
	m := New()
	if err := m.LoadROM(rom); err != nil {
		t.Fatal(err)
	}
	return m, rom[16384+0x3D00 : 16384+0x3D00+96*8]
}

func ocrScreen(m *Machine, font []byte) string {
	var b strings.Builder
	for row := 0; row < 24; row++ {
		for col := 0; col < 32; col++ {
			var glyph [8]byte
			for py := 0; py < 8; py++ {
				var bits byte
				for px := 0; px < 8; px++ {
					addr, bit := PaperAddr(col*8+px, row*8+py)
					if m.screenByte(int(addr-0x4000))&(1<<uint(bit)) != 0 {
						bits |= 1 << uint(7-px)
					}
				}
				glyph[py] = bits
			}
			ch := byte(' ')
			if glyph != [8]byte{} {
				ch = '?'
				for i := 0; i < 96; i++ {
					match := true
					for py := 0; py < 8; py++ {
						if font[i*8+py] != glyph[py] {
							match = false
							break
						}
					}
					if match {
						ch = byte(32 + i)
						break
					}
				}
			}
			b.WriteByte(ch)
		}
		b.WriteByte('\n')
	}
	return b.String()
}
