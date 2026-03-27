package main

import (
	"fmt"
	"strconv"
	"text/template"

	"github.com/awesome-gocui/gocui"
	"github.com/hasithdealwis/wuzz/pkgs/history"
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
	entries, err := history.GetHistory()
	if err != nil || len(entries) == 0 {
		return ""
	}
	return entries[0].Duration.String()
}

func (s *StatusLineFunctions) HistorySize() string {
	count, err := history.Count()
	if err != nil {
		return "0"
	}
	return strconv.Itoa(count)
}

func (s *StatusLineFunctions) RequestNumber() string {
	count, err := history.Count()
	if err != nil || count == 0 {
		return "0"
	}
	return "1"
}

func (s *StatusLineFunctions) SearchType() string {
	entries, err := history.GetHistory()
	if err == nil && len(entries) > 0 {
		req := history.EntryToRequest(entries[0], s.app.config)
		if !req.Formatter.Searchable() {
			return "none"
		}
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

func (s *StatusLineFunctions) DisableRedirect() string {
	if s.app.config.General.FollowRedirects {
		return ""
	}
	return "Activated"
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
