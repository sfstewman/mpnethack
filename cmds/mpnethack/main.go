package main

import (
	"flag"
	"log"

	"github.com/sfstewman/mpnethack"
	"github.com/sfstewman/mpnethack/chat"
	"github.com/sfstewman/mpnethack/game"
	"github.com/sfstewman/mpnethack/tui"
)

const ConsoleFlags = mpnethack.Authenticated | mpnethack.Administrator

func main() {
	var (
		hostKeyPath  string
		adminLogPath string
		err          error
	)

	flag.StringVar(&hostKeyPath, "hostkey", "", "Path to the host key")
	flag.StringVar(&adminLogPath, "adminlog", "admin.log", "Path to the admin log")
	flag.Parse()

	systemLog, err := chat.NewSystemLog(adminLogPath, nil)
	if err != nil {
		log.Fatalf("error setting up system logs: %v", err)
		return
	}

	lobby := &game.Lobby{}

	session := mpnethack.NewSession("Asron the Limited", ConsoleFlags)
	lobby.AddSession(session)
	session.UI = tui.SetupUI(session, lobby, systemLog)

	if hostKeyPath != "" {
		go mpnethack.AcceptNetworkLogins(hostKeyPath, lobby, systemLog)
	}

	if err := session.UI.Run(); err != nil {
		panic(err)
	}
}
