package main

import (
	"fmt"
	"strconv"
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

func (s *StatusLineFunctions) HistorySize() string {
	return strconv.Itoa(len(s.app.history))
}

func (s *StatusLineFunctions) RequestNumber() string {
	i := s.app.historyIndex
	if len(s.app.history) > 0 {
		i += 1
	}
	return strconv.Itoa(i)
}

func (s *StatusLineFunctions) SearchType() string {
	if len(s.app.history) > 0 && !s.app.history[s.app.historyIndex].Formatter.Searchable() {
		return "none"
	}
	if s.app.config.General.ContextSpecificSearch {
		return "response specific"
	}
	return "regex"
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
