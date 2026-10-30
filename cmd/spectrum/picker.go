package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.spectrum/internal/config"
)

// Chooser geometry in logical pixels. Text is 8×8 glyphs. The offscreen
// image is pickerW×pickerH and the window scales it by an integer.
const (
	pickerW  = 480
	pickerH  = 304
	cfgRowH  = 14
	tapeRowH = 22
	cardStep = 88
)

// Fixed regions of the chooser. The second model card is cardStep to the
// right of the first.
var (
	modelCard = box{8, 44, 80, 40}
	cfgList   = box{8, 108, 168, 126}
	tapeList  = box{184, 44, 288, 180}
	saveBtn   = box{8, 258, 112, 22}
	startBtn  = box{352, 258, 120, 22}
)

type box struct{ x, y, w, h int }

func (b box) contains(x, y int) bool {
	return x >= b.x && y >= b.y && x < b.x+b.w && y < b.y+b.h
}

// placed is where one menu row is drawn and clicked.
type placed struct {
	row int
	box box
}

// scrolls is the first visible row of each list.
type scrolls struct{ cfg, tape int }

type rowKind int

const (
	kindHeader rowKind = iota
	kindConfig
	kindModel
	kindTape
	kindBrowse
	kindStart
	kindSave
)

// menuRow is one chooser line. note, such as "Recent" or "Saved state",
// leads the detail line of a tape.
type menuRow struct {
	kind   rowKind
	label  string
	config string
	model  string
	tape   string
	note   string
}

func (r menuRow) selectable() bool { return r.kind != kindHeader }

type tapeItem struct {
	path  string
	label string
	note  string
}

// savedLabel is the folder name save states show under in the list.
const savedLabel = "Saved states"

// pickerResult is a finished choice. Quit leaves the process.
// ConfigName launches that saved file. Otherwise Model and Tape launch
// the form, and the model counts as an explicit choice.
type pickerResult struct {
	Quit       bool
	Go         bool
	ConfigName string
	Model      string
	Tape       string
}

// picker is the chooser state. Tests drive it with keys and clicks.
// Drawing and the window live in pickerGame.
type picker struct {
	workDir    string
	configDir  string
	recent     []config.Recent
	dirLabel   string
	rows       []menuRow
	cursor     int
	model      string
	tape       string
	naming     bool
	name       []rune
	status     string
	wantBrowse bool
	done       bool
	result     pickerResult
}

func newPicker(workDir, configDir string) (*picker, error) {
	p := &picker{
		workDir:   workDir,
		configDir: configDir,
		dirLabel:  shortenDir(configDir),
		model:     "48",
	}
	p.loadRecent()
	// The last program played is selected, so Start replays it.
	if len(p.recent) > 0 {
		p.model, p.tape = p.recent[0].Model, p.recent[0].Path
	}
	if err := p.rebuild(true); err != nil {
		return nil, err
	}
	return p, nil
}

// loadRecent keeps the recent entries whose files still exist.
func (p *picker) loadRecent() {
	p.recent = nil
	for _, r := range config.LoadRecent(p.configDir) {
		if st, err := os.Stat(r.Path); err == nil && !st.IsDir() {
			p.recent = append(p.recent, r)
		}
	}
}

// reopen shows the chooser again after a game, with the recent list and
// save states refreshed and the previous choice kept.
func (p *picker) reopen() {
	p.done = false
	p.result = pickerResult{}
	p.naming = false
	p.wantBrowse = false
	p.status = ""
	p.loadRecent()
	if err := p.rebuild(false); err != nil {
		p.status = err.Error()
	}
}

func (p *picker) Result() (pickerResult, bool) {
	return p.result, p.done
}

func (p *picker) Up()   { p.move(-1) }
func (p *picker) Down() { p.move(1) }

func (p *picker) move(d int) {
	if p.naming || p.done || len(p.rows) == 0 {
		return
	}
	i := p.cursor
	for step := 0; step < len(p.rows); step++ {
		i = (i + d + len(p.rows)) % len(p.rows)
		if p.rows[i].selectable() {
			p.cursor = i
			return
		}
	}
}

