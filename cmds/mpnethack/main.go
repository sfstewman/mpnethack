package main

import (
	"flag"
	"log"

	"github.com/sfstewman/mpnethack"
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

	systemLog, err := mpnethack.NewSystemLog(adminLogPath, nil)
	if err != nil {
		log.Fatalf("error setting up system logs: %v", err)
		return
	}

	lobby := &mpnethack.Lobby{}

	session := mpnethack.NewSession("Asron the Limited", ConsoleFlags)
	lobby.AddSession(session)
	session.UI = mpnethack.SetupUI(session, lobby, systemLog)

	if hostKeyPath != "" {
		go mpnethack.AcceptNetworkLogins(hostKeyPath, lobby, systemLog)
	}

	if err := session.UI.Run(); err != nil {
		panic(err)
	}
}
