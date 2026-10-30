package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.spectrum/internal/config"
)

func mustPicker(t *testing.T, work, cfg string) *picker {
	t.Helper()
	p, err := newPicker(work, cfg)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPickerFilesAndKeys(t *testing.T) {
	work := t.TempDir()
	cfg := t.TempDir()
	touch := func(path string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	touch(filepath.Join(work, "game.TAP"))
	touch(filepath.Join(work, "note.txt"))
	touch(filepath.Join(work, ".secret.tap"))
	touch(filepath.Join(work, "rom", "hidden.tap"))
	touch(filepath.Join(work, "Games", "b.z80"))
	touch(filepath.Join(work, "Games", "nested", "no.tap"))
	touch(filepath.Join(work, "Games", ".dot.sna"))

	p := mustPicker(t, work, cfg)
	if p.rows[p.cursor].label != "48K" {
		t.Fatalf("cursor %s", p.rows[p.cursor].label)
	}
	var labels []string
	for _, r := range p.rows {
		if r.kind == kindTape && r.tape != "" {
			labels = append(labels, r.label)
		}
	}
	want := []string{"game.TAP", filepath.Join("Games", "b.z80")}
	if strings.Join(labels, ",") != strings.Join(want, ",") {
		t.Fatalf("tapes %v", labels)
	}
	for i := 0; i < len(p.rows)*2; i++ {
		if p.rows[p.cursor].kind == kindHeader {
			t.Fatal("cursor landed on a header")
		}
		p.Down()
	}

	p.focusLabel(kindModel, "128K")
	p.Enter()
	if p.model != "128" {
		t.Fatal(p.model)
	}
	p.focusLabel(kindTape, "game.TAP")
	p.Enter()
	if !strings.HasSuffix(p.tape, "game.TAP") {
		t.Fatal(p.tape)
	}
	p.focusLabel(kindBrowse, "Browse...")
	p.Enter()
	if !p.wantBrowse {
		t.Fatal("browse was not requested")
	}
	outside := filepath.Join(t.TempDir(), "extra.tzx")
	p.ApplyBrowse(outside, nil)
	if p.tape != outside && p.tape != absPath(outside) {
		t.Fatalf("browsed %s", p.tape)
	}
	p.ApplyBrowse("", nil)
	if p.tape == "" {
		t.Fatal("cancel cleared the tape")
	}
	kept := p.tape
	p.ApplyBrowse(filepath.Join(work, "note.dsk"), nil)
	if p.tape != kept || !strings.Contains(p.status, "unknown extension") {
		t.Fatalf("tape %s status %q", p.tape, p.status)
	}

	p.focusLabel(kindStart, "Start")
	p.Enter()
	res, done := p.Result()
	if !done || res.ConfigName != "" || res.Model != "128" || res.Tape != kept {
		t.Fatalf("%+v done %v", res, done)
	}
}

func TestPickerSaveAndSavedLaunch(t *testing.T) {
	work := t.TempDir()
	cfg := t.TempDir()
	if err := os.WriteFile(filepath.Join(work, "game.tap"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := mustPicker(t, work, cfg)
	p.focusLabel(kindModel, "128K")
	p.Enter()
	p.focusLabel(kindTape, "game.tap")
	p.Enter()
	p.focusLabel(kindSave, "Save config")
	p.Enter()
	for _, r := range "bad/name" {
		p.Type(r)
	}
	p.Enter()
	if p.done || !p.naming || !strings.Contains(p.status, "config: name must be letters, numbers, spaces, '.', '_' or '-'") {
		t.Fatalf("done %v naming %v status %q", p.done, p.naming, p.status)
	}
	p.Escape()
	if p.naming {
		t.Fatal("escape left the prompt up")
	}
	p.Enter()
	for _, r := range "my miner" {
		p.Type(r)
	}
	p.Backspace()
	p.Type('r')
	p.Enter()
	if p.naming || p.status != `Saved "my miner"` {
		t.Fatalf("naming %v status %q", p.naming, p.status)
	}
	if p.rows[p.cursor].kind != kindSave {
		t.Fatalf("cursor %v", p.rows[p.cursor])
	}
	names, err := config.List(cfg)
	if err != nil || len(names) != 1 || names[0] != "my miner" {
		t.Fatalf("list %v %v", names, err)
	}
	raw, err := os.ReadFile(filepath.Join(cfg, "my miner.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"sound": true`) || !strings.Contains(string(raw), "game.tap") {
		t.Fatalf("saved file %s", raw)
	}

	again := mustPicker(t, work, cfg)
	if again.rows[again.cursor].kind != kindConfig || again.rows[again.cursor].label != "my miner" {
		t.Fatalf("start row %+v", again.rows[again.cursor])
	}
	again.Enter()
	res, done := again.Result()
	if !done || res.ConfigName != "my miner" {
		t.Fatalf("%+v", res)
	}
}

func TestPickerClickAndScroll(t *testing.T) {
	work := t.TempDir()
	p := mustPicker(t, work, t.TempDir())
	model := -1
	header := -1
	for i, r := range p.rows {
		if r.label == "128K" {
			model = i
		}
		if r.kind == kindHeader && r.label == "Tape or snapshot" {
			header = i
		}
	}
	p.Click(header)
	if p.rows[p.cursor].label != "48K" || p.model != "48" {
		t.Fatal("header click changed the selection")
	}
	p.Click(model)
	if p.rows[p.cursor].label != "128K" || p.model != "128" {
		t.Fatalf("click model %s cursor %s", p.model, p.rows[p.cursor].label)
	}

	if fitScroll(20, 0, 12, 30) != 9 || fitScroll(5, 10, 12, 30) != 5 || fitScroll(0, 0, 12, 4) != 0 {
		t.Fatal("scroll")
	}
	layout := layoutRows(p.rows, p.cursor, &scrolls{})
	card48 := model - 1
	// The window is 960×608 at scale 2, so card 48K starts at (16, 88).
	row, ok := rowAt(20, 100, 960, 608, layout)
	if !ok || row != card48 {
		t.Fatalf("origin row %d %v", row, ok)
	}
	row, ok = rowAt(40, 100, 1000, 608, layout)
	if !ok || row != card48 {
		t.Fatalf("letterbox row %d %v", row, ok)
	}
	if _, ok := rowAt(30, 100, 1000, 608, layout); ok {
		t.Fatal("click in the margin hit a row")
	}
	row, ok = rowAt(20, 120, 960, 648, layout)
	if !ok || row != card48 {
		t.Fatalf("vertical letterbox %d %v", row, ok)
	}
}

func TestPickerConfigNeedsSecondClick(t *testing.T) {
	cfg := t.TempDir()
	if err := config.Save(cfg, "miner", config.File{Model: "48"}); err != nil {
		t.Fatal(err)
	}
	p := mustPicker(t, t.TempDir(), cfg)
	p.focusKind(kindModel)
	p.focusLabel(kindConfig, "miner")
	row := p.cursor
	p.focusKind(kindModel)
	p.Click(row)
	if _, done := p.Result(); done || p.cursor != row {
		t.Fatal("first click launched the config")
	}
	p.Click(row)
	if res, done := p.Result(); !done || res.ConfigName != "miner" {
		t.Fatalf("%+v %v", res, done)
	}
}

func TestLayoutScrollsTapeList(t *testing.T) {
	var items []tapeItem
	for i := 0; i < 30; i++ {
		items = append(items, tapeItem{path: fmt.Sprintf("/g/%02d.tap", i), label: fmt.Sprintf("%02d.tap", i)})
	}
	rows := makeRows(nil, items)
	browse := -1
	for i, r := range rows {
		if r.kind == kindBrowse {
			browse = i
		}
	}
	var sc scrolls
	layout := layoutRows(rows, browse, &sc)
	vis := tapeList.h / tapeRowH
	if sc.tape != 32-vis {
		t.Fatalf("scroll %d", sc.tape)
	}
	for _, pl := range layout {
		if pl.row == browse && pl.box.y != tapeList.y+(vis-1)*tapeRowH {
			t.Fatalf("browse row at y %d", pl.box.y)
		}
		if pl.box.y+pl.box.h > tapeList.y+tapeList.h && pl.box.x == tapeList.x {
			t.Fatalf("row %d spills out of the list", pl.row)
		}
	}
}

func TestPrettyName(t *testing.T) {
	cases := []struct{ in, title, detail string }{
		{filepath.Join("Games", "Elite (1985)(Firebird Software)(48k) nolenslok.tap"), "Elite", "1985, 48k, nolenslok, Firebird Software"},
		{"Manic Miner (1983)(Bug-Byte Software).Tap", "Manic Miner", "1983, Bug-Byte Software"},
		{filepath.Join("Games", "b.z80"), "b", "Games"},
		{"game.tap", "game", ""},
	}
	for _, c := range cases {
		if title, detail := prettyName(c.in); title != c.title || detail != c.detail {
			t.Errorf("prettyName(%q) = %q, %q", c.in, title, detail)
		}
	}
}

func absPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}
