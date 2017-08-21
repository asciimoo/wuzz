package main

import (
	"bytes"
	"compress/gzip"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/proxy"

	"github.com/asciimoo/wuzz/config"
	"github.com/asciimoo/wuzz/formatter"

	"github.com/jroimartin/gocui"
	"github.com/mattn/go-runewidth"
	"github.com/nsf/termbox-go"
)

const VERSION = "0.4.0"

const TIMEOUT_DURATION = 5 // in seconds
const WINDOWS_OS = "windows"
const SEARCH_PROMPT = "search> "

const (
	ALL_VIEWS = ""

	URL_VIEW              = "url"
	URL_PARAMS_VIEW       = "get"
	REQUEST_METHOD_VIEW   = "method"
	REQUEST_DATA_VIEW     = "data"
	REQUEST_HEADERS_VIEW  = "headers"
	STATUSLINE_VIEW       = "status-line"
	SEARCH_VIEW           = "search"
	RESPONSE_HEADERS_VIEW = "response-headers"
	RESPONSE_BODY_VIEW    = "response-body"

	SEARCH_PROMPT_VIEW        = "prompt"
	POPUP_VIEW                = "popup_view"
	AUTOCOMPLETE_VIEW         = "autocomplete_view"
	ERROR_VIEW                = "error_view"
	HISTORY_VIEW              = "history"
	SAVE_DIALOG_VIEW          = "save-dialog"
	SAVE_RESPONSE_DIALOG_VIEW = "save-response-dialog"
	LOAD_REQUEST_DIALOG_VIEW  = "load-request-dialog"
	SAVE_REQUEST_DIALOG_VIEW  = "save-request-dialog"
	SAVE_RESULT_VIEW          = "save-result"
	METHOD_LIST_VIEW          = "method-list"
	HELP_VIEW                 = "help"
)

var VIEW_TITLES = map[string]string{
	POPUP_VIEW:                "Info",
	ERROR_VIEW:                "Error",
	HISTORY_VIEW:              "History",
	SAVE_RESPONSE_DIALOG_VIEW: "Save Response (enter to submit, ctrl+q to cancel)",
	LOAD_REQUEST_DIALOG_VIEW:  "Load Request (enter to submit, ctrl+q to cancel)",
	SAVE_REQUEST_DIALOG_VIEW:  "Save Request (enter to submit, ctrl+q to cancel)",
	SAVE_RESULT_VIEW:          "Save Result (press enter to close)",
	METHOD_LIST_VIEW:          "Methods",
	HELP_VIEW:                 "Help",
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
	STATUSLINE_VIEW: {
		position{0.0, -1},
		position{1.0, -4},
		position{1.0, 0},
		position{1.0, -1}},
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
	SEARCH_PROMPT_VIEW: {
		position{0.0, -1},
		position{1.0, -3},
		position{0.0, 8},
		position{1.0, -1}},
	POPUP_VIEW: {
		position{0.5, -9999}, // set before usage using len(msg)
		position{0.5, -1},
		position{0.5, -9999}, // set before usage using len(msg)
		position{0.5, 1}},
	AUTOCOMPLETE_VIEW: {
		position{0, -9999},
		position{0, -9999},
		position{0, -9999},
		position{0, -9999}},
}

type viewProperties struct {
	title    string
	frame    bool
	editable bool
	wrap     bool
	editor   gocui.Editor
	text     string
}

