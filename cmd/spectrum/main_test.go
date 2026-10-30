package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.spectrum/internal/config"
	"go.spectrum/internal/formats"
)

func romBytes(mark byte) []byte {
	b := make([]byte, 16384)
	b[0] = mark
	b[1] = 0x18
	b[2] = 0xFE
	return b
}

func writeROM(t *testing.T, dir string, mark byte) string {
	t.Helper()
	path := filepath.Join(dir, "48.rom")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, romBytes(mark), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func exec(t *testing.T, args []string, exe, work string) (int, string, string, *Session, bool) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	var sess *Session
	called := false
	code := Execute(args, Option{
		Stdout:    &stdout,
		Stderr:    &stderr,
		ExeDir:    exe,
		WorkDir:   work,
		ConfigDir: filepath.Join(work, "configs"),
		Start: func(s *Session) int {
			called = true
			sess = s
			return 0
		},
	})
	return code, stdout.String(), stderr.String(), sess, called
}

func TestHelpAndVersionDoNotStart(t *testing.T) {
	dir := t.TempDir()
	code, out, errText, _, called := exec(t, []string{"--help"}, dir, dir)
	if code != 0 || called || !strings.Contains(out, "spectrum [options] [file]") || errText != "" {
		t.Fatalf("help code %d called %v out %q err %q", code, called, out, errText)
	}
	code, out, errText, _, called = exec(t, []string{"--version"}, dir, dir)
	if code != 0 || called || out != "goSpectrum 0.1.0\n" {
		t.Fatalf("version code %d called %v out %q err %q", code, called, out, errText)
	}
}

