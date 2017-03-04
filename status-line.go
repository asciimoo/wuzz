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
}

var STATUSLINE_FUNCTIONS = &StatusLineFunctions{}

func (_ *StatusLineFunctions) Version() string {
	return VERSION
}

func (s *StatusLine) Update(v *gocui.View) {
	v.Clear()
	err := s.tpl.Execute(v, STATUSLINE_FUNCTIONS)
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