// Enter activates the cursor row, confirms a name, or selects a radio.
func (p *picker) Enter() {
	if p.done {
		return
	}
	if p.naming {
		p.commitName()
		return
	}
	if p.cursor < 0 || p.cursor >= len(p.rows) {
		return
	}
	r := p.rows[p.cursor]
	switch r.kind {
	case kindModel:
		p.model = r.model
		p.status = ""
	case kindTape:
		p.tape = r.tape
		p.status = ""
	case kindConfig:
		p.result = pickerResult{Go: true, ConfigName: r.config}
		p.done = true
	case kindBrowse:
		p.wantBrowse = true
	case kindStart:
		p.result = pickerResult{Go: true, Model: p.model, Tape: p.tape}
		p.done = true
	case kindSave:
		p.naming = true
		p.name = nil
		p.status = ""
	}
}

// Click moves the cursor to a row and activates it. A saved config launches
// at once, so it only selects on the first click and launches on the second.
func (p *picker) Click(i int) {
	if p.naming || p.done || i < 0 || i >= len(p.rows) || !p.rows[i].selectable() {
		return
	}
	if p.rows[i].kind == kindConfig && i != p.cursor {
		p.cursor = i
		return
	}
	p.cursor = i
	p.Enter()
}

func (p *picker) Type(r rune) {
	if !p.naming || p.done || r < 32 || r == 127 {
		return
	}
	if len(p.name) >= 40 {
		return
	}
	p.name = append(p.name, r)
}

func (p *picker) Backspace() {
	if !p.naming || len(p.name) == 0 {
		return
	}
	p.name = p.name[:len(p.name)-1]
}

// Escape cancels the name prompt and leaves the menu up.
func (p *picker) Escape() {
	if !p.naming {
		return
	}
	p.naming = false
	p.name = nil
}

func (p *picker) Quit() {
	p.done = true
	p.naming = false
	p.result = pickerResult{Quit: true}
}

// ApplyBrowse records a dialog result. An empty path is a cancel.
// A bad extension stays on the menu with the same message the CLI uses.
func (p *picker) ApplyBrowse(path string, err error) {
	p.wantBrowse = false
	if err != nil {
		p.status = err.Error()
		return
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return
	}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".tap", ".tzx", ".sna", ".z80":
	default:
		p.status = fmt.Sprintf("%s: unknown extension %q (supported: .tap, .tzx, .sna, .z80)", path, ext)
		return
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	p.tape = path
	p.status = ""
	if err := p.rebuild(false); err != nil {
		p.status = err.Error()
		return
	}
	p.focusTape(path)
}

func (p *picker) fail(msg string) {
	p.done = false
	p.result = pickerResult{}
	p.naming = false
	p.wantBrowse = false
	p.status = msg
}

func (p *picker) commitName() {
	typed := string(p.name)
	on := true
	f := config.File{Model: p.model, Scale: 2, Sound: &on}
	if p.tape != "" {
		f.Tape = p.tape
	}
	if err := config.Save(p.configDir, typed, f); err != nil {
		p.status = err.Error()
		return
	}
	canon, err := config.ValidName(typed)
	if err != nil {
		p.status = err.Error()
		return
	}
	p.status = fmt.Sprintf("Saved %q", canon)
	p.naming = false
	p.name = nil
	if err := p.rebuild(false); err != nil {
		p.status = err.Error()
		return
	}
	p.focusKind(kindSave)
}

func (p *picker) rebuild(initial bool) error {
	var keep menuRow
	if !initial && p.cursor >= 0 && p.cursor < len(p.rows) {
		keep = p.rows[p.cursor]
	}
	names, err := config.List(p.configDir)
	if err != nil {
		return err
	}
	files := listTapes(p.workDir, config.SavesDir(p.configDir))
	extra := []string{p.tape}
	var recent []string
	for _, r := range p.recent {
		extra = append(extra, r.Path)
		recent = append(recent, r.Path)
	}
	for _, path := range extra {
		if path != "" && !hasTape(files, path) {
			files = append(files, tapeItem{path: path, label: displayRel(p.workDir, path)})
		}
	}
	sortTapes(files)
	p.rows = makeRows(names, files, recent)
	if initial {
		p.cursor = firstCursor(p.rows)
		return nil
	}
	for i, r := range p.rows {
		if r.kind == keep.kind && r.label == keep.label && r.tape == keep.tape && r.config == keep.config {
			p.cursor = i
			return nil
		}
	}
	if p.cursor >= len(p.rows) {
		p.cursor = len(p.rows) - 1
	}
	if p.cursor < 0 {
		p.cursor = 0
	}
	if p.cursor < len(p.rows) && !p.rows[p.cursor].selectable() {
		p.move(1)
	}
	return nil
}

