package uilib

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/sfstewman/mpnethack/chat"
)

type LogView struct {
	*tview.TextView

	Log *chat.Log

	Offset int

	VisibleFunc func() bool
	// mu sync.Mutex
}

const LogTimeLayout = "2006/01/02 03:04:05 MST"

func NewLogViewWithLines(numLines int) *LogView {
	return NewLogView(chat.NewLog(numLines))
}

func NewLogView(gl *chat.Log) *LogView {
	txtView := tview.NewTextView().SetDynamicColors(true)

	v := &LogView{
		TextView: txtView,
		Log:      gl,
		Offset:   -1,
	}

	return v
}

type ScrollDirec int

const (
	ScrollUp ScrollDirec = iota
	ScrollDown
)

func (v *LogView) Scroll(direc ScrollDirec) {
	_, _, _, h := v.GetInnerRect()

	delta := h / 2

	if v.Offset < 0 {
		if direc == ScrollDown {
			return
		}

		delta += h
	}

	if direc == ScrollUp {
		delta = -delta
	}

	v.ScrollBy(delta)
}

func (v *LogView) ScrollBy(deltaLines int) {
	n := v.Log.NumLines()

	if v.Offset < 0 {
		v.Offset = n + deltaLines
	} else {
		v.Offset += deltaLines
	}

	if v.Offset < 0 {
		v.Offset = 0
	} else if v.Offset >= n {
		v.Offset = -1
	}
}

func (v *LogView) redrawLog() {
	wr := v.TextView.BatchWriter()
	defer wr.Close()

	wr.Clear()
	if v.Log == nil {
		return
	}

	numLines := v.Log.NumLines()

	_, _, _, h := v.GetInnerRect()

	/*
		off := numLines - h - 1
		if off < 0 {
			off = 0
		}
	*/

	count := 0
	first := true
	var minSeq, maxSeq uint
	var minCnt, maxCnt int
	v.Log.VisitLines(0, func(msg chat.Message) bool {
		if first || msg.Seq < minSeq {
			minSeq = msg.Seq
			minCnt = count
		}

		if first || maxSeq > msg.Seq {
			maxSeq = msg.Seq
			maxCnt = count
		}
		count++

		return true
	})

	off := v.Offset
	if off < 0 {
		off = -(h - 1)
	}

	lineCount := 0
	v.Log.VisitLines(off, func(msg chat.Message) bool {
		s := v.formatMessage(msg)
		fmt.Fprint(wr, s)
		lineCount++
		return lineCount < h-1
		// return true
	})

	fmt.Fprintf(wr, "<-- numLines=%d, off=%d, count=%d, minSeq=%d, maxSeq=%d -->",
		numLines, off, count, minSeq, maxSeq)
}

func (v *LogView) Draw(scr tcell.Screen) {
	v.redrawLog()
	v.TextView.Draw(scr)
}

func (v *LogView) formatMessage(msg chat.Message) string {
	var btag, etag, sfx string

	etag = "[-:-:-]"
	switch msg.Level {
	case chat.Debug:
		btag = "[gray:black]"
	case chat.Info:
		btag = ""
		etag = ""
	case chat.Chat:
		btag = "[blue:black]"
	case chat.Private:
		btag = "[pink:black]"
	case chat.Game:
		btag = "[green:black]"
	case chat.Admin:
		btag = "[red:black:b]"
	case chat.System:
		btag = "[yellow:black:b]"
	}

	timeStr := msg.Time.Format(LogTimeLayout)

	line := msg.Text
	if !strings.HasSuffix(line, "\n") && !strings.HasSuffix(line, "\r\n") {
		sfx = "\n"
	}

	return fmt.Sprintf("%s%s [%d] %s%s%s", btag, timeStr, msg.Seq, tview.Escape(line), etag, sfx)
}

func (v *LogView) AddLine(lvl chat.MsgLevel, line string) {
	v.Log.LogLine(lvl, line)
}
