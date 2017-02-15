package main

import (
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/asciimoo/wuzz/config"

	"crypto/tls"
	"github.com/jroimartin/gocui"
	"github.com/mattn/go-runewidth"
	"github.com/nwidger/jsoncolor"
)

const VERSION = "0.1.1"

const TIMEOUT_DURATION = 5 // in seconds
const WINDOWS_OS = "windows"

const (
	ALL_VIEWS             = ""

	URL_VIEW              = "url"
	URL_PARAMS_VIEW       = "get"
	REQUEST_METHOD_VIEW   = "method"
	REQUEST_DATA_VIEW     = "data"
	REQUEST_HEADERS_VIEW  = "headers"
	SEARCH_VIEW           = "search"
	RESPONSE_HEADERS_VIEW = "response-headers"
	RESPONSE_BODY_VIEW    = "response-body"

	PROMPT_VIEW           = "prompt"
	POPUP_VIEW            = "popup_view"
	ERROR_VIEW            = "error_view"
	HISTORY_VIEW          = "history"
	SAVE_DIALOG_VIEW      = "save-dialog"
	SAVE_RESULT_VIEW      = "save-result"
	METHOD_LIST_VIEW      = "method-list"
)

var VIEW_TITLES = map[string]string{
	URL_VIEW:              "URL (F2) - press ctrl+r to send request",
	URL_PARAMS_VIEW:       "URL params (F3)",
	REQUEST_METHOD_VIEW:   "Method (F4)",
	REQUEST_DATA_VIEW:     "Request data (POST/PUT) (F5)",
	REQUEST_HEADERS_VIEW:  "Request headers (F6)",
	SEARCH_VIEW:           "search> ",
	RESPONSE_HEADERS_VIEW: "Response headers (F8)",
	RESPONSE_BODY_VIEW:    "Response body (F9)",

	POPUP_VIEW:	       "Info",
	ERROR_VIEW:            "Error",
	HISTORY_VIEW:          "History",
	SAVE_DIALOG_VIEW:      "Save Response (enter to submit, ctrl+q to cancel)",
	METHOD_LIST_VIEW:      "Methods",
}

var METHODS []string = []string{
	http.MethodGet,
	http.MethodPost,
	http.MethodPut,
	http.MethodDelete,
	http.MethodPatch,
	http.MethodOptions,
	http.MethodTrace,
	http.MethodConnect,
	http.MethodHead,
}
const DEFAULT_METHOD = http.MethodGet

var SHORTCUTS map[gocui.Key]string = map[gocui.Key]string{
	// gocui.KeyF1 reserved for help popup
	gocui.KeyF2: URL_VIEW,
	gocui.KeyF3: URL_PARAMS_VIEW,
	gocui.KeyF4: REQUEST_METHOD_VIEW,
	gocui.KeyF5: REQUEST_DATA_VIEW,
	gocui.KeyF6: REQUEST_HEADERS_VIEW,
	gocui.KeyF7: SEARCH_VIEW,
	gocui.KeyF8: RESPONSE_HEADERS_VIEW,
	gocui.KeyF9: RESPONSE_BODY_VIEW,
}

var CLIENT *http.Client = &http.Client{
	Timeout: time.Duration(TIMEOUT_DURATION * time.Second),
}
var TRANSPORT *http.Transport = &http.Transport{
	Proxy: http.ProxyFromEnvironment,
}

var VIEWS []string = []string{
	URL_VIEW,
	URL_PARAMS_VIEW,
	REQUEST_METHOD_VIEW,
	REQUEST_DATA_VIEW,
	REQUEST_HEADERS_VIEW,
	SEARCH_VIEW,
	RESPONSE_HEADERS_VIEW,
	RESPONSE_BODY_VIEW,
}

var defaultEditor ViewEditor

const (
	MIN_WIDTH = 60
	MIN_HEIGHT = 20
)

type Request struct {
	Url             string
	Method          string
	GetParams       string
	Data            string
	Headers         string
	ResponseHeaders string
	RawResponseBody []byte
	ContentType     string
}

type App struct {
	viewIndex    int
	historyIndex int
	currentPopup string
	history      []*Request
	config       *config.Config
}

type ViewEditor struct {
	app           *App
	g             *gocui.Gui
	backTabEscape bool
	origEditor    gocui.Editor
}

type SearchEditor struct {
	wuzzEditor *ViewEditor
}

// The singleLineEditor removes multilines capabilities
type singleLineEditor struct {
	wuzzEditor gocui.Editor
}

func init() {
	TRANSPORT.DisableCompression = true
	CLIENT.Transport = TRANSPORT
}

// Editor funcs

