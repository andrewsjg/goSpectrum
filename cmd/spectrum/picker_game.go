package main

import (
	"image/color"
	"path/filepath"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"go.spectrum/internal/machine"
	"go.spectrum/internal/ui"
)

// Chooser colors. The title stripes are the Spectrum's red, yellow, green
// and cyan.
var (
	colBG     = color.RGBA{0x0c, 0x0e, 0x14, 0xFF}
	colBar    = color.RGBA{0x16, 0x1a, 0x24, 0xFF}
	colPanel  = color.RGBA{0x13, 0x17, 0x20, 0xFF}
	colBorder = color.RGBA{0x2a, 0x31, 0x40, 0xFF}
	colHover  = color.RGBA{0x1a, 0x22, 0x30, 0xFF}
	colCursor = color.RGBA{0x1d, 0x33, 0x52, 0xFF}
	colText   = color.RGBA{0xf0, 0xf0, 0xf0, 0xFF}
	colDim    = color.RGBA{0x7c, 0x86, 0x99, 0xFF}
	colAccent = color.RGBA{0x00, 0xd7, 0xd7, 0xFF}
	colMark   = color.RGBA{0xff, 0xd7, 0x00, 0xFF}
	colGo     = color.RGBA{0x00, 0x98, 0x40, 0xFF}
	colGoHi   = color.RGBA{0x00, 0xc8, 0x58, 0xFF}
	colWarn   = color.RGBA{0xff, 0x6a, 0x5a, 0xFF}
	colShade  = color.RGBA{0x00, 0x00, 0x00, 0xB0}
	stripes   = []color.RGBA{{0xd4, 0x00, 0x00, 0xFF}, {0xd4, 0xd4, 0x00, 0xFF}, {0x00, 0xc0, 0x00, 0xFF}, {0x00, 0xc0, 0xd4, 0xFF}}
	badges    = map[string]color.RGBA{
		".tap": {0x24, 0x5c, 0xc0, 0xFF},
		".tzx": {0x9c, 0x34, 0xa8, 0xFF},
		".sna": {0x1c, 0x8c, 0x50, 0xFF},
		".z80": {0xb0, 0x7c, 0x10, 0xFF},
	}
)

// pickerGame draws the chooser and turns host keys and the mouse into picker
// methods. It is not started on its own. shell owns the single RunGame call.
type pickerGame struct {
	p      *picker
	font   *ui.Font
	off    *ebiten.Image
	browse func() (string, error)
	winW   int
	winH   int
	sc     scrolls
	layout []placed
	hover  int
	tick   int
}

func newPickerGame(p *picker, font *ui.Font) *pickerGame {
	g := &pickerGame{p: p, font: font, browse: browseFile, hover: -1}
	g.layout = layoutRows(p.rows, p.cursor, &g.sc)
	return g
}

func (g *pickerGame) Update() error {
	p := g.p
	g.tick++
	if p.done {
		return nil
	}
	if ebiten.IsWindowBeingClosed() || (ctrlDown() && inpututil.IsKeyJustPressed(ebiten.KeyQ)) {
		p.Quit()
		return nil
	}
	if p.naming {
		for _, r := range ebiten.AppendInputChars(nil) {
			p.Type(r)
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
		p.Up()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
		p.Down()
	}
	// Left and Right step across the side-by-side model cards and buttons.
	if k := p.rows[p.cursor].kind; k == kindModel || k == kindSave || k == kindStart {
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
			p.Up()
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
			p.Down()
		}
	}
	if _, dy := ebiten.Wheel(); dy > 0 {
		p.Up()
	} else if dy < 0 {
		p.Down()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyNumpadEnter) {
		p.Enter()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
		p.Backspace()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		p.Escape()
	}
	if p.wantBrowse {
		browse := g.browse
		if browse == nil {
			browse = browseFile
		}
		path, err := browse()
		p.ApplyBrowse(path, err)
	}
	g.hover = -1
	if g.winW > 0 && !p.naming {
		x, y := ebiten.CursorPosition()
		if row, ok := rowAt(x, y, g.winW, g.winH, g.layout); ok {
			g.hover = row
			if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
				p.Click(row)
			}
		}
	}
	g.layout = layoutRows(p.rows, p.cursor, &g.sc)
	return nil
}

func (g *pickerGame) Draw(screen *ebiten.Image) {
	screen.Fill(colBG)
	if g.winW < 1 || g.winH < 1 {
		return
	}
	off := g.image()
	g.paint(off)
	scale, ox, oy := letterbox(g.winW, g.winH)
	var op ebiten.DrawImageOptions
	op.GeoM.Scale(float64(scale), float64(scale))
	op.GeoM.Translate(float64(ox), float64(oy))
	op.Filter = ebiten.FilterNearest
	screen.DrawImage(off, &op)
}

func (g *pickerGame) Layout(w, h int) (int, int) {
	g.winW, g.winH = w, h
	return w, h
}

