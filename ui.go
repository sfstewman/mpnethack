package mpnethack

import (
	"fmt"
	"log"
	"sync"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/sfstewman/mpnethack/chat"
	"github.com/sfstewman/mpnethack/uilib"
)

type Actor interface {
	PerformAction(action string)
}

type MapArea struct {
	*tview.Box
	UI *UI

	first bool
}

func NewMapArea(ui *UI) *MapArea {
	mapArea := &MapArea{
		Box: tview.NewBox(),
		UI:  ui,

		first: true,
	}

	mapArea.SetBorder(true)
	mapArea.SetTitle("Map")

	return mapArea
}

const (
	VoidChar   rune = '\u2591'
	BorderChar rune = '\u2580'
)

func (m *MapArea) Draw(screen tcell.Screen) {
	m.Box.DrawForSubclass(screen, m)
	x0, y0, w, h := m.GetInnerRect()

	ctrY := y0 + h/2

	session := m.UI.Session
	g := session.G

	if g == nil {
		tview.Print(screen, "[red:white]No game[-:-]", x0, ctrY, w, tview.AlignCenter, tcell.ColorDefault)
		return
	}

	// draw the map...
	g.mu.RLock()
	defer g.mu.RUnlock()

	lvl := g.Level
	players := g.Players
	mobs := g.Mobs
	effects := g.EffectsOverlay

	pl := players[session.User]
	if pl.S == nil {
		tview.Print(screen, "[red:white]No user[-:-]", x0, ctrY, w, tview.AlignCenter, tcell.ColorDefault)
		return
	}

	plI := pl.I
	plJ := pl.J

	deltaI := h/2 - plI
	deltaJ := w/2 - plJ

	lvlI0 := 0
	lvlJ0 := 0

	lvlI1 := lvl.H
	lvlJ1 := lvl.W

	if deltaI < 0 {
		lvlI0 = -deltaI
	}

	if deltaJ < 0 {
		lvlJ0 = -deltaJ
	}

	if lvlI1+deltaI > h {
		lvlI1 = h - deltaI
	}

	if lvlJ1+deltaJ > w {
		lvlJ1 = w - deltaJ
	}

	style := tcell.StyleDefault.
		Background(tview.Styles.PrimitiveBackgroundColor).
		Foreground(tcell.ColorWhite)
	// Foreground(clr)

	numVoid := 0
	numEmpty := 0
	numBorder := 0
	numWall := 0
	size := 0
	for i := lvlI0; i < lvlI1; i++ {
		y := y0 + i + deltaI
		for j := lvlJ0; j < lvlJ1; j++ {
			x := x0 + j + deltaJ

			var ch rune
			what := lvl.Get(i, j)
			switch what {
			case MarkerVoid:
				ch = '.' // VoidChar
				numVoid++
			case MarkerEmpty:
				ch = ' '
				numEmpty++
			case MarkerBorder:
				ch = BorderChar // FIXME: can do better!
				numBorder++
			case MarkerWall:
				ch = BorderChar // FIXME: can do better!
				numWall++
			default:
				ch = '@'
			}

			screen.SetContent(x, y, ch, nil, style)

			size++
		}
	}

	if m.first {
		session.Message(chat.System, fmt.Sprintf("[%d,%d,%d,%d] pl=(%d,%d) delta=(%d,%d) lvl0=(%d,%d) lvl1=(%d,%d), scr0=(%d,%d)",
			x0, y0, w, h, plJ, plI, deltaJ, deltaI, lvlJ0, lvlI0, lvlJ1, lvlI1, lvlJ0+x0-deltaJ, lvlI0+y0-deltaI))

		session.Message(chat.System, fmt.Sprintf("void: %d, empty: %d, border: %d, wall: %d, size: %d",
			numVoid, numEmpty, numBorder, numWall, size))
	}

	playerStyle := tcell.StyleDefault.
		Background(tcell.ColorBlue).
		Foreground(tcell.ColorWhite)
	for _, pl := range players {
		x := x0 + pl.J + deltaJ
		y := y0 + pl.I + deltaI

		ch := pl.Marker
		if ch == 0 {
			ch = '@'
		}

		if x >= x0 && x < (x0+w) && y >= y0 && y < (y0+h) { // m.InRect(x, y) {
			screen.SetContent(x, y, ch, nil, playerStyle)
		}

		if m.first {
			session.Message(chat.System, fmt.Sprintf("player (%d,%d) x=%d, y=%d, marker=\"%c\"",
				pl.J, pl.I, x, y, ch))
		}
	}

	mobStyle := tcell.StyleDefault.
		Background(tcell.ColorRed).
		Foreground(tcell.ColorWhite)
	for _, mob := range mobs {
		x := x0 + mob.J + deltaJ
		y := y0 + mob.I + deltaI

		mobInfo := LookupMobInfo(mob.Type)
		ch := mobInfo.Marker
		if ch == 0 {
			ch = '@'
		}

		if x >= x0 && x < (x0+w) && y >= y0 && y < (y0+h) { // m.InRect(x, y) {
			screen.SetContent(x, y, ch, nil, mobStyle)
		}

		if m.first {
			session.Message(chat.System, fmt.Sprintf("mob %s (%d,%d) x=%d, y=%d, marker=\"%c\"",
				mobInfo.Name, mob.J, mob.I, x, y, ch))
		}
	}

	collStyle := tcell.StyleDefault.
		Background(tcell.ColorYellow).
		Foreground(tcell.ColorWhite)

	for _, fx := range effects {
		x := x0 + fx.J + deltaJ
		y := x0 + fx.I + deltaI

		sty := tcell.StyleDefault
		if fx.Collision != nil {
			sty = collStyle
		}
		if x >= x0 && x < (x0+w) && y >= y0 && y < (y0+h) {
			screen.SetContent(x, y, fx.Rune, nil, sty)
		}
	}

	m.first = false
}

