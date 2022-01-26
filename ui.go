package main

import (
	"fmt"
	"io"
	"log"
	"strings"
	"sync"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type Actor interface {
	PerformAction(action string)
}

type MapArea struct {
	*tview.Box
	UI *UI
}

func NewMapArea(ui *UI) *MapArea {
	mapArea := &MapArea{
		Box: tview.NewBox(),
		UI:  ui,
	}

	mapArea.SetBorder(true)
	mapArea.SetTitle("Map")

	return mapArea
}

func (m *MapArea) Draw(screen tcell.Screen) {
	m.Box.DrawForSubclass(screen, m)

	// draw the rest of the map...

	x, y, w, h := m.GetInnerRect()
	s := "Map goes here"

	tview.Print(screen, s, x, y+h/2, w, tview.AlignCenter, tcell.ColorDefault)
}

type InputArea struct {
	*tview.Box
	UI         *UI
	LastKey    tcell.Key
	LastMods   tcell.ModMask
	LastRune   rune
	HasLastKey bool
}

func NewInputArea(ui *UI) *InputArea {
	inp := &InputArea{
		Box:        tview.NewBox(),
		UI:         ui,
		HasLastKey: false,
	}

	inp.SetInputCapture(inp.handleInput)

	return inp
}

func (inp *InputArea) handleInput(e *tcell.EventKey) *tcell.EventKey {
	inp.HasLastKey = true
	inp.LastKey = e.Key()
	inp.LastMods = e.Modifiers()
	inp.LastRune = e.Rune()

	return e
}

func (inp *InputArea) Draw(screen tcell.Screen) {
	inp.Box.DrawForSubclass(screen, inp)

	// draw the rest of the map...

	x, y, w, h := inp.GetInnerRect()
	var s string
	if inp.HasLastKey {
		if inp.LastKey == tcell.KeyRune {
			s = fmt.Sprintf("Last rune: %c Mods: %04x", inp.LastRune, inp.LastMods)
		} else {
			s = fmt.Sprintf("Last key:  %d Mods: %04x", inp.LastKey, inp.LastMods)
		}
	} else {
		s = "No last rune"
	}

	tview.Print(screen, s, x, y+h/2, w, tview.AlignCenter, tcell.ColorDefault)
}

type Menu struct {
	*tview.Frame
	Layout   *tview.Flex
	Actor    Actor
	Buttons  []*tview.Button
	Selected int
}

func NewMenu(title string, actor Actor) *Menu {
	layout := tview.NewFlex()
	layout.SetDirection(tview.FlexRow)

	f := tview.NewFrame(layout)

	f.SetTitle(title)
	f.SetBorder(true)

	m := &Menu{
		Frame:  f,
		Layout: layout,
		Actor:  actor,
	}

	return m
}

func (m *Menu) selectIndex(ind int) *tview.Button {
	nbtns := len(m.Buttons)

	if nbtns == 0 {
		return nil
	}

	for ind < 0 {
		ind += nbtns
	}

	for ind >= nbtns {
		ind -= nbtns
	}

	if m.Selected != ind {
		m.Selected = ind
	}

	return m.CurrentButton()
}

func (m *Menu) CurrentButton() *tview.Button {
	if m.Selected < 0 || m.Selected >= len(m.Buttons) {
		return nil
	}

	return m.Buttons[m.Selected]
}

func (m *Menu) Focus(delegate func(p tview.Primitive)) {
	m.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
		return m.inputHandler(e, delegate)
	})

	if sel := m.CurrentButton(); sel != nil {
		delegate(sel)
	}
}