func (g *pickerGame) image() *ebiten.Image {
	if g.off == nil {
		g.off = ebiten.NewImage(pickerW, pickerH)
	}
	return g.off
}

func (g *pickerGame) paint(off *ebiten.Image) {
	p := g.p
	off.Fill(colBG)

	fillRect(off, box{0, 0, pickerW, 24}, colBar)
	g.font.Text(off, "goSpectrum", 10, 4, 2, colText)
	g.font.Text(off, "48K / 128K", 180, 9, 1, colDim)
	for y := 0; y < 24; y++ {
		for i, c := range stripes {
			fillRect(off, box{pickerW - 72 + i*10 + (23 - y), y, 10, 1}, c)
		}
	}

	g.font.Text(off, "MACHINE", 8, 32, 1, colDim)
	g.font.Text(off, "SAVED CONFIGS", 8, 96, 1, colDim)
	g.font.Text(off, "TAPE OR SNAPSHOT", tapeList.x, 32, 1, colDim)
	fillRect(off, cfgList, colPanel)
	fillRect(off, tapeList, colPanel)
	strokeRect(off, cfgList, colBorder)
	strokeRect(off, tapeList, colBorder)
	if !hasKind(p.rows, kindConfig) {
		g.font.Text(off, "None yet.", cfgList.x+8, cfgList.y+8, 1, colDim)
		g.font.Text(off, "Pick a machine and", cfgList.x+8, cfgList.y+24, 1, colDim)
		g.font.Text(off, "tape, then Save.", cfgList.x+8, cfgList.y+36, 1, colDim)
	}

	for _, pl := range g.layout {
		g.drawRow(off, pl)
	}
	g.scrollbar(off, cfgList, cfgRowH, g.sc.cfg, countKinds(p.rows, kindConfig))
	g.scrollbar(off, tapeList, tapeRowH, g.sc.tape, countKinds(p.rows, kindTape, kindBrowse))

	if !p.naming {
		c := colDim
		if p.status != "" {
			c = colWarn
		}
		g.font.Text(off, fitText(p.summary(), pickerW/ui.GlyphW-2), 8, 242, 1, c)
	}
	g.font.Text(off, fitText("Configs: "+p.dirLabel, 34), 8, 290, 1, colDim)
	g.font.Text(off, "ARROWS ENTER CTRL+Q", pickerW-8-19*ui.GlyphW, 290, 1, colDim)

	if p.naming {
		g.drawPrompt(off)
	}
}

func (g *pickerGame) drawRow(off *ebiten.Image, pl placed) {
	p := g.p
	r := p.rows[pl.row]
	b := pl.box
	cursor := pl.row == p.cursor
	switch r.kind {
	case kindModel:
		bg, edge := colPanel, colBorder
		if cursor {
			bg = colCursor
		} else if pl.row == g.hover {
			bg = colHover
		}
		chosen := r.model == p.model
		if chosen {
			edge = colAccent
		}
		fillRect(off, b, bg)
		strokeRect(off, b, edge)
		centerText(g.font, off, r.label, b, b.y+8, 2, colText)
		sub := "Original"
		if r.model == "128" {
			sub = "AY sound"
		}
		centerText(g.font, off, sub, b, b.y+28, 1, colDim)
		if chosen {
			vector.FillCircle(off, float32(b.x+b.w-8), float32(b.y+8), 3, colMark, true)
		}
	case kindSave:
		bg, edge := colPanel, colBorder
		if cursor {
			bg, edge = colCursor, colAccent
		} else if pl.row == g.hover {
			bg = colHover
		}
		fillRect(off, b, bg)
		strokeRect(off, b, edge)
		centerText(g.font, off, "SAVE CONFIG", b, b.y+7, 1, colText)
	case kindStart:
		bg := colGo
		if cursor || pl.row == g.hover {
			bg = colGoHi
		}
		fillRect(off, b, bg)
		if cursor {
			strokeRect(off, b, colText)
		}
		centerText(g.font, off, "START "+p.model+"K", b, b.y+7, 1, colText)
	default:
		g.drawListRow(off, pl, r, cursor)
	}
}

