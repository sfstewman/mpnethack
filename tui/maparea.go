package tui

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/sfstewman/mpnethack"
	"github.com/sfstewman/mpnethack/chat"
)

type MapArea struct {
	*tview.Box
	Session mpnethack.Session

	first bool
}

func NewMapArea(session mpnethack.Session /* session *Session */ /* ui *UI */) *MapArea {
	mapArea := &MapArea{
		Box:     tview.NewBox(),
		Session: session,
		first:   true,
	}

	mapArea.SetBorder(true)
	mapArea.SetTitle("Map")

	return mapArea
}

const (
	VoidChar   rune = '\u2591'
	BorderChar rune = '\u2580'
	CactusChar rune = '%' // '\U0001F335'
)

func (m *MapArea) Draw(screen tcell.Screen) {
	m.Box.DrawForSubclass(screen, m)
	x0, y0, w, h := m.GetInnerRect()

	ctrY := y0 + h/2

	session := m.Session
	g := session.Game()

	if g == nil {
		tview.Print(screen, "[red:white]No game[-:-]", x0, ctrY, w, tview.AlignCenter, tcell.ColorDefault)
		return
	}

	// draw the map...
	g.RLock()
	defer g.RUnlock()

	lvl := g.Level
	players := g.Players
	mobs := g.Mobs
	effects := g.EffectsOverlay

	pl := session.Player()
	if pl.S == nil {
		tview.Print(screen, "[red:white]No user[-:-]", x0, ctrY, w, tview.AlignCenter, tcell.ColorDefault)
		return
	}

	plI := pl.I
	plJ := pl.J

	deltaI := h/2 - plI
	deltaJ := w/2 - plJ

	lvlI0 := 0
	lvlJ0 := 0

	lvlI1 := lvl.H
	lvlJ1 := lvl.W

	if deltaI < 0 {
		lvlI0 = -deltaI
	}

	if deltaJ < 0 {
		lvlJ0 = -deltaJ
	}

	if lvlI1+deltaI > h {
		lvlI1 = h - deltaI
	}

	if lvlJ1+deltaJ > w {
		lvlJ1 = w - deltaJ
	}

	defaultStyle := tcell.StyleDefault.
		Background(tview.Styles.PrimitiveBackgroundColor).
		Foreground(tcell.ColorWhite)
	// Foreground(clr)

	numVoid := 0
	numEmpty := 0
	numBorder := 0
	numWall := 0
	size := 0
	for i := lvlI0; i < lvlI1; i++ {
		y := y0 + i + deltaI
		for j := lvlJ0; j < lvlJ1; j++ {
			x := x0 + j + deltaJ

			sty := defaultStyle
			var ch rune
			what := lvl.Get(i, j)
			switch what {
			case mpnethack.MarkerVoid:
				ch = '.' // VoidChar
				numVoid++
			case mpnethack.MarkerEmpty:
				ch = ' '
				numEmpty++
			case mpnethack.MarkerBorder:
				ch = BorderChar // FIXME: can do better!
				numBorder++
			case mpnethack.MarkerWall:
				ch = BorderChar // FIXME: can do better!
				numWall++
			case mpnethack.MarkerCactus:
				ch = CactusChar
				sty = defaultStyle.Foreground(tcell.ColorGreen)
			default:
				ch = '@'
			}

			screen.SetContent(x, y, ch, nil, sty)

			size++
		}
	}

	if m.first {
		session.Message(chat.System, fmt.Sprintf("[%d,%d,%d,%d] pl=(%d,%d) delta=(%d,%d) lvl0=(%d,%d) lvl1=(%d,%d), scr0=(%d,%d)",
			x0, y0, w, h, plJ, plI, deltaJ, deltaI, lvlJ0, lvlI0, lvlJ1, lvlI1, lvlJ0+x0-deltaJ, lvlI0+y0-deltaI))

		session.Message(chat.System, fmt.Sprintf("void: %d, empty: %d, border: %d, wall: %d, size: %d",
			numVoid, numEmpty, numBorder, numWall, size))
	}

	playerStyle := tcell.StyleDefault.
		Background(tcell.ColorBlue).
		Foreground(tcell.ColorWhite)
	for _, pl := range players {
		x := x0 + pl.J + deltaJ
		y := y0 + pl.I + deltaI

		ch := pl.Marker
		if ch == 0 {
			ch = '@'
		}

		if x >= x0 && x < (x0+w) && y >= y0 && y < (y0+h) { // m.InRect(x, y) {
			screen.SetContent(x, y, ch, nil, playerStyle)
		}

		if m.first {
			session.Message(chat.System, fmt.Sprintf("player (%d,%d) x=%d, y=%d, marker=\"%c\"",
				pl.J, pl.I, x, y, ch))
		}
	}

	deadMobStyle := tcell.StyleDefault.
		Background(tcell.ColorWhite).
		Foreground(tcell.ColorRed)

	mobStyle := tcell.StyleDefault.
		Background(tcell.ColorRed).
		Foreground(tcell.ColorWhite)
	for _, mob := range mobs {
		x := x0 + mob.J + deltaJ
		y := y0 + mob.I + deltaI

		mobInfo, _ := mpnethack.LookupMobInfo(mob.Type)
		ch := mobInfo.Marker
		if ch == 0 {
			ch = '@'
		}

		sty := mobStyle
		if !mob.IsAlive() {
			sty = deadMobStyle
		}

		if x >= x0 && x < (x0+w) && y >= y0 && y < (y0+h) { // m.InRect(x, y) {
			screen.SetContent(x, y, ch, nil, sty)
		}

		if m.first {
			session.Message(chat.System, fmt.Sprintf("mob %s (%d,%d) x=%d, y=%d, marker=\"%c\"",
				mobInfo.Name, mob.J, mob.I, x, y, ch))
		}
	}

	collStyle := tcell.StyleDefault.
		Background(tcell.ColorYellow).
		Foreground(tcell.ColorWhite)

	for _, fx := range effects {
		x := x0 + fx.J + deltaJ
		y := x0 + fx.I + deltaI

		sty := tcell.StyleDefault
		if fx.Collision != nil {
			sty = collStyle
		}
		if x >= x0 && x < (x0+w) && y >= y0 && y < (y0+h) {
			screen.SetContent(x, y, fx.Rune, nil, sty)
		}
	}

	m.first = false
}
