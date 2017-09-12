package app

import (
	"encoding/base64"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
)

func respond(w http.ResponseWriter, req *http.Request, reply string, status int) {
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		host = req.RemoteAddr
	}

	log.Printf("%s %s %s %d \"%s\"\n", host, req.Method, req.RequestURI, status, reply)
	http.Error(w, reply, status)
}

func serveBasicAuth(backend Backend, w http.ResponseWriter, req *http.Request) bool {
	if authInfo := backend.GetInfo().BasicAuth; authInfo != nil {
		authError := func() bool {
			w.Header().Set("WWW-Authenticate", "Basic realm=\"Restricted Access\"")
			http.Error(w, "authorization failed", http.StatusUnauthorized)
			return true
		}

		auth := strings.SplitN(req.Header.Get("Authorization"), " ", 2)
		if len(auth) != 2 || auth[0] != "Basic" {
			return authError()
		}

		payload, err := base64.StdEncoding.DecodeString(auth[1])
		if err != nil {
			return authError()
		}

		pair := strings.SplitN(string(payload), ":", 2)
		if len(pair) != 2 || !(pair[0] == authInfo.Username && pair[1] == authInfo.Password) {
			return authError()
		}
	}
	return false
}

func forward(w http.ResponseWriter, req *http.Request) {
	log.Printf("%s %s%s", req.Method, req.Host, req.URL.Path)
	backend := LookupBackend(req.Host, req.URL.Path)
	if backend == nil {
		respond(w, req, "Path not mapped", http.StatusNotFound)
		return
	}

	if serveBasicAuth(backend, w, req) {
		return
	}

	// handles /__ug_auth?code=$code
	if serveValidateServerAuth(backend, w, req) {
		return
	}

	// if server auth is enabled, verify that the user is authenticated.
	if serveServerAuth(backend, w, req) {
		return
	}

	if serveProgress(backend, w, req) {
		return
	}

	if serveStatic(backend, w, req) {
		return
	}

	conn := backend.Connect()
	if conn == nil {
		respond(w, req, "Couldn't connect to backend server", http.StatusServiceUnavailable)
		return
	}

	director := func(req *http.Request) {
		req.URL.Path = backend.GetInfo().Backend.BasePath + strings.TrimPrefix(req.URL.Path, backend.GetInfo().Prefix)
		req.URL.Scheme = "http"
		req.URL.Host = backend.GetInfo().Backend.Address
	}

	var revProxy http.Handler
	if isWebsocket(req) {
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
