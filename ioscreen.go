// Copyright 2021 The TCell Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use file except in compliance with the License.
// You may obtain a copy of the license at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"errors"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/term"
	"golang.org/x/text/transform"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/gdamore/tcell/v2/terminfo"

	// import the stock terminals
	_ "github.com/gdamore/tcell/v2/terminfo/base"
)

type IOScreenConfig struct {
	Term      string
	Width     int
	Height    int
	TrueColor bool
}

// NewIOScreen returns a Screen that uses the terminfo description given by the
// term parameter over an io.ReaderWriter It returns an error if the terminal is
// not supported for any reason.
//
//
// NewTerminfoScreen returns a Screen that uses the stock TTY interface
// and POSIX terminal control, combined with a terminfo description taken from
// the $TERM environment variable.  It returns an error if the terminal
// is not supported for any reason.
//
// For terminals that do not support dynamic resize events, the $LINES
// $COLUMNS environment variables can be set to the actual window size,
// otherwise defaults taken from the terminal database are used.
func NewIOScreen() (tcell.Screen, error) {
	term := os.Getenv("TERM")

	ti, e := terminfo.LookupTerminfo(term)
	if e != nil {
		return nil, e
	}

	cfg := IOScreenConfig{
		Term:      term,
		Width:     ti.Columns,
		Height:    ti.Lines,
		TrueColor: false,
	}

	if lines, _ := strconv.Atoi(os.Getenv("LINES")); lines != 0 {
		cfg.Height = lines
	}

	if cols, _ := strconv.Atoi(os.Getenv("COLUMNS")); cols != 0 {
		cfg.Width = cols
	}

	if ti.SetFgBgRGB != "" || ti.SetFgRGB != "" || ti.SetBgRGB != "" {
		cfg.TrueColor = true
	}

	if os.Getenv("TCELL_TRUECOLOR") == "disable" {
		cfg.TrueColor = false
	}

	return NewIOScreenFromTty(nil, cfg)
}

// NewTerminfoScreenFromTty returns a Screen using a custom Tty implementation.
// If the passed in tty is nil, then a reasonable default (typically /dev/tty)
// is presumed, at least on UNIX hosts. (Windows hosts will typically fail this
// call altogether.)
func NewIOScreenFromTty(tty tcell.Tty, cfg IOScreenConfig) (*IOScreen, error) {
	/*
		ti, e := terminfo.LookupTerminfo(os.Getenv("TERM"))
		if e != nil {
			ti, e = loadDynamicTerminfo(os.Getenv("TERM"))
			if e != nil {
				return nil, e
			}
			terminfo.AddTerminfo(ti)
		}
	*/
	ti, e := terminfo.LookupTerminfo(cfg.Term)
	if e != nil {
		return nil, e
	}

	t := &IOScreen{ti: ti, tty: tty}
	t.w = cfg.Width
	t.h = cfg.Height
	t.truecolor = cfg.TrueColor

	t.keyexist = make(map[tcell.Key]bool)
	t.keycodes = make(map[string]*tKeyCode)
	if len(ti.Mouse) > 0 {
		t.mouse = []byte(ti.Mouse)
	}
	t.prepareKeys()
	t.buildAcsMap()
	t.resizeQ = make(chan bool, 1)
	t.fallback = make(map[rune]string)
	for k, v := range tcell.RuneFallbacks {
		t.fallback[k] = v
	}

	return t, nil
}

// tKeyCode represents a combination of a key code and modifiers.
type tKeyCode struct {
	key tcell.Key
	mod tcell.ModMask
}

// IOScreen represents a screen backed by a terminfo implementation.
type IOScreen struct {
	r            io.ReadWriter
	ti           *terminfo.Terminfo
	tty          tcell.Tty
	h            int
	w            int
	fini         bool
	cells        tcell.CellBuffer
	buffering    bool // true if we are collecting writes to buf instead of sending directly to out
	buf          bytes.Buffer
	curstyle     tcell.Style
	style        tcell.Style
	evch         chan tcell.Event
	resizeQ      chan bool
	quit         chan struct{}
	keyexist     map[tcell.Key]bool
	keycodes     map[string]*tKeyCode
	keychan      chan []byte
	keytimer     *time.Timer
	keyexpire    time.Time
	cx           int
	cy           int
	mouse        []byte
	clear        bool
	cursorx      int
	cursory      int
	wasbtn       bool
	acs          map[rune]string
	charset      string
	encoder      transform.Transformer
	decoder      transform.Transformer
	fallback     map[rune]string
	colors       map[tcell.Color]tcell.Color
	palette      []tcell.Color
	truecolor    bool
	escaped      bool
	buttondn     bool
	finiOnce     sync.Once
	enablePaste  string
	disablePaste string
	saved        *term.State
	stopQ        chan struct{}
	running      bool
	wg           sync.WaitGroup
	mouseFlags   tcell.MouseFlags
	pasteEnabled bool

	sync.Mutex
}

