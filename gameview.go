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
	Sess *Session
	Mode InputMode
}

func (inp *GameInput) onKey(ev *tcell.EventKey) *tcell.EventKey {
	inp.mu.Lock()
	defer inp.mu.Unlock()

	if inp.Mode == InputChat {
		return ev
	}

	k := ev.Key()
	s := inp.Sess

	switch k {
	case tcell.KeyRune:
		r := ev.Rune()
		switch r {
		case '`':
			inp.Mode = InputChat

		case 'a':
			s.Move(MoveLeft)
		case 'd':
			s.Move(MoveRight)
		case 'w':
			s.Move(MoveUp)
		case 's':
			s.Move(MoveDown)

		case ' ':
			s.Attack()
		case 'v':
			s.Defend()

		case '1', '2', '3', '4':
		}

	case tcell.KeyLeft:
		s.Move(MoveLeft)
	case tcell.KeyRight:
		s.Move(MoveRight)
	case tcell.KeyUp:
		s.Move(MoveUp)
	case tcell.KeyDown:
		s.Move(MoveDown)
	}

	return nil
}

func (inp *GameInput) onDone(key tcell.Key) {
	inp.mu.Lock()
	defer inp.mu.Unlock()

	switch key {
	case tcell.KeyEnter:
		// XXX: handle message
		txt := inp.GetText()
		if txt != "" {
			log.Printf("[console] %s", txt)
			inp.SetText("")
			inp.Sess.ConsoleInput(txt)
			inp.Mode = InputDirect
		}

	case tcell.KeyEsc:
		inp.Mode = InputDirect
	}
}

func NewGameInput(sess *Session) *GameInput {
	inp := &GameInput{
		InputField: tview.NewInputField(),

		Sess: sess,
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

func NewGameView(s *Session) *GameView {
	gv := &GameView{
		Pages:   tview.NewPages(),
		G:       s.G,
		Popup:   tview.NewModal(),
		W:       tview.NewTextView(),
		Status:  tview.NewTextView(),
		ChatLog: tview.NewTextView(),
		Input:   NewGameInput(s),
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
