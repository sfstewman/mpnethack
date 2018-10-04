package main

import (
	"fmt"
	"log"
	"sync"

	"github.com/gdamore/tcell"
	"github.com/rivo/tview"
)

const (
	ModalHeight = 10
	ModalWidth  = 40
)

type InputMode int

const (
	InputDirect InputMode = iota
	InputChat
)

type GameInput struct {
	mu sync.Mutex
	*tview.InputField
	G    *Game
	Mode InputMode
}

func (inp *GameInput) onKey(ev *tcell.EventKey) *tcell.EventKey {
	inp.mu.Lock()
	defer inp.mu.Unlock()

	if inp.Mode == InputChat {
		return ev
	}

	k := ev.Key()
	if k == tcell.KeyRune && ev.Rune() == '/' {
		inp.Mode = InputChat
		return nil
	}

	return nil
}

func (inp *GameInput) onDone(key tcell.Key) {
	inp.mu.Lock()
	defer inp.mu.Unlock()

	switch key {
	case tcell.KeyEnter:
		// XXX: handle message
		msg := inp.GetText()
		log.Printf("[chat] %s", msg)
		inp.SetText("")
		inp.G.Message(MsgChat, msg)
		inp.Mode = InputDirect

	case tcell.KeyEsc:
		inp.Mode = InputDirect
	}
}

func NewGameInput(g *Game) *GameInput {
	inp := &GameInput{
		InputField: tview.NewInputField(),
		G:          g,
	}

	inp.SetInputCapture(inp.onKey)
	inp.SetDoneFunc(inp.onDone)

	inp.SetLabel("/ ")

	return inp
}

type GameView struct {
	*tview.Pages

	G *Game

	Popup *tview.Modal

	// P *Player
	W       *tview.TextView
	Status  *tview.TextView
	ChatLog *tview.TextView
	Input   *GameInput
}

func NewGameView(g *Game) *GameView {
	gv := &GameView{
		Pages:   tview.NewPages(),
		G:       g,
		Popup:   tview.NewModal(),
		W:       tview.NewTextView(),
		Status:  tview.NewTextView(),
		ChatLog: tview.NewTextView(),
		Input:   NewGameInput(g),
	}

	// Configure base UI layout
	base := tview.NewGrid()
	base.SetBorders(true)

	base.AddItem(gv.W, 0, 0, 2, 2, 0, 0, false)

	/*
		statusBox := tview.NewFlex()
		statusBox.SetBorder(true)
		statusBox.SetTitle("Stats")
		statusBox.AddItem(gv.Status, 0, 1, false)
		base.AddItem(statusBox, 0, 2, 2, 1, 0, 0, false)
	*/
	base.AddItem(gv.Status, 0, 2, 2, 1, 0, 0, false)

	chatBox := tview.NewFlex()
	chatBox.SetDirection(tview.FlexRow)
	chatBox.SetTitle("Log")
	chatBox.AddItem(gv.ChatLog, 0, 1, false)
	chatBox.AddItem(gv.Input, 1, 1, true)
	/*
		chatBox.SetBorder(true)
	*/
	base.AddItem(chatBox, 2, 0, 1, 3, 0, 0, true)
	// base.AddItem(gv.ChatLog, 2, 0, 1, 3, 0, 0, false)

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