func TestExitOneBeforeWindow(t *testing.T) {
	dir := t.TempDir()
	cases := [][]string{
		{"--scale", "0"},
		{"missing.tap", "also.tap"},
		{"--not-a-flag"},
		{"game.dsk"},
		{"missing.tap"},
	}
	for _, args := range cases {
		code, _, errText, _, called := exec(t, args, dir, dir)
		if code != 1 || called {
			t.Fatalf("%v code %d called %v err %q", args, code, called, errText)
		}
		if errText == "" {
			t.Fatalf("%v produced no stderr", args)
		}
	}
	if _, _, errText, _, called := exec(t, []string{"--rom", filepath.Join(dir, "nope.rom")}, dir, dir); called {
		t.Fatal("missing --rom started")
	} else if !strings.Contains(errText, "nope.rom") {
		t.Fatalf("stderr %q", errText)
	}

	short := filepath.Join(dir, "short.rom")
	if err := os.WriteFile(short, []byte{0}, 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, errText, _, called := exec(t, []string{"--rom", short}, dir, dir)
	if code != 1 || called || !strings.Contains(errText, "16384") {
		t.Fatalf("short rom code %d called %v %q", code, called, errText)
	}
}

func TestROMSearchOrder(t *testing.T) {
	root := t.TempDir()
	exe := filepath.Join(root, "exe")
	work := filepath.Join(root, "work")
	writeROM(t, filepath.Join(exe, "rom"), 0x11)
	writeROM(t, filepath.Join(work, "rom"), 0x22)
	_, _, _, sess, called := exec(t, []string{"--model", "48"}, exe, work)
	if !called || sess.Mach.Mem().ROM[0] != 0x11 {
		t.Fatalf("exe rom called %v mark %02X", called, sess.Mach.Mem().ROM[0])
	}
	if err := os.Remove(filepath.Join(exe, "rom", "48.rom")); err != nil {
		t.Fatal(err)
	}
	_, _, _, sess, _ = exec(t, []string{"--model", "48"}, exe, work)
	if sess.Mach.Mem().ROM[0] != 0x22 {
		t.Fatalf("work rom mark %02X", sess.Mach.Mem().ROM[0])
	}

	if err := os.WriteFile(filepath.Join(exe, "rom", "48.rom"), []byte{1, 2, 3}, 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, errText, _, called := exec(t, []string{"--model", "48"}, exe, work)
	if code != 1 || called || !strings.Contains(errText, "16384") {
		t.Fatalf("bad exe rom fell through: code %d called %v %q", code, called, errText)
	}

	empty := t.TempDir()
	code, _, errText, _, called = exec(t, []string{"--model", "48"}, empty, empty)
	if code != 1 || called || !strings.Contains(errText, filepath.Join(empty, "rom", "48.rom")) {
		t.Fatalf("missing both code %d called %v %q", code, called, errText)
	}
}

func TestFlagsAndGoodFiles(t *testing.T) {
	dir := t.TempDir()
	rom := writeROM(t, filepath.Join(dir, "rom"), 0x33)
	code, _, _, sess, called := exec(t, []string{
		"--rom", rom, "--scale", "4", "--sound=false", "--realtime-tape", "--no-autoload",
	}, dir, dir)
	if code != 0 || !called || sess.Scale != 4 || sess.Sound || !sess.RealtimeTape || !sess.NoAutoload || sess.Notice != "sound off" {
		t.Fatalf("code %d called %v %+v", code, called, sess)
	}
	if sess.Name != "" || sess.Mach.Mem().ROM[0] != 0x33 {
		t.Fatalf("name %q mark %02X", sess.Name, sess.Mach.Mem().ROM[0])
	}

	payload := []byte{0xFF, 0x80, 0x7F}
	tap := filepath.Join(dir, "GAME.TAP")
	if err := os.WriteFile(tap, formats.WriteTAP([][]byte{payload}), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, errText, sess, called := exec(t, []string{"--rom", rom, tap}, dir, dir)
	if !called || errText != "" || sess.Mach.TapePlaying() || sess.Name != "GAME.TAP" || sess.NoAutoload {
		t.Fatalf("tap called %v err %q playing %v name %q", called, errText, sess.Mach.TapePlaying(), sess.Name)
	}
	for i := 0; i < 199; i++ {
		sess.Mach.RunFrame()
	}
	if !sess.Mach.TapePlaying() {
		t.Fatal("autoload Enter should start the tape")
	}

	var snap formats.Snapshot
	snap.Regs.SP = 0x8000
	snap.Regs.PC = 0x1234
	snap.Regs.A = 0x77
	snap.RAM[0] = 0x5A
	raw, err := formats.WriteSNA(snap)
	if err != nil {
		t.Fatal(err)
	}
	sna := filepath.Join(dir, "state.sna")
	if err := os.WriteFile(sna, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, errText, sess, _ = exec(t, []string{"--rom", rom, sna}, dir, dir)
	if errText != "" || sess.Mach.Mem().RAM[0] != 0x5A || sess.Mach.CPU().PC != 0x1234 || sess.Mach.CPU().A != 0x77 {
		t.Fatalf("sna err %q ram %02X pc %04X", errText, sess.Mach.Mem().RAM[0], sess.Mach.CPU().PC)
	}
}

func TestBadFilesStillStart(t *testing.T) {
	dir := t.TempDir()
	rom := writeROM(t, filepath.Join(dir, "rom"), 0x01)
	tap := filepath.Join(dir, "bad.tap")
	if err := os.WriteFile(tap, []byte{0x01, 0x00, 0x00}, 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, errText, sess, called := exec(t, []string{"--rom", rom, tap}, dir, dir)
	if code != 0 || !called || sess.Mach.Mem().RAM[0] != 0 || sess.Mach.Status() != "error" || !strings.Contains(errText, "spectrum:") {
		t.Fatalf("tap code %d called %v status %q err %q", code, called, sess.Mach.Status(), errText)
	}

	sna := filepath.Join(dir, "big.sna")
	if err := os.WriteFile(sna, make([]byte, 49180), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, errText, sess, called = exec(t, []string{"--rom", rom, sna}, dir, dir)
	if code != 0 || !called || sess.Mach.CPU().PC != 0 || sess.Mach.Mem().RAM[0] != 0 {
		t.Fatalf("sna applied code %d pc %04X", code, sess.Mach.CPU().PC)
	}
	if !strings.Contains(errText, "beyond the emulated 48K") || sess.ErrorText == "" {
		t.Fatalf("err %q session %q", errText, sess.ErrorText)
	}
}

func TestBareStartDoesNotOpenWindow(t *testing.T) {
	dir := t.TempDir()
	for _, args := range [][]string{nil, {"--scale", "3"}, {"--sound=false"}} {
		code, _, errText, _, called := exec(t, args, dir, dir)
		if code != 0 || called || errText != "" {
			t.Fatalf("%v code %d called %v err %q", args, code, called, errText)
		}
	}
}

func TestConfigLaunch(t *testing.T) {
	dir := t.TempDir()
	rom := make([]byte, 32768)
	rom[0] = 0x10
	if err := os.MkdirAll(filepath.Join(dir, "rom"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rom", "spec128uk.rom"), rom, 0o644); err != nil {
		t.Fatal(err)
	}
	payload := []byte{0xFF, 0x80, 0x7F}
	tap := filepath.Join(dir, "GAME.TAP")
	if err := os.WriteFile(tap, formats.WriteTAP([][]byte{payload}), 0o644); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(dir, "OTHER.TAP")
	if err := os.WriteFile(other, formats.WriteTAP([][]byte{payload}), 0o644); err != nil {
		t.Fatal(err)
	}
	off := false
	cfgDir := filepath.Join(dir, "configs")
	if err := config.Save(cfgDir, "play", config.File{
		Model:        "128",
		Tape:         "GAME.TAP",
		Scale:        3,
		Sound:        &off,
		RealtimeTape: true,
		NoAutoload:   true,
	}); err != nil {
		t.Fatal(err)
	}

	code, _, errText, sess, called := exec(t, []string{"--config", "play"}, dir, dir)
	if code != 0 || !called || errText != "" {
		t.Fatalf("config code %d called %v err %q", code, called, errText)
	}
	if !sess.Mach.Is128() || sess.Name != "GAME.TAP" || sess.Scale != 3 || sess.Sound || sess.Notice != "sound off" {
		t.Fatalf("128 %v name %q scale %d sound %v notice %q", sess.Mach.Is128(), sess.Name, sess.Scale, sess.Sound, sess.Notice)
	}
	if !sess.RealtimeTape || !sess.NoAutoload {
		t.Fatalf("realtime %v autoload %v", sess.RealtimeTape, sess.NoAutoload)
	}

	_, _, _, sess, _ = exec(t, []string{"--config", "play", "--scale", "4"}, dir, dir)
	if sess.Scale != 4 || sess.Sound {
		t.Fatalf("override scale %d sound %v", sess.Scale, sess.Sound)
	}
	_, _, _, sess, _ = exec(t, []string{"--config", "play", other}, dir, dir)
	if sess.Name != "OTHER.TAP" {
		t.Fatalf("positional name %q", sess.Name)
	}

	code, _, errText, _, called = exec(t, []string{"--config", "missing"}, dir, dir)
	if code != 1 || called || !strings.Contains(errText, "not found") {
		t.Fatalf("missing code %d called %v %q", code, called, errText)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "bad.json"), []byte(`{"model":"plus3"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, errText, _, called = exec(t, []string{"--config", "bad"}, dir, dir)
	if code != 1 || called || !strings.Contains(errText, "config: model must be 48 or 128") {
		t.Fatalf("bad model code %d called %v %q", code, called, errText)
	}
}

func TestConfigModelDoesNotUpgrade(t *testing.T) {
	dir := t.TempDir()
	writeROM(t, filepath.Join(dir, "rom"), 0x01)
	sna := filepath.Join(dir, "big.sna")
	raw := make([]byte, 131103)
	raw[49179] = 0x34
	raw[49180] = 0x12
	if err := os.WriteFile(sna, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := config.Save(filepath.Join(dir, "configs"), "forty", config.File{Model: "48", Tape: sna}); err != nil {
		t.Fatal(err)
	}
	code, _, errText, _, called := exec(t, []string{"--config", "forty"}, dir, dir)
	if code != 1 || called || !strings.Contains(errText, "this snapshot needs --model 128 and a 32768-byte ROM") {
		t.Fatalf("code %d called %v %q", code, called, errText)
	}

	rom := make([]byte, 32768)
	if err := os.WriteFile(filepath.Join(dir, "rom", "spec128uk.rom"), rom, 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, errText, sess, called := exec(t, []string{sna}, dir, dir)
	if code != 0 || !called || errText != "" || !sess.Mach.Is128() || sess.Mach.CPU().PC != 0x1234 {
		t.Fatalf("upgrade code %d called %v 128 %v pc %04X err %q", code, called, sess != nil && sess.Mach.Is128(), pcOf(sess), errText)
	}
}

func pcOf(s *Session) uint16 {
	if s == nil || s.Mach == nil {
		return 0
	}
	return s.Mach.CPU().PC
}