func (e *ViewEditor) Edit(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
	// handle back-tab (\033[Z) sequence
	if e.backTabEscape {
		if ch == 'Z' {
			e.app.PrevView(e.g, nil)
			e.backTabEscape = false
			return
		} else {
			e.origEditor.Edit(v, 0, '[', gocui.ModAlt)
		}
	}
	if ch == '[' && mod == gocui.ModAlt {
		e.backTabEscape = true
		return
	}

	e.origEditor.Edit(v, key, ch, mod)
}

func (e *SearchEditor) Edit(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
	e.wuzzEditor.Edit(v, key, ch, mod)
	e.wuzzEditor.g.Execute(func(g *gocui.Gui) error {
		e.wuzzEditor.app.PrintBody(g)
		return nil
	})
}

// The singleLineEditor removes multilines capabilities
func (e singleLineEditor) Edit(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
	switch {
	case (ch != 0 || key == gocui.KeySpace) && mod == 0:
		e.wuzzEditor.Edit(v, key, ch, mod)
		// At the end of the line the default gcui editor adds a whitespace
		// Force him to remove
		ox, _ := v.Cursor()
		if ox > 1 && ox >= len(v.Buffer())-2 {
			v.EditDelete(false)
		}
		return
	case key == gocui.KeyEnter:
		return
	case key == gocui.KeyArrowRight:
		ox, _ := v.Cursor()
		if ox >= len(v.Buffer())-1 {
			return
		}
	case key == gocui.KeyHome || key == gocui.KeyArrowUp:
		v.SetCursor(0, 0)
		return
	case key == gocui.KeyEnd || key == gocui.KeyArrowDown:
		v.SetCursor(len(v.Buffer())-1, 0)
		return
	}
	e.wuzzEditor.Edit(v, key, ch, mod)
}

//

func (a *App) Layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	if maxX < MIN_WIDTH || maxY < MIN_HEIGHT {
		if v, err := g.SetView(ERROR_VIEW, 0, 0, maxX-1, maxY-1); err != nil {
			if err != gocui.ErrUnknownView {
				return err
			}
			setViewDefaults(v)
			v.Title = VIEW_TITLES[ERROR_VIEW]
			g.Cursor = false
			fmt.Fprintln(v, "Terminal is too small")
		}
		return nil
	}
	if _, err := g.View(ERROR_VIEW); err == nil {
		g.DeleteView(ERROR_VIEW)
		g.Cursor = true
		a.setView(g)
	}
	splitX := int(0.3 * float32(maxX))
	splitY := int(0.25 * float32(maxY-3))
	if v, err := g.SetView(URL_VIEW, 0, 0, maxX-1, 3); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		setViewDefaults(v)
		v.Title = VIEW_TITLES[URL_VIEW]
		v.Editable = true
		v.Overwrite = false
		v.Editor = &singleLineEditor{&defaultEditor}
		setViewTextAndCursor(v, a.config.General.DefaultURLScheme+"://")
	}
	if v, err := g.SetView(URL_PARAMS_VIEW, 0, 3, splitX, splitY+1); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		setViewDefaults(v)
		v.Editable = true
		v.Title = VIEW_TITLES[URL_PARAMS_VIEW]
		v.Editor = &defaultEditor
	}
	if v, err := g.SetView(REQUEST_METHOD_VIEW, 0, splitY+1, splitX, splitY+3); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		setViewDefaults(v)
		v.Editable = true
		v.Title = VIEW_TITLES[REQUEST_METHOD_VIEW]
		v.Editor = &singleLineEditor{&defaultEditor}

		setViewTextAndCursor(v, DEFAULT_METHOD)
	}
	if v, err := g.SetView(REQUEST_DATA_VIEW, 0, 3+splitY, splitX, 2*splitY+3); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		setViewDefaults(v)
		v.Editable = true
		v.Title = VIEW_TITLES[REQUEST_DATA_VIEW]
		v.Editor = &defaultEditor
	}
	if v, err := g.SetView(REQUEST_HEADERS_VIEW, 0, 3+(splitY*2), splitX, maxY-2); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		setViewDefaults(v)
		v.Wrap = false
		v.Editable = true
		v.Title = VIEW_TITLES[REQUEST_HEADERS_VIEW]
		v.Editor = &defaultEditor
	}
	if v, err := g.SetView(RESPONSE_HEADERS_VIEW, splitX, 3, maxX-1, splitY+3); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		setViewDefaults(v)
		v.Title = VIEW_TITLES[RESPONSE_HEADERS_VIEW]
		v.Editable = true
		v.Editor = &ViewEditor{a, g, false, gocui.EditorFunc(func(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
			return
		})}
	}
	if v, err := g.SetView(RESPONSE_BODY_VIEW, splitX, 3+splitY, maxX-1, maxY-2); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		setViewDefaults(v)
		v.Title = VIEW_TITLES[RESPONSE_BODY_VIEW]
		v.Editable = true
		v.Editor = &ViewEditor{a, g, false, gocui.EditorFunc(func(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
			return
		})}
	}
	if v, err := g.SetView(PROMPT_VIEW, -1, maxY-2, 7, maxY); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Frame = false
		v.Wrap = true
		setViewTextAndCursor(v, VIEW_TITLES[SEARCH_VIEW])
	}
	if v, err := g.SetView(SEARCH_VIEW, 7, maxY-2, maxX, maxY); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Frame = false
		v.Editable = true
		v.Editor = &singleLineEditor{&SearchEditor{&defaultEditor}}
		v.Wrap = true
	}
	return nil
}

