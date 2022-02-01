package uilib

import (
	"log"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/sfstewman/mpnethack/chat"
)

type InputMode int

const (
	InputGame InputMode = iota
	InputConsole
)

type InputArea struct {
	*tview.Flex

	Log   *LogView
	Input *tview.InputField

	InputMode InputMode

	// UI *UI

	DirectKeyFunc    func(e *tcell.EventKey) *tcell.EventKey
	ConsoleInputFunc func(string)

	LastKey    tcell.Key
	LastMods   tcell.ModMask
	LastRune   rune
	HasLastKey bool
}

func NewInputArea( /* ui *UI, */ gl *chat.Log) *InputArea {
	inp := &InputArea{
		Flex:      tview.NewFlex(),
		InputMode: InputGame,
		// UI:         ui,
		HasLastKey: false,
	}

	inp.Input = tview.NewInputField().
		SetLabel("~ ").
		SetFieldWidth(0).
		SetDoneFunc(inp.handleConsoleCmd)

	inp.Input.SetInputCapture(inp.handleInput)

	// inp.Log = NewLogView(ui.Session.SessionLog)
	inp.Log = NewLogView(gl)

	inp.SetDirection(tview.FlexRow).
		AddItem(inp.Log, 0, 1, false).
		AddItem(inp.Input, 1, 1, true)

	inp.SetBorderPadding(1, 1, 1, 1)

	// inp.SetInputCapture(inp.handleInput)
	inp.Input.SetInputCapture(inp.handleInput)

	return inp
}

func (inp *InputArea) Draw(scr tcell.Screen) {
	inp.Flex.Draw(scr)

	if inp.InputMode == InputGame {
		scr.HideCursor()
	}
}

func (inp *InputArea) handleConsoleCmd(key tcell.Key) {
	switch key {
	case tcell.KeyEnter:
		// XXX: handle message
		txt := inp.Input.GetText()
		if txt != "" {
			inp.Input.SetText("")
			inp.InputMode = InputGame

			if inp.ConsoleInputFunc != nil {
				inp.ConsoleInputFunc(txt)
			} else {
				log.Printf("[console] %s", txt)
				// inp.UI.Session.ConsoleInput(txt)
			}
		}

	case tcell.KeyEsc:
		inp.InputMode = InputGame
	}
}

func (inp *InputArea) handleInput(e *tcell.EventKey) *tcell.EventKey {
	inp.HasLastKey = true

	k := e.Key()
	m := e.Modifiers()
	r := e.Rune()

	switch inp.InputMode {
	case InputGame:
		if inp.DirectKeyFunc != nil {
			e = inp.DirectKeyFunc(e)

			if e == nil {
				return nil
			}

			k = e.Key()
			m = e.Modifiers()
			r = e.Rune()
		}

		/*
			if k == tcell.KeyEsc && m == tcell.ModNone {
				// bring up menu
				inp.UI.toggleModal(ModalMenu)
			}
		*/

		if k == tcell.KeyPgUp {
			inp.Log.Scroll(ScrollUp)
			return nil
		}

		if k == tcell.KeyPgDn {
			inp.Log.Scroll(ScrollDown)
			return nil
		}

		if k == tcell.KeyRune && m == tcell.ModNone && (r == '`' || r == '~' || r == '/') {
			inp.InputMode = InputConsole
			return nil
		}

	case InputConsole:
		if k == tcell.KeyEsc && m == tcell.ModNone {
			inp.InputMode = InputGame
		} else if k == tcell.KeyTab && m == tcell.ModNone {
			inp.InputMode = InputGame
		}
	}

	return e
}