var VIEW_PROPERTIES = map[string]viewProperties{
	URL_VIEW: {
		title:    "URL - press F1 for help",
		frame:    true,
		editable: true,
		wrap:     false,
		editor:   &singleLineEditor{&defaultEditor},
	},
	URL_PARAMS_VIEW: {
		title:    "URL params",
		frame:    true,
		editable: true,
		wrap:     false,
		editor:   &defaultEditor,
	},
	REQUEST_METHOD_VIEW: {
		title:    "Method",
		frame:    true,
		editable: true,
		wrap:     false,
		editor:   &singleLineEditor{&defaultEditor},
		text:     DEFAULT_METHOD,
	},
	REQUEST_DATA_VIEW: {
		title:    "Request data (POST/PUT/PATCH)",
		frame:    true,
		editable: true,
		wrap:     false,
		editor:   &defaultEditor,
	},
	REQUEST_HEADERS_VIEW: {
		title:    "Request headers",
		frame:    true,
		editable: true,
		wrap:     false,
		editor: &AutocompleteEditor{&defaultEditor, func(str string) []string {
			return completeFromSlice(str, REQUEST_HEADERS)
		}, []string{}, false},
	},
	RESPONSE_HEADERS_VIEW: {
		title:    "Response headers",
		frame:    true,
		editable: true,
		wrap:     true,
		editor:   nil, // should be set using a.getViewEditor(g)
	},
	RESPONSE_BODY_VIEW: {
		title:    "Response body",
		frame:    true,
		editable: true,
		wrap:     true,
		editor:   nil, // should be set using a.getViewEditor(g)
	},
	SEARCH_VIEW: {
		title:    "",
		frame:    false,
		editable: true,
		wrap:     false,
		editor:   &singleLineEditor{&SearchEditor{&defaultEditor}},
	},
	STATUSLINE_VIEW: {
		title:    "",
		frame:    false,
		editable: false,
		wrap:     false,
		editor:   nil,
		text:     "",
	},
	SEARCH_PROMPT_VIEW: {
		title:    "",
		frame:    false,
		editable: false,
		wrap:     false,
		editor:   nil,
		text:     SEARCH_PROMPT,
	},
	POPUP_VIEW: {
		title:    "Info",
		frame:    true,
		editable: false,
		wrap:     false,
		editor:   nil,
	},
	AUTOCOMPLETE_VIEW: {
		title:    "",
		frame:    false,
		editable: false,
		wrap:     false,
		editor:   nil,
	},
}

var METHODS = []string{
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

var DEFAULT_FORMATTER = &formatter.TextFormatter{}

var CLIENT = &http.Client{
	Timeout: time.Duration(TIMEOUT_DURATION * time.Second),
}
var TRANSPORT = &http.Transport{
	Proxy: http.ProxyFromEnvironment,
}

var VIEWS = []string{
	URL_VIEW,
	URL_PARAMS_VIEW,
	REQUEST_METHOD_VIEW,
	REQUEST_DATA_VIEW,
	REQUEST_HEADERS_VIEW,
	SEARCH_VIEW,
	RESPONSE_HEADERS_VIEW,
	RESPONSE_BODY_VIEW,
}

var TLS_VERSIONS = map[string]uint16{
	"SSL3.0": tls.VersionSSL30,
	"TLS1.0": tls.VersionTLS10,
	"TLS1.1": tls.VersionTLS11,
	"TLS1.2": tls.VersionTLS12,
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
	Duration        time.Duration
	Formatter       formatter.ResponseFormatter
}

type App struct {
	viewIndex    int
	historyIndex int
	currentPopup string
	history      []*Request
	config       *config.Config
	statusLine   *StatusLine
}

type ViewEditor struct {
	app           *App
	g             *gocui.Gui
	backTabEscape bool
	origEditor    gocui.Editor
}

type AutocompleteEditor struct {
	wuzzEditor         *ViewEditor
	completions        func(string) []string
	currentCompletions []string
	isAutocompleting   bool
}

type SearchEditor struct {
	wuzzEditor *ViewEditor
}

// The singleLineEditor removes multi lines capabilities
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

	// disable infinite down scroll
	if key == gocui.KeyArrowDown && mod == gocui.ModNone {
		_, cY := v.Cursor()
		_, err := v.Line(cY)
		if err != nil {
			return
		}
	}

	e.origEditor.Edit(v, key, ch, mod)
}

var symbolPattern = regexp.MustCompile("[a-zA-Z0-9-]+$")

func getLastSymbol(str string) string {
	return symbolPattern.FindString(str)
}

func completeFromSlice(str string, completions []string) []string {
	completed := []string{}
	if str == "" || strings.TrimRight(str, " \n") != str {
		return completed
	}
	for _, completion := range completions {
		if strings.HasPrefix(completion, str) && str != completion {
			completed = append(completed, completion)
		}
	}
	return completed
}

