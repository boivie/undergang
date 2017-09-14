package app

import (
	"io"
	"net"
	"net/http"
	"strings"
)

type websocketReverseProxy struct {
	Backend  Backend
	Director func(*http.Request)
	Dial     func(network, addr string) (net.Conn, error)
}

func (wrp *websocketReverseProxy) ServeHTTP(rw http.ResponseWriter, origReq *http.Request) {
	log := wrp.Backend.GetLogger().WithField("type", "websocket")
	req := *origReq
	wrp.Director(&req)

	d, err := wrp.Dial("tcp", req.URL.Host)
	if err != nil {
		http.Error(rw, "Error connecting to backend", http.StatusServiceUnavailable)
		log.Infof("Error dialing websocket backend %s: %v", req.URL, err)
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
	connHdr := ""
	connHdrs := req.Header["Connection"]
	if len(connHdrs) > 0 {
		connHdr = connHdrs[0]
	}

	upgradeWebsocket := false
	if strings.ToLower(connHdr) == "upgrade" {
		upgradeHdrs := req.Header["Upgrade"]
		if len(upgradeHdrs) > 0 {
			upgradeWebsocket = (strings.ToLower(upgradeHdrs[0]) == "websocket")
		}
	}

	return upgradeWebsocket
}
