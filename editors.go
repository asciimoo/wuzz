package main

import (
	"github.com/jroimartin/gocui"
)

// Acts as an editor but does nothing
type EmptyEditor struct {
	// (!) Deliberately left empty
}

func (e EmptyEditor) Edit(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
	// (!) Deliberately left empty
}

// Handles BackTab (\033[Z) sequence
type BackTabEditor struct {
	editor                      gocui.Editor
	goBack                      func()
	waitingForSecondBackTabRune bool
}

func (e *BackTabEditor) Edit(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
	if e.waitingForSecondBackTabRune {
		if ch == 'Z' {
			e.goBack()
			e.waitingForSecondBackTabRune = false
			return
		} else {
			e.editor.Edit(v, 0, '[', gocui.ModAlt)
		}
	}

	if ch == '[' && mod == gocui.ModAlt {
		e.waitingForSecondBackTabRune = true
	} else {
		e.editor.Edit(v, key, ch, mod)
	}
}

// Calls search function
type SearchEditor struct {
	editor gocui.Editor
	search func()
}

func (e SearchEditor) Edit(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
	e.editor.Edit(v, key, ch, mod)
	e.search()
}

// Adds home and end buttons functionality
type HomeEndEditor struct {
	editor gocui.Editor
}

func (e HomeEndEditor) Edit(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
	switch key {
	case gocui.KeyHome:
		v.SetCursor(0, 0)
	case gocui.KeyEnd:
		v.SetCursor(len(v.Buffer())-1, 0)
	default:
		e.editor.Edit(v, key, ch, mod)
	}
}

// Removes multi lines capabilities
type SingleLineEditor struct {
	editor gocui.Editor
}

func (e SingleLineEditor) Edit(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
	switch {
	case (ch != 0 || key == gocui.KeySpace) && mod == 0:
		e.editor.Edit(v, key, ch, mod)
		// At the end of the line the default gocui editor adds a whitespace
		// Force him to remove
		o, _ := v.Cursor()
		if o > 1 && o >= len(v.Buffer())-2 {
			v.EditDelete(false)
		}
		return
	case key == gocui.KeyArrowUp:
		key = gocui.KeyHome
	case key == gocui.KeyArrowDown:
		key = gocui.KeyEnd
	case key == gocui.KeyEnter:
		return
	case key == gocui.KeyArrowRight:
		if x, _ := v.Cursor(); x > len(v.Buffer()) {
			return
		}
	}

	e.editor.Edit(v, key, ch, mod)
}