func (t *IOScreen) initialize() error {
	var err error
	if t.tty == nil {
		t.tty, err = tcell.NewDevTty()
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *IOScreen) Init() error {
	if e := t.initialize(); e != nil {
		return e
	}

	t.evch = make(chan tcell.Event, 10)
	t.keychan = make(chan []byte, 10)
	t.keytimer = time.NewTimer(time.Millisecond * 50)
	t.charset = "UTF-8"

	// t.charset = getCharset()
	if enc := tcell.GetEncoding(t.charset); enc != nil {
		t.encoder = enc.NewEncoder()
		t.decoder = enc.NewDecoder()
	} else {
		return tcell.ErrNoCharset
	}

	t.colors = make(map[tcell.Color]tcell.Color)
	t.palette = make([]tcell.Color, t.nColors())
	for i := 0; i < t.nColors(); i++ {
		t.palette[i] = tcell.Color(i) | tcell.ColorValid
		// identity map for our builtin colors
		t.colors[tcell.Color(i)|tcell.ColorValid] = tcell.Color(i) | tcell.ColorValid
	}

	t.quit = make(chan struct{})

	t.Lock()
	t.cx = -1
	t.cy = -1
	t.style = tcell.StyleDefault
	t.cells.Resize(t.w, t.h)
	t.cursorx = -1
	t.cursory = -1
	t.resize()
	t.Unlock()

	if err := t.engage(); err != nil {
		return err
	}

	return nil
}

func (t *IOScreen) prepareKeyMod(key tcell.Key, mod tcell.ModMask, val string) {
	if val != "" {
		// Do not override codes that already exist
		if _, exist := t.keycodes[val]; !exist {
			t.keyexist[key] = true
			t.keycodes[val] = &tKeyCode{key: key, mod: mod}
		}
	}
}

func (t *IOScreen) prepareKeyModReplace(key tcell.Key, replace tcell.Key, mod tcell.ModMask, val string) {
	if val != "" {
		// Do not override codes that already exist
		if old, exist := t.keycodes[val]; !exist || old.key == replace {
			t.keyexist[key] = true
			t.keycodes[val] = &tKeyCode{key: key, mod: mod}
		}
	}
}

func (t *IOScreen) prepareKeyModXTerm(key tcell.Key, val string) {

	if strings.HasPrefix(val, "\x1b[") && strings.HasSuffix(val, "~") {

		// Drop the trailing ~
		val = val[:len(val)-1]

		// These suffixes are calculated assuming Xterm style modifier suffixes.
		// Please see https://invisible-island.net/xterm/ctlseqs/ctlseqs.pdf for
		// more information (specifically "PC-Style Function Keys").
		t.prepareKeyModReplace(key, key+12, tcell.ModShift, val+";2~")
		t.prepareKeyModReplace(key, key+48, tcell.ModAlt, val+";3~")
		t.prepareKeyModReplace(key, key+60, tcell.ModAlt|tcell.ModShift, val+";4~")
		t.prepareKeyModReplace(key, key+24, tcell.ModCtrl, val+";5~")
		t.prepareKeyModReplace(key, key+36, tcell.ModCtrl|tcell.ModShift, val+";6~")
		t.prepareKeyMod(key, tcell.ModAlt|tcell.ModCtrl, val+";7~")
		t.prepareKeyMod(key, tcell.ModShift|tcell.ModAlt|tcell.ModCtrl, val+";8~")
		t.prepareKeyMod(key, tcell.ModMeta, val+";9~")
		t.prepareKeyMod(key, tcell.ModMeta|tcell.ModShift, val+";10~")
		t.prepareKeyMod(key, tcell.ModMeta|tcell.ModAlt, val+";11~")
		t.prepareKeyMod(key, tcell.ModMeta|tcell.ModAlt|tcell.ModShift, val+";12~")
		t.prepareKeyMod(key, tcell.ModMeta|tcell.ModCtrl, val+";13~")
		t.prepareKeyMod(key, tcell.ModMeta|tcell.ModCtrl|tcell.ModShift, val+";14~")
		t.prepareKeyMod(key, tcell.ModMeta|tcell.ModCtrl|tcell.ModAlt, val+";15~")
		t.prepareKeyMod(key, tcell.ModMeta|tcell.ModCtrl|tcell.ModAlt|tcell.ModShift, val+";16~")
	} else if strings.HasPrefix(val, "\x1bO") && len(val) == 3 {
		val = val[2:]
		t.prepareKeyModReplace(key, key+12, tcell.ModShift, "\x1b[1;2"+val)
		t.prepareKeyModReplace(key, key+48, tcell.ModAlt, "\x1b[1;3"+val)
		t.prepareKeyModReplace(key, key+24, tcell.ModCtrl, "\x1b[1;5"+val)
		t.prepareKeyModReplace(key, key+36, tcell.ModCtrl|tcell.ModShift, "\x1b[1;6"+val)
		t.prepareKeyModReplace(key, key+60, tcell.ModAlt|tcell.ModShift, "\x1b[1;4"+val)
		t.prepareKeyMod(key, tcell.ModAlt|tcell.ModCtrl, "\x1b[1;7"+val)
		t.prepareKeyMod(key, tcell.ModShift|tcell.ModAlt|tcell.ModCtrl, "\x1b[1;8"+val)
		t.prepareKeyMod(key, tcell.ModMeta, "\x1b[1;9"+val)
		t.prepareKeyMod(key, tcell.ModMeta|tcell.ModShift, "\x1b[1;10"+val)
		t.prepareKeyMod(key, tcell.ModMeta|tcell.ModAlt, "\x1b[1;11"+val)
		t.prepareKeyMod(key, tcell.ModMeta|tcell.ModAlt|tcell.ModShift, "\x1b[1;12"+val)
		t.prepareKeyMod(key, tcell.ModMeta|tcell.ModCtrl, "\x1b[1;13"+val)
		t.prepareKeyMod(key, tcell.ModMeta|tcell.ModCtrl|tcell.ModShift, "\x1b[1;14"+val)
		t.prepareKeyMod(key, tcell.ModMeta|tcell.ModCtrl|tcell.ModAlt, "\x1b[1;15"+val)
		t.prepareKeyMod(key, tcell.ModMeta|tcell.ModCtrl|tcell.ModAlt|tcell.ModShift, "\x1b[1;16"+val)
	}
}

func (t *IOScreen) prepareXtermModifiers() {
	if t.ti.Modifiers != terminfo.ModifiersXTerm {
		return
	}
	t.prepareKeyModXTerm(tcell.KeyRight, t.ti.KeyRight)
	t.prepareKeyModXTerm(tcell.KeyLeft, t.ti.KeyLeft)
	t.prepareKeyModXTerm(tcell.KeyUp, t.ti.KeyUp)
	t.prepareKeyModXTerm(tcell.KeyDown, t.ti.KeyDown)
	t.prepareKeyModXTerm(tcell.KeyInsert, t.ti.KeyInsert)
	t.prepareKeyModXTerm(tcell.KeyDelete, t.ti.KeyDelete)
	t.prepareKeyModXTerm(tcell.KeyPgUp, t.ti.KeyPgUp)
	t.prepareKeyModXTerm(tcell.KeyPgDn, t.ti.KeyPgDn)
	t.prepareKeyModXTerm(tcell.KeyHome, t.ti.KeyHome)
	t.prepareKeyModXTerm(tcell.KeyEnd, t.ti.KeyEnd)
	t.prepareKeyModXTerm(tcell.KeyF1, t.ti.KeyF1)
	t.prepareKeyModXTerm(tcell.KeyF2, t.ti.KeyF2)
	t.prepareKeyModXTerm(tcell.KeyF3, t.ti.KeyF3)
	t.prepareKeyModXTerm(tcell.KeyF4, t.ti.KeyF4)
	t.prepareKeyModXTerm(tcell.KeyF5, t.ti.KeyF5)
	t.prepareKeyModXTerm(tcell.KeyF6, t.ti.KeyF6)
	t.prepareKeyModXTerm(tcell.KeyF7, t.ti.KeyF7)
	t.prepareKeyModXTerm(tcell.KeyF8, t.ti.KeyF8)
	t.prepareKeyModXTerm(tcell.KeyF9, t.ti.KeyF9)
	t.prepareKeyModXTerm(tcell.KeyF10, t.ti.KeyF10)
	t.prepareKeyModXTerm(tcell.KeyF11, t.ti.KeyF11)
	t.prepareKeyModXTerm(tcell.KeyF12, t.ti.KeyF12)
}

// HACK
const (
	// These key codes are used internally, and will never appear to applications.
	keyPasteStart tcell.Key = iota + 16384
	keyPasteEnd
)

func (t *IOScreen) prepareBracketedPaste() {
	// Another workaround for lack of reporting in terminfo.
	// We assume if the terminal has a mouse entry, that it
	// offers bracketed paste.  But we allow specific overrides
	// via our terminal database.
	if t.ti.EnablePaste != "" {
		t.enablePaste = t.ti.EnablePaste
		t.disablePaste = t.ti.DisablePaste
		t.prepareKey(keyPasteStart, t.ti.PasteStart)
		t.prepareKey(keyPasteEnd, t.ti.PasteEnd)
	} else if t.ti.Mouse != "" {
		t.enablePaste = "\x1b[?2004h"
		t.disablePaste = "\x1b[?2004l"
		t.prepareKey(keyPasteStart, "\x1b[200~")
		t.prepareKey(keyPasteEnd, "\x1b[201~")
	}
}

func (t *IOScreen) prepareKey(key tcell.Key, val string) {
	t.prepareKeyMod(key, tcell.ModNone, val)
}

func (t *IOScreen) prepareKeys() {
	ti := t.ti
	t.prepareKey(tcell.KeyBackspace, ti.KeyBackspace)
	t.prepareKey(tcell.KeyF1, ti.KeyF1)
	t.prepareKey(tcell.KeyF2, ti.KeyF2)
	t.prepareKey(tcell.KeyF3, ti.KeyF3)
	t.prepareKey(tcell.KeyF4, ti.KeyF4)
	t.prepareKey(tcell.KeyF5, ti.KeyF5)
	t.prepareKey(tcell.KeyF6, ti.KeyF6)
	t.prepareKey(tcell.KeyF7, ti.KeyF7)
	t.prepareKey(tcell.KeyF8, ti.KeyF8)
	t.prepareKey(tcell.KeyF9, ti.KeyF9)
	t.prepareKey(tcell.KeyF10, ti.KeyF10)
	t.prepareKey(tcell.KeyF11, ti.KeyF11)
	t.prepareKey(tcell.KeyF12, ti.KeyF12)
	t.prepareKey(tcell.KeyF13, ti.KeyF13)
	t.prepareKey(tcell.KeyF14, ti.KeyF14)
	t.prepareKey(tcell.KeyF15, ti.KeyF15)
	t.prepareKey(tcell.KeyF16, ti.KeyF16)
	t.prepareKey(tcell.KeyF17, ti.KeyF17)
	t.prepareKey(tcell.KeyF18, ti.KeyF18)
	t.prepareKey(tcell.KeyF19, ti.KeyF19)
	t.prepareKey(tcell.KeyF20, ti.KeyF20)
	t.prepareKey(tcell.KeyF21, ti.KeyF21)
	t.prepareKey(tcell.KeyF22, ti.KeyF22)
	t.prepareKey(tcell.KeyF23, ti.KeyF23)
	t.prepareKey(tcell.KeyF24, ti.KeyF24)
	t.prepareKey(tcell.KeyF25, ti.KeyF25)
	t.prepareKey(tcell.KeyF26, ti.KeyF26)
	t.prepareKey(tcell.KeyF27, ti.KeyF27)
	t.prepareKey(tcell.KeyF28, ti.KeyF28)
	t.prepareKey(tcell.KeyF29, ti.KeyF29)
	t.prepareKey(tcell.KeyF30, ti.KeyF30)
	t.prepareKey(tcell.KeyF31, ti.KeyF31)
	t.prepareKey(tcell.KeyF32, ti.KeyF32)
	t.prepareKey(tcell.KeyF33, ti.KeyF33)
	t.prepareKey(tcell.KeyF34, ti.KeyF34)
	t.prepareKey(tcell.KeyF35, ti.KeyF35)
	t.prepareKey(tcell.KeyF36, ti.KeyF36)
	t.prepareKey(tcell.KeyF37, ti.KeyF37)
	t.prepareKey(tcell.KeyF38, ti.KeyF38)
	t.prepareKey(tcell.KeyF39, ti.KeyF39)
	t.prepareKey(tcell.KeyF40, ti.KeyF40)
	t.prepareKey(tcell.KeyF41, ti.KeyF41)
	t.prepareKey(tcell.KeyF42, ti.KeyF42)
	t.prepareKey(tcell.KeyF43, ti.KeyF43)
	t.prepareKey(tcell.KeyF44, ti.KeyF44)
	t.prepareKey(tcell.KeyF45, ti.KeyF45)
	t.prepareKey(tcell.KeyF46, ti.KeyF46)
	t.prepareKey(tcell.KeyF47, ti.KeyF47)
	t.prepareKey(tcell.KeyF48, ti.KeyF48)
	t.prepareKey(tcell.KeyF49, ti.KeyF49)
	t.prepareKey(tcell.KeyF50, ti.KeyF50)
	t.prepareKey(tcell.KeyF51, ti.KeyF51)
	t.prepareKey(tcell.KeyF52, ti.KeyF52)
	t.prepareKey(tcell.KeyF53, ti.KeyF53)
	t.prepareKey(tcell.KeyF54, ti.KeyF54)
	t.prepareKey(tcell.KeyF55, ti.KeyF55)
	t.prepareKey(tcell.KeyF56, ti.KeyF56)
	t.prepareKey(tcell.KeyF57, ti.KeyF57)
	t.prepareKey(tcell.KeyF58, ti.KeyF58)
	t.prepareKey(tcell.KeyF59, ti.KeyF59)
	t.prepareKey(tcell.KeyF60, ti.KeyF60)
	t.prepareKey(tcell.KeyF61, ti.KeyF61)
	t.prepareKey(tcell.KeyF62, ti.KeyF62)
	t.prepareKey(tcell.KeyF63, ti.KeyF63)
	t.prepareKey(tcell.KeyF64, ti.KeyF64)
	t.prepareKey(tcell.KeyInsert, ti.KeyInsert)
	t.prepareKey(tcell.KeyDelete, ti.KeyDelete)
	t.prepareKey(tcell.KeyHome, ti.KeyHome)
	t.prepareKey(tcell.KeyEnd, ti.KeyEnd)
	t.prepareKey(tcell.KeyUp, ti.KeyUp)
	t.prepareKey(tcell.KeyDown, ti.KeyDown)
	t.prepareKey(tcell.KeyLeft, ti.KeyLeft)
	t.prepareKey(tcell.KeyRight, ti.KeyRight)
	t.prepareKey(tcell.KeyPgUp, ti.KeyPgUp)
	t.prepareKey(tcell.KeyPgDn, ti.KeyPgDn)
	t.prepareKey(tcell.KeyHelp, ti.KeyHelp)
	t.prepareKey(tcell.KeyPrint, ti.KeyPrint)
	t.prepareKey(tcell.KeyCancel, ti.KeyCancel)
	t.prepareKey(tcell.KeyExit, ti.KeyExit)
	t.prepareKey(tcell.KeyBacktab, ti.KeyBacktab)

	t.prepareKeyMod(tcell.KeyRight, tcell.ModShift, ti.KeyShfRight)
	t.prepareKeyMod(tcell.KeyLeft, tcell.ModShift, ti.KeyShfLeft)
	t.prepareKeyMod(tcell.KeyUp, tcell.ModShift, ti.KeyShfUp)
	t.prepareKeyMod(tcell.KeyDown, tcell.ModShift, ti.KeyShfDown)
	t.prepareKeyMod(tcell.KeyHome, tcell.ModShift, ti.KeyShfHome)
	t.prepareKeyMod(tcell.KeyEnd, tcell.ModShift, ti.KeyShfEnd)
	t.prepareKeyMod(tcell.KeyPgUp, tcell.ModShift, ti.KeyShfPgUp)
	t.prepareKeyMod(tcell.KeyPgDn, tcell.ModShift, ti.KeyShfPgDn)

	t.prepareKeyMod(tcell.KeyRight, tcell.ModCtrl, ti.KeyCtrlRight)
	t.prepareKeyMod(tcell.KeyLeft, tcell.ModCtrl, ti.KeyCtrlLeft)
	t.prepareKeyMod(tcell.KeyUp, tcell.ModCtrl, ti.KeyCtrlUp)
	t.prepareKeyMod(tcell.KeyDown, tcell.ModCtrl, ti.KeyCtrlDown)
	t.prepareKeyMod(tcell.KeyHome, tcell.ModCtrl, ti.KeyCtrlHome)
	t.prepareKeyMod(tcell.KeyEnd, tcell.ModCtrl, ti.KeyCtrlEnd)

	// Sadly, xterm handling of keycodes is somewhat erratic.  In
	// particular, different codes are sent depending on application
	// mode is in use or not, and the entries for many of these are
	// simply absent from terminfo on many systems.  So we insert
	// a number of escape sequences if they are not already used, in
	// order to have the widest correct usage.  Note that prepareKey
	// will not inject codes if the escape sequence is already known.
	// We also only do this for terminals that have the application
	// mode present.

	// Cursor mode
	if ti.EnterKeypad != "" {
		t.prepareKey(tcell.KeyUp, "\x1b[A")
		t.prepareKey(tcell.KeyDown, "\x1b[B")
		t.prepareKey(tcell.KeyRight, "\x1b[C")
		t.prepareKey(tcell.KeyLeft, "\x1b[D")
		t.prepareKey(tcell.KeyEnd, "\x1b[F")
		t.prepareKey(tcell.KeyHome, "\x1b[H")
		t.prepareKey(tcell.KeyDelete, "\x1b[3~")
		t.prepareKey(tcell.KeyHome, "\x1b[1~")
		t.prepareKey(tcell.KeyEnd, "\x1b[4~")
		t.prepareKey(tcell.KeyPgUp, "\x1b[5~")
		t.prepareKey(tcell.KeyPgDn, "\x1b[6~")

		// Application mode
		t.prepareKey(tcell.KeyUp, "\x1bOA")
		t.prepareKey(tcell.KeyDown, "\x1bOB")
		t.prepareKey(tcell.KeyRight, "\x1bOC")
		t.prepareKey(tcell.KeyLeft, "\x1bOD")
		t.prepareKey(tcell.KeyHome, "\x1bOH")
	}

	t.prepareKey(keyPasteStart, ti.PasteStart)
	t.prepareKey(keyPasteEnd, ti.PasteEnd)
	t.prepareXtermModifiers()
	t.prepareBracketedPaste()

outer:
	// Add key mappings for control keys.
	for i := 0; i < ' '; i++ {
		// Do not insert direct key codes for ambiguous keys.
		// For example, ESC is used for lots of other keys, so
		// when parsing this we don't want to fast path handling
		// of it, but instead wait a bit before parsing it as in
		// isolation.
		for esc := range t.keycodes {
			if []byte(esc)[0] == byte(i) {
				continue outer
			}
		}

		t.keyexist[tcell.Key(i)] = true

		mod := tcell.ModCtrl
		switch tcell.Key(i) {
		case tcell.KeyBS, tcell.KeyTAB, tcell.KeyESC, tcell.KeyCR:
			// directly type-able- no control sequence
			mod = tcell.ModNone
		}
		t.keycodes[string(rune(i))] = &tKeyCode{key: tcell.Key(i), mod: mod}
	}
}

func (t *IOScreen) Fini() {
	t.finiOnce.Do(t.finish)
}

func (t *IOScreen) finish() {
	close(t.quit)
	t.finalize()
}

func (t *IOScreen) SetStyle(style tcell.Style) {
	t.Lock()
	if !t.fini {
		t.style = style
	}
	t.Unlock()
}

func (t *IOScreen) Clear() {
	t.Fill(' ', t.style)
}

func (t *IOScreen) Fill(r rune, style tcell.Style) {
	t.Lock()
	if !t.fini {
		t.cells.Fill(r, style)
	}
	t.Unlock()
}

func (t *IOScreen) SetContent(x, y int, mainc rune, combc []rune, style tcell.Style) {
	t.Lock()
	if !t.fini {
		t.cells.SetContent(x, y, mainc, combc, style)
	}
	t.Unlock()
}

func (t *IOScreen) GetContent(x, y int) (rune, []rune, tcell.Style, int) {
	t.Lock()
	mainc, combc, style, width := t.cells.GetContent(x, y)
	t.Unlock()
	return mainc, combc, style, width
}

func (t *IOScreen) SetCell(x, y int, style tcell.Style, ch ...rune) {
	if len(ch) > 0 {
		t.SetContent(x, y, ch[0], ch[1:], style)
	} else {
		t.SetContent(x, y, ' ', nil, style)
	}
}

func (t *IOScreen) encodeRune(r rune, buf []byte) []byte {

	nb := make([]byte, 6)
	ob := make([]byte, 6)
	num := utf8.EncodeRune(ob, r)
	ob = ob[:num]
	dst := 0
	var err error
	if enc := t.encoder; enc != nil {
		enc.Reset()
		dst, _, err = enc.Transform(nb, ob, true)
	}
	if err != nil || dst == 0 || nb[0] == '\x1a' {
		// Combining characters are elided
		if len(buf) == 0 {
			if acs, ok := t.acs[r]; ok {
				buf = append(buf, []byte(acs)...)
			} else if fb, ok := t.fallback[r]; ok {
				buf = append(buf, []byte(fb)...)
			} else {
				buf = append(buf, '?')
			}
		}
	} else {
		buf = append(buf, nb[:dst]...)
	}

	return buf
}

func (t *IOScreen) sendFgBg(fg tcell.Color, bg tcell.Color) {
	ti := t.ti
	if ti.Colors == 0 {
		return
	}
	if fg == tcell.ColorReset || bg == tcell.ColorReset {
		t.TPuts(ti.ResetFgBg)
	}
	if t.truecolor {
		if ti.SetFgBgRGB != "" && fg.IsRGB() && bg.IsRGB() {
			r1, g1, b1 := fg.RGB()
			r2, g2, b2 := bg.RGB()
			t.TPuts(ti.TParm(ti.SetFgBgRGB,
				int(r1), int(g1), int(b1),
				int(r2), int(g2), int(b2)))
			return
		}

		if fg.IsRGB() && ti.SetFgRGB != "" {
			r, g, b := fg.RGB()
			t.TPuts(ti.TParm(ti.SetFgRGB, int(r), int(g), int(b)))
			fg = tcell.ColorDefault
		}

		if bg.IsRGB() && ti.SetBgRGB != "" {
			r, g, b := bg.RGB()
			t.TPuts(ti.TParm(ti.SetBgRGB,
				int(r), int(g), int(b)))
			bg = tcell.ColorDefault
		}
	}

	if fg.Valid() {
		if v, ok := t.colors[fg]; ok {
			fg = v
		} else {
			v = tcell.FindColor(fg, t.palette)
			t.colors[fg] = v
			fg = v
		}
	}

	if bg.Valid() {
		if v, ok := t.colors[bg]; ok {
			bg = v
		} else {
			v = tcell.FindColor(bg, t.palette)
			t.colors[bg] = v
			bg = v
		}
	}

	if fg.Valid() && bg.Valid() && ti.SetFgBg != "" {
		t.TPuts(ti.TParm(ti.SetFgBg, int(fg&0xff), int(bg&0xff)))
	} else {
		if fg.Valid() && ti.SetFg != "" {
			t.TPuts(ti.TParm(ti.SetFg, int(fg&0xff)))
		}
		if bg.Valid() && ti.SetBg != "" {
			t.TPuts(ti.TParm(ti.SetBg, int(bg&0xff)))
		}
	}
}

func (t *IOScreen) drawCell(x, y int) int {

	ti := t.ti

	// log.Printf("drawCell(%d,%d)", x, y)

	mainc, combc, style, width := t.cells.GetContent(x, y)
	if !t.cells.Dirty(x, y) {
		return width
	}

	if y == t.h-1 && x == t.w-1 && t.ti.AutoMargin && ti.InsertChar != "" {
		// our solution is somewhat goofy.
		// we write to the second to the last cell what we want in the last cell, then we
		// insert a character at that 2nd to last position to shift the last column into
		// place, then we rewrite that 2nd to last cell.  Old terminals suck.
		t.TPuts(ti.TGoto(x-1, y))
		defer func() {
			t.TPuts(ti.TGoto(x-1, y))
			t.TPuts(ti.InsertChar)
			t.cy = y
			t.cx = x - 1
			t.cells.SetDirty(x-1, y, true)
			_ = t.drawCell(x-1, y)
			t.TPuts(t.ti.TGoto(0, 0))
			t.cy = 0
			t.cx = 0
		}()
	} else if t.cy != y || t.cx != x {
		t.TPuts(ti.TGoto(x, y))
		t.cx = x
		t.cy = y
	}

	if style == tcell.StyleDefault {
		style = t.style
	}
	if style != t.curstyle {
		fg, bg, attrs := style.Decompose()

		t.TPuts(ti.AttrOff)

		t.sendFgBg(fg, bg)
		if attrs&tcell.AttrBold != 0 {
			t.TPuts(ti.Bold)
		}
		if attrs&tcell.AttrUnderline != 0 {
			t.TPuts(ti.Underline)
		}
		if attrs&tcell.AttrReverse != 0 {
			t.TPuts(ti.Reverse)
		}
		if attrs&tcell.AttrBlink != 0 {
			t.TPuts(ti.Blink)
		}
		if attrs&tcell.AttrDim != 0 {
			t.TPuts(ti.Dim)
		}
		if attrs&tcell.AttrItalic != 0 {
			t.TPuts(ti.Italic)
		}
		if attrs&tcell.AttrStrikeThrough != 0 {
			t.TPuts(ti.StrikeThrough)
		}
		t.curstyle = style
	}
	// now emit runes - taking care to not overrun width with a
	// wide character, and to ensure that we emit exactly one regular
	// character followed up by any residual combing characters

	if width < 1 {
		width = 1
	}

	var str string

	buf := make([]byte, 0, 6)

	buf = t.encodeRune(mainc, buf)
	for _, r := range combc {
		buf = t.encodeRune(r, buf)
	}

	str = string(buf)
	if width > 1 && str == "?" {
		// No FullWidth character support
		str = "? "
		t.cx = -1
	}

	if x > t.w-width {
		// too wide to fit; emit a single space instead
		width = 1
		str = " "
	}
	t.writeString(str)
	t.cx += width
	t.cells.SetDirty(x, y, false)
	if width > 1 {
		t.cx = -1
	}

	log.Printf("drawCell(%d,%d) done", x, y)

	return width
}

func (t *IOScreen) ShowCursor(x, y int) {
	t.Lock()
	t.cursorx = x
	t.cursory = y
	t.Unlock()
}

func (t *IOScreen) HideCursor() {
	t.ShowCursor(-1, -1)
}

func (t *IOScreen) showCursor() {

	x, y := t.cursorx, t.cursory
	w, h := t.cells.Size()
	if x < 0 || y < 0 || x >= w || y >= h {
		t.hideCursor()
		return
	}
	t.TPuts(t.ti.TGoto(x, y))
	t.TPuts(t.ti.ShowCursor)
	t.cx = x
	t.cy = y
}

// writeString sends a string to the terminal. The string is sent as-is and
// this function does not expand inline padding indications (of the form
// $<[delay]> where [delay] is msec). In order to have these expanded, use
// TPuts. If the screen is "buffering", the string is collected in a buffer,
// with the intention that the entire buffer be sent to the terminal in one
// write operation at some point later.
func (t *IOScreen) writeString(s string) {
	if t.buffering {
		_, _ = io.WriteString(&t.buf, s)
	} else {
		_, _ = io.WriteString(t.tty, s)
	}
}

func (t *IOScreen) TPuts(s string) {
	if t.buffering {
		t.ti.TPuts(&t.buf, s)
	} else {
		t.ti.TPuts(t.tty, s)
	}
}

func (t *IOScreen) Show() {
	t.Lock()
	if !t.fini {
		t.resize()
		t.draw()
	}
	t.Unlock()
}

func (t *IOScreen) clearScreen() {
	fg, bg, _ := t.style.Decompose()
	t.sendFgBg(fg, bg)
	t.TPuts(t.ti.Clear)
	t.clear = false
}

func (t *IOScreen) hideCursor() {
	// does not update cursor position
	if t.ti.HideCursor != "" {
		t.TPuts(t.ti.HideCursor)
	} else {
		// No way to hide cursor, stick it
		// at bottom right of screen
		t.cx, t.cy = t.cells.Size()
		t.TPuts(t.ti.TGoto(t.cx, t.cy))
	}
}

func (t *IOScreen) draw() {
	// clobber cursor position, because we're gonna change it all
	t.cx = -1
	t.cy = -1

	t.buf.Reset()
	t.buffering = true
	defer func() {
		t.buffering = false
	}()

	// hide the cursor while we move stuff around
	t.hideCursor()

	if t.clear {
		t.clearScreen()
	}

	// log.Printf("draw: %d x %d", t.h, t.w)
	for y := 0; y < t.h; y++ {
		for x := 0; x < t.w; x++ {
			width := t.drawCell(x, y)

			// fmt.Printf("width=%d", width)
			if width > 1 {
				if x+1 < t.w {
					// this is necessary so that if we ever
					// go back to drawing that cell, we
					// actually will *draw* it.
					t.cells.SetDirty(x+1, y, true)
				}
			}
			if width > 0 {
				x += width - 1
			}
		}
	}

	// restore the cursor
	t.showCursor()

	_, _ = t.buf.WriteTo(t.tty)
}

func (t *IOScreen) EnableMouse(flags ...tcell.MouseFlags) {
	var f tcell.MouseFlags
	flagsPresent := false
	for _, flag := range flags {
		f |= flag
		flagsPresent = true
	}
	if !flagsPresent {
		f = tcell.MouseMotionEvents
	}

	t.Lock()
	t.mouseFlags = f
	t.enableMouse(f)
	t.Unlock()
}

func (t *IOScreen) enableMouse(f tcell.MouseFlags) {
	// Rather than using terminfo to find mouse escape sequences, we rely on the fact that
	// pretty much *every* terminal that supports mouse tracking follows the
	// XTerm standards (the modern ones).
	if len(t.mouse) != 0 {
		// start by disabling all tracking.
		t.TPuts("\x1b[?1000l\x1b[?1002l\x1b[?1003l\x1b[?1006l")
		if f&tcell.MouseMotionEvents != 0 {
			t.TPuts("\x1b[?1003h\x1b[?1006h")
		} else if f&tcell.MouseDragEvents != 0 {
			t.TPuts("\x1b[?1002h\x1b[?1006h")
		} else if f&tcell.MouseButtonEvents != 0 {
			t.TPuts("\x1b[?1000h\x1b[?1006h")
		}
	}
}

func (t *IOScreen) DisableMouse() {
	t.Lock()
	t.mouseFlags = 0
	t.enableMouse(0)
	t.Unlock()
}

func (t *IOScreen) EnablePaste() {
	t.Lock()
	t.pasteEnabled = true
	t.enablePasting(true)
	t.Unlock()
}

func (t *IOScreen) DisablePaste() {
	t.Lock()
	t.pasteEnabled = false
	t.enablePasting(false)
	t.Unlock()
}

func (t *IOScreen) enablePasting(on bool) {
	var s string
	if on {
		s = t.enablePaste
	} else {
		s = t.disablePaste
	}
	if s != "" {
		t.TPuts(s)
	}
}

func (t *IOScreen) Size() (int, int) {
	t.Lock()
	w, h := t.w, t.h
	t.Unlock()
	return w, h
}

func (t *IOScreen) resize() {
	if w, h, e := t.tty.WindowSize(); e == nil {
		if w != t.w || h != t.h {
			t.cx = -1
			t.cy = -1

			t.cells.Resize(w, h)
			t.cells.Invalidate()
			t.h = h
			t.w = w
			ev := tcell.NewEventResize(w, h)
			_ = t.PostEvent(ev)
		}
	}
}

func (t *IOScreen) Colors() int {
	// this doesn't change, no need for lock
	if t.truecolor {
		return 1 << 24
	}
	return t.ti.Colors
}

// nColors returns the size of the built-in palette.
// This is distinct from Colors(), as it will generally
// always be a small number. (<= 256)
func (t *IOScreen) nColors() int {
	return t.ti.Colors
}

func (t *IOScreen) ChannelEvents(ch chan<- tcell.Event, quit <-chan struct{}) {
	defer close(ch)
	for {
		select {
		case <-quit:
			return
		case <-t.quit:
			return
		case ev := <-t.evch:
			select {
			case <-quit:
				return
			case <-t.quit:
				return
			case ch <- ev:
			}
		}
	}
}

func (t *IOScreen) PollEvent() tcell.Event {
	select {
	case <-t.quit:
		return nil
	case ev := <-t.evch:
		return ev
	}
}

func (t *IOScreen) HasPendingEvent() bool {
	return len(t.evch) > 0
}

// vtACSNames is a map of bytes defined by terminfo that are used in
// the terminals Alternate Character Set to represent other glyphs.
// For example, the upper left corner of the box drawing set can be
// displayed by printing "l" while in the alternate character set.
// Its not quite that simple, since the "l" is the terminfo name,
// and it may be necessary to use a different character based on
// the terminal implementation (or the terminal may lack support for
// this altogether).  See buildAcsMap below for detail.
var vtACSNames = map[byte]rune{
	'+': tcell.RuneRArrow,
	',': tcell.RuneLArrow,
	'-': tcell.RuneUArrow,
	'.': tcell.RuneDArrow,
	'0': tcell.RuneBlock,
	'`': tcell.RuneDiamond,
	'a': tcell.RuneCkBoard,
	'b': '␉', // VT100, Not defined by terminfo
	'c': '␌', // VT100, Not defined by terminfo
	'd': '␋', // VT100, Not defined by terminfo
	'e': '␊', // VT100, Not defined by terminfo
	'f': tcell.RuneDegree,
	'g': tcell.RunePlMinus,
	'h': tcell.RuneBoard,
	'i': tcell.RuneLantern,
	'j': tcell.RuneLRCorner,
	'k': tcell.RuneURCorner,
	'l': tcell.RuneULCorner,
	'm': tcell.RuneLLCorner,
	'n': tcell.RunePlus,
	'o': tcell.RuneS1,
	'p': tcell.RuneS3,
	'q': tcell.RuneHLine,
	'r': tcell.RuneS7,
	's': tcell.RuneS9,
	't': tcell.RuneLTee,
	'u': tcell.RuneRTee,
	'v': tcell.RuneBTee,
	'w': tcell.RuneTTee,
	'x': tcell.RuneVLine,
	'y': tcell.RuneLEqual,
	'z': tcell.RuneGEqual,
	'{': tcell.RunePi,
	'|': tcell.RuneNEqual,
	'}': tcell.RuneSterling,
	'~': tcell.RuneBullet,
}

// buildAcsMap builds a map of characters that we translate from Unicode to
// alternate character encodings.  To do this, we use the standard VT100 ACS
// maps.  This is only done if the terminal lacks support for Unicode; we
// always prefer to emit Unicode glyphs when we are able.
func (t *IOScreen) buildAcsMap() {
	acsstr := t.ti.AltChars
	t.acs = make(map[rune]string)
	for len(acsstr) > 2 {
		srcv := acsstr[0]
		dstv := string(acsstr[1])
		if r, ok := vtACSNames[srcv]; ok {
			t.acs[r] = t.ti.EnterAcs + dstv + t.ti.ExitAcs
		}
		acsstr = acsstr[2:]
	}
}

func (t *IOScreen) PostEventWait(ev tcell.Event) {
	t.evch <- ev
}

func (t *IOScreen) PostEvent(ev tcell.Event) error {
	select {
	case t.evch <- ev:
		return nil
	default:
		return tcell.ErrEventQFull
	}
}

func (t *IOScreen) clip(x, y int) (int, int) {
	w, h := t.cells.Size()
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	if x > w-1 {
		x = w - 1
	}
	if y > h-1 {
		y = h - 1
	}
	return x, y
}

// buildMouseEvent returns an event based on the supplied coordinates and button
// state. Note that the screen's mouse button state is updated based on the
// input to this function (i.e. it mutates the receiver).
func (t *IOScreen) buildMouseEvent(x, y, btn int) *tcell.EventMouse {

	// XTerm mouse events only report at most one button at a time,
	// which may include a wheel button.  Wheel motion events are
	// reported as single impulses, while other button events are reported
	// as separate press & release events.

	button := tcell.ButtonNone
	mod := tcell.ModNone

	// Mouse wheel has bit 6 set, no release events.  It should be noted
	// that wheel events are sometimes misdelivered as mouse button events
	// during a click-drag, so we debounce these, considering them to be
	// button press events unless we see an intervening release event.
	switch btn & 0x43 {
	case 0:
		button = tcell.Button1
		t.wasbtn = true
	case 1:
		button = tcell.Button3 // Note we prefer to treat right as button 2
		t.wasbtn = true
	case 2:
		button = tcell.Button2 // And the middle button as button 3
		t.wasbtn = true
	case 3:
		button = tcell.ButtonNone
		t.wasbtn = false
	case 0x40:
		if !t.wasbtn {
			button = tcell.WheelUp
		} else {
			button = tcell.Button1
		}
	case 0x41:
		if !t.wasbtn {
			button = tcell.WheelDown
		} else {
			button = tcell.Button2
		}
	}

	if btn&0x4 != 0 {
		mod |= tcell.ModShift
	}
	if btn&0x8 != 0 {
		mod |= tcell.ModAlt
	}
	if btn&0x10 != 0 {
		mod |= tcell.ModCtrl
	}

	// Some terminals will report mouse coordinates outside the
	// screen, especially with click-drag events.  Clip the coordinates
	// to the screen in that case.
	x, y = t.clip(x, y)

	return tcell.NewEventMouse(x, y, button, mod)
}

// parseSgrMouse attempts to locate an SGR mouse record at the start of the
// buffer.  It returns true, true if it found one, and the associated bytes
// be removed from the buffer.  It returns true, false if the buffer might
// contain such an event, but more bytes are necessary (partial match), and
// false, false if the content is definitely *not* an SGR mouse record.
func (t *IOScreen) parseSgrMouse(buf *bytes.Buffer, evs *[]tcell.Event) (bool, bool) {

	b := buf.Bytes()

	var x, y, btn, state int
	dig := false
	neg := false
	motion := false
	i := 0
	val := 0

	for i = range b {
		switch b[i] {
		case '\x1b':
			if state != 0 {
				return false, false
			}
			state = 1

		case '\x9b':
			if state != 0 {
				return false, false
			}
			state = 2

		case '[':
			if state != 1 {
				return false, false
			}
			state = 2

		case '<':
			if state != 2 {
				return false, false
			}
			val = 0
			dig = false
			neg = false
			state = 3

		case '-':
			if state != 3 && state != 4 && state != 5 {
				return false, false
			}
			if dig || neg {
				return false, false
			}
			neg = true // stay in state

		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			if state != 3 && state != 4 && state != 5 {
				return false, false
			}
			val *= 10
			val += int(b[i] - '0')
			dig = true // stay in state

		case ';':
			if neg {
				val = -val
			}
			switch state {
			case 3:
				btn, val = val, 0
				neg, dig, state = false, false, 4
			case 4:
				x, val = val-1, 0
				neg, dig, state = false, false, 5
			default:
				return false, false
			}

		case 'm', 'M':
			if state != 5 {
				return false, false
			}
			if neg {
				val = -val
			}
			y = val - 1

			motion = (btn & 32) != 0
			btn &^= 32
			if b[i] == 'm' {
				// mouse release, clear all buttons
				btn |= 3
				btn &^= 0x40
				t.buttondn = false
			} else if motion {
				/*
				 * Some broken terminals appear to send
				 * mouse button one motion events, instead of
				 * encoding 35 (no buttons) into these events.
				 * We resolve these by looking for a non-motion
				 * event first.
				 */
				if !t.buttondn {
					btn |= 3
					btn &^= 0x40
				}
			} else {
				t.buttondn = true
			}
			// consume the event bytes
			for i >= 0 {
				_, _ = buf.ReadByte()
				i--
			}
			*evs = append(*evs, t.buildMouseEvent(x, y, btn))
			return true, true
		}
	}

	// incomplete & inconclusive at this point
	return true, false
}

// parseXtermMouse is like parseSgrMouse, but it parses a legacy
// X11 mouse record.
func (t *IOScreen) parseXtermMouse(buf *bytes.Buffer, evs *[]tcell.Event) (bool, bool) {

	b := buf.Bytes()

	state := 0
	btn := 0
	x := 0
	y := 0

	for i := range b {
		switch state {
		case 0:
			switch b[i] {
			case '\x1b':
				state = 1
			case '\x9b':
				state = 2
			default:
				return false, false
			}
		case 1:
			if b[i] != '[' {
				return false, false
			}
			state = 2
		case 2:
			if b[i] != 'M' {
				return false, false
			}
			state++
		case 3:
			btn = int(b[i])
			state++
		case 4:
			x = int(b[i]) - 32 - 1
			state++
		case 5:
			y = int(b[i]) - 32 - 1
			for i >= 0 {
				_, _ = buf.ReadByte()
				i--
			}
			*evs = append(*evs, t.buildMouseEvent(x, y, btn))
			return true, true
		}
	}
	return true, false
}

func (t *IOScreen) parseFunctionKey(buf *bytes.Buffer, evs *[]tcell.Event) (bool, bool) {
	b := buf.Bytes()
	partial := false
	for e, k := range t.keycodes {
		esc := []byte(e)
		if (len(esc) == 1) && (esc[0] == '\x1b') {
			continue
		}
		if bytes.HasPrefix(b, esc) {
			// matched
			var r rune
			if len(esc) == 1 {
				r = rune(b[0])
			}
			mod := k.mod
			if t.escaped {
				mod |= tcell.ModAlt
				t.escaped = false
			}
			switch k.key {
			case keyPasteStart:
				*evs = append(*evs, tcell.NewEventPaste(true))
			case keyPasteEnd:
				*evs = append(*evs, tcell.NewEventPaste(false))
			default:
				*evs = append(*evs, tcell.NewEventKey(k.key, r, mod))
			}
			for i := 0; i < len(esc); i++ {
				_, _ = buf.ReadByte()
			}
			return true, true
		}
		if bytes.HasPrefix(esc, b) {
			partial = true
		}
	}
	return partial, false
}

func (t *IOScreen) parseRune(buf *bytes.Buffer, evs *[]tcell.Event) (bool, bool) {
	b := buf.Bytes()
	if b[0] >= ' ' && b[0] <= 0x7F {
		// printable ASCII easy to deal with -- no encodings
		mod := tcell.ModNone
		if t.escaped {
			mod = tcell.ModAlt
			t.escaped = false
		}
		*evs = append(*evs, tcell.NewEventKey(tcell.KeyRune, rune(b[0]), mod))
		_, _ = buf.ReadByte()
		return true, true
	}

	if b[0] < 0x80 {
		// Low numbered values are control keys, not runes.
		return false, false
	}

	utf := make([]byte, 12)
	for l := 1; l <= len(b); l++ {
		t.decoder.Reset()
		nOut, nIn, e := t.decoder.Transform(utf, b[:l], true)
		if e == transform.ErrShortSrc {
			continue
		}
		if nOut != 0 {
			r, _ := utf8.DecodeRune(utf[:nOut])
			if r != utf8.RuneError {
				mod := tcell.ModNone
				if t.escaped {
					mod = tcell.ModAlt
					t.escaped = false
				}
				*evs = append(*evs, tcell.NewEventKey(tcell.KeyRune, r, mod))
			}
			for nIn > 0 {
				_, _ = buf.ReadByte()
				nIn--
			}
			return true, true
		}
	}
	// Looks like potential escape
	return true, false
}

func (t *IOScreen) scanInput(buf *bytes.Buffer, expire bool) {
	evs := t.collectEventsFromInput(buf, expire)

	for _, ev := range evs {
		t.PostEventWait(ev)
	}
}

// Return an array of Events extracted from the supplied buffer. This is done
// while holding the screen's lock - the events can then be queued for
// application processing with the lock released.
func (t *IOScreen) collectEventsFromInput(buf *bytes.Buffer, expire bool) []tcell.Event {

	res := make([]tcell.Event, 0, 20)

	t.Lock()
	defer t.Unlock()

	for {
		b := buf.Bytes()
		if len(b) == 0 {
			buf.Reset()
			return res
		}

		partials := 0

		if part, comp := t.parseRune(buf, &res); comp {
			continue
		} else if part {
			partials++
		}

		if part, comp := t.parseFunctionKey(buf, &res); comp {
			continue
		} else if part {
			partials++
		}

		// Only parse mouse records if this term claims to have
		// mouse support

		if t.ti.Mouse != "" {
			if part, comp := t.parseXtermMouse(buf, &res); comp {
				continue
			} else if part {
				partials++
			}

			if part, comp := t.parseSgrMouse(buf, &res); comp {
				continue
			} else if part {
				partials++
			}
		}

		if partials == 0 || expire {
			if b[0] == '\x1b' {
				if len(b) == 1 {
					res = append(res, tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
					t.escaped = false
				} else {
					t.escaped = true
				}
				_, _ = buf.ReadByte()
				continue
			}
			// Nothing was going to match, or we timed out
			// waiting for more data -- just deliver the characters
			// to the app & let them sort it out.  Possibly we
			// should only do this for control characters like ESC.
			by, _ := buf.ReadByte()
			mod := tcell.ModNone
			if t.escaped {
				t.escaped = false
				mod = tcell.ModAlt
			}
			res = append(res, tcell.NewEventKey(tcell.KeyRune, rune(by), mod))
			continue
		}

		// well we have some partial data, wait until we get
		// some more
		break
	}

	return res
}

func (t *IOScreen) mainLoop(stopQ chan struct{}) {
	defer t.wg.Done()
	buf := &bytes.Buffer{}
	for {
		select {
		case <-stopQ:
			return
		case <-t.quit:
			return
		case <-t.resizeQ:
			t.Lock()
			t.cx = -1
			t.cy = -1
			t.resize()
			t.cells.Invalidate()
			t.draw()
			t.Unlock()
			continue
		case <-t.keytimer.C:
			// If the timer fired, and the current time
			// is after the expiration of the escape sequence,
			// then we assume the escape sequence reached it's
			// conclusion, and process the chunk independently.
			// This lets us detect conflicts such as a lone ESC.
			if buf.Len() > 0 {
				if time.Now().After(t.keyexpire) {
					t.scanInput(buf, true)
				}
			}
			if buf.Len() > 0 {
				if !t.keytimer.Stop() {
					select {
					case <-t.keytimer.C:
					default:
					}
				}
				t.keytimer.Reset(time.Millisecond * 50)
			}
		case chunk := <-t.keychan:
			buf.Write(chunk)
			t.keyexpire = time.Now().Add(time.Millisecond * 50)
			t.scanInput(buf, false)
			if !t.keytimer.Stop() {
				select {
				case <-t.keytimer.C:
				default:
				}
			}
			if buf.Len() > 0 {
				t.keytimer.Reset(time.Millisecond * 50)
			}
		}
	}
}

func (t *IOScreen) inputLoop(stopQ chan struct{}) {

	defer t.wg.Done()
	for {
		select {
		case <-stopQ:
			return
		default:
		}
		chunk := make([]byte, 128)
		n, e := t.tty.Read(chunk)
		switch e {
		case nil:
		default:
			_ = t.PostEvent(tcell.NewEventError(e))
			return
		}
		if n > 0 {
			t.keychan <- chunk[:n]
		}
	}
}

func (t *IOScreen) Sync() {
	t.Lock()
	t.cx = -1
	t.cy = -1
	if !t.fini {
		t.resize()
		t.clear = true
		t.cells.Invalidate()
		t.draw()
	}
	t.Unlock()
}

func (t *IOScreen) CharacterSet() string {
	return t.charset
}

func (t *IOScreen) RegisterRuneFallback(orig rune, fallback string) {
	t.Lock()
	t.fallback[orig] = fallback
	t.Unlock()
}

func (t *IOScreen) UnregisterRuneFallback(orig rune) {
	t.Lock()
	delete(t.fallback, orig)
	t.Unlock()
}

func (t *IOScreen) CanDisplay(r rune, checkFallbacks bool) bool {

	if enc := t.encoder; enc != nil {
		nb := make([]byte, 6)
		ob := make([]byte, 6)
		num := utf8.EncodeRune(ob, r)

		enc.Reset()
		dst, _, err := enc.Transform(nb, ob[:num], true)
		if dst != 0 && err == nil && nb[0] != '\x1A' {
			return true
		}
	}
	// Terminal fallbacks always permitted, since we assume they are
	// basically nearly perfect renditions.
	if _, ok := t.acs[r]; ok {
		return true
	}
	if !checkFallbacks {
		return false
	}
	if _, ok := t.fallback[r]; ok {
		return true
	}
	return false
}

func (t *IOScreen) HasMouse() bool {
	return len(t.mouse) != 0
}

func (t *IOScreen) HasKey(k tcell.Key) bool {
	if k == tcell.KeyRune {
		return true
	}
	return t.keyexist[k]
}

func (t *IOScreen) Resize(int, int, int, int) {}

func (t *IOScreen) Suspend() error {
	t.disengage()
	return nil
}

func (t *IOScreen) Resume() error {
	return t.engage()
}

// engage is used to place the terminal in raw mode and establish screen size, etc.
// Thing of this is as tcell "engaging" the clutch, as it's going to be driving the
// terminal interface.
func (t *IOScreen) engage() error {
	t.Lock()
	defer t.Unlock()
	if t.tty == nil {
		return tcell.ErrNoScreen
	}
	t.tty.NotifyResize(func() {
		select {
		case t.resizeQ <- true:
		default:
		}
	})
	if t.running {
		return errors.New("already engaged")
	}
	if err := t.tty.Start(); err != nil {
		return err
	}
	t.running = true
	if w, h, err := t.tty.WindowSize(); err == nil && w != 0 && h != 0 {
		t.cells.Resize(w, h)
	}
	stopQ := make(chan struct{})
	t.stopQ = stopQ
	t.enableMouse(t.mouseFlags)
	t.enablePasting(t.pasteEnabled)

	ti := t.ti
	t.TPuts(ti.EnterCA)
	t.TPuts(ti.EnterKeypad)
	t.TPuts(ti.HideCursor)
	t.TPuts(ti.EnableAcs)
	t.TPuts(ti.Clear)

	t.wg.Add(2)
	go t.inputLoop(stopQ)
	go t.mainLoop(stopQ)
	return nil
}

// disengage is used to release the terminal back to support from the caller.
// Think of this as tcell disengaging the clutch, so that another application
// can take over the terminal interface.  This restores the TTY mode that was
// present when the application was first started.
func (t *IOScreen) disengage() {

	t.Lock()
	if !t.running {
		t.Unlock()
		return
	}
	t.running = false
	stopQ := t.stopQ
	close(stopQ)
	_ = t.tty.Drain()
	t.Unlock()

	t.tty.NotifyResize(nil)
	// wait for everything to shut down
	t.wg.Wait()

	// shutdown the screen and disable special modes (e.g. mouse and bracketed paste)
	ti := t.ti
	t.cells.Resize(0, 0)
	t.TPuts(ti.ShowCursor)
	t.TPuts(ti.ResetFgBg)
	t.TPuts(ti.AttrOff)
	t.TPuts(ti.Clear)
	t.TPuts(ti.ExitCA)
	t.TPuts(ti.ExitKeypad)
	t.enableMouse(0)
	t.enablePasting(false)

	_ = t.tty.Stop()
}

// Beep emits a beep to the terminal.
func (t *IOScreen) Beep() error {
	t.writeString(string(byte(7)))
	return nil
}

// finalize is used to at application shutdown, and restores the terminal
// to it's initial state.  It should not be called more than once.
func (t *IOScreen) finalize() {
	t.disengage()
	_ = t.tty.Close()
}