func (m *Menu) inputHandler(e *tcell.EventKey, setFocus func(tview.Primitive)) *tcell.EventKey {
	k := e.Key()
	mods := e.Modifiers()
	r := e.Rune()

	switch {
	case k == tcell.KeyUp && mods == tcell.ModNone:
		m.selectIndex(m.Selected - 1)

	case k == tcell.KeyTab && mods == tcell.ModShift:
		m.selectIndex(m.Selected + 1)

	case k == tcell.KeyRune && mods == tcell.ModNone && r == 'k':
		m.selectIndex(m.Selected - 1)

	case k == tcell.KeyRune && mods == tcell.ModCtrl && r == 'p':
		m.selectIndex(m.Selected - 1)

	case k == tcell.KeyDown && mods == tcell.ModNone:
		m.selectIndex(m.Selected + 1)

	case k == tcell.KeyTab && mods == tcell.ModNone:
		m.selectIndex(m.Selected + 1)

	case k == tcell.KeyRune && mods == tcell.ModNone && r == 'j':
		m.selectIndex(m.Selected + 1)

	case k == tcell.KeyRune && mods == tcell.ModCtrl && r == 'n':
		m.selectIndex(m.Selected + 1)

	default:
		return e
	}

	if btn := m.CurrentButton(); btn != nil {
		setFocus(btn)
	}

	return nil
}

func (m *Menu) AddButton(label string, action string) *tview.Button {
	return m.AddButtonWithCallback(label, func() {
		m.Actor.PerformAction(action)
	})
}

func (m *Menu) AddButtonWithCallback(label string, cb func()) *tview.Button {
	btn := tview.NewButton(label)
	btn.SetSelectedFunc(cb)
	m.Layout.AddItem(btn, 1, 1, true)

	m.Buttons = append(m.Buttons, btn)

	return btn
}

type PageNumber int

const (
	PageMain PageNumber = iota
	PageAdmin
)

type ModalState uint32

const (
	ModalNone ModalState = 0
	ModalMenu ModalState = 1 << iota
)

type UI struct {
	Logger  *log.Logger
	Session *Session

	App *tview.Application

	Pages *tview.Pages

	Menu tview.Primitive
	Main tview.Primitive

	Focus tview.Primitive

	Util *UtilityFrame
	Map  *MapArea

	PageShown  PageNumber
	ModalShown ModalState

	Actions   map[string]func()
	PageNames map[string]tview.Primitive

	AdminLogView *AdminLogView

	mu sync.Mutex
}

func (ui *UI) setupAction(action string, fn func()) func() {
	ui.Actions[action] = fn
	return ui.makeAction(action)
}

func (ui *UI) makeAction(action string) func() {
	return func() {
		ui.PerformAction(action)
	}
}

func (ui *UI) isAdmin() bool {
	return ui.Session.IsAdministrator()
}

func (ui *UI) PerformAction(action string) {
	if f := ui.Actions[action]; f != nil {
		f()
	}
}

func (ui *UI) toggleModal(modalState ModalState) {
	ui.ModalShown = ui.ModalShown ^ modalState
	ui.updateModalVisibilityAll(ui.ModalShown)
}

func (ui *UI) setModal(modalState ModalState, isOn bool) {
	if isOn {
		ui.ModalShown = ui.ModalShown | modalState
	} else {
		ui.ModalShown = ui.ModalShown &^ modalState
	}

	ui.updateModalVisibilityAll(ui.ModalShown)
}

func (ui *UI) updateModalVisibility(modalState ModalState, flag ModalState, modalName string) {
	if (modalState & flag) != 0 {
		ui.Pages.ShowPage(modalName)
	} else {
		ui.Pages.HidePage(modalName)
	}
}

func (ui *UI) updateModalVisibilityAll(modalState ModalState) {
	ui.updateModalVisibility(modalState, ModalMenu, "menu")
}

func (ui *UI) showModal(modalState ModalState) {
	ui.ModalShown = modalState

	switch ui.ModalShown {
	case ModalMenu:
		ui.Pages.ShowPage("menu")

	case ModalNone:
		ui.Pages.HidePage("menu")
	}
}

