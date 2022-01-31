package util

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// Game message levels
type MsgLevel int

const (
	MsgDebug MsgLevel = iota
	MsgInfo
	MsgChat
	MsgPrivate
	MsgGame
	MsgAdmin
	MsgSystem
	// MsgWarn
	// MsgCrit
	// MsgAdmin
)

type LogMessage struct {
	Level MsgLevel
	Time  time.Time
	Text  string
	Seq   uint
}

type GameLog struct {
	Lines    []LogMessage
	LastLine int
	Seq      uint

	Callback func(LogMessage)

	mu sync.RWMutex
}

func NewGameLog(numLines int) *GameLog {
	return &GameLog{
		Lines: make([]LogMessage, 0, numLines),
	}
}

func (gl *GameLog) addLine(lvl MsgLevel, line string) LogMessage {
	gl.mu.Lock()
	defer gl.mu.Unlock()

	if strings.HasSuffix(line, "\r\n") {
		line = line[:len(line)-2]
	} else if strings.HasSuffix(line, "\n") {
		line = line[:len(line)-1]
	}

	seq := gl.Seq
	gl.Seq++
	msg := LogMessage{Level: lvl, Time: time.Now(), Text: line, Seq: seq}

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

func (gl *GameLog) LogLine(lvl MsgLevel, line string) {
	msg := gl.addLine(lvl, line)

	if gl.Callback != nil {
		gl.Callback(msg)
	}
}

func (gl *GameLog) Log(lvl MsgLevel, format string, args ...interface{}) {
	entry := fmt.Sprintf(format, args...)
	lines := strings.Split(entry, "\n")
	for _, l := range lines {
		gl.LogLine(lvl, l)
	}
}

func (gl *GameLog) NumLines() int {
	gl.mu.RLock()
	defer gl.mu.RUnlock()

	return len(gl.Lines)
}

func (gl *GameLog) VisitLines(offset int, visitor func(LogMessage) bool) bool {
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
