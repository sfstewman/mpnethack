package network

import (
	"io"
	"log"
	"sync"

	"github.com/sfstewman/mpnethack/tui"
)

type SshTty struct {
	io.ReadWriteCloser

	Config         tui.IOScreenConfig
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
