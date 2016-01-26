package app


import (
	"golang.org/x/crypto/ssh"
	"log"
	"time"
	"net/http"
	"net/http/httputil"
	"net"
	"strings"
)

type getConnReq struct {
	ServerInfo *ServerInfo
	Reply      chan *ssh.Client
}

var mapping []PathInfo = make([]PathInfo, 0)
var getConnChan chan getConnReq = make(chan getConnReq)

const MAX_RETRIES_SERVER = 16

func connectSSH(info *ServerInfo, resp chan ConnectSSHResponse) {
	log.Printf("Connecting to SSH server at %s\n", info.Address)

	key, err := ssh.ParsePrivateKey([]byte(info.SSHKey))
	if err != nil {
		log.Println(`Failed to parse PEM key.`)
		resp <- ConnectSSHResponse{ServerInfo: info}
		return
	}

	config := &ssh.ClientConfig{
		User: info.Username,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(key),
		},
	}

	currentRetriesServer := 0
	var sshClientConn *ssh.Client

	for {
		if sshClientConn, err = ssh.Dial(`tcp`, info.Address, config); err == nil {
			break
		}

		currentRetriesServer++
		log.Printf("SSH Connection failed %s: %s\n", info.Address, err.Error())

		if currentRetriesServer < MAX_RETRIES_SERVER {
			log.Println(`Retry...`)
			time.Sleep(1 * time.Second)
		} else {
			log.Println(`No more retries for connecting the SSH server.`)
			resp <- ConnectSSHResponse{ServerInfo: info}
			return
		}
	}
	log.Printf("SSH Connected to %s\n", info.Address)

	for _, cmd := range info.Bootstrap {
		log.Printf("Bootstrap: %s\n", cmd)
		session, _ := sshClientConn.NewSession()
		defer session.Close()
		session.Run(cmd)
	}
	log.Printf("Bootstrap for %s done", info.Address)
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

func AddPath(path PathInfo) {
	mapping = append(mapping, path)
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
			addr := req.ServerInfo.Address
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
			addr := msg.ServerInfo.Address
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

func findLongestPrefix(mapping []PathInfo, path string) (info *PathInfo) {
	for _, iter := range mapping {
		if strings.HasPrefix(path, iter.Prefix) {
			if info == nil ||  len(info.Prefix) < len(iter.Prefix) {
				info = &iter
			}
		}
	}
	return
}

func getSSHConnection(info *ServerInfo) *ssh.Client {
	clientChan := make(chan *ssh.Client)
	getConnChan <- getConnReq{info, clientChan}
	return <-clientChan
}

func Forward(w http.ResponseWriter, req *http.Request) {
	info := findLongestPrefix(mapping, req.URL.Path)
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
	go sshConnector()
}