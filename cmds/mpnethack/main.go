package main

import (
	"flag"
	"log"

	"github.com/sfstewman/mpnethack"
	"github.com/sfstewman/mpnethack/chat"
	"github.com/sfstewman/mpnethack/network"
	"github.com/sfstewman/mpnethack/store"
	"github.com/sfstewman/mpnethack/tui"
	"github.com/sfstewman/mpnethack/user"
)

const ConsoleFlags = user.Authenticated | user.Administrator

func main() {
	var (
		hostKeyPath  string
		adminLogPath string
		err          error
	)
	var storePath string = "store.db"

	flag.StringVar(&hostKeyPath, "hostkey", "", "Path to the host key")
	flag.StringVar(&adminLogPath, "adminlog", "admin.log", "Path to the admin log")
	flag.Parse()

	db, err := store.Open(storePath)
	if err != nil {
		log.Fatalf("error opening store path \"%s\": %v", storePath, err)
	}

	if err := LoadBuiltinData(db); err != nil {
		log.Fatalf("error loading builtin data: %v", err)
	}

	systemLog, err := chat.NewSystemLog(adminLogPath, nil)
	if err != nil {
		log.Fatalf("error setting up system logs: %v", err)
		return
	}

	mpnethack.LookupItem = db.LookupItem

	lobby := &mpnethack.Lobby{}

	session := user.NewSession("Asron the Limited", ConsoleFlags)
	lobby.AddSession(session)
	session.UI = tui.SetupUI(session, lobby, systemLog)

	if hostKeyPath != "" {
		go network.AcceptNetworkLogins(hostKeyPath, lobby, systemLog)
	}

	if err := session.UI.Run(); err != nil {
		panic(err)
	}
}
