package app

import (
	"encoding/json"
	"html/template"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/websocket"
)

type progressCmd struct {
	Kind string      `json:"kind"`
	Data interface{} `json:"data"`
}

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type connection struct {
	ws       *websocket.Conn
	progress chan progressCmd
}

// readPump pumps messages from the websocket connection to the hub.
func (c *connection) readPump() {
	defer func() {
		// TODO: Unsubscribe from worker?
		c.ws.Close()
	}()
	c.ws.SetReadLimit(maxMessageSize)
	c.ws.SetReadDeadline(time.Now().Add(pongWait))
	c.ws.SetPongHandler(func(string) error {
		c.ws.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		_, _, err := c.ws.ReadMessage()
		if err != nil {
			break
		}
	}
}

// write writes a message with the given message type and payload.
func (c *connection) write(mt int, payload []byte) error {
	c.ws.SetWriteDeadline(time.Now().Add(writeWait))
	return c.ws.WriteMessage(mt, payload)
}

// writePump pumps messages from the hub to the websocket connection.
func (c *connection) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.ws.Close()
	}()
	for {
		select {
		case message, ok := <-c.progress:
			if !ok {
				c.write(websocket.CloseMessage, []byte{})
				return
			}
			buf, _ := json.Marshal(message)
			if err := c.write(websocket.TextMessage, buf); err != nil {
				return
			}
		case <-ticker.C:
			if err := c.write(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}
}

func serveProgressWebSocket(log *logrus.Entry, backend Backend, w http.ResponseWriter, req *http.Request) bool {
	if !strings.HasSuffix(req.RequestURI, "/__undergang_02648018bfd74fa5a4ed50db9bb07859_ws") {
		return false
	}

	ws, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		log.Infof("Failed to upgrade: %v", err)
		return true
	}
	progress := make(chan progressCmd, 256)
	c := &connection{ws: ws, progress: progress}

	backend.Subscribe(progress)

	go c.writePump()
	c.readPump()
	return true
}

func progressBroker(progressChan <-chan progressCmd, subscribeChan <-chan chan progressCmd) {
	progress := make([]progressCmd, 0)
	subscribers := make([]chan progressCmd, 0)
	for {
		select {
		case msg := <-progressChan:
			progress = append(progress, msg)
			for _, sub := range subscribers {
				sub <- msg
			}
		case q := <-subscribeChan:
			// Send all old progress first
			for _, p := range progress {
				q <- p
			}
			subscribers = append(subscribers, q)
		}
	}
}

func serveProgressScript(log *logrus.Entry, backend Backend, w http.ResponseWriter, req *http.Request) bool {
	if !strings.HasSuffix(req.RequestURI, "/__undergang_02648018bfd74fa5a4ed50db9bb07859_script.js") {
		return false
	}

	w.Header().Add("Content-Length", strconv.Itoa(len(script)))
	w.Write([]byte(script))

	return true
}

func serveProgress(backend Backend, w http.ResponseWriter, req *http.Request) bool {
	log := backend.GetLogger().WithField("type", "progress")
	if serveProgressWebSocket(log, backend, w, req) {
		return true
	}

	if serveProgressScript(log, backend, w, req) {
		return true
	}

	if serveProgressHTML(log, backend, w, req) {
		return true
	}

	return false
}

func serveProgressHTML(log *logrus.Entry, backend Backend, w http.ResponseWriter, req *http.Request) bool {
	// Only do this for modern browsers.
	useragent := req.Header.Get("User-Agent")
	if !strings.Contains(useragent, "Mozilla") || isWebsocket(req) {
		return false
	}

	// TODO: Not for images and those kind of stuff?

	// Only show when we're provisioning
	if backend.IsReady() {
		return false
	}

	info := backend.GetInfo()
	// Okey, we're the ones sending the data.
	w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Add("Pragma", "no-cache")
	w.Header().Add("Expires", "0")

	// Serve custom progress file?
	if info.ProgressPage != nil && info.ProgressPage.Filename != "" {
		http.ServeFile(w, req, info.ProgressPage.Filename)
	} else if info.ProgressPage != nil && info.ProgressPage.URL != "" {
		director := func(req *http.Request) {
			req.URL, _ = url.Parse(info.ProgressPage.URL)
			req.Host = req.URL.Host
			if info.ProgressPage.Hostname != "" {
				req.Host = info.ProgressPage.Hostname
			}
		}
		proxy := &httputil.ReverseProxy{Director: director}
		proxy.ServeHTTP(w, req)
	} else {
		templateVars := make(map[string]string)
		templateVars["BackgroundColor"] = "#41964B"

		if info.ProgressPage != nil && info.ProgressPage.Style != nil {
			if info.ProgressPage.Style.BackgroundColor != "" {
				templateVars["BackgroundColor"] = info.ProgressPage.Style.BackgroundColor
			}
		}

		tmpl, err := template.New("test").Parse(contents)
		if err != nil {
			log.Panicf("Failed to parse template: %v", err)
		}

		err = tmpl.Execute(w, templateVars)
		if err != nil {
			log.Errorf("Failed to render template: %v", err)
			io.WriteString(w, "Failed to render template")
		}
	}
	return true
}