type Menu struct {
	*tview.Frame
	List *tview.List

	// Layout   *tview.Flex
	Actor Actor
	// Buttons  []*tview.Button
	// Selected int
}

func NewMenu(title string, actor Actor, cancel func()) *Menu {
	// layout := tview.NewFlex()
	// layout.SetDirection(tview.FlexRow)

	lst := tview.NewList().ShowSecondaryText(false)
	f := tview.NewFrame(lst)
	// f := tview.NewFrame(layout)

	f.SetTitle(title)
	f.SetBorder(true)

	m := &Menu{
		Frame: f,
		List:  lst,
		// Layout: layout,
		Actor: actor,
	}

	if cancel != nil {
		lst.SetDoneFunc(cancel)
	}

	return m
}

func (m *Menu) AddButton(label string, shortcut rune, action string) {
	m.AddButtonWithCallback(label, shortcut, func() {
		m.Actor.PerformAction(action)
	})
}

func (m *Menu) AddButtonWithCallback(label string, shortcut rune, cb func()) {
	m.List.AddItem(label, "", shortcut, cb)
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
	// Logger  *log.Logger
	Session *Session
	Lobby   *Lobby

	App *tview.Application

	Pages *tview.Pages

	Menu      tview.Primitive
	Main      tview.Primitive
	LobbyView tview.Primitive

	Focus tview.Primitive

	Status *StatusFrame
	Items  *tview.Box // ItemFrame
	Map    *MapArea

	PageShown  PageNumber
	LastPage   PageNumber
	ModalShown ModalState

	Actions   map[string]func()
	PageNames map[string]tview.Primitive

	AdminLog   *chat.Log
	AdminInput *uilib.InputArea
	// LogView *LogView

	mu sync.Mutex
}

func (ui *UI) quit() {
	ui.App.Stop()
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
			ui.Pages.SwitchToPage("lobby")
		}
	case PageAdmin:
		ui.Pages.SwitchToPage("admin")
	}

	switch ui.ModalShown {
	case ModalMenu:
		ui.Pages.ShowPage("menu")
	}

	if ui.PageShown != page {
		ui.LastPage = ui.PageShown
		ui.PageShown = page
	}

	log.Printf("Page=%v, Modal=%v", ui.PageShown, ui.ModalShown)
}