func (p *picker) focusKind(k rowKind) {
	for i, r := range p.rows {
		if r.kind == k {
			p.cursor = i
			return
		}
	}
}

func (p *picker) focusTape(path string) {
	for i, r := range p.rows {
		if r.kind == kindTape && r.tape == path {
			p.cursor = i
			return
		}
	}
}

func (p *picker) focusLabel(kind rowKind, label string) {
	for i, r := range p.rows {
		if r.kind == kind && r.label == label {
			p.cursor = i
			return
		}
	}
}

func firstCursor(rows []menuRow) int {
	for _, k := range []rowKind{kindConfig, kindModel} {
		for i, r := range rows {
			if r.kind == k {
				return i
			}
		}
	}
	return 0
}

// makeRows lists recent programs, newest first, straight after No tape,
// then every other file in name order.
func makeRows(configs []string, files []tapeItem, recent []string) []menuRow {
	var rows []menuRow
	if len(configs) > 0 {
		rows = append(rows, menuRow{kind: kindHeader, label: "Saved configs"})
		for _, n := range configs {
			rows = append(rows, menuRow{kind: kindConfig, label: n, config: n})
		}
	}
	rows = append(rows,
		menuRow{kind: kindHeader, label: "Model"},
		menuRow{kind: kindModel, label: "48K", model: "48"},
		menuRow{kind: kindModel, label: "128K", model: "128"},
		menuRow{kind: kindHeader, label: "Tape or snapshot"},
		menuRow{kind: kindTape, label: "No tape"},
	)
	tapeRow := func(f tapeItem, isRecent bool) menuRow {
		note := f.note
		if isRecent {
			note = strings.TrimSuffix("Recent, "+note, ", ")
		}
		return menuRow{kind: kindTape, label: f.label, tape: f.path, note: note}
	}
	seen := map[string]bool{}
	for _, path := range recent {
		for _, f := range files {
			if f.path == path && !seen[path] {
				rows = append(rows, tapeRow(f, true))
				seen[path] = true
			}
		}
	}
	for _, f := range files {
		if !seen[f.path] {
			rows = append(rows, tapeRow(f, false))
		}
	}
	rows = append(rows,
		menuRow{kind: kindBrowse, label: "Browse..."},
		menuRow{kind: kindSave, label: "Save config"},
		menuRow{kind: kindStart, label: "Start"},
	)
	return rows
}

// layoutRows places every visible row. Configs fill one list, tapes and
// Browse the other; makeRows keeps each list's rows contiguous. The list
// that holds the cursor scrolls to keep it in view.
func layoutRows(rows []menuRow, cursor int, sc *scrolls) []placed {
	var out []placed
	list := func(in func(rowKind) bool, area box, rowH int, scroll *int) {
		first, n := -1, 0
		for i, r := range rows {
			if in(r.kind) {
				if first < 0 {
					first = i
				}
				n++
			}
		}
		vis := area.h / rowH
		if c := cursor - first; first >= 0 && c >= 0 && c < n {
			*scroll = fitScroll(c, *scroll, vis, n)
		} else {
			*scroll = fitScroll(*scroll, *scroll, vis, n)
		}
		for k := *scroll; k < n && k < *scroll+vis; k++ {
			out = append(out, placed{first + k, box{area.x, area.y + (k-*scroll)*rowH, area.w, rowH}})
		}
	}
	list(func(k rowKind) bool { return k == kindConfig }, cfgList, cfgRowH, &sc.cfg)
	list(func(k rowKind) bool { return k == kindTape || k == kindBrowse }, tapeList, tapeRowH, &sc.tape)
	card := modelCard
	for i, r := range rows {
		switch r.kind {
		case kindModel:
			out = append(out, placed{i, card})
			card.x += cardStep
		case kindSave:
			out = append(out, placed{i, saveBtn})
		case kindStart:
			out = append(out, placed{i, startBtn})
		}
	}
	return out
}

