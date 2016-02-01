package app


import (
	"log"
	"net/http"
	"net/http/httputil"
	"net"
	"strings"
	"runtime"
)

func logRequest(req *http.Request, status int, reason string) {
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		host = req.RemoteAddr
	}

	log.Printf("%s %s %s %d \"%s\"\n", host, req.Method, req.RequestURI, http.StatusForbidden, reason)
}

func showConnectionProgress(backend backend, w http.ResponseWriter, req *http.Request) bool {
	// Only do this for modern browsers.
	useragent := req.Header.Get("User-Agent")
	if !strings.Contains(useragent, "Mozilla") || isWebsocket(req) {
		return false
	}

	// Not for images and those kind of stuff?
	if backend.IsReady() {
		return false
	}

	serveProgressPage(w, req)
	return true
}

func Forward(w http.ResponseWriter, req *http.Request) {
	if strings.HasSuffix(req.RequestURI, "/__ugdump") {
		buf := make([]byte, 1 << 20)
		runtime.Stack(buf, true)
		log.Printf("=== received SIGQUIT ===\n*** goroutine dump...\n%s\n*** end\n", buf)
		return
	}
	token := ""
	if cookie, err := req.Cookie("access_token"); err == nil {
		token = cookie.Value
	}
	if !LookupAccess(token, req.URL.Path) {
		logRequest(req, http.StatusForbidden, "Invalid access token")
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	backend := LookupBackend(req.URL.Path)
	if backend == nil {
		logRequest(req, http.StatusNotFound, "Path mapping not found")
		http.Error(w, "Path not mapped", http.StatusNotFound)
		return
	}

	if serveProgressWebSocket(backend, w, req) {
		return
	}

	if showConnectionProgress(backend, w, req) {
		return
	}

	if serveStatic(backend.GetInfo().StaticOverrides, w, req) {
		return
	}

	conn := backend.Connect()
	if conn == nil {
		logRequest(req, http.StatusInternalServerError, "Couldn't connect to backend server")
		return
	}

	director := func(req *http.Request) {
		req.URL.Path = backend.GetInfo().Backend.BasePath + strings.TrimPrefix(req.URL.Path, backend.GetInfo().Prefix)
		req.URL.Scheme = "http"
		req.URL.Host = backend.GetInfo().Backend.Address
	}

	var revProxy http.Handler
	if (isWebsocket(req)) {
		revProxy = &WebsocketReverseProxy{
			Director: director,
			Dial: func(network, addr string) (net.Conn, error) {
				return conn, nil
			},
		}

	} else {
		revProxy = &httputil.ReverseProxy{
			Director: director,
			Transport: &http.Transport{
				Dial: func(network, addr string) (net.Conn, error) {
					return conn, nil
				},
			},
		}
	}
	revProxy.ServeHTTP(w, req)
}
