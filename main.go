package main

import (
	"flag"
	"log"

	"github.com/gdamore/tcell/v2"
	// "io"
	// "golang.org/x/crypto/ssh/terminal"
	// "github.com/gdamore/tcell/views"
)

func main1() {
	var hostKeyPath string

	flag.StringVar(&hostKeyPath, "hostkey", "", "Path to the host key")
	flag.Parse()

	if hostKeyPath != "" {
		go acceptNetworkLogins(hostKeyPath)
	}

	var err error

	scr, err := tcell.NewScreen()
	if err != nil {
		log.Printf("error creating screen: %v", err)
		return
	}

	sess := &Session{
		Screen: scr,
	}

	defStyle := tcell.StyleDefault.Background(tcell.ColorReset).Foreground(tcell.ColorReset)
	// boxStyle := tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorPurple)

	scr.SetStyle(defStyle)
	// s.EnableMouse()
	// s.EnablePaste()
	scr.Clear()

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

func main() {
	systemLog, err := NewSystemLog("admin.log", nil)
	if err != nil {
		log.Fatalf("error setting up system logs: %v", err)
		return
	}

	session := &Session{
		Flags: Authenticated | Administrator,
	}

	app := setupUI(session, systemLog)

	if err := app.Run(); err != nil {
		panic(err)
	}
}
