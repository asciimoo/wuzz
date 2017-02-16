package main

import (
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
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

const VERSION = "0.1.0"

const TIMEOUT_DURATION = 5 // in seconds
const WINDOWS_OS = "windows"

const (
	ALL_VIEWS = ""

	URL_VIEW              = "url"
	URL_PARAMS_VIEW       = "get"
	REQUEST_METHOD_VIEW   = "method"
	REQUEST_DATA_VIEW     = "data"
	REQUEST_HEADERS_VIEW  = "headers"
	SEARCH_VIEW           = "search"
	RESPONSE_HEADERS_VIEW = "response-headers"
	RESPONSE_BODY_VIEW    = "response-body"

	PROMPT           = "prompt"
	POPUP_VIEW       = "popup_view"
	ERROR_VIEW       = "error_view"
	HISTORY_VIEW     = "history"
	SAVE_DIALOG_VIEW = "save-dialog"
	SAVE_RESULT_VIEW = "save-result"
	METHOD_LIST_VIEW = "method-list"
	HELP_VIEW        = "help"
)

var VIEW_TITLES = map[string]string{
	URL_VIEW:              "URL - press F1 for help",
	URL_PARAMS_VIEW:       "URL params",
	REQUEST_METHOD_VIEW:   "Method",
	REQUEST_DATA_VIEW:     "Request data (POST/PUT/PATCH)",
	REQUEST_HEADERS_VIEW:  "Request headers",
	SEARCH_VIEW:           "search> ",
	RESPONSE_HEADERS_VIEW: "Response headers",
	RESPONSE_BODY_VIEW:    "Response body",

	POPUP_VIEW:       "Info",
	ERROR_VIEW:       "Error",
	HISTORY_VIEW:     "History",
	SAVE_DIALOG_VIEW: "Save Response (enter to submit, ctrl+q to cancel)",
	METHOD_LIST_VIEW: "Methods",
}

type position struct {
	// value = prc * MAX + abs
	pct float32
	abs int
}

type viewPosition struct {
	x0, y0, x1, y1 position
}

var VIEW_POSITIONS = map[string]viewPosition{
	URL_VIEW: {
		position{0.0, 0},
		position{0.0, 0},
		position{1.0, -2},
		position{0.0, 3}},
	URL_PARAMS_VIEW: {
		position{0.0, 0},
		position{0.0, 3},
		position{0.3, 0},
		position{0.25, 0}},
	REQUEST_METHOD_VIEW: {
		position{0.0, 0},
		position{0.25, 0},
		position{0.3, 0},
		position{0.25, 2}},
	REQUEST_DATA_VIEW: {
		position{0.0, 0},
		position{0.25, 2},
		position{0.3, 0},
		position{0.5, 1}},
	REQUEST_HEADERS_VIEW: {
		position{0.0, 0},
		position{0.5, 1},
		position{0.3, 0},
		position{1.0, -3}},
	RESPONSE_HEADERS_VIEW: {
		position{0.3, 0},
		position{0.0, 3},
		position{1.0, -2},
		position{0.25, 2}},
	RESPONSE_BODY_VIEW: {
		position{0.3, 0},
		position{0.25, 2},
		position{1.0, -2},
		position{1.0, -3}},
	SEARCH_VIEW: {
		position{0.0, 7},
		position{1.0, -3},
		position{1.0, -1},
		position{1.0, -1}},
	ERROR_VIEW: {
		position{0.0, 0},
		position{0.0, 0},
		position{1.0, -2},
		position{1.0, -2}},
	PROMPT: {
		position{0.0, -1},
		position{1.0, -3},
		position{0.0, 8},
		position{1.0, -1}},
	POPUP_VIEW: {
		position{0.5, -9999}, // set before usage using len(msg)
		position{0.5, -1},
		position{0.5, -9999}, // set before usage using len(msg)
		position{0.5, 1}},
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
	MIN_WIDTH  = 60
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

func (p position) getCoordinate(max int) int {
	return int(p.pct*float32(max)) + p.abs
}

func setView(g *gocui.Gui, maxX, maxY int, viewName string) (*gocui.View, error) {
	position := VIEW_POSITIONS[viewName]
	return g.SetView(viewName,
		position.x0.getCoordinate(maxX),
		position.y0.getCoordinate(maxY),
		position.x1.getCoordinate(maxX),
		position.y1.getCoordinate(maxY))
}

func (a *App) Layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	maxX++
	maxY++

	if maxX-1 < MIN_WIDTH || maxY-1 < MIN_HEIGHT {
		if v, err := setView(g, maxX, maxY, ERROR_VIEW); err != nil {
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

	if v, err := setView(g, maxX, maxY, URL_VIEW); err != nil {
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
	if v, err := setView(g, maxX, maxY, URL_PARAMS_VIEW); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		setViewDefaults(v)
		v.Editable = true
		v.Title = VIEW_TITLES[URL_PARAMS_VIEW]
		v.Editor = &defaultEditor
	}
	if v, err := setView(g, maxX, maxY, REQUEST_METHOD_VIEW); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		setViewDefaults(v)
		v.Editable = true
		v.Title = VIEW_TITLES[REQUEST_METHOD_VIEW]
		v.Editor = &singleLineEditor{&defaultEditor}

		setViewTextAndCursor(v, DEFAULT_METHOD)
	}
	if v, err := setView(g, maxX, maxY, REQUEST_DATA_VIEW); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		setViewDefaults(v)
		v.Editable = true
		v.Title = VIEW_TITLES[REQUEST_DATA_VIEW]
		v.Editor = &defaultEditor
	}
	if v, err := setView(g, maxX, maxY, REQUEST_HEADERS_VIEW); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		setViewDefaults(v)
		v.Editable = true
		v.Title = VIEW_TITLES[REQUEST_HEADERS_VIEW]
		v.Editor = &defaultEditor
	}
	if v, err := setView(g, maxX, maxY, RESPONSE_HEADERS_VIEW); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		setViewDefaults(v)
		v.Wrap = true
		v.Title = VIEW_TITLES[RESPONSE_HEADERS_VIEW]
		v.Editable = true
		v.Editor = &ViewEditor{a, g, false, gocui.EditorFunc(func(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
			return
		})}
	}
	if v, err := setView(g, maxX, maxY, RESPONSE_BODY_VIEW); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		setViewDefaults(v)
		v.Wrap = true
		v.Title = VIEW_TITLES[RESPONSE_BODY_VIEW]
		v.Editable = true
		v.Editor = &ViewEditor{a, g, false, gocui.EditorFunc(func(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
			return
		})}
	}
	if v, err := setView(g, maxX, maxY, PROMPT); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Frame = false
		setViewTextAndCursor(v, VIEW_TITLES[SEARCH_VIEW])
	}
	if v, err := setView(g, maxX, maxY, SEARCH_VIEW); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Frame = false
		v.Editable = true
		v.Editor = &singleLineEditor{&SearchEditor{&defaultEditor}}
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
	maxX++
	maxY++

	p := VIEW_POSITIONS[POPUP_VIEW]
	p.x0.abs = -len(msg)/2 - 1
	p.x1.abs = len(msg)/2 + 1
	VIEW_POSITIONS[POPUP_VIEW] = p
	if popup, err = setView(g, maxX, maxY, POPUP_VIEW); err != nil {
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

		// set headers
		headers := http.Header{}
		headers.Set("User-Agent", "")
		r.Headers = getViewValue(g, REQUEST_HEADERS_VIEW)
		for _, header := range strings.Split(r.Headers, "\n") {
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
				headers.Set(header_parts[0], header_parts[1])
			}
		}

		var body io.Reader

		// parse POST/PUT/PATCH data
		if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
			bodyStr := getViewValue(g, REQUEST_DATA_VIEW)
			if headers.Get("Content-Type") == "application/x-www-form-urlencoded" {
				bodyStr = strings.Replace(bodyStr, "\n", "&", -1)
			}
			body = bytes.NewBufferString(bodyStr)
		}

		// create request
		req, err := http.NewRequest(r.Method, u.String(), body)
		if err != nil {
			g.Execute(func(g *gocui.Gui) error {
				vrb, _ := g.View(RESPONSE_BODY_VIEW)
				fmt.Fprintf(vrb, "Request error: %v", err)
				return nil
			})
			return nil
		}
		req.Header = headers

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

