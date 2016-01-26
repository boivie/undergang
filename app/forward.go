package app


import (
	"log"
	"net/http"
	"net/http/httputil"
	"net"
	"strings"
)

func Forward(w http.ResponseWriter, req *http.Request) {
	info := LookupPath(req.URL.Path)
	if info == nil {
		log.Println("Path not in mapping: " + req.URL.Path)
		http.Error(w, "Path not mapped", http.StatusNotFound)
		return
	}

	sshClient := getSSHConnection(&info.Server)
	if sshClient == nil {
		http.Error(w, "Backend connection failure", http.StatusInternalServerError)
		return
	}

	var revProxy http.Handler
	director := func(req *http.Request) {
		req.URL.Path = info.HttpProxy.BasePath + strings.TrimPrefix(req.URL.Path, info.Prefix)
		req.URL.Scheme = "http"
		req.URL.Host = info.HttpProxy.Address
	}

	if (isWebsocket(req)) {
		revProxy = &WebsocketReverseProxy{
			Director: director,
			Dial: func(network, addr string) (net.Conn, error) {
				log.Println(`SSH->WebSocket @ ` + info.HttpProxy.Address)
				return sshClient.Dial(`tcp`, addr)
			},
		}

	} else {
		revProxy = &httputil.ReverseProxy{
			Director: director,
			Transport: &http.Transport{
				Dial: func(network, addr string) (net.Conn, error) {
					log.Println(`SSH->HTTP @ ` + info.HttpProxy.Address)
					return sshClient.Dial(`tcp`, addr)
				},
			},
		}
	}
	revProxy.ServeHTTP(w, req)
}
