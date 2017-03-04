package main

import (
	"fmt"
	"text/template"

	"github.com/jroimartin/gocui"
)

type StatusLine struct {
	tpl *template.Template
}

type StatusLineFunctions struct {
	app *App
}

func (_ *StatusLineFunctions) Version() string {
	return VERSION
}

func (s *StatusLineFunctions) Duration() string {
	if len(s.app.history) == 0 {
		return ""
	}
	return s.app.history[s.app.historyIndex].Duration.String()
}

func (s *StatusLine) Update(v *gocui.View, a *App) {
	v.Clear()
	err := s.tpl.Execute(v, &StatusLineFunctions{app: a})
	if err != nil {
		fmt.Fprintf(v, "StatusLine update error: %v", err)
	}
}

func NewStatusLine(format string) (*StatusLine, error) {
	tpl, err := template.New("status line").Parse(format)
	if err != nil {
		return nil, err
	}
	return &StatusLine{
		tpl: tpl,
	}, nil
}
