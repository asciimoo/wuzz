package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jroimartin/gocui"
)

var CLIENT *http.Client = &http.Client{
	Timeout: time.Duration(5 * time.Second),
}
var TRANSPORT *http.Transport = &http.Transport{}

var VIEWS []string = []string{
	"url",
	"method",
	"get",
	"data",
	"headers",
	"response-headers",
	"response-body",
	"search",
}

type RequestParam struct {
	Key   string
	Value string
}

type App struct {
	viewIndex       int
	rawResponseBody []byte
	contentType     string
}

type SearchEditor struct {
	app *App
	g   *gocui.Gui
}

func init() {
	TRANSPORT.DisableCompression = true
	CLIENT.Transport = TRANSPORT
}

func (e *SearchEditor) Edit(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
	gocui.DefaultEditor.Edit(v, key, ch, mod)
	e.g.Execute(func(g *gocui.Gui) error {
		e.app.PrintBody(g)
		return nil
	})
}

func (a *App) Layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	splitX := int(0.3 * float32(maxX))
	splitY := int(0.25 * float32(maxY-3))
	if v, err := g.SetView("url", 0, 0, maxX-1, 3); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		setViewDefaults(v)
		v.Title = "URL - press ctrl+r to send request"
		v.Editable = true
		setViewTextAndCursor(v, "https://")
	}
	if v, err := g.SetView("method", 0, 3, splitX, 5); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		setViewDefaults(v)
		v.Editable = true
		v.Title = "Method"
		setViewTextAndCursor(v, "GET")
	}
	if v, err := g.SetView("get", 0, 5, splitX, splitY+3); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		setViewDefaults(v)
		v.Editable = true
		v.Title = "URL params"
	}
	if v, err := g.SetView("data", 0, 3+splitY, splitX, 2*splitY+3); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		setViewDefaults(v)
		v.Editable = true
		v.Title = "Request data (POST/PUT)"
	}
	if v, err := g.SetView("headers", 0, 3+(splitY*2), splitX, maxY-2); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		setViewDefaults(v)
		v.Editable = true
		v.Title = "Request headers"
	}
	if v, err := g.SetView("response-headers", splitX, 3, maxX-1, splitY+3); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		setViewDefaults(v)
		v.Title = "Response headers"
	}
	if v, err := g.SetView("response-body", splitX, 3+splitY, maxX-1, maxY-2); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		setViewDefaults(v)
		v.Title = "Response body"
	}
	if v, err := g.SetView("prompt", -1, maxY-2, 7, maxY); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Frame = false
		v.Wrap = true
		setViewTextAndCursor(v, "search> ")
	}
	if v, err := g.SetView("search", 7, maxY-2, maxX, maxY); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Frame = false
		v.Editable = true
		v.Editor = &SearchEditor{a, g}
		v.Wrap = true
	}

	g.SetCurrentView(VIEWS[a.viewIndex])
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
	_, err := g.SetCurrentView(VIEWS[a.viewIndex])
	return err
}

func popup(g *gocui.Gui, msg string) {
	var popup *gocui.View
	var err error
	maxX, maxY := g.Size()
	if popup, err = g.SetView("popup", maxX/2-len(msg)/2-1, maxY/2-1, maxX/2+len(msg)/2+1, maxY/2+1); err != nil {
		if err != gocui.ErrUnknownView {
			return
		}
		setViewDefaults(popup)
		popup.Title = "Info"
		setViewTextAndCursor(popup, msg)
		g.SetViewOnTop("popup")
	}
}

func (a *App) SubmitRequest(g *gocui.Gui, _ *gocui.View) error {
	vrb, _ := g.View("response-body")
	vrb.Clear()
	vrh, _ := g.View("response-headers")
	vrh.Clear()
	popup(g, "Sending request..")

	go func(g *gocui.Gui) error {
		defer g.DeleteView("popup")
		// parse url
		base_url := getViewValue(g, "url")
		u, err := url.Parse(base_url)
		if err != nil {
			g.Execute(func(g *gocui.Gui) error {
				vrb, _ := g.View("response-body")
				fmt.Fprintf(vrb, "URL parse error: %v", err)
				return nil
			})
			return nil
		}
		u.RawQuery = strings.Replace(getViewValue(g, "get"), "\n", "&", -1)

		// parse method
		method := getViewValue(g, "method")

		// parse POST/PUT data
		data := bytes.NewBufferString("")
		if method == "POST" || method == "PUT" {
			data.WriteString(strings.Replace(getViewValue(g, "data"), "\n", "&", -1))
		}

		// create request
		req, err := http.NewRequest(method, u.String(), data)
		if err != nil {
			g.Execute(func(g *gocui.Gui) error {
				vrb, _ := g.View("response-body")
				fmt.Fprintf(vrb, "Request error: %v", err)
				return nil
			})
			return nil
		}

		// set headers
		req.Header.Set("User-Agent", "")
		headers := strings.Split(getViewValue(g, "headers"), "\n")
		for _, header := range headers {
			header_parts := strings.SplitN(header, ": ", 2)
			if len(header_parts) != 2 {
				continue
			}
			req.Header.Set(header_parts[0], header_parts[1])
		}

		// do request
		response, err := CLIENT.Do(req)
		if err != nil {
			g.Execute(func(g *gocui.Gui) error {
				vrb, _ := g.View("response-body")
				fmt.Fprintf(vrb, "Response error: %v", err)
				return nil
			})
			return nil
		}
		defer response.Body.Close()

		// print body
		a.contentType = response.Header.Get("Content-Type")
		bodyBytes, err := ioutil.ReadAll(response.Body)
		if err == nil {
			a.rawResponseBody = bodyBytes
		}

		g.Execute(func(g *gocui.Gui) error {
			vrh, _ := g.View("response-headers")

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
			fmt.Fprintf(vrh,
				"\x1b[0;%dmHTTP/1.1 %v %v\x1b[0;0m\n",
				status_color,
				response.StatusCode,
				http.StatusText(response.StatusCode),
			)
			for _, hname := range hkeys {
				fmt.Fprintf(vrh, "\x1b[0;33m%v:\x1b[0;0m %v\n", hname, strings.Join(response.Header[hname], ","))
			}
			return nil
		})
		return nil
	}(g)

	return nil
}

