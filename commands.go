package main

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/jroimartin/gocui"
)

type CommandFunc func(*gocui.Gui, *gocui.View) error

var COMMANDS map[string]func(string, *App) CommandFunc = map[string]func(string, *App) CommandFunc{
	"submit": func(_ string, a *App) CommandFunc {
		return a.SubmitRequest
	},
	"save": func(_ string, a *App) CommandFunc {
		return a.OpenSaveDialog
	},
	"history": func(_ string, a *App) CommandFunc {
		return a.ToggleHistory
	},
	"quit": func(_ string, _ *App) CommandFunc {
		return quit
	},
	"focus": func(args string, a *App) CommandFunc {
		return func(g *gocui.Gui, _ *gocui.View) error {
			return a.setViewByName(g, args)
		}
	},
	"nextView": func(_ string, a *App) CommandFunc {
		return a.NextView
	},
	"prevView": func(_ string, a *App) CommandFunc {
		return a.PrevView
	},
	"scrollDown": func(_ string, _ *App) CommandFunc {
		return scrollViewDown
	},
	"scrollUp": func(_ string, _ *App) CommandFunc {
		return scrollViewUp
	},
	"pageDown": func(_ string, _ *App) CommandFunc {
		return pageDown
	},
	"pageUp": func(_ string, _ *App) CommandFunc {
		return pageUp
	},
	"deleteLine": func(_ string, _ *App) CommandFunc {
		return deleteLine
	},
	"deleteWord": func(_ string, _ *App) CommandFunc {
		return deleteWord
	},
}

func scrollView(v *gocui.View, dy int) error {
	v.Autoscroll = false
	ox, oy := v.Origin()
	if oy+dy < 0 {
		dy = -oy
	}
	if _, err := v.Line(dy); dy > 0 && err != nil {
		dy = 0
	}
	v.SetOrigin(ox, oy+dy)
	return nil
}

func scrollViewUp(_ *gocui.Gui, v *gocui.View) error {
	return scrollView(v, -1)
}

func scrollViewDown(_ *gocui.Gui, v *gocui.View) error {
	return scrollView(v, 1)
}

func pageUp(_ *gocui.Gui, v *gocui.View) error {
	_, height := v.Size()
	scrollView(v, -height*2/3)
	return nil
}

func pageDown(_ *gocui.Gui, v *gocui.View) error {
	_, height := v.Size()
	scrollView(v, height*2/3)
	return nil
}

func deleteLine(_ *gocui.Gui, v *gocui.View) error {
	if !v.Editable {
		return nil
	}
	_, curY := v.Cursor()
	_, oY := v.Origin()
	currentLine := curY + oY
	viewLines := strings.Split(strings.TrimSpace(v.Buffer()), "\n")
	if currentLine >= len(viewLines) {
		return nil
	}
	v.Clear()
	if currentLine > 0 {
		fmt.Fprintln(v, strings.Join(viewLines[:currentLine], "\n"))
	}
	fmt.Fprint(v, strings.Join(viewLines[currentLine+1:], "\n"))
	v.SetCursor(0, currentLine)
	v.SetOrigin(0, oY)
	return nil
}

func deleteWord(_ *gocui.Gui, v *gocui.View) error {
	cX, cY := v.Cursor()
	oX, _ := v.Origin()
	cX = cX - 1 + oX
	line, err := v.Line(cY)
	if err != nil || line == "" || cX <= 0 {
		return nil
	}
	if cX >= len(line) {
		cX = len(line) - 1
	}
	origCharCateg := getCharCategory(rune(line[cX]))
	v.EditDelete(true)
	cX -= 1
	for cX >= 0 {
		c := rune(line[cX])
		if origCharCateg != getCharCategory(c) {
			break
		}
		v.EditDelete(true)
		cX -= 1
	}
	return nil
}

func getCharCategory(chr rune) int {
	switch {
	case unicode.IsDigit(chr):
		return 0
	case unicode.IsLetter(chr):
		return 1
	case unicode.IsSpace(chr):
		return 2
	case unicode.IsPunct(chr):
		return 3
	}
	return int(chr)
}

func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}
