package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.spectrum/internal/config"
	"go.spectrum/internal/formats"
)

func TestStatePath(t *testing.T) {
	cfg := t.TempDir()
	saves := config.SavesDir(cfg)
	tap := statePath(cfg, "Elite (1985)(Firebird Software)(48k).Tap", "/g/Elite (1985)(Firebird Software)(48k).Tap")
	sna := statePath(cfg, "Elite (1985)(Firebird Software)(48k).sna", "/g/Elite (1985)(Firebird Software)(48k).sna")
	if tap == sna || filepath.Dir(tap) != saves || !strings.HasSuffix(tap, "(48k) [tap].z80") {
		t.Fatalf("tap %s sna %s", tap, sna)
	}
	if got := statePath(cfg, "", ""); got != filepath.Join(saves, "BASIC.z80") {
		t.Fatal(got)
	}
	own := filepath.Join(saves, "Elite [tap].z80")
	if got := statePath(cfg, "Elite [tap].z80", own); got != own {
		t.Fatalf("a loaded state saves over itself, got %s", got)
	}
	if title, detail := prettyName(filepath.Join(savedLabel, "Manic Miner (1983)(Bug-Byte Software) [tap].z80")); title != "Manic Miner" || !strings.Contains(detail, "tap") {
		t.Fatalf("state shows as %q %q", title, detail)
	}
}

func TestGameMenu(t *testing.T) {
	g := newGameMenu(false, true)
	for _, it := range g.items {
		if it == itemChooser {
			t.Fatal("chooser offered without a config folder")
		}
	}
	if g.items[0] != itemResume || g.items[len(g.items)-1] != itemQuit || !g.wasPaused {
		t.Fatalf("items %v", g.items)
	}
	g.move(-1)
	if g.items[g.cursor] != itemQuit {
		t.Fatal("up from the top does not wrap")
	}
	g = newGameMenu(true, false)
	b := menuBox(len(g.items))
	last := menuRowBox(b, len(g.items)-1)
	if b.x < 0 || b.y < 0 || last.y+last.h > b.y+b.h-menuFoot || b.y+b.h > 296 {
		t.Fatalf("menu %+v does not fit the frame", b)
	}
}

func TestPickerRecentAndSaves(t *testing.T) {
	work := t.TempDir()
	cfg := t.TempDir()
	for _, name := range []string{"a.tap", "b.tap", "c.tap"} {
		if err := os.WriteFile(filepath.Join(work, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	saves := config.SavesDir(cfg)
	if err := os.MkdirAll(saves, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(saves, "a [tap].z80"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	gone := filepath.Join(work, "gone.tap")
	for _, r := range []config.Recent{
		{Model: "48", Path: filepath.Join(work, "b.tap")},
		{Model: "48", Path: gone},
		{Model: "128", Path: filepath.Join(work, "c.tap")},
	} {
		if err := config.AddRecent(cfg, r); err != nil {
			t.Fatal(err)
		}
	}
	p := mustPicker(t, work, cfg)
	if p.model != "128" || p.tape != filepath.Join(work, "c.tap") {
		t.Fatalf("last game not preselected: %s %s", p.model, p.tape)
	}
	var order []string
	for _, r := range p.rows {
		if r.kind == kindTape && r.tape != "" {
			order = append(order, r.label+"|"+r.note)
		}
	}
	want := []string{"c.tap|Recent", "b.tap|Recent", "a.tap|", filepath.Join(savedLabel, "a [tap].z80") + "|Saved state"}
	if strings.Join(order, ",") != strings.Join(want, ",") {
		t.Fatalf("order %v", order)
	}
	p.Quit()
	p.reopen()
	if _, done := p.Result(); done {
		t.Fatal("reopen left the picker finished")
	}
}

func TestBuildSessionFromDroppedData(t *testing.T) {
	exe := t.TempDir()
	writeROM(t, filepath.Join(exe, "rom"), 0xAA)
	var s formats.Snapshot
	s.Regs.PC = 0x8000
	s.Regs.SP = 0xFF00
	sess, err := buildSession(launch{Model: "48", File: "dropped.z80", Data: formats.WriteZ80(s)}, exe, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if sess.Name != "dropped.z80" || sess.Path != "" || sess.Mach.CPU().PC != 0x8000 {
		t.Fatalf("name %q path %q pc %04X", sess.Name, sess.Path, sess.Mach.CPU().PC)
	}
	if _, err := buildSession(launch{Model: "48", File: "notes.txt", Data: []byte("x")}, exe, t.TempDir()); err == nil || !strings.Contains(err.Error(), "unknown extension") {
		t.Fatalf("bad drop %v", err)
	}
}
