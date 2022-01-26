package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type SessionState int

const (
	WelcomeState SessionState = iota
	LobbyState
	GameState
	QuitCheckState
	QuitState
)

type SessionFlag uint32

const (
	Authenticated SessionFlag = 1 << iota
	Administrator
)

type SshTty struct {
	io.ReadWriteCloser

	Config         IOScreenConfig
	ResizeCallback func()
	mu             sync.Mutex
}

func (*SshTty) Start() error {
	return nil
}

func (*SshTty) Stop() error {
	return nil
}

func (*SshTty) Drain() error {
	return nil
}

func (tty *SshTty) NotifyResize(cb func()) {
	tty.mu.Lock()
	defer tty.mu.Unlock()

	tty.ResizeCallback = cb
}

func (tty *SshTty) WindowSize() (width int, height int, err error) {
	tty.mu.Lock()
	defer tty.mu.Unlock()

	return tty.Config.Width, tty.Config.Height, nil
}

func (tty *SshTty) Resize(w int, h int) {
	tty.mu.Lock()
	tty.Config.Width = w
	tty.Config.Height = h

	cb := tty.ResizeCallback

	tty.mu.Unlock()

	if cb != nil {
		cb()
	}
}

type Session struct {
	Tty    *SshTty
	Screen tcell.Screen
	App    *tview.Application

	G  *Game
	GV *GameView

	State SessionState
	Flags SessionFlag
}

func (s *Session) IsAdministrator() bool {
	return (s.Flags & Administrator) != 0
}

func (s *Session) HasGame() bool {
	return s.G != nil
}

func (s *Session) Message(l MsgLevel, msg string) error {
	err := s.GV.Message(l, msg)
	s.Update()
	return err
}

func (s *Session) WindowResize(w, h int) {
	if s.Tty != nil {
		s.Tty.Resize(w, h)
	}
}

type EventUpdate struct {
	tcell.EventTime
}

func (s *Session) Update() error {
	s.App.Draw()
	return nil

	/*
		upd := &EventUpdate{}
		return s.Screen.PostEvent(upd)
	*/
}

func (s *Session) Move(direc uint16) {
	s.G.UserAction(s, Move, direc)
}

func (s *Session) Attack() {
	s.G.UserAction(s, Attack, 0)
}

func (s *Session) Defend() {
	s.G.UserAction(s, Defend, 0)
}

func (s *Session) ConsoleInput(txt string) {
	switch {
	case txt == "":
		/* nop */

	case txt[0] == '/':
		s.G.Command(s, txt)
	default:
		s.G.Input(MsgChat, s, txt)
	}
}

func (s *Session) Run() error {
	s.App = tview.NewApplication()
	s.App.SetScreen(s.Screen)

	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	s.G = NewGame([]*Session{s}, ctx)
	s.GV = NewGameView(s)

	go s.G.Loop()

	s.App.SetRoot(s.GV, true)
	// box := tview.NewBox().SetBorder(true).SetTitle("Hello, world!")
	// s.App.SetRoot(box, true)

	/*
		if err := s.Screen.Init(); err != nil {
			return err
		}

		defer s.Screen.Fini()

		return s.Loop()
	*/
	return s.App.Run()
}

func (s *Session) Loop() error {
	s.Screen.Clear()

	var savedState SessionState = WelcomeState

loop:
	for {
		ev := s.Screen.PollEvent()
		// fmt.Printf("event: %+v\n", ev)
		if kev, ok := ev.(*tcell.EventKey); ok {
			// check global key bindings...

			switch kev.Key() {
			case tcell.KeyCtrlQ:
				if s.State == QuitCheckState {
					s.State = QuitState
				} else if s.State != QuitState {
					savedState = s.State
					s.State = QuitCheckState
				}

			case tcell.KeyCtrlL:
				s.Screen.Sync()
				ev = nil
			}
		}

		_ = savedState

		switch s.State {
		case WelcomeState:
			s.welcomeLoop(ev)

		case LobbyState:
			s.lobbyLoop(ev)

		case GameState:
			s.gameLoop(ev)

		case QuitCheckState:
			// show dialog to confirm quit

		case QuitState:
			// show goodbye?
			break loop

		default:
			return errors.New(fmt.Sprintf("invalid state: %d", int(s.State)))
		}

	}

	fmt.Printf("done\n")

	return nil
}

func (s *Session) welcomeLoop(tcell.Event) {
}

func (s *Session) lobbyLoop(tcell.Event) {
}

func (s *Session) gameLoop(tcell.Event) {
}
