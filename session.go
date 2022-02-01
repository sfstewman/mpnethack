package mpnethack

import (
	"errors"
	"fmt"
	"io"
	"log"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/sfstewman/mpnethack/chat"
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
	log.Printf("Start called")
	return nil
}

func (*SshTty) Stop() error {
	log.Printf("Stop called")
	return nil
}

func (*SshTty) Drain() error {
	log.Printf("Drain called")
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
	cb := (func() func() {
		tty.mu.Lock()
		defer tty.mu.Unlock()
		tty.Config.Width = w
		tty.Config.Height = h

		return tty.ResizeCallback
	})()

	log.Printf("RESIZE request w=%d, h=%d, cb=%v", w, h, cb)
	if cb != nil {
		cb()
	}
}

type Session struct {
	Tty    *SshTty
	Screen tcell.Screen

	User string

	UI *UI

	G      *Game
	Player *Player

	SessionLog *chat.Log

	State SessionState
	Flags SessionFlag
}

const SessionGameLogLines = 100

func NewSession(user string, flags SessionFlag) *Session {
	s := &Session{
		User:       user,
		SessionLog: chat.NewLog(SessionGameLogLines),
		Flags:      flags,
	}

	return s
}

func (s *Session) IsAdministrator() bool {
	return (s.Flags & Administrator) != 0
}

func (s *Session) HasGame() bool {
	return s.G != nil
}

func (s *Session) Message(lvl chat.MsgLevel, msg string) error {
	s.SessionLog.LogLine(lvl, msg)
	// err := s.GV.Message(l, msg)
	// s.Update()
	return nil // err
}

func (s *Session) WindowResize(w, h int) {
	log.Printf("Session[%p].WindowResize(w=%d,h=%d)  tty=%v", s, w, h, s.Tty)
	if s.Tty != nil {
		s.Tty.Resize(w, h)
	}
}

type EventUpdate struct {
	tcell.EventTime
}

func (s *Session) Update() error {
	s.UI.Update()
	return nil

	/*
		upd := &EventUpdate{}
		return s.Screen.PostEvent(upd)
	*/
}

func (s *Session) Quit() {
	s.UI.Quit()
}

func (s *Session) Move(direc MoveDirection) {
	s.G.UserAction(s, Move, int16(direc))
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
		s.G.Input(chat.Chat, s, txt)
	}
}

func (s *Session) Join(g *Game) error {
	if s.G != nil {
		return fmt.Errorf("game is nil")
	}

	s.G = g
	pl, err := g.PlayerJoin(s)
	if err != nil {
		return err
	}

	s.Player = pl

	return nil
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
