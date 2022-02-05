package tui

import (
	"fmt"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type ItemFrame struct {
	*tview.Box
	UI *UI
}

func NewItemFrame(ui *UI) *ItemFrame {
	return &ItemFrame{
		Box: tview.NewBox(),
		UI:  ui,
	}
}

func (fr *ItemFrame) Draw(screen tcell.Screen) {
	fr.Box.DrawForSubclass(screen, fr)

	x0, y0, w, h := fr.GetInnerRect()

	if w <= 0 || h <= 0 {
		return
	}

	session := fr.UI.Session
	player := session.Player()

	ymax := y0 + h
	y := y0

	s := fmt.Sprintf("[::b]Weapon:[::-] %s", player.Weapon.Name())
	tview.Print(screen, s, x0, y, w, tview.AlignLeft, tcell.ColorDefault)

	if y++; y >= ymax {
		// ... HANDLE BETTER ...
		return
	}

	DrawHorizontalDivider(fr.Box, screen, y)

	if y++; y >= ymax {
		// ... HANDLE BETTER ...
		return
	}
}
