package main

import (
	"errors"
	"fmt"
	"log"
	"net"

	"github.com/gdamore/tcell"
	"github.com/rivo/tview"
	"golang.org/x/crypto/ssh"
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

type Session struct {
	Screen *IOScreen
	App    *tview.Application
	Game   *Game
	State  SessionState
	Flags  SessionFlag
}

func (s *Session) Resize(w, h uint32) {
	if s.Screen != nil {
		s.Screen.SizeChange(int(w), int(h))
	}
}

type EventUpdate struct {
	tcell.EventTime
}

func (s *Session) Update() error {
	upd := &EventUpdate{}
	return s.Screen.PostEvent(upd)
}

func (s *Session) Run() error {
	s.App = tview.NewApplication()
	s.App.SetScreen(s.Screen)
	box := tview.NewBox().SetBorder(true).SetTitle("Hello, world!")
	s.App.SetRoot(box, true)

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

func (s *Session) SizeChange(w, h int) {
	if s.Screen != nil {
		s.Screen.SizeChange(w, h)
	}
}

type WindowSize struct {
	Width, Height, WidthPix, HeightPix uint32
}

type PtyReq struct {
	Term string

	Width, Height, WidthPix, HeightPix uint32

	Modes []byte
}

func channelRequests(sess *Session, in <-chan *ssh.Request, cfgCh chan<- IOScreenConfig) {
	for req := range in {
		fmt.Printf("request '%s' reply=%v len(payload)=%d\n", req.Type, req.WantReply, len(req.Payload))
		switch req.Type {
		case "shell":
			req.Reply(true, nil)

		case "pty-req":
			if cfgCh == nil {
				req.Reply(false, nil)
				continue
			}

			req.Reply(true, nil)
			pty := PtyReq{}
			err := ssh.Unmarshal(req.Payload, &pty)
			if err != nil {
				log.Printf("error pty request: %v\n", err)
				continue
			}

			fmt.Printf("pty request: %+v\n", pty)
			cfgCh <- IOScreenConfig{
				Term:      pty.Term,
				Width:     int(pty.Width),
				Height:    int(pty.Height),
				TrueColor: false,
			}
			close(cfgCh)
			cfgCh = nil

		case "window-change":
			fmt.Printf("window change: %d bytes\n", len(req.Payload))
			wsz := WindowSize{}
			err := ssh.Unmarshal(req.Payload, &wsz)
			if err == nil {
				fmt.Printf("window dims: %d x %d (%dpx x %dpx)\n",
					wsz.Width, wsz.Height, wsz.WidthPix, wsz.HeightPix)

				sess.SizeChange(int(wsz.Width), int(wsz.Height))
				if req.WantReply {
					req.Reply(true, nil)
				}
			} else {
				log.Printf("error parsing window change: %v\n", err)
			}
		default:
			req.Reply(false, nil)
		}
	}
}

func handleConnection(c net.Conn, cfg *ssh.ServerConfig) {
	conn, chans, reqs, err := ssh.NewServerConn(c, cfg)
	if err != nil {
		log.Printf("failed to handshake: %v", err)
		return
	}

	defer conn.Close()

	go ssh.DiscardRequests(reqs)

	for chReq := range chans {
		if chReq.ChannelType() != "session" {
			chReq.Reject(ssh.UnknownChannelType, "unknown/unsupported channel type")
			continue
		}

		channel, requests, err := chReq.Accept()
		if err != nil {
			log.Printf("could not accept channel: %v", err)
			return
		}

		cfgCh := make(chan IOScreenConfig)
		sess := &Session{}

		go channelRequests(sess, requests, cfgCh)
		cfg := <-cfgCh

		sess.Screen, err = NewIOScreen(channel, cfg)
		if err != nil {
			log.Printf("error creating screen: %v", err)
			return
		}

		/*
			term := terminal.NewTerminal(channel, "> ")
			go func() {
				defer channel.Close()
				for {
					line, err := term.ReadLine()

					if err == io.EOF {
						fmt.Printf("connection closed\n")
						break
					}
					if err != nil {
						log.Printf("error reading line: %v", err)
						break
					}

					fmt.Fprintf(term, "received: %s\n", line)
					fmt.Println(line)
				}
			}()
		*/

		if err := sess.Run(); err != nil {
			log.Printf("session error: %v", err)
		}
		return
	}
}
