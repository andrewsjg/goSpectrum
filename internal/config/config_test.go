package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveLoadListAndNames(t *testing.T) {
	dir := t.TempDir()
	if names, err := List(filepath.Join(dir, "missing")); err != nil || names != nil {
		t.Fatalf("missing dir names %v err %v", names, err)
	}
	off := false
	f := File{
		Model:        "128",
		Tape:         "/tmp/game.tap",
		ROM:          "/tmp/128.rom",
		Scale:        3,
		Sound:        &off,
		RealtimeTape: true,
		NoAutoload:   true,
	}
	if err := Save(dir, "Elite.json", f); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "Elite.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"sound": false`) {
		t.Fatalf("sound not written: %s", raw)
	}
	got, err := Load(filepath.Join(dir, "Elite.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got.Model != "128" || got.Tape != f.Tape || got.ScaleOr(2) != 3 || got.SoundOn() || !got.RealtimeTape || !got.NoAutoload {
		t.Fatalf("%+v", got)
	}
	path, err := Resolve(dir, "Elite.json")
	if err != nil || filepath.Base(path) != "Elite.json" {
		t.Fatalf("resolve %s %v", path, err)
	}
	if _, err := Resolve(dir, "Elite"); err != nil {
		t.Fatal(err)
	}
	names, err := List(dir)
	if err != nil || len(names) != 1 || names[0] != "Elite" {
		t.Fatalf("list %v %v", names, err)
	}

	onDisk := filepath.Join(dir, "subdir", "other.json")
	if err := os.MkdirAll(filepath.Dir(onDisk), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(onDisk, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := Resolve(dir, onDisk); err != nil || got != onDisk {
		t.Fatalf("path resolve %s %v", got, err)
	}
	missing := filepath.Join(dir, "nope.json")
	_, err = Resolve(dir, missing)
	if err == nil || !strings.Contains(err.Error(), `not found in`) {
		t.Fatalf("missing path %v", err)
	}
	_, err = Resolve(dir, "absent")
	if err == nil || !strings.Contains(err.Error(), `not found in`) || !strings.Contains(err.Error(), dir) {
		t.Fatalf("missing name %v", err)
	}
}

func TestOmittedSoundAndScale(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plain.json")
	if err := os.WriteFile(path, []byte(`{"model":"48"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !got.SoundOn() || got.ScaleOr(2) != 2 {
		t.Fatalf("sound %v scale %d", got.SoundOn(), got.ScaleOr(2))
	}
	if err := os.WriteFile(path, []byte(`{"model":"plus3"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = Load(path)
	if err == nil || err.Error() != "config: model must be 48 or 128" {
		t.Fatalf("%v", err)
	}
	if err := Save(dir, "ok", File{Model: "16"}); err == nil || err.Error() != "config: model must be 48 or 128" {
		t.Fatalf("save model %v", err)
	}
}

func TestBadNames(t *testing.T) {
	dir := t.TempDir()
	bad := []string{"", " ", ".", "..", "a/b", "a+b", strings.Repeat("a", 41), "ok.json.json/no"}
	for _, name := range bad {
		if _, err := ValidName(name); err == nil || err.Error() != "config: name must be letters, numbers, spaces, '.', '_' or '-'" {
			t.Fatalf("%q err %v", name, err)
		}
	}
	if err := Save(dir, "my miner", File{Model: "48"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "my miner.json")); err != nil {
		t.Fatal(err)
	}
	got, err := ValidName("  Foo.JSON  ")
	if err != nil || got != "Foo" {
		t.Fatalf("%q %v", got, err)
	}
}

func TestRecent(t *testing.T) {
	dir := t.TempDir()
	if LoadRecent(dir) != nil {
		t.Fatal("missing file is not empty")
	}
	for i := 0; i < 10; i++ {
		if err := AddRecent(dir, Recent{Model: "48", Path: filepath.Join(dir, fmt.Sprintf("g%d.tap", i))}); err != nil {
			t.Fatal(err)
		}
	}
	again := filepath.Join(dir, "g5.tap")
	if err := AddRecent(dir, Recent{Model: "128", Path: again}); err != nil {
		t.Fatal(err)
	}
	got := LoadRecent(dir)
	if len(got) != maxRecent || got[0] != (Recent{Model: "128", Path: again}) || got[1].Path != filepath.Join(dir, "g9.tap") {
		t.Fatalf("recent %v", got)
	}
	for _, r := range got[1:] {
		if r.Path == again {
			t.Fatal("duplicate path kept")
		}
	}
	if names, _ := List(dir); len(names) != 0 {
		t.Fatalf("recent list shows as a config: %v", names)
	}
}