func (a *App) NextView(g *gocui.Gui, v *gocui.View) error {
	a.viewIndex = (a.viewIndex + 1) % len(VIEWS)
	return a.setView(g)
}

func (a *App) PrevView(g *gocui.Gui, v *gocui.View) error {
	a.viewIndex = (a.viewIndex - 1 + len(VIEWS)) % len(VIEWS)
	return a.setView(g)
}

func (a *App) setView(g *gocui.Gui) error {
	a.closePopup(g, a.currentPopup)
	_, err := g.SetCurrentView(VIEWS[a.viewIndex])
	return err
}

func (a *App) setViewByName(g *gocui.Gui, name string) error {
	for i, v := range VIEWS {
		if v == name {
			a.viewIndex = i
			return a.setView(g)
		}
	}
	return fmt.Errorf("View not found")
}

func popup(g *gocui.Gui, msg string) {
	var popup *gocui.View
	var err error
	maxX, maxY := g.Size()
	if popup, err = g.SetView(POPUP_VIEW, maxX/2-len(msg)/2-1, maxY/2-1, maxX/2+len(msg)/2+1, maxY/2+1); err != nil {
		if err != gocui.ErrUnknownView {
			return
		}
		setViewDefaults(popup)
		popup.Title = VIEW_TITLES[POPUP_VIEW]
		setViewTextAndCursor(popup, msg)
		g.SetViewOnTop(POPUP_VIEW)
	}
}

