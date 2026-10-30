package machine

// Pulse is one stretch of a constant EAR level, in T-states.
type Pulse struct {
	Level bool
	Dur   int
}

// Player walks an edge list. It does not trap the ROM loader.
type Player struct {
	edges    []Pulse
	i        int
	left     int
	level    bool
	playing  bool
	forceLow bool
}

func (p *Player) Arm(edges []Pulse, forceLow bool) {
	*p = Player{edges: append([]Pulse(nil), edges...), forceLow: forceLow}
	if len(p.edges) == 0 {
		p.level = false
		return
	}
	p.playing = true
	p.level = p.edges[0].Level
	p.left = p.edges[0].Dur
	p.skipEmpty()
}

func (p *Player) skipEmpty() {
	for p.playing && p.left <= 0 {
		p.i++
		if p.i >= len(p.edges) {
			p.finish()
			return
		}
		p.level = p.edges[p.i].Level
		p.left = p.edges[p.i].Dur
	}
}

func (p *Player) finish() {
	p.playing = false
	p.left = 0
	if p.forceLow {
		p.level = false
	}
}

func (p *Player) Playing() bool { return p.playing }

// Stop ends playback and forces the EAR level low. A TZX pause of 0 ms does this.
func (p *Player) Stop() {
	p.playing = false
	p.left = 0
	p.level = false
	p.forceLow = true
}

func (p *Player) Level() bool {
	if !p.playing && p.forceLow {
		return false
	}
	return p.level
}

// Advance consumes n T-states. LevelAt does the same and returns the EAR bit.
func (p *Player) Advance(n int) {
	if !p.playing || n <= 0 {
		return
	}
	for n > 0 && p.playing {
		if p.left > n {
			p.left -= n
			return
		}
		n -= p.left
		p.i++
		if p.i >= len(p.edges) {
			p.finish()
			return
		}
		p.level = p.edges[p.i].Level
		p.left = p.edges[p.i].Dur
		p.skipEmpty()
	}
}

func (p *Player) LevelAt(n int) bool {
	p.Advance(n)
	return p.Level()
}