// prettyName splits a TOSEC-style file name such as
// "Elite (1985)(Firebird Software)(48k).tap" into "Elite" and
// "1985, 48k, Firebird Software". The publisher goes last so a truncated
// detail still shows the flags that tell versions apart. A plain name keeps
// its folder as detail.
func prettyName(label string) (title, detail string) {
	base := strings.TrimSuffix(filepath.Base(label), filepath.Ext(label))
	i := strings.IndexAny(base, "([")
	if i <= 0 {
		if dir := filepath.Dir(label); dir != "." {
			detail = dir
		}
		return base, detail
	}
	title = strings.TrimSpace(base[:i])
	var parts []string
	for _, f := range strings.FieldsFunc(base[i:], func(r rune) bool { return strings.ContainsRune("()[]", r) }) {
		if f = strings.TrimSpace(f); f != "" {
			parts = append(parts, f)
		}
	}
	if len(parts) > 2 {
		parts = append(append([]string{parts[0]}, parts[2:]...), parts[1])
	}
	return title, strings.Join(parts, ", ")
}

func (p *picker) summary() string {
	if p.status != "" {
		return p.status
	}
	tape := "No tape"
	if p.tape != "" {
		tape = filepath.Base(p.tape)
	}
	return p.model + "K   " + tape
}

// listTapes finds programs in work, work/Games, and the save-state folder.
func listTapes(work, saves string) []tapeItem {
	var items []tapeItem
	scan := func(dir, label, note string) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if strings.HasPrefix(name, ".") || !tapeExt(name) {
				continue
			}
			abs := filepath.Join(dir, name)
			if a, err := filepath.Abs(abs); err == nil {
				abs = a
			}
			l := displayRel(work, abs)
			if label != "" {
				l = filepath.Join(label, name)
			}
			items = append(items, tapeItem{path: abs, label: l, note: note})
		}
	}
	scan(work, "", "")
	scan(filepath.Join(work, "Games"), "", "")
	scan(saves, savedLabel, "Saved state")
	sortTapes(items)
	return items
}

func sortTapes(items []tapeItem) {
	for i := 1; i < len(items); i++ {
		j := i
		for j > 0 && strings.ToLower(items[j].label) < strings.ToLower(items[j-1].label) {
			items[j], items[j-1] = items[j-1], items[j]
			j--
		}
	}
}

func hasTape(items []tapeItem, path string) bool {
	for _, it := range items {
		if it.path == path {
			return true
		}
	}
	return false
}

func tapeExt(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".tap", ".tzx", ".sna", ".z80":
		return true
	default:
		return false
	}
}

func displayRel(work, abs string) string {
	rel, err := filepath.Rel(work, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return abs
	}
	return rel
}

func shortenDir(dir string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return dir
	}
	if dir == home {
		return "~"
	}
	prefix := home + string(os.PathSeparator)
	if strings.HasPrefix(dir, prefix) {
		return "~" + dir[len(home):]
	}
	return dir
}

// letterbox maps the window onto the 400×280 chooser with an integer scale.
func letterbox(winW, winH int) (scale, ox, oy int) {
	scale = 1
	if winW < pickerW || winH < pickerH {
		ox = (winW - pickerW) / 2
		oy = (winH - pickerH) / 2
		return
	}
	scale = winW / pickerW
	if sy := winH / pickerH; sy < scale {
		scale = sy
	}
	if scale < 1 {
		scale = 1
	}
	ox = (winW - pickerW*scale) / 2
	oy = (winH - pickerH*scale) / 2
	return
}

// rowAt converts a window point into the menu row placed under it,
// undoing the letterbox.
func rowAt(x, y, winW, winH int, layout []placed) (int, bool) {
	scale, ox, oy := letterbox(winW, winH)
	if scale < 1 || x < ox || y < oy {
		return 0, false
	}
	lx := (x - ox) / scale
	ly := (y - oy) / scale
	for _, pl := range layout {
		if pl.box.contains(lx, ly) {
			return pl.row, true
		}
	}
	return 0, false
}

func fitScroll(cursor, scroll, vis, n int) int {
	if vis < 1 {
		vis = 1
	}
	if n <= vis {
		return 0
	}
	if cursor < scroll {
		scroll = cursor
	}
	if cursor >= scroll+vis {
		scroll = cursor - vis + 1
	}
	if scroll < 0 {
		scroll = 0
	}
	if max := n - vis; scroll > max {
		scroll = max
	}
	return scroll
}