func (a *App) SubmitRequest(g *gocui.Gui, _ *gocui.View) error {
	vrb, _ := g.View(RESPONSE_BODY_VIEW)
	vrb.Clear()
	vrh, _ := g.View(RESPONSE_HEADERS_VIEW)
	vrh.Clear()
	popup(g, "Sending request..")

	var r *Request = &Request{}

	go func(g *gocui.Gui, a *App, r *Request) error {
		defer g.DeleteView(POPUP_VIEW)
		// parse url
		r.Url = getViewValue(g, URL_VIEW)
		u, err := url.Parse(r.Url)
		if err != nil {
			g.Execute(func(g *gocui.Gui) error {
				vrb, _ := g.View(RESPONSE_BODY_VIEW)
				fmt.Fprintf(vrb, "URL parse error: %v", err)
				return nil
			})
			return nil
		}

		q, err := url.ParseQuery(strings.Replace(getViewValue(g, URL_PARAMS_VIEW), "\n", "&", -1))
		if err != nil {
			g.Execute(func(g *gocui.Gui) error {
				vrb, _ := g.View(RESPONSE_BODY_VIEW)
				fmt.Fprintf(vrb, "Invalid GET parameters: %v", err)
				return nil
			})
			return nil
		}
		originalQuery := u.Query()
		for k, v := range q {
			originalQuery.Add(k, strings.Join(v, ""))
		}
		u.RawQuery = originalQuery.Encode()
		r.GetParams = u.RawQuery

		// parse method
		r.Method = getViewValue(g, REQUEST_METHOD_VIEW)

		// parse POST/PUT data
		data := bytes.NewBufferString("")
		r.Data = strings.Replace(getViewValue(g, REQUEST_DATA_VIEW), "\n", "&", -1)
		if r.Method == http.MethodPost || r.Method == http.MethodPut {
			data.WriteString(r.Data)
		}

		// create request
		req, err := http.NewRequest(r.Method, u.String(), data)
		if err != nil {
			g.Execute(func(g *gocui.Gui) error {
				vrb, _ := g.View(RESPONSE_BODY_VIEW)
				fmt.Fprintf(vrb, "Request error: %v", err)
				return nil
			})
			return nil
		}

		// set headers
		req.Header.Set("User-Agent", "")
		r.Headers = getViewValue(g, REQUEST_HEADERS_VIEW)
		headers := strings.Split(r.Headers, "\n")
		for _, header := range headers {
			if header != "" {
				header_parts := strings.SplitN(header, ": ", 2)
				if len(header_parts) != 2 {
					g.Execute(func(g *gocui.Gui) error {
						vrb, _ := g.View(RESPONSE_BODY_VIEW)
						fmt.Fprintf(vrb, "Invalid header: %v", header)
						return nil
					})
					return nil
				}
				req.Header.Set(header_parts[0], header_parts[1])
			}
		}

		// do request
		response, err := CLIENT.Do(req)
		if err != nil {
			g.Execute(func(g *gocui.Gui) error {
				vrb, _ := g.View(RESPONSE_BODY_VIEW)
				fmt.Fprintf(vrb, "Response error: %v", err)
				return nil
			})
			return nil
		}
		defer response.Body.Close()

		// extract body
		r.ContentType = response.Header.Get("Content-Type")
		if response.Header.Get("Content-Encoding") == "gzip" {
			reader, err := gzip.NewReader(response.Body)
			if err == nil {
				defer reader.Close()
				response.Body = reader
			} else {
				g.Execute(func(g *gocui.Gui) error {
					vrb, _ := g.View(RESPONSE_BODY_VIEW)
					fmt.Fprintf(vrb, "Cannot uncompress response: %v", err)
					return nil
				})
				return nil
			}
		}

		bodyBytes, err := ioutil.ReadAll(response.Body)
		if err == nil {
			r.RawResponseBody = bodyBytes
		}

		// add to history
		a.history = append(a.history, r)
		a.historyIndex = len(a.history) - 1

		// render response
		g.Execute(func(g *gocui.Gui) error {
			vrh, _ := g.View(RESPONSE_HEADERS_VIEW)

			a.PrintBody(g)

			// print status code and sorted headers
			hkeys := make([]string, 0, len(response.Header))
			for hname, _ := range response.Header {
				hkeys = append(hkeys, hname)
			}
			sort.Strings(hkeys)
			status_color := 32
			if response.StatusCode != 200 {
				status_color = 31
			}
			header_str := fmt.Sprintf(
				"\x1b[0;%dmHTTP/1.1 %v %v\x1b[0;0m\n",
				status_color,
				response.StatusCode,
				http.StatusText(response.StatusCode),
			)
			for _, hname := range hkeys {
				header_str += fmt.Sprintf("\x1b[0;33m%v:\x1b[0;0m %v\n", hname, strings.Join(response.Header[hname], ","))
			}
			fmt.Fprint(vrh, header_str)
			if _, err := vrh.Line(0); err != nil {
				vrh.SetOrigin(0, 0)
			}
			r.ResponseHeaders = header_str
			return nil
		})
		return nil
	}(g, a, r)

	return nil
}

func (a *App) PrintBody(g *gocui.Gui) {
	g.Execute(func(g *gocui.Gui) error {
		if len(a.history) == 0 {
			return nil
		}
		req := a.history[a.historyIndex]
		if req.RawResponseBody == nil {
			return nil
		}
		vrb, _ := g.View(RESPONSE_BODY_VIEW)
		vrb.Clear()

		responseBody := req.RawResponseBody
		// pretty-print json
		if strings.Contains(req.ContentType, "application/json") && a.config.General.FormatJSON {
			formatter := jsoncolor.NewFormatter()
			buf := bytes.NewBuffer(make([]byte, 0, len(req.RawResponseBody)))
			err := formatter.Format(buf, req.RawResponseBody)
			if err == nil {
				responseBody = buf.Bytes()
			}
		}

		is_binary := strings.Index(req.ContentType, "text") == -1 && strings.Index(req.ContentType, "application") == -1
		search_text := getViewValue(g, SEARCH_VIEW)
		if search_text == "" || is_binary {
			vrb.Title = RESPONSE_BODY_VIEW
			if is_binary {
				vrb.Title += " [binary content]"
				fmt.Fprint(vrb, hex.Dump(req.RawResponseBody))
			} else {
				vrb.Write(responseBody)
			}
			if _, err := vrb.Line(0); !a.config.General.PreserveScrollPosition || err != nil {
				vrb.SetOrigin(0, 0)
			}
			return nil
		}
		vrb.SetOrigin(0, 0)
		search_re, err := regexp.Compile(search_text)
		if err != nil {
			fmt.Fprint(vrb, "Error: invalid search regexp")
			return nil
		}
		results := search_re.FindAll(req.RawResponseBody, 1000)
		if len(results) == 0 {
			vrb.Title = "No results"
			fmt.Fprint(vrb, "Error: no results")
			return nil
		}
		vrb.Title = fmt.Sprintf("%d results", len(results))
		for _, result := range results {
			fmt.Fprintf(vrb, "-----\n%s\n", result)
		}
		return nil
	})
}

