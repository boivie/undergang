package app

import (
	"golang.org/x/crypto/ssh"
	"log"
	"time"
	"io/ioutil"
	"net"
)

const MAX_RETRIES_SERVER = 16

type getConnReq struct {
	info  *PathInfo
	Reply chan net.Conn
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

func connectSSH(info *PathInfo, resp chan *ssh.Client, progress chan progressCmd, proxyCommand string) {
	var err error

	log.Printf("Connecting to SSH server at %s\n", info.SSHTunnel.Address)
	sshKey := []byte(info.SSHTunnel.SSHKeyContents)
	if info.SSHTunnel.SSHKeyFileName != "" {
		sshKey, err = ioutil.ReadFile(info.SSHTunnel.SSHKeyFileName)
		if err != nil {
			log.Printf("Failed to read SSH key: '%s'\n", info.SSHTunnel.SSHKeyFileName)
			resp <- nil
		}
	}

	key, err := ssh.ParsePrivateKey(sshKey)
	if err != nil {
		log.Println(`Failed to parse PEM key.`)
		resp <- nil
		return
	}

	config := &ssh.ClientConfig{
		User: info.SSHTunnel.Username,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(key),
		},
	}

	currentRetriesServer := 0
	var sshClientConn *ssh.Client

	for {
		if sshClientConn, err = dialSSH(info.SSHTunnel, config, proxyCommand); err == nil {
			break
		}

		currentRetriesServer++
		log.Printf("SSH Connection failed %s: %s\n", info.SSHTunnel.Address, err.Error())

		if currentRetriesServer < MAX_RETRIES_SERVER {
			log.Println(`Retry...`)
			time.Sleep(1 * time.Second)
		} else {
			log.Println(`No more retries for connecting the SSH server.`)
			resp <- nil
			return
		}
	}
	log.Printf("SSH Connected to %s\n", info.SSHTunnel.Address)

	for _, cmd := range info.SSHTunnel.Bootstrap {
		log.Printf("Bootstrap: %s\n", cmd)
		session, _ := sshClientConn.NewSession()
		defer session.Close()
		session.Run(cmd)
	}
	log.Printf("Bootstrap for %s done", info.SSHTunnel.Address)
	if info.SSHTunnel.Run != "" {
		session, _ := sshClientConn.NewSession()
		defer session.Close()
		session.Start(info.SSHTunnel.Run)
		time.Sleep(500 * time.Millisecond)
	}
	resp <- sshClientConn
}

type PrefixWorker struct {
	getConn chan getConnReq
}

type progressCmd struct {
	kind string
	data interface{}
}

func (w *PrefixWorker) run(info *PathInfo, proxyCommand string) {
	var client    *ssh.Client
	waitq := make([]chan net.Conn, 0)
	progress := make([]progressCmd, 0)

	progressChan := make(chan progressCmd)
	connectionDoneChan := make(chan *ssh.Client)
	go connectSSH(info, connectionDoneChan, progressChan, proxyCommand)

	for {
		select {
		case req := <-w.getConn:
			if client != nil {
				c, _ := client.Dial("tcp", req.info.Backend.Address)
				req.Reply <- c
			} else {
				waitq = append(waitq, req.Reply)
			}

		case msg := <-connectionDoneChan:
			client = msg
			for _, reply := range waitq {
				c, _ := client.Dial("tcp", info.Backend.Address)
				reply <- c
			}
			waitq = nil
		case msg := <-progressChan:
			progress = append(progress, msg)
		}
	}
}

func sshMuxer(proxyCommand string) {
	workers := make(map[string]*PrefixWorker)
	for {
		select {
		case req := <-getConnChan:
			worker, _ := workers[req.info.Prefix];
			if worker == nil {
				worker = &PrefixWorker{getConn: make(chan getConnReq)}
				workers[req.info.Prefix] = worker
				go worker.run(req.info, proxyCommand)
			}

			worker.getConn <- req
		}
	}
}

func getBackendConnection(info *PathInfo) net.Conn {
	replyChan := make(chan net.Conn)
	getConnChan <- getConnReq{info, replyChan}
	return <-replyChan
}
