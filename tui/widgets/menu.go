package widgets

import "github.com/rivo/tview"

type Actor interface {
	PerformAction(action string)
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
