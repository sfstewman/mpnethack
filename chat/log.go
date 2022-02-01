package chat

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// Game message levels
type MsgLevel int

const (
	Debug MsgLevel = iota
	Info
	Chat
	Private
	Game
	Admin
	System
	// MsgWarn
	// MsgCrit
	// MsgAdmin
)

type Message struct {
	Level MsgLevel
	Time  time.Time
	Text  string
	Seq   uint
}

type Log struct {
	Lines    []Message
	LastLine int
	Seq      uint

	Callback func(Message)

	mu sync.RWMutex
}

func NewLog(numLines int) *Log {
	return &Log{
		Lines: make([]Message, 0, numLines),
	}
}

func (gl *Log) addLine(lvl MsgLevel, line string) Message {
	gl.mu.Lock()
	defer gl.mu.Unlock()

	if strings.HasSuffix(line, "\r\n") {
		line = line[:len(line)-2]
	} else if strings.HasSuffix(line, "\n") {
		line = line[:len(line)-1]
	}

	seq := gl.Seq
	gl.Seq++
	msg := Message{Level: lvl, Time: time.Now(), Text: line, Seq: seq}

	if len(gl.Lines) < cap(gl.Lines) {
		gl.Lines = append(gl.Lines, msg)
		gl.LastLine = len(gl.Lines) - 1
	} else {
		ind := gl.LastLine + 1
		if ind >= len(gl.Lines) {
			ind = 0
		}

		gl.Lines[ind] = msg
		gl.LastLine = ind
	}

	return msg
}

func (gl *Log) LogLine(lvl MsgLevel, line string) {
	msg := gl.addLine(lvl, line)

	if gl.Callback != nil {
		gl.Callback(msg)
	}
}

func (gl *Log) Log(lvl MsgLevel, format string, args ...interface{}) {
	entry := fmt.Sprintf(format, args...)
	lines := strings.Split(entry, "\n")
	for _, l := range lines {
		gl.LogLine(lvl, l)
	}
}

func (gl *Log) NumLines() int {
	gl.mu.RLock()
	defer gl.mu.RUnlock()

	return len(gl.Lines)
}

func (gl *Log) VisitLines(offset int, visitor func(Message) bool) bool {
	gl.mu.RLock()
	defer gl.mu.RUnlock()

	n := len(gl.Lines)
	ll := gl.LastLine

	if offset < 0 {
		offset += n
		if offset < 0 {
			offset = 0
		}
	}

	if offset >= n {
		return true
	}

	// (LastLine+1) % n -- first line in the buffer (offset == 0)
	//
	// want offset ... n lines

	for i := offset; i < n; i++ {
		ind := (ll + i + 1) % n

		if !visitor(gl.Lines[ind]) {
			return false
		}
	}

	return true
}