func (e *AutocompleteEditor) Edit(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
	if key != gocui.KeyEnter {
		e.wuzzEditor.Edit(v, key, ch, mod)
	}

	cx, cy := v.Cursor()
	line, err := v.Line(cy)
	trimmedLine := line[:cx]

	if err != nil {
		e.wuzzEditor.Edit(v, key, ch, mod)
		return
	}

	lastSymbol := getLastSymbol(trimmedLine)
	if key == gocui.KeyEnter && e.isAutocompleting {
		currentCompletion := e.currentCompletions[0]
		shouldDelete := true
		if len(e.currentCompletions) == 1 {
			shouldDelete = false
		}

		if shouldDelete {
			for range lastSymbol {
				v.EditDelete(true)
			}
		}
		for _, char := range currentCompletion {
			v.EditWrite(char)
		}
		closeAutocomplete(e.wuzzEditor.g)
		e.isAutocompleting = false
		return
	} else if key == gocui.KeyEnter {
		e.wuzzEditor.Edit(v, key, ch, mod)
	}

	closeAutocomplete(e.wuzzEditor.g)
	e.isAutocompleting = false

	completions := e.completions(lastSymbol)
	e.currentCompletions = completions

	cx, cy = v.Cursor()
	sx, _ := v.Size()
	ox, oy, _, _, _ := e.wuzzEditor.g.ViewPosition(v.Name())

	maxWidth := sx - cx
	maxHeight := 10

	if len(completions) > 0 {
		comps := completions
		x := ox + cx
		y := oy + cy
		if len(comps) == 1 {
			comps[0] = comps[0][len(lastSymbol):]
		} else {
			y += 1
			x -= len(lastSymbol)
			maxWidth += len(lastSymbol)
		}
		showAutocomplete(comps, x, y, maxWidth, maxHeight, e.wuzzEditor.g)
		e.isAutocompleting = true
	}
}

func (e *SearchEditor) Edit(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
	e.wuzzEditor.Edit(v, key, ch, mod)
	e.wuzzEditor.g.Update(func(g *gocui.Gui) error {
		e.wuzzEditor.app.PrintBody(g)
		return nil
	})
}

// The singleLineEditor removes multi lines capabilities
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
		v.SetOrigin(0, 0)
		return
	case key == gocui.KeyEnd || key == gocui.KeyArrowDown:
		width, _ := v.Size()
		lineWidth := len(v.Buffer()) - 1
		if lineWidth > width {
			v.SetOrigin(lineWidth-width, 0)
			lineWidth = width - 1
		}
		v.SetCursor(lineWidth, 0)
		return
	}
	e.wuzzEditor.Edit(v, key, ch, mod)
}

//

func (a *App) getResponseViewEditor(g *gocui.Gui) gocui.Editor {
	return &ViewEditor{a, g, false, gocui.EditorFunc(func(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
		return
	})}
}

func (p position) getCoordinate(max int) int {
	return int(p.pct*float32(max)) + p.abs
}

func setView(g *gocui.Gui, viewName string) (*gocui.View, error) {
	maxX, maxY := g.Size()
	position := VIEW_POSITIONS[viewName]
	return g.SetView(viewName,
		position.x0.getCoordinate(maxX+1),
		position.y0.getCoordinate(maxY+1),
		position.x1.getCoordinate(maxX+1),
		position.y1.getCoordinate(maxY+1))
}

func setViewProperties(v *gocui.View, name string) {
	v.Title = VIEW_PROPERTIES[name].title
	v.Frame = VIEW_PROPERTIES[name].frame
	v.Editable = VIEW_PROPERTIES[name].editable
	v.Wrap = VIEW_PROPERTIES[name].wrap
	v.Editor = VIEW_PROPERTIES[name].editor
	setViewTextAndCursor(v, VIEW_PROPERTIES[name].text)
}