func (ui *UI) toggleFocus() {
	ui.toggleModal(ModalMenu)
}

func (ui *UI) Run() error {
	return ui.App.Run()
}

func (ui *UI) Quit() {
	ui.App.Stop()
}

func setupAdminPage(ui *UI, sysLog *SystemLog) {
	main := tview.NewPages()

	frame := tview.NewFrame(main)
	frame.SetBorders(0, 0, 0, 0, 0, 0)

	frame.SetBackgroundColor(tcell.ColorBlue)
	frame.AddText("Admin page", true, tview.AlignCenter, tcell.ColorWhite)

	/*
		logMain := tview.NewFlex().SetDirection(tview.FlexRow)

		logView := NewLogViewWithLines(1000)
		logView.SetBorder(true)
		logView.SetTitle("Logs")

		logView.VisibleFunc = func() bool {
			return (ui.PageShown == PageAdmin)
		}

		sysLog.SetCallback(func(line string) {
			logView.AddLine(MsgSystem, line)
		})

		// place holder
		consoleView := tview.NewBox()
		consoleView.SetBorder(true)
		consoleView.SetTitle("Console")

		logMain.AddItem(logView, 0, 1, false)
		logMain.AddItem(consoleView, 5, 1, true)

		logFrame := tview.NewFrame(logMain)

		main.AddPage("logs", logFrame, true, true)
	*/

	adminLog := chat.NewLog(1000)
	sysLog.SetCallback(func(line string) {
		adminLog.LogLine(chat.System, line)
	})

	ui.AdminLog = adminLog
	ui.AdminInput = uilib.NewInputArea(adminLog)

	ui.AdminInput.DirectKeyFunc = func(e *tcell.EventKey) *tcell.EventKey {
		k := e.Key()
		m := e.Modifiers()
		if k == tcell.KeyEsc && m == tcell.ModNone {
			// bring up menu
			ui.toggleModal(ModalMenu)
			return nil
		}

		return e
	}

	ui.AdminInput.SetBorder(true)
	ui.AdminInput.SetTitle("Console")

	main.AddPage("console", ui.AdminInput, true, true)

	ui.Pages.AddPage("admin", frame, true, false)
	ui.PageNames["admin"] = frame

	// ui.LogView = logView
}

func (ui *UI) handleGameKeys(e *tcell.EventKey) *tcell.EventKey {
	k := e.Key()
	m := e.Modifiers()
	r := e.Rune()

	s := ui.Session

	if m == tcell.ModNone {
		switch k {
		case tcell.KeyEsc:
			ui.toggleModal(ModalMenu)

		case tcell.KeyLeft:
			s.Move(MoveLeft)

		case tcell.KeyRight:
			s.Move(MoveRight)

		case tcell.KeyUp:
			s.Move(MoveUp)

		case tcell.KeyDown:
			s.Move(MoveDown)

		case tcell.KeyRune:
			switch r {
			case 'w':
				s.Move(MoveUp)
			case 'a':
				s.Move(MoveLeft)
			case 's':
				s.Move(MoveDown)
			case 'd':
				s.Move(MoveRight)

			case ' ', 'x':
				s.Attack()

			case 'v', 'z':
				s.Defend()

				// case '1', '2', '3', '4', '5':
				// Special

			default:
				return e
			}

		default:
			return e
		}
	} else {
		return e
	}

	return nil
}

func (ui *UI) globalKeyHandler(e *tcell.EventKey) *tcell.EventKey {
	k := e.Key()
	r := e.Rune()
	mods := e.Modifiers()

	if k == tcell.KeyRune && r == 'q' && mods&(tcell.ModAlt|tcell.ModMeta) != 0 {
		ui.quit()
		return nil
	}

	/*
		if k == tcell.KeyEsc && mods == tcell.ModNone {
			// set focus to menu
			ui.toggleFocus()
			return nil
		}
	*/

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
				if ui.PageShown == PageAdmin {
					ui.showPage(ui.LastPage)
				} else {
					ui.showPage(PageAdmin)
				}
				return nil
			}
		}
	}

	return e
}