func (a *App) SetKeys(g *gocui.Gui) {
	// global key bindings
	g.SetManagerFunc(a.Layout)

	g.SetKeybinding(ALL_VIEWS, gocui.KeyCtrlC, gocui.ModNone, quit)

	g.SetKeybinding(ALL_VIEWS, gocui.KeyTab, gocui.ModNone, a.NextView)
	g.SetKeybinding(ALL_VIEWS, gocui.KeyCtrlJ, gocui.ModNone, a.NextView)
	g.SetKeybinding(ALL_VIEWS, gocui.KeyCtrlK, gocui.ModNone, a.PrevView)
	g.SetKeybinding(REQUEST_METHOD_VIEW, gocui.KeyEnter, gocui.ModNone, a.ToggleMethodList)

	// Cycle for each SHORTCUTS
	for key, view := range SHORTCUTS {
		handler := func(name string) func(*gocui.Gui, *gocui.View) error {
			return func(g *gocui.Gui, _ *gocui.View) error {
				return a.setViewByName(g, name)
			}
		}
		g.SetKeybinding(ALL_VIEWS, key, gocui.ModNone, handler(view))
	}

	if runtime.GOOS != WINDOWS_OS {
		g.SetKeybinding(ALL_VIEWS, gocui.KeyCtrlH, gocui.ModNone, a.ToggleHistory)
	}
	g.SetKeybinding(ALL_VIEWS, 'h', gocui.ModAlt, a.ToggleHistory)

	g.SetKeybinding(ALL_VIEWS, gocui.KeyCtrlS, gocui.ModNone, a.OpenSaveDialog)

	g.SetKeybinding(ALL_VIEWS, gocui.KeyCtrlR, gocui.ModNone, a.SubmitRequest)
	g.SetKeybinding(URL_VIEW, gocui.KeyEnter, gocui.ModNone, a.SubmitRequest)

	// responses common key bindings
	for _, view := range []string{RESPONSE_BODY_VIEW, RESPONSE_HEADERS_VIEW} {
		g.SetKeybinding(view, gocui.KeyArrowUp, gocui.ModNone, scrollViewUp)
		g.SetKeybinding(view, gocui.KeyArrowDown, gocui.ModNone, scrollViewDown)
		g.SetKeybinding(view, gocui.KeyPgup, gocui.ModNone, func(_ *gocui.Gui, v *gocui.View) error {
			_, height := v.Size()
			scrollView(v, -height*2/3)
			return nil
		})
		g.SetKeybinding(view, gocui.KeyPgdn, gocui.ModNone, func(_ *gocui.Gui, v *gocui.View) error {
			_, height := v.Size()
			scrollView(v, height*2/3)
			return nil
		})
	}

	cursDown := func(g *gocui.Gui, v *gocui.View) error {
		cx, cy := v.Cursor()
		v.SetCursor(cx, cy+1)
		return nil
	}
	cursUp := func(g *gocui.Gui, v *gocui.View) error {
		cx, cy := v.Cursor()
		if cy > 0 {
			cy -= 1
		}
		v.SetCursor(cx, cy)
		return nil
	}
	// history key bindings
	g.SetKeybinding(HISTORY_VIEW, gocui.KeyArrowDown, gocui.ModNone, cursDown)
	g.SetKeybinding(HISTORY_VIEW, gocui.KeyArrowUp, gocui.ModNone, cursUp)
	g.SetKeybinding(HISTORY_VIEW, gocui.KeyEnter, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		_, cy := v.Cursor()
		// TODO error
		if len(a.history) <= cy {
			return nil
		}
		a.restoreRequest(g, cy)
		return nil
	})

	// method key bindings
	g.SetKeybinding(REQUEST_METHOD_VIEW, gocui.KeyArrowDown, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		value := strings.TrimSpace(v.Buffer())
		for i, val := range METHODS {
			if val == value && i != len(METHODS)-1 {
				setViewTextAndCursor(v, METHODS[i+1])
			}
		}
		return nil
	})

	g.SetKeybinding(REQUEST_METHOD_VIEW, gocui.KeyArrowUp, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		value := strings.TrimSpace(v.Buffer())
		for i, val := range METHODS {
			if val == value && i != 0 {
				setViewTextAndCursor(v, METHODS[i-1])
			}
		}
		return nil
	})
	g.SetKeybinding(METHOD_LIST_VIEW, gocui.KeyArrowDown, gocui.ModNone, cursDown)
	g.SetKeybinding(METHOD_LIST_VIEW, gocui.KeyArrowUp, gocui.ModNone, cursUp)
	g.SetKeybinding(METHOD_LIST_VIEW, gocui.KeyEnter, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		_, cy := v.Cursor()
		v, _ = g.View(REQUEST_METHOD_VIEW)
		setViewTextAndCursor(v, METHODS[cy])
		a.closePopup(g, METHOD_LIST_VIEW)
		return nil
	})

	g.SetKeybinding(SAVE_DIALOG_VIEW, gocui.KeyEnter, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		defer a.closePopup(g, SAVE_DIALOG_VIEW)

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

		popupTitle := "Save Result (press enter to close)"

		saveResHeight := 1
		saveResWidth := len(saveResult) + 1
		if len(popupTitle)+2 > saveResWidth {
			saveResWidth = len(popupTitle) + 2
		}
		maxX, _ := g.Size()
		if saveResWidth > maxX {
			saveResHeight = saveResWidth/maxX + 1
			saveResWidth = maxX
		}

		saveResultPopup, err := a.CreatePopupView(SAVE_RESULT_VIEW, saveResWidth, saveResHeight, g)
		saveResultPopup.Title = popupTitle
		setViewTextAndCursor(saveResultPopup, saveResult)
		g.SetViewOnTop(SAVE_RESULT_VIEW)
		g.SetCurrentView(SAVE_RESULT_VIEW)

		return err
	})

	g.SetKeybinding(SAVE_DIALOG_VIEW, gocui.KeyCtrlQ, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		a.closePopup(g, SAVE_DIALOG_VIEW)
		return nil
	})

	g.SetKeybinding(SAVE_RESULT_VIEW, gocui.KeyEnter, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		a.closePopup(g, SAVE_RESULT_VIEW)
		return nil
	})
}