func parseKey(k string) (interface{}, gocui.Modifier, error) {
	mod := gocui.ModNone
	if strings.Index(k, "Alt") == 0 {
		mod = gocui.ModAlt
		k = k[3:]
	}
	switch len(k) {
	case 0:
		return 0, 0, errors.New("Empty key string")
	case 1:
		if mod != gocui.ModNone {
			k = strings.ToLower(k)
		}
		return rune(k[0]), mod, nil
	}

	key, found := KEYS[k]
	if !found {
		return 0, 0, fmt.Errorf("Unknown key: %v", k)
	}
	return key, mod, nil
}

func (a *App) setKey(g *gocui.Gui, keyStr, commandStr, viewName string) error {
	if commandStr == "" {
		return nil
	}
	key, mod, err := parseKey(keyStr)
	if err != nil {
		return err
	}
	commandParts := strings.SplitN(commandStr, " ", 2)
	command := commandParts[0]
	var commandArgs string
	if len(commandParts) == 2 {
		commandArgs = commandParts[1]
	}
	keyFnGen, found := COMMANDS[command]
	if !found {
		return fmt.Errorf("Unknown command: %v", command)
	}
	keyFn := keyFnGen(commandArgs, a)
	if err := g.SetKeybinding(viewName, key, mod, keyFn); err != nil {
		return fmt.Errorf("Failed to set key '%v': %v", keyStr, err)
	}
	return nil
}

