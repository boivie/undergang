package app

import (
	"io"
	"log"
	"net"
	"net/http"
	"strings"
)

type WebsocketReverseProxy struct {
	Director func(*http.Request)
	Dial     func(network, addr string) (net.Conn, error)
}

func (wrp *WebsocketReverseProxy) ServeHTTP(rw http.ResponseWriter, origReq *http.Request) {
	req := *origReq
	wrp.Director(&req)

	d, err := wrp.Dial("tcp", req.URL.Host)
	if err != nil {
		http.Error(rw, "Error connecting to backend", http.StatusServiceUnavailable)
		log.Printf("Error dialing websocket backend %s: %v", req.URL, err)
		return
	}

	hj, ok := rw.(http.Hijacker)
	if !ok {
		return
	}

	nc, _, err := hj.Hijack()
	if err != nil {
		return
	}

	defer nc.Close()
	defer d.Close()

	// We can now perform the request manually
	err = req.Write(d)
	if err != nil {
		return
	}

	errc := make(chan error, 2)
	cp := func(dst io.Writer, src io.Reader) {
		_, err := io.Copy(dst, src)
		errc <- err
	}

	go cp(d, nc)
	go cp(nc, d)
	<-errc
}

func isWebsocket(req *http.Request) bool {
	conn_hdr := ""
	conn_hdrs := req.Header["Connection"]
	if len(conn_hdrs) > 0 {
		conn_hdr = conn_hdrs[0]
	}

	upgrade_websocket := false
	if strings.ToLower(conn_hdr) == "upgrade" {
		upgrade_hdrs := req.Header["Upgrade"]
		if len(upgrade_hdrs) > 0 {
			upgrade_websocket = (strings.ToLower(upgrade_hdrs[0]) == "websocket")
		}
	}

	return upgrade_websocket
}