func (a *App) PrintBody(g *gocui.Gui) {
	g.Execute(func(g *gocui.Gui) error {
		if a.rawResponseBody == nil {
			return nil
		}
		vrb, _ := g.View("response-body")
		vrb.Clear()
		if strings.Index(a.contentType, "text") == -1 && strings.Index(a.contentType, "application") == -1 {
			vrb.Title = "Response body"
			fmt.Fprint(vrb, "[binary content]")
			return nil
		}
		search_text := getViewValue(g, "search")
		if search_text == "" {
			vrb.Title = "Response body"
			vrb.Write(a.rawResponseBody)
			return nil
		}
		search_re, err := regexp.Compile(search_text)
		if err != nil {
			fmt.Fprint(vrb, "Error: invalid search regexp")
			return nil
		}
		results := search_re.FindAll(a.rawResponseBody, 1000)
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
	// global keybindings
	g.SetManagerFunc(a.Layout)

	g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit)

	g.SetKeybinding("", gocui.KeyTab, gocui.ModNone, a.NextView)
	g.SetKeybinding("", gocui.KeyCtrlJ, gocui.ModNone, a.NextView)
	g.SetKeybinding("", gocui.KeyCtrlK, gocui.ModNone, a.PrevView)

	g.SetKeybinding("", gocui.KeyCtrlR, gocui.ModNone, a.SubmitRequest)
	g.SetKeybinding("url", gocui.KeyEnter, gocui.ModNone, a.SubmitRequest)

	// responses common keybindings
	for _, view := range []string{"response-body", "response-headers"} {
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
}

func (a *App) ParseArgs(g *gocui.Gui) error {
	a.Layout(g)
	vheader, _ := g.View("headers")
	vheader.Clear()
	vget, _ := g.View("get")
	vget.Clear()
	arg_index := 1
	args_len := len(os.Args)
	for arg_index < args_len {
		arg := os.Args[arg_index]
		switch arg {
		case "-H", "--header":
			if arg_index == args_len-1 {
				return errors.New("No header value specified")
			}
			arg_index += 1
			header := os.Args[arg_index]
			fmt.Fprintf(vheader, "%v\n", header)
		case "-D", "--data":
			if arg_index == args_len-1 {
				return errors.New("No POST/PUT value specified")
			}

			vmethod, _ := g.View("method")
			setViewTextAndCursor(vmethod, "POST")

			arg_index += 1
			data, _ := url.QueryUnescape(os.Args[arg_index])
			vdata, _ := g.View("data")
			setViewTextAndCursor(vdata, data)
		case "-t", "--timeout":
			if arg_index == args_len-1 {
				return errors.New("No timeout value specified")
			}
			arg_index += 1
			timeout, err := strconv.Atoi(os.Args[arg_index])
			if err != nil || timeout <= 0 {
				return errors.New("Invalid timeout value")
			}
			CLIENT.Timeout = time.Duration(timeout) * time.Millisecond
		default:
			u := os.Args[arg_index]
			parsed_url, err := url.Parse(u)
			if err != nil || parsed_url.Host == "" {
				return errors.New("Invalid url")
			}
			vurl, _ := g.View("url")
			vurl.Clear()
			for k, v := range parsed_url.Query() {
				fmt.Fprintf(vget, "%v=%v\n", k, strings.Join(v, ""))
			}
			parsed_url.RawQuery = ""
			setViewTextAndCursor(vurl, parsed_url.String())
		}
		arg_index += 1
	}
	return nil
}

func createApp(g *gocui.Gui) *App {
	g.Cursor = true
	g.BgColor = gocui.ColorDefault
	g.FgColor = gocui.ColorDefault
	a := &App{}
	a.SetKeys(g)
	return a
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
	fmt.Println(`wuzz
Interactive cli tool for HTTP inspection

Usage: wuzz [-H|--header=HEADER]... [-D|--data=POST_DATA] [-t|--timeout=MSECS]  [URL]

Key bindings:
 ctrl+r         Send request
 tab, ctrl+j    Next window
 ctrl+k         Previous window
 pageUp         Scroll up the current window
 pageDown       Scroll down the current window`,
	)
}

func main() {
	for _, arg := range os.Args {
		if arg == "-h" || arg == "--help" {
			help()
			return
		}
	}
	g, err := gocui.NewGui(gocui.Output256)
	if err != nil {
		log.Panicln(err)
	}
	defer g.Close()

	app := createApp(g)
	app.ParseArgs(g)

	if err := g.MainLoop(); err != nil && err != gocui.ErrQuit {
		log.Panicln(err)
	}
}
