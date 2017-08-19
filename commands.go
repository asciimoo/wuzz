package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"unicode"

	"github.com/jroimartin/gocui"
	"github.com/nsf/termbox-go"
)

type CommandFunc func(*gocui.Gui, *gocui.View) error

var COMMANDS map[string]func(string, *App) CommandFunc = map[string]func(string, *App) CommandFunc{
	"submit": func(_ string, a *App) CommandFunc {
		return a.SubmitRequest
	},
	"saveResponse": func(_ string, a *App) CommandFunc {
		return func(g *gocui.Gui, _ *gocui.View) error {
			return a.OpenSaveDialog(VIEW_TITLES[SAVE_RESPONSE_DIALOG_VIEW], g,
				func(g *gocui.Gui, _ *gocui.View) error {
					saveLocation := getViewValue(g, SAVE_DIALOG_VIEW)

					if len(a.history) == 0 {
						return nil
					}
					req := a.history[a.historyIndex]
					if req.RawResponseBody == nil {
						return nil
					}

					err := ioutil.WriteFile(saveLocation, req.RawResponseBody, 0644)

					var saveResult string
					if err == nil {
						saveResult = "Response saved successfully."
					} else {
						saveResult = "Error saving response: " + err.Error()
					}
					viewErr := a.OpenSaveResultView(saveResult, g)
					return viewErr
				})
		}
	},
	"loadRequest": func(_ string, a *App) CommandFunc {
		return func(g *gocui.Gui, _ *gocui.View) error {
			return a.OpenSaveDialog(VIEW_TITLES[LOAD_REQUEST_DIALOG_VIEW], g,
				func(g *gocui.Gui, _ *gocui.View) error {
					defer a.closePopup(g, SAVE_DIALOG_VIEW)
					loadLocation := getViewValue(g, SAVE_DIALOG_VIEW)
					return a.LoadRequest(g, loadLocation)
				})
		}
	},
	"saveRequest": func(_ string, a *App) CommandFunc {
		return func(g *gocui.Gui, _ *gocui.View) error {
			return a.OpenSaveDialog(VIEW_TITLES[SAVE_REQUEST_DIALOG_VIEW], g,
				func(g *gocui.Gui, _ *gocui.View) error {
					defer a.closePopup(g, SAVE_DIALOG_VIEW)
					saveLocation := getViewValue(g, SAVE_DIALOG_VIEW)

					var requestMap map[string]string
					requestMap = make(map[string]string)
					requestMap[URL_VIEW] = getViewValue(g, URL_VIEW)
					requestMap[REQUEST_METHOD_VIEW] = getViewValue(g, REQUEST_METHOD_VIEW)
					requestMap[URL_PARAMS_VIEW] = getViewValue(g, URL_PARAMS_VIEW)
					requestMap[REQUEST_DATA_VIEW] = getViewValue(g, REQUEST_DATA_VIEW)
					requestMap[REQUEST_HEADERS_VIEW] = getViewValue(g, REQUEST_HEADERS_VIEW)

					requestJson, err := json.Marshal(requestMap)
					if err != nil {
						return err
					}

					ioerr := ioutil.WriteFile(saveLocation, []byte(requestJson), 0644)

					var saveResult string
					if ioerr == nil {
						saveResult = "Request saved successfully."
					} else {
						saveResult = "Error saving request: " + err.Error()
					}
					viewErr := a.OpenSaveResultView(saveResult, g)

					return viewErr
				})
		}
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
	"openEditor": func(_ string, a *App) CommandFunc {
		return func(g *gocui.Gui, v *gocui.View) error {
			return openEditor(g, v, a.config.General.Editor)
		}
	},
	"toggleContextSpecificSearch": func(_ string, a *App) CommandFunc {
		return func(g *gocui.Gui, _ *gocui.View) error {
			a.config.General.ContextSpecificSearch = !a.config.General.ContextSpecificSearch
			a.PrintBody(g)
			return nil
		}
	},
	"clearHistory": func(_ string, a *App) CommandFunc {
		return func(g *gocui.Gui, _ *gocui.View) error {
			a.history = make([]*Request, 0, 31)
			a.historyIndex = 0
			a.Layout(g)
			return nil
		}
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
	if err != nil || line == "" || cX < 0 {
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

func openEditor(g *gocui.Gui, v *gocui.View, editor string) error {
	file, err := ioutil.TempFile(os.TempDir(), "wuzz-")
	if err != nil {
		return nil
	}
	defer os.Remove(file.Name())

	val := getViewValue(g, v.Name())
	if val != "" {
		fmt.Fprint(file, val)
	}
	file.Close()

	info, err := os.Stat(file.Name())
	if err != nil {
		return nil
	}

	cmd := exec.Command(editor, file.Name())
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	// sync termbox to reset console settings
	// this is required because the external editor can modify the console
	defer g.Update(func(_ *gocui.Gui) error {
		termbox.Sync()
		return nil
	})
	if err != nil {
		rv, _ := g.View(RESPONSE_BODY_VIEW)
		rv.Clear()
		fmt.Fprintf(rv, "Editor open error: %v", err)
		return nil
	}

	newInfo, err := os.Stat(file.Name())
	if err != nil || newInfo.ModTime().Before(info.ModTime()) {
		return nil
	}

	newVal, err := ioutil.ReadFile(file.Name())
	if err != nil {
		return nil
	}

	v.SetCursor(0, 0)
	v.SetOrigin(0, 0)
	v.Clear()
	fmt.Fprint(v, strings.TrimSpace(string(newVal)))

	return nil
}