// drawListRow draws a config, tape, or Browse row inside its list.
func (g *pickerGame) drawListRow(off *ebiten.Image, pl placed, r menuRow, cursor bool) {
	b := pl.box
	b.x, b.w = b.x+1, b.w-2
	if cursor {
		fillRect(off, b, colCursor)
		fillRect(off, box{b.x, b.y, 2, b.h}, colAccent)
	} else if pl.row == g.hover {
		fillRect(off, b, colHover)
	}
	if r.kind == kindConfig {
		fillRect(off, box{b.x + 7, b.y + 5, 4, 4}, colAccent)
		g.font.Text(off, fitText(r.label, (b.w-24)/ui.GlyphW), b.x+16, b.y+3, 1, colText)
		return
	}
	textX := b.x + 52
	chars := (b.w - 60) / ui.GlyphW
	var title, detail string
	switch {
	case r.kind == kindBrowse:
		g.font.Text(off, "+", b.x+28, b.y+7, 1, colAccent)
		title, detail = "Browse...", "Or drop a file on the window"
	case r.tape == "":
		title, detail = r.label, "Start in BASIC"
	default:
		ext := strings.ToLower(filepath.Ext(r.tape))
		fillRect(off, box{b.x + 18, b.y + 5, 28, 12}, badges[ext])
		g.font.Text(off, strings.ToUpper(ext[1:]), b.x+20, b.y+7, 1, colText)
		title, detail = prettyName(r.label)
		if detail == savedLabel {
			detail = ""
		}
	}
	if r.kind == kindTape {
		cx, cy := float32(b.x+9), float32(b.y+11)
		if r.tape == g.p.tape {
			vector.FillCircle(off, cx, cy, 3.5, colMark, true)
		} else {
			vector.StrokeCircle(off, cx, cy, 3, 1, colBorder, true)
		}
	}
	g.font.Text(off, fitText(title, chars), textX, b.y+2, 1, colText)
	if r.note != "" {
		g.font.Text(off, r.note, textX, b.y+12, 1, colAccent)
		n := len(r.note) + 2
		textX += n * ui.GlyphW
		chars -= n
	}
	g.font.Text(off, fitText(detail, chars), textX, b.y+12, 1, colDim)
}

// scrollbar draws a thumb at the right edge of a list that overflows.
func (g *pickerGame) scrollbar(off *ebiten.Image, area box, rowH, scroll, n int) {
	vis := area.h / rowH
	if n <= vis {
		return
	}
	h := area.h * vis / n
	y := area.y + (area.h-h)*scroll/(n-vis)
	fillRect(off, box{area.x + area.w - 3, y, 2, h}, colDim)
}

// drawPrompt is the modal name field for Save config.
func (g *pickerGame) drawPrompt(off *ebiten.Image) {
	p := g.p
	fillRect(off, box{0, 0, pickerW, pickerH}, colShade)
	d := box{(pickerW - 320) / 2, 100, 320, 96}
	fillRect(off, d, colBar)
	strokeRect(off, d, colAccent)
	g.font.Text(off, "SAVE CONFIG", d.x+12, d.y+10, 1, colAccent)
	field := box{d.x + 12, d.y + 28, d.w - 24, 18}
	fillRect(off, field, colBG)
	strokeRect(off, field, colBorder)
	name := []rune(string(p.name))
	if max := field.w/ui.GlyphW - 2; len(name) > max {
		name = name[len(name)-max:]
	}
	g.font.Text(off, string(name), field.x+5, field.y+5, 1, colText)
	if g.tick/30%2 == 0 {
		fillRect(off, box{field.x + 5 + len(name)*ui.GlyphW, field.y + 4, 2, 10}, colAccent)
	}
	if p.status != "" {
		g.font.Text(off, fitText(p.status, d.w/ui.GlyphW-3), d.x+12, d.y+54, 1, colWarn)
	}
	g.font.Text(off, "ENTER save   ESC cancel", d.x+12, d.y+76, 1, colDim)
}

// centerText draws s centered across b at height y.
func centerText(f *ui.Font, dst *ebiten.Image, s string, b box, y, scale int, c color.Color) {
	f.Text(dst, s, b.x+(b.w-len(s)*ui.GlyphW*scale)/2, y, scale, c)
}

func fillRect(dst *ebiten.Image, b box, c color.Color) {
	vector.FillRect(dst, float32(b.x), float32(b.y), float32(b.w), float32(b.h), c, false)
}

func strokeRect(dst *ebiten.Image, b box, c color.Color) {
	vector.StrokeRect(dst, float32(b.x)+0.5, float32(b.y)+0.5, float32(b.w)-1, float32(b.h)-1, 1, c, false)
}

func hasKind(rows []menuRow, k rowKind) bool { return countKinds(rows, k) > 0 }

func countKinds(rows []menuRow, kinds ...rowKind) int {
	n := 0
	for _, r := range rows {
		for _, k := range kinds {
			if r.kind == k {
				n++
			}
		}
	}
	return n
}

func fitText(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 0 {
		return ""
	}
	if n <= 3 {
		return string(r[:n])
	}
	return string(r[:n-3]) + "..."
}

func ctrlDown() bool {
	return ebiten.IsKeyPressed(ebiten.KeyControl) ||
		ebiten.IsKeyPressed(ebiten.KeyControlLeft) ||
		ebiten.IsKeyPressed(ebiten.KeyControlRight)
}

// chooserFont reads the character set from whichever ROM is present. A
// missing ROM is reported when a machine starts; until then the chooser
// uses the fallback font.
func chooserFont(exe, work string) *ui.Font {
	rom, _ := findROM(exe, work, "48.rom", "spec128uk.rom", "128.rom")
	return ui.NewFont(machine.Charset(rom))
}
