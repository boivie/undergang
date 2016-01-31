package app


import (
	"log"
	"net/http"
	"net/http/httputil"
	"net"
	"strings"
)

func logRequest(req *http.Request, status int, reason string) {
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		host = req.RemoteAddr
	}

	log.Printf("%s %s %s %d \"%s\"\n", host, req.Method, req.RequestURI, http.StatusForbidden, reason)
}

func showConnectionProgress(info *PathInfo, w http.ResponseWriter, req *http.Request) bool {
	return false
}

func Forward(w http.ResponseWriter, req *http.Request) {
	token := ""
	if cookie, err := req.Cookie("access_token"); err == nil {
		token = cookie.Value
	}
	if !LookupAccess(token, req.URL.Path) {
		logRequest(req, http.StatusForbidden, "Invalid access token")
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}
	info := LookupPath(req.URL.Path)
	if info == nil {
		logRequest(req, http.StatusNotFound, "Path mapping not found")
		http.Error(w, "Path not mapped", http.StatusNotFound)
		return
	}

	if serveStatic(info.StaticOverrides, w, req) {
		return
	}

	if showConnectionProgress(info, w, req) {
		return
	}

	conn := getBackendConnection(info)
	if conn == nil {
		logRequest(req, http.StatusInternalServerError, "Couldn't connect to backend server")
		return
	}

	director := func(req *http.Request) {
		req.URL.Path = info.Backend.BasePath + strings.TrimPrefix(req.URL.Path, info.Prefix)
		req.URL.Scheme = "http"
		req.URL.Host = info.Backend.Address
	}

	var revProxy http.Handler
	if (isWebsocket(req)) {
		revProxy = &WebsocketReverseProxy{
			Director: director,
			Dial: func(network, addr string) (net.Conn, error) {
				log.Println(`SSH->WebSocket @ ` + info.Backend.Address)
				return conn, nil
			},
		}

	} else {
		revProxy = &httputil.ReverseProxy{
			Director: director,
			Transport: &http.Transport{
				Dial: func(network, addr string) (net.Conn, error) {
					log.Println(`SSH->HTTP @ ` + info.Backend.Address)
					return conn, nil
				},
			},
		}
	}
	revProxy.ServeHTTP(w, req)
}
