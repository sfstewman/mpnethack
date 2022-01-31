package main

import (
	"flag"
	"log"
)

func main() {
	var (
		hostKeyPath  string
		adminLogPath string
		err          error
	)

	flag.StringVar(&hostKeyPath, "hostkey", "", "Path to the host key")
	flag.StringVar(&adminLogPath, "adminlog", "admin.log", "Path to the admin log")
	flag.Parse()

	systemLog, err := NewSystemLog(adminLogPath, nil)
	if err != nil {
		log.Fatalf("error setting up system logs: %v", err)
		return
	}

	lobby := &Lobby{}

	if hostKeyPath != "" {
		go acceptNetworkLogins(hostKeyPath, lobby, systemLog)
	}

	session := NewSession("Asron the Limited", Authenticated|Administrator)

	lobby.AddSession(session)

	session.UI = setupUI(session, lobby, systemLog)

	if err := session.UI.Run(); err != nil {
		panic(err)
	}
}