func (ui *UI) showPage(page PageNumber) {
	ui.mu.Lock()
	defer ui.mu.Unlock()

	if page == PageAdmin && !ui.isAdmin() {
		page = PageMain
	}

	switch page {
	case PageMain:
		if ui.Session.HasGame() {
			ui.Pages.SwitchToPage("main")
		} else {
			// ui.Pages.SwitchToPage("lobby")
			ui.Pages.SwitchToPage("main")
		}
	case PageAdmin:
		ui.Pages.SwitchToPage("admin")
	}

	switch ui.ModalShown {
	case ModalMenu:
		ui.Pages.ShowPage("menu")
	}

	ui.PageShown = page

	log.Printf("Page=%v, Modal=%v", ui.PageShown, ui.ModalShown)
}

func (ui *UI) toggleFocus() {
	ui.toggleModal(ModalMenu)

	/*
		switch ui.ModalShown {
		case ModalMenu:
			ui.showModal(ModalNone)

		case ModalNone:
			ui.showModal(ModalMenu)
		}
	*/

	/*
		_, focus := ui.Pages.GetFrontPage()
		switch focus {
		case ui.Menu:
			ui.focusMain()
		case ui.Main:
			ui.focusMenu()
		}
	*/
}

func (ui *UI) Run() error {
	return ui.App.Run()
}

func (ui *UI) Quit() {
	ui.App.Stop()
}

type AdminLogView struct {
	*tview.TextView

	Lines    []string
	LastLine int

	VisibleFunc func() bool

	mu sync.Mutex
}

func NewAdminLogView(numLines int) *AdminLogView {
	return &AdminLogView{
		TextView: tview.NewTextView(),
		Lines:    make([]string, 0, numLines),
	}
}

func (v *AdminLogView) AddLine(line string) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if len(v.Lines) < cap(v.Lines) {
		v.Lines = append(v.Lines, line)
		v.LastLine = len(v.Lines) - 1
	} else {
		ind := v.LastLine + 1
		if ind >= len(v.Lines) {
			ind = 0
		}

		v.Lines[ind] = line
		v.LastLine = ind
	}

	if strings.HasSuffix(line, "\n") || strings.HasSuffix(line, "\r\n") {
		io.WriteString(v.TextView, line)
	} else {
		fmt.Fprintf(v.TextView, "%s\n", line)
	}
}

func setupAdminPage(ui *UI, sysLog *SystemLog) {
	main := tview.NewPages()

	frame := tview.NewFrame(main)
	// frame.SetBorder(true)
	// frame.SetTitle("Admin")
	frame.SetBorders(0, 0, 0, 0, 0, 0)

	frame.SetBackgroundColor(tcell.ColorBlue)
	frame.AddText("Admin page", true, tview.AlignCenter, tcell.ColorWhite)

	logMain := tview.NewFlex().SetDirection(tview.FlexRow)

	logView := NewAdminLogView(1000)
	logView.SetBorder(true)
	logView.SetTitle("Logs")

	logView.VisibleFunc = func() bool {
		return (ui.PageShown == PageAdmin)
	}

	logView.SetChangedFunc(func() {
		ui.mu.Lock()
		defer ui.mu.Unlock()
		if ui.PageShown == PageAdmin {
			ui.App.Draw()
		}
	})

	sysLog.SetCallback(logView.AddLine)

	// place holder
	consoleView := tview.NewBox()
	consoleView.SetBorder(true)
	consoleView.SetTitle("Console")

	logMain.AddItem(logView, 0, 1, false)
	logMain.AddItem(consoleView, 5, 1, true)

	logFrame := tview.NewFrame(logMain)
	// logFrame.SetBorder(true)
	// logFrame.SetTitle("Logs")

	main.AddPage("logs", logFrame, true, true)

	ui.Pages.AddPage("admin", frame, true, false)
	ui.PageNames["admin"] = frame

	ui.AdminLogView = logView
}