func (ui *UI) Update() {
	if ui.App != nil {
		ui.App.Draw()
	}
}

type StatusFrame struct {
	*tview.Box
	UI *UI

	cooldowns Cooldowns
}

func NewStatusFrame(ui *UI) *StatusFrame {
	return &StatusFrame{
		Box: tview.NewBox(),
		UI:  ui,
	}
}

func (fr *StatusFrame) horizontalDivider(screen tcell.Screen, y int) {
	_, y0, _, h := fr.GetInnerRect()
	boxX0, _, boxW, _ := fr.GetRect()

	ymax := y0 + h
	if y >= ymax {
		return
	}

	tview.PrintJoinedSemigraphics(screen, boxX0, y, tview.BoxDrawingsLightDownAndRight, tcell.StyleDefault)
	for dx := 1; dx < boxW-1; dx++ {
		tview.PrintJoinedSemigraphics(screen, boxX0+dx, y, tview.BoxDrawingsLightHorizontal, tcell.StyleDefault)
	}
	tview.PrintJoinedSemigraphics(screen, boxX0+boxW-1, y, tview.BoxDrawingsLightDownAndLeft, tcell.StyleDefault)
}

func (fr *StatusFrame) Draw(screen tcell.Screen) {
	fr.Box.DrawForSubclass(screen, fr)

	x0, y0, w, h := fr.GetInnerRect()

	session := fr.UI.Session

	adminStr := ""
	if session.IsAdministrator() {
		adminStr = "[blue:white:b]ADMIN"
	}

	ymax := y0 + h
	y := y0

	s := fmt.Sprintf("[:]%s %s[-:-:-]", session.User, adminStr)
	if w > 0 && h > 0 {
		tview.Print(screen, s, x0, y, w, tview.AlignCenter, tcell.ColorDefault)
	}

	if y++; y >= ymax {
		// ... HANDLE BETTER ...
		return
	}

	fr.horizontalDivider(screen, y)

	if y++; y >= ymax {
		// ... HANDLE BETTER ...
		return
	}

	fr.cooldowns = session.G.GetCooldowns(session, fr.cooldowns)
	cooldowns := fr.cooldowns

	for actInd, nticks := range cooldowns {
		act := ActionType(actInd)

		var s string
		switch act {
		case Nothing:
			continue

		case Move:
			s = "MV "
		case Attack:
			s = "ATT"
		case Defend:
			s = "DEF"
		default:
			s = fmt.Sprintf("[%d]", int(act))
		}

		/*
			clr := tcell.ColorWhite
			if nticks > 0 {
				clr = tcell.ColorGray
			}
		*/

		/*
			style := tcell.StyleDefault.
				Background(tview.Styles.PrimitiveBackgroundColor).
				Foreground(clr)
		*/

		prog := ""
		switch {
		case nticks > 50:
			prog = fmt.Sprintf("<==%d==>", nticks/10)
		case nticks > 20:
			prog = "<===>"
		case nticks > 15:
			prog = "<==>"
		case nticks > 10:
			prog = "<=>"
		case nticks > 5:
			prog = "<>"
		case nticks == 0:
			prog = ""
		}

		var tag string = "[::b]"
		if nticks > 0 {
			tag = ""
		}

		tview.Print(screen, fmt.Sprintf("%s%s %s[-:-]", tag, s, prog), x0, y, w, tview.AlignLeft, tcell.ColorWhite) // clr)

		if y++; y >= ymax {
			// ... HANDLE BETTER ...
			return
		}
	}

	fr.horizontalDivider(screen, y)

	if y++; y >= ymax {
		// ... HANDLE BETTER ...
		return
	}
}

type LobbyScreen struct {
	*tview.Frame
	Menu *tview.List
	RHS  *tview.Pages

	GameList *tview.List

	UI *UI
}

func (l *LobbyScreen) newGame() {
	ui := l.UI
	sess := ui.Session
	lobby := ui.Lobby

	if sess.G != nil {
		// error?
		return
	}

	g, err := lobby.NewGame(sess)
	if err != nil {
		log.Printf("\"%s\" [sess %p] error creating new game: %v", sess.User, sess, err)

		// TODO: popup with error
		return
	}

	log.Printf("\"%s\" [sess %p] created new game: %v", sess.User, sess, g)
	ui.showPage(PageMain)
}

