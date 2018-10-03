package main

import (
	"flag"
	"fmt"
	// "io"
	"io/ioutil"
	"log"
	"net"

	"golang.org/x/crypto/ssh"
	// "golang.org/x/crypto/ssh/terminal"
	// "github.com/gdamore/tcell/views"
)

func authLog(conn ssh.ConnMetadata, method string, err error) {
	fmt.Printf("login attempt[%s] %v : %v\n", method, conn, err)
}

func main() {
	var hostKeyPath string

	flag.StringVar(&hostKeyPath, "hostkey", "", "Path to the host key")
	flag.Parse()

	if hostKeyPath == "" {
		log.Fatal("must suppose a host key with -hostkey option")
	}

	cfg := &ssh.ServerConfig{
		NoClientAuth:    true,
		AuthLogCallback: authLog,
		BannerCallback: func(conn ssh.ConnMetadata) string {
			return "WELCOME to multiplayer nethack"
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

		go handleConnection(conn, cfg)
	}

	fmt.Println("vim-go")
}
