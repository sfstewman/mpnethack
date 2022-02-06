package tui

import (
	"fmt"
	"log"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/sfstewman/mpnethack"
)

type StatusFrame struct {
	*tview.Box
	UI *UI

	cooldowns mpnethack.Cooldowns
}

func NewStatusFrame(ui *UI) *StatusFrame {
	return &StatusFrame{
		Box: tview.NewBox(),
		UI:  ui,
	}
}

func (fr *StatusFrame) Draw(screen tcell.Screen) {
	fr.Box.DrawForSubclass(screen, fr)

	x0, y0, w, h := fr.GetInnerRect()

	session := fr.UI.Session
	pl := session.Player()

	adminStr := ""
	if session.IsAdministrator() {
		adminStr = "[blue:white:b]ADMIN"
	}

	ymax := y0 + h
	y := y0

	s := fmt.Sprintf("[:]%s %s[-:-:-]", session.UserName(), adminStr)
	if w > 0 && h > 0 {
		tview.Print(screen, s, x0, y, w, tview.AlignCenter, tcell.ColorDefault)
	}

	if y++; y >= ymax {
		// ... HANDLE BETTER ...
		return
	}

	DrawHorizontalDivider(fr.Box, screen, y)

	if y++; y >= ymax {
		// ... HANDLE BETTER ...
		return
	}

	g := session.Game()
	fr.cooldowns = g.GetCooldowns(session, fr.cooldowns)
	cooldowns := fr.cooldowns

	attackCooldown := int(pl.BusyTick)
	if pl.SwingState > 0 {
		wait := int(pl.SwingRate)*(int(pl.SwingState)-1) + int(pl.SwingTick)
		if wait > attackCooldown {
			attackCooldown = wait
		}

		if attackCooldown < 0 {
			log.Printf("negative cooldown! cd=%d,rate=%d,state=%d,tick=%d, busy=%d",
				attackCooldown, pl.SwingRate, pl.SwingState, pl.SwingTick, pl.BusyTick)
			attackCooldown = 0
		}
	}

	playerDead := !pl.IsAlive()

	for actInd, nticks := range cooldowns {
		act := mpnethack.ActionType(actInd)

		var s string
		switch act {
		case mpnethack.Nothing, mpnethack.Move:
			continue

		case mpnethack.Attack:
			s = "ATT"
			nticks = uint32(attackCooldown)

		case mpnethack.Defend:
			s = "DEF"
		default:
			s = fmt.Sprintf("[%d]", int(act))
		}

		/*
			clr := tcell.ColorWhite
			if nticks > 0 {
				clr = tcell.ColorGray
			}
		*/

		/*
			style := tcell.StyleDefault.
				Background(tview.Styles.PrimitiveBackgroundColor).
				Foreground(clr)
		*/

		prog := ""
		switch {
		case playerDead:
			prog = " [yellow:]dead[-:-]"
		case nticks > 50:
			prog = fmt.Sprintf("<==%d==>", nticks/10)
		case nticks > 20:
			prog = "<====>"
		case nticks > 15:
			prog = "<===>"
		case nticks > 10:
			prog = "<==>"
		case nticks > 5:
			prog = "<=>"
		case nticks > 2:
			prog = "<>"
		case nticks == 0:
			prog = ""
		}

		var tag string = "[::b]"
		if playerDead || nticks > 0 {
			tag = ""
		}

		tview.Print(screen, fmt.Sprintf("%s%s %s[-:-]", tag, s, prog), x0, y, w, tview.AlignLeft, tcell.ColorWhite) // clr)

		if y++; y >= ymax {
			// ... HANDLE BETTER ...
			return
		}
	}

	DrawHorizontalDivider(fr.Box, screen, y)

	if y++; y >= ymax {
		// ... HANDLE BETTER ...
		return
	}

	{
		stats := pl.GetStats()

		tag := ""
		if stats.HP < 3 {
			tag = "[red:]"
		}
		tview.Print(screen, fmt.Sprintf("%sHealth %d[-:-]", tag, stats.HP), x0, y, w, tview.AlignLeft, tcell.ColorWhite)

		if y++; y >= ymax {
			// ... handle better ...
			return
		}
	}

	DrawHorizontalDivider(fr.Box, screen, y)

	if y++; y >= ymax {
		// ... HANDLE BETTER ...
		return
	}

}