func (a *App) printViewKeybindings(v io.Writer, viewName string) {
	keys, found := a.config.Keys[viewName]
	if !found {
		return
	}
	mk := make([]string, len(keys))
	i := 0
	for k, _ := range keys {
		mk[i] = k
		i++
	}
	sort.Strings(mk)
	fmt.Fprintf(v, "\n %v\n", viewName)
	for _, key := range mk {
		fmt.Fprintf(v, "  %-15v %v\n", key, keys[key])
	}
}

func (a *App) SetKeys(g *gocui.Gui) error {
	// load config keybindings
	for viewName, keys := range a.config.Keys {
		if viewName == "global" {
			viewName = ALL_VIEWS
		}
		for keyStr, commandStr := range keys {
			if err := a.setKey(g, keyStr, commandStr, viewName); err != nil {
				return err
			}
		}
	}

	g.SetKeybinding(ALL_VIEWS, gocui.KeyF1, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		if a.currentPopup == HELP_VIEW {
			a.closePopup(g, HELP_VIEW)
			return nil
		}

		help, err := a.CreatePopupView(HELP_VIEW, 60, 40, g)
		if err != nil {
			return err
		}
		help.Title = "Help"
		help.Highlight = false
		fmt.Fprint(help, "Keybindings:\n")
		a.printViewKeybindings(help, "global")
		for _, viewName := range VIEWS {
			if _, found := a.config.Keys[viewName]; !found {
				continue
			}
			a.printViewKeybindings(help, viewName)
		}
		g.SetViewOnTop(HELP_VIEW)
		g.SetCurrentView(HELP_VIEW)
		return nil
	})

	g.SetKeybinding(REQUEST_METHOD_VIEW, gocui.KeyEnter, gocui.ModNone, a.ToggleMethodList)

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
	return nil
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
	if height > maxY-4 {
		height = maxY - 4
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
		a.config.Keys = config.DefaultKeys
		return nil
	}

	conf, err := config.LoadConfig(configPath)
	if err != nil {
		a.config = &config.DefaultConfig
		a.config.Keys = config.DefaultKeys
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
	set_data := false
	set_method := false
	set_binary_data := false
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
		case "-d", "--data", "--data-binary":
			if arg_index == args_len-1 {
				return errors.New("No POST/PUT/PATCH value specified")
			}

			arg_index += 1
			set_data = true
			set_binary_data = arg == "--data-binary"

			data := args[arg_index]
			if !set_binary_data {
				data, _ = url.QueryUnescape(data)
				add_content_type = true
			}
			vdata, _ := g.View(REQUEST_DATA_VIEW)
			setViewTextAndCursor(vdata, data)
		case "-X", "--request":
			if arg_index == args_len-1 {
				return errors.New("No HTTP method specified")
			}
			arg_index++
			set_method = true
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

	if set_data && !set_method {
		vmethod, _ := g.View(REQUEST_METHOD_VIEW)
		setViewTextAndCursor(vmethod, http.MethodPost)
	}

	if !set_binary_data && add_content_type && strings.Index(getViewValue(g, REQUEST_HEADERS_VIEW), "Content-Type") == -1 {
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
	g.SetManagerFunc(a.Layout)
}

func getViewValue(g *gocui.Gui, name string) string {
	v, err := g.View(name)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(v.Buffer())
}

func setViewDefaults(v *gocui.View) {
	v.Frame = true
	v.Wrap = false
}

func setViewTextAndCursor(v *gocui.View, s string) {
	v.Clear()
	fmt.Fprint(v, s)
	v.SetCursor(len(s), 0)
}

func help() {
	fmt.Println(`wuzz - Interactive cli tool for HTTP inspection

Usage: wuzz [-H|--header HEADER]... [-d|--data|--data-binary DATA] [-X|--request METHOD] [-t|--timeout MSECS] [URL]

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
		g.ASCII = true
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

	err = app.SetKeys(g)

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
