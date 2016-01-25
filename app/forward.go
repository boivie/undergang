package app


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
	Bootstrap  []string
	Run        string
}

type getConnReq struct {
	ServerInfo *ServerInfo
	Reply      chan *ssh.Client
}

var mapping map[string]*ServerInfo = make(map[string]*ServerInfo)
var getConnChan chan getConnReq = make(chan getConnReq)

const MAX_RETRIES_SERVER = 16

func connectSSH(info *ServerInfo, resp chan ConnectSSHResponse) {
	log.Printf("Connecting to SSH server at %s\n", info.ServerAddr)

	currentRetriesServer := 0
	var sshClientConn *ssh.Client
	var err error

	for {
		if sshClientConn, err = ssh.Dial(`tcp`, info.ServerAddr, info.Config); err == nil {
			break
		}

		currentRetriesServer++
		log.Printf("SSH Connection failed %s: %s\n", info.ServerAddr, err.Error())

		if currentRetriesServer < MAX_RETRIES_SERVER {
			log.Println(`Retry...`)
			time.Sleep(1 * time.Second)
		} else {
			log.Println(`No more retries for connecting the SSH server.`)
			resp <- ConnectSSHResponse{ServerInfo: info}
			return
		}
	}
	log.Printf("SSH Connected to %s\n", info.ServerAddr)

	for _, cmd := range info.Bootstrap {
		log.Printf("Bootstrap: %s\n", cmd)
		session, _ := sshClientConn.NewSession()
		defer session.Close()
		session.Run(cmd)
	}
	log.Printf("Bootstrap for %s done", info.ServerAddr)
	if info.Run != "" {
		session, _ := sshClientConn.NewSession()
		defer session.Close()
		session.Start(info.Run)
		time.Sleep(500 * time.Millisecond)
	}
	resp <- ConnectSSHResponse{ServerInfo:info, client: sshClientConn}
}

type ConnectSSHResponse  struct {
	ServerInfo *ServerInfo
	client     *ssh.Client
}

func sshConnector() {
	type ServerConnection struct {
		client    *ssh.Client
		connected bool
		waitq     []chan *ssh.Client
	}

	connections := make(map[string]*ServerConnection)
	connectionDone := make(chan ConnectSSHResponse)

	for {
		select {
		case req := <-getConnChan:
			addr := req.ServerInfo.ServerAddr
			conn, _ := connections[addr];

			if conn != nil && conn.connected {
				req.Reply <- conn.client
			} else {
				if conn == nil {
					conn = &ServerConnection{waitq: make([]chan *ssh.Client, 0)}
					connections[addr] = conn
					go connectSSH(req.ServerInfo, connectionDone)
				}
				conn.waitq = append(conn.waitq, req.Reply)
			}
		case msg := <-connectionDone:
			addr := msg.ServerInfo.ServerAddr
			conn, _ := connections[addr];
			if conn == nil {
				log.Println("Unsolicited connection done - should not happen")
			} else {
				conn.client = msg.client
				conn.connected = true
				for _, reply := range conn.waitq {
					reply <- conn.client
				}
				conn.waitq = nil
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

	mapping["/fremen/bash/"] = &ServerInfo{
		ServerAddr: "1.1.1.1:22",
		Config: config,
		HttpProxy: HttpProxy{
			LocalAddr: "127.0.0.1:8083",
			LocalPathPrefix: "/",
		},
		Bootstrap: []string{
			"/bin/sh -c '/usr/bin/curl -L https://github.com/yudai/gotty/releases/download/v0.0.12/gotty_linux_arm.tar.gz | /bin/tar zxv ./gotty -O - > /tmp/gotty'",
			"/bin/chmod a+x /tmp/gotty",
		},
		Run: "/usr/bin/nohup /tmp/gotty -p 8083 -w -a 127.0.0.1 /bin/bash",
	}

	go sshConnector()
}