func (a *App) Layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()

	if maxX < MIN_WIDTH || maxY < MIN_HEIGHT {
		if v, err := setView(g, ERROR_VIEW); err != nil {
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

	for _, name := range []string{RESPONSE_HEADERS_VIEW, RESPONSE_BODY_VIEW} {
		vp := VIEW_PROPERTIES[name]
		vp.editor = a.getResponseViewEditor(g)
		VIEW_PROPERTIES[name] = vp
	}

	if a.config.General.DefaultURLScheme != "" && !strings.HasSuffix(a.config.General.DefaultURLScheme, "://") {
		p := VIEW_PROPERTIES[URL_VIEW]
		p.text = a.config.General.DefaultURLScheme + "://"
		VIEW_PROPERTIES[URL_VIEW] = p
	}

	for _, name := range []string{
		URL_VIEW,
		URL_PARAMS_VIEW,
		REQUEST_METHOD_VIEW,
		REQUEST_DATA_VIEW,
		REQUEST_HEADERS_VIEW,
		RESPONSE_HEADERS_VIEW,
		RESPONSE_BODY_VIEW,
		STATUSLINE_VIEW,
		SEARCH_PROMPT_VIEW,
		SEARCH_VIEW,
	} {
		if v, err := setView(g, name); err != nil {
			if err != gocui.ErrUnknownView {
				return err
			}
			setViewProperties(v, name)
		}
	}
	sv, _ := g.View(STATUSLINE_VIEW)
	sv.BgColor = gocui.ColorDefault | gocui.AttrReverse
	sv.FgColor = gocui.ColorDefault | gocui.AttrReverse
	a.statusLine.Update(sv, a)

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
	pos := VIEW_POSITIONS[POPUP_VIEW]
	pos.x0.abs = -len(msg)/2 - 1
	pos.x1.abs = len(msg)/2 + 1
	VIEW_POSITIONS[POPUP_VIEW] = pos

	p := VIEW_PROPERTIES[POPUP_VIEW]
	p.text = msg
	VIEW_PROPERTIES[POPUP_VIEW] = p

	if v, err := setView(g, POPUP_VIEW); err != nil {
		if err != gocui.ErrUnknownView {
			return
		}
		setViewProperties(v, POPUP_VIEW)
		g.SetViewOnTop(POPUP_VIEW)
	}
}

func minInt(x, y int) int {
	if x < y {
		return x
	}
	return y
}

func closeAutocomplete(g *gocui.Gui) {
	g.DeleteView(AUTOCOMPLETE_VIEW)
}

func showAutocomplete(completions []string, left, top, maxWidth, maxHeight int, g *gocui.Gui) {
	// Get the width of the widest completion
	completionsWidth := 0
	for _, completion := range completions {
		thisCompletionWidth := len(completion)
		if thisCompletionWidth > completionsWidth {
			completionsWidth = thisCompletionWidth
		}
	}

	// Get the width and height of the autocomplete window
	width := minInt(completionsWidth, maxWidth)
	height := minInt(len(completions), maxHeight)

	newPos := viewPosition{
		x0: position{0, left},
		y0: position{0, top},
		x1: position{0, left + width + 1},
		y1: position{0, top + height + 1},
	}

	VIEW_POSITIONS[AUTOCOMPLETE_VIEW] = newPos

	p := VIEW_PROPERTIES[AUTOCOMPLETE_VIEW]
	p.text = strings.Join(completions, "\n")
	VIEW_PROPERTIES[AUTOCOMPLETE_VIEW] = p

	if v, err := setView(g, AUTOCOMPLETE_VIEW); err != nil {
		if err != gocui.ErrUnknownView {
			return
		}
		setViewProperties(v, AUTOCOMPLETE_VIEW)
		v.BgColor = gocui.ColorBlue
		v.FgColor = gocui.ColorDefault
		g.SetViewOnTop(AUTOCOMPLETE_VIEW)
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
			g.Update(func(g *gocui.Gui) error {
				vrb, _ := g.View(RESPONSE_BODY_VIEW)
				fmt.Fprintf(vrb, "URL parse error: %v", err)
				return nil
			})
			return nil
		}

		q, err := url.ParseQuery(strings.Replace(getViewValue(g, URL_PARAMS_VIEW), "\n", "&", -1))
		if err != nil {
			g.Update(func(g *gocui.Gui) error {
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
					g.Update(func(g *gocui.Gui) error {
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
			if headers.Get("Content-Type") != "multipart/form-data" {
				if headers.Get("Content-Type") == "application/x-www-form-urlencoded" {
					bodyStr = strings.Replace(bodyStr, "\n", "&", -1)
				}
				body = bytes.NewBufferString(bodyStr)
			} else {
				var bodyBytes bytes.Buffer
				multiWriter := multipart.NewWriter(&bodyBytes)
				defer multiWriter.Close()
				postData, err := url.ParseQuery(strings.Replace(getViewValue(g, REQUEST_DATA_VIEW), "\n", "&", -1))
				if err != nil {
					return err
				}
				for postKey, postValues := range postData {
					for i := range postValues {
						if len([]rune(postValues[i])) > 0 && postValues[i][0] == '@' {
							file, err := os.Open(postValues[i][1:])
							if err != nil {
								g.Update(func(g *gocui.Gui) error {
									vrb, _ := g.View(RESPONSE_BODY_VIEW)
									fmt.Fprintf(vrb, "Error: %v", err)
									return nil
								})
								return err
							}
							defer file.Close()
							fw, err := multiWriter.CreateFormFile(postKey, path.Base(postValues[i][1:]))
							if err != nil {
								return err
							}
							if _, err := io.Copy(fw, file); err != nil {
								return err
							}
						} else {
							fw, err := multiWriter.CreateFormField(postKey)
							if err != nil {
								return err
							}
							if _, err := fw.Write([]byte(postValues[i])); err != nil {
								return err
							}
						}
					}
				}
				body = bytes.NewReader(bodyBytes.Bytes())
			}
		}

		// create request
		req, err := http.NewRequest(r.Method, u.String(), body)
		if err != nil {
			g.Update(func(g *gocui.Gui) error {
				vrb, _ := g.View(RESPONSE_BODY_VIEW)
				fmt.Fprintf(vrb, "Request error: %v", err)
				return nil
			})
			return nil
		}
		req.Header = headers

		// set the `Host` header
		if headers.Get("Host") != "" {
			req.Host = headers.Get("Host")
		}

		// do request
		start := time.Now()
		response, err := CLIENT.Do(req)
		r.Duration = time.Since(start)
		if err != nil {
			g.Update(func(g *gocui.Gui) error {
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
				g.Update(func(g *gocui.Gui) error {
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

		r.Formatter = formatter.New(a.config, r.ContentType)

		// add to history
		a.history = append(a.history, r)
		a.historyIndex = len(a.history) - 1

		// render response
		g.Update(func(g *gocui.Gui) error {
			vrh, _ := g.View(RESPONSE_HEADERS_VIEW)

			a.PrintBody(g)

			// print status code and sorted headers
			hkeys := make([]string, 0, len(response.Header))
			for hname := range response.Header {
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
	g.Update(func(g *gocui.Gui) error {
		if len(a.history) == 0 {
			return nil
		}
		req := a.history[a.historyIndex]
		if req.RawResponseBody == nil {
			return nil
		}
		vrb, _ := g.View(RESPONSE_BODY_VIEW)
		vrb.Clear()

		var responseFormatter formatter.ResponseFormatter
		responseFormatter = req.Formatter

		vrb.Title = VIEW_PROPERTIES[vrb.Name()].title + " " + responseFormatter.Title()

		search_text := getViewValue(g, "search")
		if search_text == "" || !responseFormatter.Searchable() {
			err := responseFormatter.Format(vrb, req.RawResponseBody)
			if err != nil {
				fmt.Fprintf(vrb, "Error: cannot decode response body: %v", err)
				return nil
			}
			if _, err := vrb.Line(0); !a.config.General.PreserveScrollPosition || err != nil {
				vrb.SetOrigin(0, 0)
			}
			return nil
		}
		if !a.config.General.ContextSpecificSearch {
			responseFormatter = DEFAULT_FORMATTER
		}
		vrb.SetOrigin(0, 0)
		results, err := responseFormatter.Search(search_text, req.RawResponseBody)
		if err != nil {
			fmt.Fprint(vrb, "Search error: ", err)
			return nil
		}
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
	for k := range keys {
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
		help.Title = VIEW_TITLES[HELP_VIEW]
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
	v.SelBgColor = gocui.ColorDefault
	a.currentPopup = name
	return
}

func (a *App) LoadRequest(g *gocui.Gui, loadLocation string) (err error) {
	requestJson, ioErr := ioutil.ReadFile(loadLocation)
	if ioErr != nil {
		g.Update(func(g *gocui.Gui) error {
			vrb, _ := g.View(RESPONSE_BODY_VIEW)
			vrb.Clear()
			fmt.Fprintf(vrb, "File reading error: %v", ioErr)
			return nil
		})
		return nil
	}

	var requestMap map[string]string
	jsonErr := json.Unmarshal(requestJson, &requestMap)
	if jsonErr != nil {
		g.Update(func(g *gocui.Gui) error {
			vrb, _ := g.View(RESPONSE_BODY_VIEW)
			vrb.Clear()
			fmt.Fprintf(vrb, "JSON decoding error: %v", jsonErr)
			return nil
		})
		return nil
	}

	var v *gocui.View
	url, exists := requestMap[URL_VIEW]
	if exists {
		v, _ = g.View(URL_VIEW)
		setViewTextAndCursor(v, url)
	}

	method, exists := requestMap[REQUEST_METHOD_VIEW]
	if exists {
		v, _ = g.View(REQUEST_METHOD_VIEW)
		setViewTextAndCursor(v, method)
	}

	params, exists := requestMap[URL_PARAMS_VIEW]
	if exists {
		v, _ = g.View(URL_PARAMS_VIEW)
		setViewTextAndCursor(v, params)
	}

	data, exists := requestMap[REQUEST_DATA_VIEW]
	if exists {
		v, _ = g.View(REQUEST_DATA_VIEW)
		setViewTextAndCursor(v, data)
	}

	headers, exists := requestMap[REQUEST_HEADERS_VIEW]
	if exists {
		v, _ = g.View(REQUEST_HEADERS_VIEW)
		setViewTextAndCursor(v, headers)
	}
	return nil
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

func (a *App) OpenSaveDialog(title string, g *gocui.Gui, save func(g *gocui.Gui, v *gocui.View) error) error {
	dialog, err := a.CreatePopupView(SAVE_DIALOG_VIEW, 60, 1, g)
	if err != nil {
		return err
	}
	g.Cursor = true

	dialog.Title = title
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
	g.DeleteKeybinding(SAVE_DIALOG_VIEW, gocui.KeyEnter, gocui.ModNone)
	g.SetKeybinding(SAVE_DIALOG_VIEW, gocui.KeyEnter, gocui.ModNone, save)
	return nil
}

func (a *App) OpenSaveResultView(saveResult string, g *gocui.Gui) (err error) {
	popupTitle := VIEW_TITLES[SAVE_RESULT_VIEW]
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
		a.statusLine, _ = NewStatusLine(a.config.General.StatusLine)
		return nil
	}

	conf, err := config.LoadConfig(configPath)
	if err != nil {
		a.config = &config.DefaultConfig
		a.config.Keys = config.DefaultKeys
		return err
	}

	a.config = conf
	sl, err := NewStatusLine(conf.General.StatusLine)
	if err != nil {
		a.config = &config.DefaultConfig
		a.config.Keys = config.DefaultKeys
		return err
	}
	a.statusLine = sl
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
	content_type := ""
	set_data := false
	set_method := false
	set_binary_data := false
	arg_index := 1
	args_len := len(args)
	accept_types := make([]string, 0, 8)
	var body_data []string
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
		case "-d", "--data", "--data-binary", "--data-urlencode":
			if arg_index == args_len-1 {
				return errors.New("No POST/PUT/PATCH value specified")
			}

			arg_index += 1
			set_data = true
			set_binary_data = arg == "--data-binary"
			arg_data := args[arg_index]

			if !set_binary_data {
				content_type = "form"
			}

			if arg == "--data-urlencode" {
				// TODO: Replace with `url.PathEscape(..)` in Go 1.8
				arg_data_url := &url.URL{Path: arg_data}
				arg_data = arg_data_url.String()
			}

			body_data = append(body_data, arg_data)
		case "-j", "--json":
			if arg_index == args_len-1 {
				return errors.New("No POST/PUT/PATCH value specified")
			}

			arg_index += 1
			json_str := args[arg_index]
			content_type = "json"
			accept_types = append(accept_types, config.ContentTypes["json"])
			set_data = true
			vdata, _ := g.View(REQUEST_DATA_VIEW)
			setViewTextAndCursor(vdata, json_str)
		case "-X", "--request":
			if arg_index == args_len-1 {
				return errors.New("No HTTP method specified")
			}
			arg_index++
			set_method = true
			method := args[arg_index]
			if content_type == "" && (method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch) {
				content_type = "form"
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
			a.config.General.Timeout = config.Duration{Duration: time.Duration(timeout) * time.Millisecond}
		case "--compressed":
			vh, _ := g.View(REQUEST_HEADERS_VIEW)
			if !strings.Contains(getViewValue(g, REQUEST_HEADERS_VIEW), "Accept-Encoding") {
				fmt.Fprintln(vh, "Accept-Encoding: gzip, deflate")
			}
		case "-e", "--editor":
			if arg_index == args_len-1 {
				return errors.New("No timeout value specified")
			}
			arg_index += 1
			a.config.General.Editor = args[arg_index]
		case "-k", "--insecure":
			a.config.General.Insecure = true
		case "-R", "--disable-redirects":
			a.config.General.FollowRedirects = false
		case "--tlsv1.0":
			a.config.General.TLSVersionMin = tls.VersionTLS10
			a.config.General.TLSVersionMax = tls.VersionTLS10
		case "--tlsv1.1":
			a.config.General.TLSVersionMin = tls.VersionTLS11
			a.config.General.TLSVersionMax = tls.VersionTLS11
		case "--tlsv1.2":
			a.config.General.TLSVersionMin = tls.VersionTLS12
			a.config.General.TLSVersionMax = tls.VersionTLS12
		case "-1", "--tlsv1":
			a.config.General.TLSVersionMin = tls.VersionTLS10
			a.config.General.TLSVersionMax = tls.VersionTLS12
		case "-T", "--tls":
			if arg_index >= args_len-1 {
				return errors.New("Missing TLS version range: MIN,MAX")
			}
			arg_index++
			arg := args[arg_index]
			v := strings.Split(arg, ",")
			min := v[0]
			max := min
			if len(v) > 1 {
				max = v[1]
			}
			minV, minFound := TLS_VERSIONS[min]
			if !minFound {
				return errors.New("Minimum TLS version not found: " + min)
			}
			maxV, maxFound := TLS_VERSIONS[max]
			if !maxFound {
				return errors.New("Maximum TLS version not found: " + max)
			}
			a.config.General.TLSVersionMin = minV
			a.config.General.TLSVersionMax = maxV
		case "-x", "--proxy":
			if arg_index == args_len-1 {
				return errors.New("Missing proxy URL")
			}
			arg_index += 1
			u, err := url.Parse(args[arg_index])
			if err != nil {
				return fmt.Errorf("Invalid proxy URL: %v", err)
			}
			switch u.Scheme {
			case "", "http", "https":
				TRANSPORT.Proxy = http.ProxyURL(u)
			case "socks", "socks5":
				dialer, err := proxy.SOCKS5("tcp", u.Host, nil, proxy.Direct)
				if err != nil {
					return fmt.Errorf("Can't connect to proxy: %v", err)
				}
				TRANSPORT.Dial = dialer.Dial
			default:
				return errors.New("Unknown proxy protocol")
			}
		case "-F", "--form":
			if arg_index == args_len-1 {
				return errors.New("No POST/PUT/PATCH value specified")
			}

			arg_index += 1
			form_str := args[arg_index]
			content_type = "multipart"
			set_data = true
			vdata, _ := g.View(REQUEST_DATA_VIEW)
			setViewTextAndCursor(vdata, form_str)
		case "-f", "--file":
			if arg_index == args_len-1 {
				return errors.New("-f or --file requires a file path be provided as an argument")
			}
			arg_index += 1
			loadLocation := args[arg_index]
			a.LoadRequest(g, loadLocation)
		default:
			u := args[arg_index]
			if strings.Index(u, "http://") != 0 && strings.Index(u, "https://") != 0 {
				u = fmt.Sprintf("%v://%v", a.config.General.DefaultURLScheme, u)
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

	if !set_binary_data && content_type != "" && !a.hasHeader(g, "Content-Type") {
		fmt.Fprintf(vheader, "Content-Type: %v\n", config.ContentTypes[content_type])
	}

	if len(accept_types) > 0 && !a.hasHeader(g, "Accept") {
		fmt.Fprintf(vheader, "Accept: %v\n", strings.Join(accept_types, ","))
	}

	var merged_body_data string
	if set_data && !set_binary_data {
		merged_body_data = strings.Join(body_data, "&")
	}

	vdata, _ := g.View(REQUEST_DATA_VIEW)
	setViewTextAndCursor(vdata, merged_body_data)

	return nil
}

func (a *App) hasHeader(g *gocui.Gui, h string) bool {
	for _, header := range strings.Split(getViewValue(g, REQUEST_HEADERS_VIEW), "\n") {
		if header == "" {
			continue
		}
		header_parts := strings.SplitN(header, ": ", 2)
		if len(header_parts) != 2 {
			continue
		}
		if header_parts[0] == h {
			return true
		}
	}
	return false
}

// Apply startup config values. This is run after a.ParseArgs, so that
// args can override the provided config values
func (a *App) InitConfig() {
	CLIENT.Timeout = a.config.General.Timeout.Duration
	TRANSPORT.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: a.config.General.Insecure,
		MinVersion:         a.config.General.TLSVersionMin,
		MaxVersion:         a.config.General.TLSVersionMax,
	}
	if !a.config.General.FollowRedirects {
		CLIENT.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}
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
  -c, --config PATH        Specify custom configuration file
  -e, --editor EDITOR      Specify external editor command
  -f, --file REQUEST       Load a previous request
  -F, --form DATA          Add multipart form request data and set related request headers
                           If the value starts with @ it will be handled as a file path for upload
  -h, --help               Show this
  -j, --json JSON          Add JSON request data and set related request headers
  -k, --insecure           Allow insecure SSL certs
  -R, --disable-redirects  Do not follow HTTP redirects
  -T, --tls MIN,MAX        Restrict allowed TLS versions (values: SSL3.0,TLS1.0,TLS1.1,TLS1.2)
                           Examples: wuzz -T TLS1.1        (TLS1.1 only)
                                     wuzz -T TLS1.0,TLS1.1 (from TLS1.0 up to TLS1.1)
  --tlsv1.0                Forces TLS1.0 only
  --tlsv1.1                Forces TLS1.1 only
  --tlsv1.2                Forces TLS1.2 only
  -1, --tlsv1              Forces TLS version 1.x (1.0, 1.1 or 1.2)
  -v, --version            Display version number
  -x, --proxy URL          Set HTTP(S) or SOCKS5 proxy

Key bindings:
  ctrl+r              Send request
  ctrl+s              Save response
  ctrl+e              Save request
  ctrl+f              Load request
  tab, ctrl+j         Next window
  shift+tab, ctrl+k   Previous window
  alt+h               Show history
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
	var g *gocui.Gui
	var err error
	for _, outputMode := range []gocui.OutputMode{gocui.Output256, gocui.OutputNormal, gocui.OutputMode(termbox.OutputGrayscale)} {
		g, err = gocui.NewGui(outputMode)
		if err == nil {
			break
		}
	}
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