func (a *App) closePopup(g *gocui.Gui, viewname string) {
	_, err := g.View(viewname)
	if err == nil {
		a.currentPopup = ""
		g.DeleteView(viewname)
		g.SetCurrentView(VIEWS[a.viewIndex%len(VIEWS)])
		g.Cursor = true
	}
}

// CreatePopupView create a popup like view
func (a *App) CreatePopupView(name string, width, height int, g *gocui.Gui) (v *gocui.View, err error) {
	// Remove any concurrent popup
	a.closePopup(g, a.currentPopup)

	g.Cursor = false
	maxX, maxY := g.Size()
	if height > maxY-1 {
		height = maxY - 1
	}
	if width > maxX-4 {
		width = maxX - 4
	}
	v, err = g.SetView(name, maxX/2-width/2-1, maxY/2-height/2-1, maxX/2+width/2, maxY/2+height/2+1)
	if err != nil && err != gocui.ErrUnknownView {
		return
	}
	err = nil
	v.Wrap = false
	v.Frame = true
	v.Highlight = true
	v.SelFgColor = gocui.ColorYellow
	a.currentPopup = name
	return
}

func (a *App) ToggleHistory(g *gocui.Gui, _ *gocui.View) (err error) {
	// Destroy if present
	if a.currentPopup == HISTORY_VIEW {
		a.closePopup(g, HISTORY_VIEW)
		return
	}

	history, err := a.CreatePopupView(HISTORY_VIEW, 100, len(a.history), g)
	if err != nil {
		return
	}

	history.Title = VIEW_TITLES[HISTORY_VIEW]

	if len(a.history) == 0 {
		setViewTextAndCursor(history, "[!] No items in history")
		return
	}
	for i, r := range a.history {
		req_str := fmt.Sprintf("[%02d] %v %v", i, r.Method, r.Url)
		if r.GetParams != "" {
			req_str += fmt.Sprintf("?%v", strings.Replace(r.GetParams, "\n", "&", -1))
		}
		if r.Data != "" {
			req_str += fmt.Sprintf(" %v", strings.Replace(r.Data, "\n", "&", -1))
		}
		if r.Headers != "" {
			req_str += fmt.Sprintf(" %v", strings.Replace(r.Headers, "\n", ";", -1))
		}
		fmt.Fprintln(history, req_str)
	}
	g.SetViewOnTop(HISTORY_VIEW)
	g.SetCurrentView(HISTORY_VIEW)
	history.SetCursor(0, a.historyIndex)
	return
}

func (a *App) ToggleMethodList(g *gocui.Gui, _ *gocui.View) (err error) {
	// Destroy if present
	if a.currentPopup == METHOD_LIST_VIEW {
		a.closePopup(g, METHOD_LIST_VIEW)
		return
	}

	method, err := a.CreatePopupView(METHOD_LIST_VIEW, 50, len(METHODS), g)
	if err != nil {
		return
	}
	method.Title = VIEW_TITLES[METHOD_LIST_VIEW]

	cur := getViewValue(g, REQUEST_METHOD_VIEW)

	for i, r := range METHODS {
		fmt.Fprintln(method, r)
		if cur == r {
			method.SetCursor(0, i)
		}
	}
	g.SetViewOnTop(METHOD_LIST_VIEW)
	g.SetCurrentView(METHOD_LIST_VIEW)
	return
}

