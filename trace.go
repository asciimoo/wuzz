package main

import (
	"container/ring"
	"crypto/tls"
	"fmt"
	"net/http/httptrace"
	"net/textproto"
	"strings"
	"sync"
	"time"

	"github.com/jroimartin/gocui"
)

const (
	ViewWidth       = 80
	ViewHeight      = 25
	BufferLength    = 20
	TimestampLayout = "15:04:05.000000"
)

type ClientTrace struct {
	sync.Mutex
	data         *ring.Ring
	recChannel   chan string
	writeChannel chan string
}

var (
	clientTrace *ClientTrace
)

func NewClientTrace() *ClientTrace {
	result := new(ClientTrace)
	result.data = ring.New(BufferLength)
	result.recChannel = make(chan string, 0)
	result.writeChannel = make(chan string, 1)

	go result.writer()
	go result.receiver()

	return result
}

func (c *ClientTrace) Write(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	lineLen := ViewWidth - (len(TimestampLayout) + 3)
	if len(message) > lineLen {
		sb := strings.Builder{}
		prefix := string(make([]rune, len(TimestampLayout)+3))

		for i, c := range []rune(message) {
			if i > 0 && i%lineLen == 0 && i < len(message) {
				sb.WriteString(prefix)
			}
			sb.WriteRune(c)
		}

		message = sb.String()
	}
	message = fmt.Sprintf("[%s] %s", time.Now().Format(TimestampLayout), message)
	c.writeChannel <- fmt.Sprintf(message)
}

func (c *ClientTrace) Dump() string {
	c.Lock()
	defer c.Unlock()

	sb := strings.Builder{}
	c.data.Do(func(v interface{}) {
		if v != nil {
			s := fmt.Sprintf("%v\n", v)
			sb.WriteString(s)
		}
	})
	return sb.String()
}

func (c *ClientTrace) receiver() {
	for {
		select {
		case message, more := <-c.recChannel:
			c.writeChannel <- message
			if !more {
				break
			}
		}
	}
}

func (c *ClientTrace) writer() {
	f := func(message string) {
		c.Lock()
		defer c.Unlock()
		c.data.Value = message
		c.data = c.data.Next()
	}
	for {
		select {
		case message, more := <-c.writeChannel:
			f(message)
			if !more {
				break
			}
		}
	}
}

func getClientTrace() *httptrace.ClientTrace {
	if clientTrace == nil {
		clientTrace = NewClientTrace()
	}

	return &httptrace.ClientTrace{
		GetConn: func(hostPort string) {
			clientTrace.Write("GetConn(%v)", hostPort)
		},
		GotConn: func(info httptrace.GotConnInfo) {
			clientTrace.Write("GotConn(%v -> %v)", info.Conn.LocalAddr(), info.Conn.RemoteAddr())
		},
		GotFirstResponseByte: func() {
			clientTrace.Write("GotFirstResponseByte()")
		},
		Got100Continue: func() {
			clientTrace.Write("Got100Continue()")
		},
		Got1xxResponse: func(code int, header textproto.MIMEHeader) error {
			clientTrace.Write("Got1xxResponse(%v, %v)", code, header)
			return nil
		},
		DNSStart: func(info httptrace.DNSStartInfo) {
			clientTrace.Write("DNSStart(%v)", info.Host)
		},
		DNSDone: func(info httptrace.DNSDoneInfo) {
			if info.Err != nil {
				clientTrace.Write("DNSDone(): ", info.Err)
			} else {
				clientTrace.Write("DNSDone(%v)", info.Addrs)
			}
		},
		ConnectStart: func(network, addr string) {
			clientTrace.Write("ConnectStart(%v)", addr)
		},
		ConnectDone: func(network, addr string, err error) {
			if err != nil {
				clientTrace.Write("ConnectDone(): %v", err)
			} else {
				clientTrace.Write("ConnectDone(%v)", addr)
			}
		},
		TLSHandshakeDone: func(state tls.ConnectionState, err error) {
			if err != nil {
				clientTrace.Write("TLSHandshakeDone(): %v", err)
			} else {
				sb := strings.Builder{}
				sb.WriteString(func(version uint16) string {
					for k, v := range TLS_VERSIONS {
						if v == state.Version {
							return k
						}
					}
					return "Unknown"
				}(state.Version))
				sb.WriteRune(' ')
				sb.WriteString(state.ServerName)
				sb.WriteString(" <=> ")
				sb.WriteString(state.PeerCertificates[0].Subject.CommonName)
				clientTrace.Write("TLSHandshakeDone(%v)", sb.String())
			}
		},
		WroteHeaderField: func(key string, value []string) {
			clientTrace.Write("WroteHeaderField(%v, %v)", key, value)
		},
		Wait100Continue: func() {
			clientTrace.Write("Wait100Continue()")
		},
		WroteRequest: func(info httptrace.WroteRequestInfo) {
			if info.Err != nil {
				clientTrace.Write("WroteRequest(): %v", info)
			} else {
				clientTrace.Write("WroteRequest()")
			}
		},
	}
}

func dumpTraceClient(g *gocui.Gui, a *App) {
	if a.currentPopup == TRACE_VIEW {
		a.closePopup(g, TRACE_VIEW)
		return
	}
	trace, err := a.CreatePopupView(TRACE_VIEW, ViewWidth, ViewHeight, g)
	if err != nil {
		return
	}
	trace.Title = VIEW_TITLES[TRACE_VIEW]
	trace.Highlight = false
	trace.Wrap = true
	if clientTrace != nil {
		go func() {
			for a.currentPopup == TRACE_VIEW {
				trace.Clear()
				_, err = fmt.Fprintf(trace, clientTrace.Dump())
				time.Sleep(100 * time.Millisecond)
			}
		}()
	}
	_, _ = g.SetViewOnTop(TRACE_VIEW)
	_, _ = g.SetCurrentView(TRACE_VIEW)
}
