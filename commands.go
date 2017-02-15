package main

import (
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

func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}
