package ssht

import (
	"io"
	"log"
	"net"
	"net/http"
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
		http.Error(rw, "Error connecting to backend", http.StatusInternalServerError)
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
