package network

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"

	"golang.org/x/crypto/ssh"

	"github.com/sfstewman/mpnethack"
	"github.com/sfstewman/mpnethack/chat"
	"github.com/sfstewman/mpnethack/tui"
	"github.com/sfstewman/mpnethack/user"
)

func authLog(conn ssh.ConnMetadata, method string, err error) {
	log.Printf("login attempt[%s] %v : %v\n", method, conn, err)
}

func AcceptNetworkLogins(hostKeyPath string, lobby *mpnethack.Lobby, systemLog *chat.SystemLog) {
	cfg := &ssh.ServerConfig{
		NoClientAuth:    true,
		AuthLogCallback: authLog,
		BannerCallback: func(conn ssh.ConnMetadata) string {
			return "WELCOME to multiplayer nethack\r\n"
		},
		ServerVersion: "SSH-2.0-mpnethack",
	}

	{
		hkData, err := ioutil.ReadFile(hostKeyPath)
		if err != nil {
			log.Fatalf("cannot read host key from '%s': %v", hostKeyPath, err)
		}

		hk, err := ssh.ParsePrivateKey(hkData)
		if err != nil {
			log.Fatalf("'%s' has an invalid host key: %v", hostKeyPath, err)
		}

		cfg.AddHostKey(hk)
	}

	ln, err := net.Listen("tcp", "localhost:5612")
	if err != nil {
		log.Fatalf("error listening for connections: %v", err)
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("error in accept: %v", err)
			continue
		}

		go handleConnection(conn, cfg, lobby, systemLog)
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

func channelRequests(sess *user.Session, in <-chan *ssh.Request, cfgCh chan<- tui.IOScreenConfig) {
	for req := range in {
		log.Printf("request '%s' reply=%v len(payload)=%d\n", req.Type, req.WantReply, len(req.Payload))
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

			log.Printf("pty request: %+v\n", pty)
			cfgCh <- tui.IOScreenConfig{
				Term:      pty.Term,
				Width:     int(pty.Width),
				Height:    int(pty.Height),
				TrueColor: false,
			}
			close(cfgCh)
			cfgCh = nil

		case "window-change":
			log.Printf("window change: %d bytes\n", len(req.Payload))
			wsz := WindowSize{}
			err := ssh.Unmarshal(req.Payload, &wsz)
			if err == nil {
				log.Printf("window dims: %d x %d (%dpx x %dpx)\n",
					wsz.Width, wsz.Height, wsz.WidthPix, wsz.HeightPix)

				sess.WindowResize(int(wsz.Width), int(wsz.Height))
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

func handleConnection(c net.Conn, cfg *ssh.ServerConfig, lobby *mpnethack.Lobby, systemLog *chat.SystemLog) {
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

		cfgCh := make(chan tui.IOScreenConfig)
		sess := user.NewSession("Grufmore the Dominable", user.Authenticated)

		fmt.Fprintf(channel, "\r\nConfiguring terminal\r\n")

		go channelRequests(sess, requests, cfgCh)
		cfg := <-cfgCh

		tty := &SshTty{
			Config:          cfg,
			ReadWriteCloser: channel,
		}

		fmt.Fprintf(channel, "\r\nConfiguring terminal: %+v\r\n\r\n", cfg)

		log.Printf("creating screen with config: %+v", cfg)

		scr, err := tui.NewIOScreenFromTty(tty, cfg)
		if err != nil {
			log.Printf("error creating screen: %v", err)
			return
		}

		if err := scr.Init(); err != nil {
			log.Printf("error initializing screen: %v", err)
			return
		}

		// !!! FIXME !!!
		lobby := &mpnethack.Lobby{}

		ui := tui.SetupUI(sess, lobby, systemLog)
		sess.UI = ui
		sess.Tty = tty
		ui.App.SetScreen(scr)

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

		if err := ui.App.Run(); err != nil {
			log.Printf("session [%s : %p] error: %v", sess.User, sess, err)
		}

		return
	}
}
