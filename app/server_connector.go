package app

import (
	"golang.org/x/crypto/ssh"
	"log"
	"time"
	"io/ioutil"
)

const MAX_RETRIES_SERVER = 16

type getConnReq struct {
	SSHTunnel *SSHTunnel
	Reply      chan *ssh.Client
}

type ConnectSSHResponse  struct {
	SSHTunnel *SSHTunnel
	client     *ssh.Client
}

var getConnChan chan getConnReq = make(chan getConnReq)

func dialSSH(info *SSHTunnel, config *ssh.ClientConfig, proxyCommand string) (*ssh.Client, error) {
	if proxyCommand == "" {
		return ssh.Dial(`tcp`, info.Address, config)
	} else {
		conn, err := connectProxy(proxyCommand, info.Address)
		if err != nil {
			return nil, err
		}
		c, chans, reqs, err := ssh.NewClientConn(conn, info.Address, config)
		if err != nil {
			return nil, err
		}
		return ssh.NewClient(c, chans, reqs), nil
	}
}

func connectSSH(info *SSHTunnel, resp chan ConnectSSHResponse, proxyCommand string) {
	var err error

	log.Printf("Connecting to SSH server at %s\n", info.Address)
	sshKey := []byte(info.SSHKeyContents)
	if info.SSHKeyFileName != "" {
		sshKey, err = ioutil.ReadFile(info.SSHKeyFileName)
		if err != nil {
			log.Printf("Failed to read SSH key: '%s'\n", info.SSHKeyFileName)
			resp <- ConnectSSHResponse{SSHTunnel: info}
		}
	}

	key, err := ssh.ParsePrivateKey(sshKey)
	if err != nil {
		log.Println(`Failed to parse PEM key.`)
		resp <- ConnectSSHResponse{SSHTunnel: info}
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
		if sshClientConn, err = dialSSH(info, config, proxyCommand); err == nil {
			break
		}

		currentRetriesServer++
		log.Printf("SSH Connection failed %s: %s\n", info.Address, err.Error())

		if currentRetriesServer < MAX_RETRIES_SERVER {
			log.Println(`Retry...`)
			time.Sleep(1 * time.Second)
		} else {
			log.Println(`No more retries for connecting the SSH server.`)
			resp <- ConnectSSHResponse{SSHTunnel: info}
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
	resp <- ConnectSSHResponse{SSHTunnel:info, client: sshClientConn}
}

func sshConnector(proxyCommand string) {
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
			addr := req.SSHTunnel.Address
			conn, _ := connections[addr];

			if conn != nil && conn.connected {
				req.Reply <- conn.client
			} else {
				if conn == nil {
					conn = &ServerConnection{waitq: make([]chan *ssh.Client, 0)}
					connections[addr] = conn
					go connectSSH(req.SSHTunnel, connectionDone, proxyCommand)
				}
				conn.waitq = append(conn.waitq, req.Reply)
			}
		case msg := <-connectionDone:
			addr := msg.SSHTunnel.Address
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

func getSSHConnection(info *SSHTunnel) *ssh.Client {
	clientChan := make(chan *ssh.Client)
	getConnChan <- getConnReq{info, clientChan}
	return <-clientChan
}