func (l *LobbyScreen) existingGame() {
}

func NewLobbyScreen(ui *UI) *LobbyScreen {
	flx := tview.NewFlex()
	menu := tview.NewList()
	rhs := tview.NewPages()

	scr := &LobbyScreen{
		Frame: tview.NewFrame(flx),

		Menu: menu,
		RHS:  rhs,
		UI:   ui,
	}

	menu.SetBorder(true).SetTitle("Menu")
	menu.ShowSecondaryText(false).
		AddItem("New game", "Creates a new game", 'n', scr.newGame).
		AddItem("Existing game", "Selects an existing game", 'e', scr.existingGame).
		AddItem("Quit", "Quits", 'q', ui.quit)

	gameList := tview.NewList()
	gameList.SetBorder(true).SetTitle("Existing games")

	rhs.AddPage("game_list", gameList, true, false)

	flx.SetDirection(tview.FlexColumn).
		AddItem(menu, 0, 1, true).
		AddItem(rhs, 0, 1, true)

	scr.AddText("Lobby", true, tview.AlignCenter, tcell.ColorWhite)

	return scr
}

func SetupUI(sess *Session, lobby *Lobby, sysLog *SystemLog) *UI {
	app := tview.NewApplication()

	pages := tview.NewPages()

	menu := tview.NewGrid()
	main := tview.NewFlex()

	ui := &UI{
		Session: sess,
		Lobby:   lobby,

		App:   app,
		Pages: pages,
		Menu:  menu,
		Main:  main,

		Focus: nil,

		PageShown:  PageMain,
		LastPage:   PageMain,
		ModalShown: ModalNone,

		Actions:   make(map[string]func()),
		PageNames: make(map[string]tview.Primitive),
	}

	lobbyScr := NewLobbyScreen(ui)
	ui.LobbyView = lobbyScr

	mapArea := NewMapArea(ui)
	statusArea := NewStatusFrame(ui)
	statusArea.SetBorder(true).SetTitle("Status")

	itemView := tview.NewBox().SetBorder(true).SetTitle("Items")

	inputArea := uilib.NewInputArea(ui.Session.SessionLog)
	inputArea.DirectKeyFunc = ui.handleGameKeys
	inputArea.ConsoleInputFunc = ui.Session.ConsoleInput
	inputArea.SetBorder(true).SetTitle("Input")

	ui.Map = mapArea
	ui.Status = statusArea
	ui.Items = itemView

	ui.setupAction("quit", app.Stop)
	ui.setupAction("resume", func() {
		ui.setModal(ModalMenu, false)
	})

	menuBox := NewMenu("System menu", ui, func() {
		ui.setModal(ModalMenu, false)
	})
	menuBox.AddButton("Resume", 'r', "resume")
	menuBox.AddButton("Quit", 'q', "quit")

	menu.SetColumns(0, 40, 0).SetRows(0, 20, 0).AddItem(menuBox, 1, 1, 1, 1, 0, 0, true)

	main.SetDirection(tview.FlexColumn).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(mapArea, 0, 1, false).
			AddItem(inputArea, 10, 1, true), 0, 4, true).
		AddItem(tview.NewFlex().
			SetDirection(tview.FlexRow).
			AddItem(statusArea, 0, 2, false).
			AddItem(itemView, 0, 1, false), 0, 1, false)

	// statusArea, 0, 1, false)

	if sess.IsAdministrator() {
		setupAdminPage(ui, sysLog)
	}

	pages.AddPage("lobby", lobbyScr, true, sess.HasGame() == false)
	ui.PageNames["lobby"] = lobbyScr

	pages.AddPage("main", main, true, sess.HasGame() == true)
	ui.PageNames["main"] = main

	pages.AddPage("menu", menu, true, false)

	app.SetRoot(pages, true)
	app.SetInputCapture(ui.globalKeyHandler)

	log.Printf("set up UI")

	return ui
}