func (ui *UI) globalKeyHandler(e *tcell.EventKey) *tcell.EventKey {
	k := e.Key()
	r := e.Rune()
	mods := e.Modifiers()

	if k == tcell.KeyRune && r == 'q' && mods&(tcell.ModAlt|tcell.ModMeta) != 0 {
		ui.App.Stop()
		return nil
	}

	if k == tcell.KeyEsc && mods == tcell.ModNone {
		// set focus to menu
		ui.toggleFocus()
		return nil
	}

	if k == tcell.KeyCtrlL && (mods == tcell.ModNone || mods == tcell.ModCtrl) {
		// fmt.Printf("Resync!\r\n")
		ui.App.Sync()
		return nil
	}

	if k == tcell.KeyRune && (mods == tcell.ModAlt || mods == tcell.ModMeta) {
		switch r {
		case '1':
			ui.showPage(PageMain)
			return nil

		case '9':
			if ui.isAdmin() {
				ui.showPage(PageAdmin)
				return nil
			}
		}
	}

	return e
}

type UtilityFrame struct {
	*tview.Box
	UI *UI
}

func NewUtilityFrame(ui *UI) *UtilityFrame {
	return &UtilityFrame{
		Box: tview.NewBox(),
		UI:  ui,
	}
}

func (fr *UtilityFrame) Draw(screen tcell.Screen) {
	fr.Box.DrawForSubclass(screen, fr)

	x0, y0, w, h := fr.GetInnerRect()

	var s string
	if fr.UI.isAdmin() {
		s = "[blue:white]ADMIN"
	} else {
		s = "[blue:white]User"
	}

	if w > 0 && h > 0 {
		tview.Print(screen, s, x0, y0, w, tview.AlignCenter, tcell.ColorDefault)
	}
}

func setupUI(sess *Session, sysLog *SystemLog) *UI {
	app := tview.NewApplication()

	pages := tview.NewPages()

	menu := tview.NewGrid()
	main := tview.NewFlex()

	ui := &UI{
		Session: sess,
		App:     app,
		Pages:   pages,
		Menu:    menu,
		Main:    main,
		Focus:   nil,

		// Lobby: lobby,

		Actions:   make(map[string]func()),
		PageNames: make(map[string]tview.Primitive),
	}

	mapArea := NewMapArea(ui)
	utilArea := NewUtilityFrame(ui)
	utilArea.SetBorder(true).SetTitle("Utility")

	inputArea := NewInputArea(ui)
	inputArea.SetBorder(true).SetTitle("Input")

	ui.Map = mapArea
	ui.Util = utilArea

	ui.setupAction("quit", app.Stop)
	ui.setupAction("resume", func() {
		ui.setModal(ModalMenu, false)
		// ui.
		// ui.focusMain)
	})

	menuBox := NewMenu("System menu", ui)
	menuBox.AddButton("Resume", "resume")
	menuBox.AddButton("Quit", "quit")

	menu.SetColumns(0, 40, 0).SetRows(0, 20, 0).AddItem(menuBox, 1, 1, 1, 1, 0, 0, true)

	main.SetDirection(tview.FlexColumn).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(mapArea, 0, 1, false).
			AddItem(inputArea, 10, 1, true), 0, 4, true).
		AddItem(utilArea, 0, 1, false)

	/*
		tview.NewBox().SetBorder(true).SetTitle("Left (1/2 x width of Top)"), 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(tview.NewBox().SetBorder(true).SetTitle("Top"), 0, 1, false).
			AddItem(tview.NewBox().SetBorder(true).SetTitle("Middle (3 x height of Top)"), 0, 3, false).
			AddItem(tview.NewBox().SetBorder(true).SetTitle("Bottom (5 rows)"), 5, 1, false), 0, 2, false).
		AddItem(tview.NewBox().SetBorder(true).SetTitle("Right (20 cols)"), 20, 1, false)
	*/

	if sess.IsAdministrator() {
		setupAdminPage(ui, sysLog)
	}

	pages.AddPage("main", main, true, true)
	ui.PageNames["main"] = main

	pages.AddPage("menu", menu, true, false)

	app.SetRoot(pages, true)
	app.SetInputCapture(ui.globalKeyHandler)

	log.Printf("set up UI")

	return ui
}