func (a *App) OpenSaveDialog(g *gocui.Gui, _ *gocui.View) (err error) {
	dialog, err := a.CreatePopupView(SAVE_DIALOG_VIEW, 60, 1, g)
	if err != nil {
		return
	}

	g.Cursor = true

	dialog.Title = VIEW_TITLES[SAVE_DIALOG_VIEW]
	dialog.Editable = true
	dialog.Wrap = false

	currentDir, err := os.Getwd()
	if err != nil {
		currentDir = ""
	}
	currentDir += "/"

	setViewTextAndCursor(dialog, currentDir)

	g.SetViewOnTop(SAVE_DIALOG_VIEW)
	g.SetCurrentView(SAVE_DIALOG_VIEW)
	dialog.SetCursor(0, len(currentDir))
	return
}

func (a *App) restoreRequest(g *gocui.Gui, idx int) {
	if idx < 0 || idx >= len(a.history) {
		return
	}
	a.closePopup(g, HISTORY_VIEW)
	a.historyIndex = idx
	r := a.history[idx]

	v, _ := g.View(URL_VIEW)
	setViewTextAndCursor(v, r.Url)

	v, _ = g.View(REQUEST_METHOD_VIEW)
	setViewTextAndCursor(v, r.Method)

	v, _ = g.View(URL_PARAMS_VIEW)
	setViewTextAndCursor(v, r.GetParams)

	v, _ = g.View(REQUEST_DATA_VIEW)
	setViewTextAndCursor(v, r.Data)

	v, _ = g.View(REQUEST_HEADERS_VIEW)
	setViewTextAndCursor(v, r.Headers)

	v, _ = g.View(RESPONSE_HEADERS_VIEW)
	setViewTextAndCursor(v, r.ResponseHeaders)

	a.PrintBody(g)

}

func (a *App) LoadConfig(configPath string) error {
	if configPath == "" {
		// Load config from default path
		configPath = config.GetDefaultConfigLocation()
	}

	// If the config file doesn't exist, load the default config
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		a.config = &config.DefaultConfig
		return nil
	}

	conf, err := config.LoadConfig(configPath)
	if err != nil {
		a.config = &config.DefaultConfig
		return err
	}

	a.config = conf
	return nil
}

func (a *App) ParseArgs(g *gocui.Gui, args []string) error {
	a.Layout(g)
	g.SetCurrentView(VIEWS[a.viewIndex])
	vheader, err := g.View(REQUEST_HEADERS_VIEW)
	if err != nil {
		return errors.New("Too small screen")
	}
	vheader.Clear()
	vget, _ := g.View(URL_PARAMS_VIEW)
	vget.Clear()
	add_content_type := false
	arg_index := 1
	args_len := len(args)
	for arg_index < args_len {
		arg := args[arg_index]
		switch arg {
		case "-H", "--header":
			if arg_index == args_len-1 {
				return errors.New("No header value specified")
			}
			arg_index += 1
			header := args[arg_index]
			fmt.Fprintf(vheader, "%v\n", header)
		case "-d", "--data":
			if arg_index == args_len-1 {
				return errors.New("No POST/PUT value specified")
			}

			vmethod, _ := g.View(REQUEST_METHOD_VIEW)
			setViewTextAndCursor(vmethod, http.MethodPost)

			arg_index += 1
			add_content_type = true

			data, _ := url.QueryUnescape(args[arg_index])
			vdata, _ := g.View(REQUEST_DATA_VIEW)
			setViewTextAndCursor(vdata, data)
		case "-X", "--request":
			if arg_index == args_len-1 {
				return errors.New("No HTTP method specified")
			}
			arg_index++
			method := args[arg_index]
			if method == http.MethodPost || method == http.MethodPut {
				add_content_type = true
			}
			vmethod, _ := g.View(REQUEST_METHOD_VIEW)
			setViewTextAndCursor(vmethod, method)
		case "-t", "--timeout":
			if arg_index == args_len-1 {
				return errors.New("No timeout value specified")
			}
			arg_index += 1
			timeout, err := strconv.Atoi(args[arg_index])
			if err != nil || timeout <= 0 {
				return errors.New("Invalid timeout value")
			}
			a.config.General.Timeout = config.Duration{time.Duration(timeout) * time.Millisecond}
		case "--compressed":
			vh, _ := g.View(REQUEST_HEADERS_VIEW)
			if strings.Index(getViewValue(g, REQUEST_HEADERS_VIEW), "Accept-Encoding") == -1 {
				fmt.Fprintln(vh, "Accept-Encoding: gzip, deflate")
			}
		case "--insecure":
			a.config.General.Insecure = true
		default:
			u := args[arg_index]
			if strings.Index(u, "http://") != 0 && strings.Index(u, "https://") != 0 {
				u = "http://" + u
			}
			parsed_url, err := url.Parse(u)
			if err != nil || parsed_url.Host == "" {
				return errors.New("Invalid url")
			}
			if parsed_url.Path == "" {
				parsed_url.Path = "/"
			}
			vurl, _ := g.View(URL_VIEW)
			vurl.Clear()
			for k, v := range parsed_url.Query() {
				fmt.Fprintf(vget, "%v=%v\n", k, strings.Join(v, ""))
			}
			parsed_url.RawQuery = ""
			setViewTextAndCursor(vurl, parsed_url.String())
		}
		arg_index += 1
	}
	if add_content_type && strings.Index(getViewValue(g, REQUEST_HEADERS_VIEW), "Content-Type") == -1 {
		setViewTextAndCursor(vheader, "Content-Type: application/x-www-form-urlencoded")
	}
	return nil
}

