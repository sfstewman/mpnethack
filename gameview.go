package main

import (
	"fmt"

	"github.com/rivo/tview"
)

const (
	ModalHeight = 10
	ModalWidth  = 40
)

type GameView struct {
	*tview.Pages

	G *Game

	Popup *tview.Modal

	// P *Player
	W       *tview.TextView
	Status  *tview.TextView
	ChatLog *tview.TextView
}

func NewGameView(g *Game) *GameView {
	gv := &GameView{
		Pages:   tview.NewPages(),
		G:       g,
		Popup:   tview.NewModal(),
		W:       tview.NewTextView(),
		Status:  tview.NewTextView(),
		ChatLog: tview.NewTextView(),
		// Input:   tview.NewTextView(),
	}

	// Configure base UI layout
	base := tview.NewGrid()
	base.AddItem(gv.W, 0, 0, 2, 2, 0, 0, false)

	/*
		statusBox := tview.NewFlex()
		statusBox.SetBorder(true)
		statusBox.SetTitle("Stats")
		statusBox.AddItem(gv.Status, 0, 1, false)
		base.AddItem(statusBox, 0, 2, 2, 1, 0, 0, false)
	*/
	base.AddItem(gv.Status, 0, 2, 2, 1, 0, 0, false)

	/*
		chatBox := tview.NewFlex()
		chatBox.SetBorder(true)
		chatBox.SetTitle("Log")
		chatBox.AddItem(gv.ChatLog, 0, 1, false)
		base.AddItem(chatBox, 2, 0, 1, 3, 0, 0, false)
	*/
	base.AddItem(gv.ChatLog, 2, 0, 1, 3, 0, 0, false)
	base.SetBorders(true)

	gv.AddPage("base", base, true, true)

	// Configure popup dialog
	modal := tview.NewFlex()
	modal.AddItem(nil, 0, 1, false)
	{
		inner := tview.NewFlex()
		inner.SetDirection(tview.FlexRow)
		inner.AddItem(nil, 0, 1, false)
		inner.AddItem(gv.Popup, ModalHeight, 1, true)
		inner.AddItem(nil, 0, 1, false)

		modal.AddItem(inner, ModalWidth, 1, true)
	}
	modal.AddItem(nil, 0, 1, false)

	gv.AddPage("modal", modal, true, false)

	return gv
}

func (gv *GameView) Message(l MsgLevel, s string) error {
	_, err := fmt.Fprintln(gv.ChatLog, s)
	return err
}
