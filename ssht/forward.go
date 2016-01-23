package ssht


import (
	"golang.org/x/crypto/ssh"
	"log"
	"time"
	"net/http"
	"net/http/httputil"
	"net"
	"strings"
	"io/ioutil"
)

type HttpProxy struct {
	LocalAddr       string
	LocalPathPrefix string
}

type ServerInfo struct {
	ServerAddr string
	Config     *ssh.ClientConfig
	HttpProxy  HttpProxy
}

type getConnReq struct {
	ServerInfo *ServerInfo
	Reply      chan *ssh.Client
}

var mapping map[string]*ServerInfo = make(map[string]*ServerInfo)
var getConnChan chan getConnReq = make(chan getConnReq)

const MAX_RETRIES_SERVER = 16

func sshConnector() {
	connections := make(map[string]*ssh.Client)
	for {
		req := <-getConnChan
		addr := req.ServerInfo.ServerAddr

		if conn, ok := connections[addr]; ok {
			req.Reply <- conn
		} else {
			currentRetriesServer := 0
			for {
				if sshClientConn, err := ssh.Dial(`tcp`, addr, req.ServerInfo.Config); err != nil {
					currentRetriesServer++
					log.Printf("SSH Connection failed %s: %s\n", addr, err.Error())

					if currentRetriesServer < MAX_RETRIES_SERVER {
						log.Println(`Retry...`)
						time.Sleep(1 * time.Second)
					} else {
						log.Println(`No more retries for connecting the SSH server.`)
						req.Reply <- sshClientConn
						break
					}
				} else {
					connections[addr] = sshClientConn
					log.Println(`Connected to the SSH server ` + addr)
					req.Reply <- sshClientConn
					break
				}
			}
		}
	}
}

func findLongestPrefix(mapping map[string]*ServerInfo, path string) (prefix string, info *ServerInfo) {
	for iterPrefix, iterInfo := range mapping {
		if strings.HasPrefix(path, iterPrefix) {
			if len(prefix) < len(iterPrefix) {
				prefix = iterPrefix
				info = iterInfo
			}
		}
	}
	return
}

func Forward(w http.ResponseWriter, req *http.Request) {
	prefix, info := findLongestPrefix(mapping, req.URL.Path)
	if info == nil {
		log.Println("Path not in mapping: " + req.URL.Path)
		http.Error(w, "Path not mapped", http.StatusNotFound)
		return
	}
	remainingPath := info.HttpProxy.LocalPathPrefix + strings.TrimPrefix(req.URL.Path, prefix)

	clientChan := make(chan *ssh.Client)
	getConnChan <- getConnReq{info, clientChan}
	sshClient := <-clientChan

	if sshClient == nil {
		http.Error(w, "Backend connection failure", http.StatusInternalServerError)
		return
	}

	var revProxy http.Handler
	director := func(req *http.Request) {
		req.URL.Path = remainingPath
		req.URL.Scheme = "http"
		req.URL.Host = info.HttpProxy.LocalAddr
	}

	if (isWebsocket(req)) {
		revProxy = &WebsocketReverseProxy{
			Director: director,
			Dial: func(network, addr string) (net.Conn, error) {
				log.Println(`SSH->WebSocket @ ` + info.HttpProxy.LocalAddr)
				return sshClient.Dial(`tcp`, addr)
			},
		}

	} else {
		revProxy = &httputil.ReverseProxy{
			Director: director,
			Transport: &http.Transport{
				Dial: func(network, addr string) (net.Conn, error) {
					log.Println(`SSH->HTTP @ ` + info.HttpProxy.LocalAddr)
					return sshClient.Dial(`tcp`, addr)
				},
			},
		}
	}
	revProxy.ServeHTTP(w, req)
}

func getKeyFile() (key ssh.Signer, err error) {
	file := "go_id_rsa"
	buf, err := ioutil.ReadFile(file)
	if err != nil {
		return
	}
	key, err = ssh.ParsePrivateKey(buf)
	if err != nil {
		return
	}
	return
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

func Init() {
	key, err := getKeyFile()
	if err != nil {
		panic(err)
	}

	config := &ssh.ClientConfig{
		User: "victor",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(key),
		},
	}

	mapping["/pi/helloworld/"] = &ServerInfo{
		ServerAddr: "192.168.1.150:22",
		Config: config,
		HttpProxy: HttpProxy{
			LocalAddr: "127.0.0.1:8899",
			LocalPathPrefix: "/",
		},
	}

	mapping["/pi/lovebeat/"] = &ServerInfo{
		ServerAddr: "192.168.1.150:22",
		Config: config,
		HttpProxy: HttpProxy{
			LocalAddr: "127.0.0.1:8080",
			LocalPathPrefix: "/",
		},
	}

	go sshConnector()
}