// Apply startup config values. This is run after a.ParseArgs, so that
// args can override the provided config values
func (a *App) InitConfig() {
	CLIENT.Timeout = a.config.General.Timeout.Duration
	TRANSPORT.TLSClientConfig = &tls.Config{InsecureSkipVerify: a.config.General.Insecure}
}

func initApp(a *App, g *gocui.Gui) {
	g.Cursor = true
	g.InputEsc = false
	g.BgColor = gocui.ColorDefault
	g.FgColor = gocui.ColorDefault
	a.SetKeys(g)
}

func getViewValue(g *gocui.Gui, name string) string {
	v, err := g.View(name)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(v.Buffer())
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

func setViewDefaults(v *gocui.View) {
	v.Frame = true
	v.Wrap = true
}

func setViewTextAndCursor(v *gocui.View, s string) {
	v.Clear()
	fmt.Fprint(v, s)
	v.SetCursor(len(s), 0)
}

func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}

func help() {
	fmt.Println(`wuzz - Interactive cli tool for HTTP inspection

Usage: wuzz [-H|--header HEADER]... [-d|--data POST_DATA] [-X|--request METHOD] [-t|--timeout MSECS] [URL]

Other command line options:
  -c, --config PATH   Specify custom configuration file
  -h, --help          Show this
  -v, --version       Display version number

Key bindings:
  ctrl+r              Send request
  ctrl+s              Save response
  tab, ctrl+j         Next window
  shift+tab, ctrl+k   Previous window
  ctrl+h, alt+h       Show history
  pageUp              Scroll up the current window
  pageDown            Scroll down the current window`,
	)
}

func main() {
	configPath := ""
	args := os.Args
	for i, arg := range os.Args {
		switch arg {
		case "-h", "--help":
			help()
			return
		case "-v", "--version":
			fmt.Printf("wuzz %v\n", VERSION)
			return
		case "-c", "--config":
			configPath = os.Args[i+1]
			args = append(os.Args[:i], os.Args[i+2:]...)
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				log.Fatal("Config file specified but does not exist: \"" + configPath + "\"")
			}
		}
	}
	g, err := gocui.NewGui(gocui.Output256)
	if err != nil {
		log.Panicln(err)
	}
	if runtime.GOOS == WINDOWS_OS && runewidth.IsEastAsian() {
		g.Ascii = true
	}

	app := &App{history: make([]*Request, 0, 31)}

	// overwrite default editor
	defaultEditor = ViewEditor{app, g, false, gocui.DefaultEditor}

	initApp(app, g)

	// load config (must be done *before* app.ParseArgs, as arguments
	// should be able to override config values). An empty string passed
	// to LoadConfig results in LoadConfig loading the default config
	// location. If there is no config, the values in
	// config.DefaultConfig will be used.
	err = app.LoadConfig(configPath)
	if err != nil {
		g.Close()
		log.Fatalf("Error loading config file: %v", err)
	}

	err = app.ParseArgs(g, args)

	// Some of the values in the config need to have some startup
	// behavior associated with them. This is run after ParseArgs so
	// that command-line arguments can override configuration values.
	app.InitConfig()

	if err != nil {
		g.Close()
		fmt.Println("Error!", err)
		os.Exit(1)
	}

	defer g.Close()

	if err := g.MainLoop(); err != nil && err != gocui.ErrQuit {
		log.Panicln(err)
	}
}
