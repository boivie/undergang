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

type isReadyReq struct {
	info  *PathInfo
	Reply chan bool
}

type subscribeReq struct {
	prefix string
	topic  chan ProgressCmd
}

var getConnChan chan getConnReq = make(chan getConnReq)
var subscribeChan chan subscribeReq = make(chan subscribeReq)
var isReadyChan chan isReadyReq = make(chan isReadyReq)

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

func connectSSH(info *PathInfo, resp chan *ssh.Client, progress chan ProgressCmd, proxyCommand string) {
	var err error
	progress <- ProgressCmd{"connection_start", nil}
	sshKey := []byte(info.SSHTunnel.SSHKeyContents)
	if info.SSHTunnel.SSHKeyFileName != "" {
		sshKey, err = ioutil.ReadFile(info.SSHTunnel.SSHKeyFileName)
		if err != nil {
			log.Printf("Failed to read SSH key: '%s'\n", info.SSHTunnel.SSHKeyFileName)
			resp <- nil
			progress <- ProgressCmd{"connection_failed", nil}
		}
	}

	key, err := ssh.ParsePrivateKey(sshKey)
	if err != nil {
		log.Println(`Failed to parse PEM key.`)
		resp <- nil
		progress <- ProgressCmd{"connection_failed", nil}
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
		progress <- ProgressCmd{"connection_try", nil}
		if sshClientConn, err = dialSSH(info.SSHTunnel, config, proxyCommand); err == nil {
			break
		}

		currentRetriesServer++
		log.Printf("SSH Connection failed %s: %s\n", info.SSHTunnel.Address, err.Error())

		if currentRetriesServer < MAX_RETRIES_SERVER {
			log.Println(`Retry...`)
			progress <- ProgressCmd{"connection_retry", nil}
			time.Sleep(1 * time.Second)
		} else {
			log.Println(`No more retries for connecting the SSH server.`)
			resp <- nil
			progress <- ProgressCmd{"connection_failed", nil}
			return
		}
	}
	log.Printf("SSH Connected to %s\n", info.SSHTunnel.Address)
	progress <- ProgressCmd{"connection_ok", nil}

	for _, cmd := range info.SSHTunnel.Bootstrap {
		progress <- ProgressCmd{"bootstrap_begin", cmd}
		session, _ := sshClientConn.NewSession()
		defer session.Close()
		session.Run(cmd)
		progress <- ProgressCmd{"bootstrap_end", cmd}
	}
	progress <- ProgressCmd{"bootstrap_completed", nil}
	if info.SSHTunnel.Run != "" {
		session, _ := sshClientConn.NewSession()
		defer session.Close()
		session.Start(info.SSHTunnel.Run)
		time.Sleep(500 * time.Millisecond)
	}
	log.Println("End connect SSH")
	resp <- sshClientConn
	progress <- ProgressCmd{"connection_success", nil}
}

type PrefixWorker struct {
	getConn           chan getConnReq
	subscribeProgress chan chan ProgressCmd
	isReadyChan       chan isReadyReq
}

func (w *PrefixWorker) run(info *PathInfo, proxyCommand string) {
	var client    *ssh.Client
	waitq := make([]chan net.Conn, 0)
	progress := make([]ProgressCmd, 0)
	subscribers := make([]chan ProgressCmd, 0)

	progressChan := make(chan ProgressCmd)
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
		case q := <-w.subscribeProgress:
		// Send all old progress first
			for _, p := range progress {
				q <- p
			}
			subscribers = append(subscribers, q)
		case req := <-w.isReadyChan:
			req.Reply <- client != nil
		case msg := <-connectionDoneChan:
			client = msg
			for _, reply := range waitq {
				c, _ := client.Dial("tcp", info.Backend.Address)
				reply <- c
			}
			waitq = nil
		case msg := <-progressChan:
			progress = append(progress, msg)
			for _, sub := range subscribers {
				sub <- msg
			}
		}
	}
}

func ensurePresent(workers map[string]*PrefixWorker, info *PathInfo, proxyCommand string) *PrefixWorker {
	worker, _ := workers[info.Prefix];
	if worker == nil {
		worker = &PrefixWorker{
			getConn: make(chan getConnReq),
			subscribeProgress: make(chan chan ProgressCmd),
			isReadyChan: make(chan isReadyReq),
		}
		workers[info.Prefix] = worker
		go worker.run(info, proxyCommand)
	}
	return worker
}

func sshMuxer(proxyCommand string) {
	workers := make(map[string]*PrefixWorker)
	for {
		select {
		case req := <-getConnChan:
			worker := ensurePresent(workers, req.info, proxyCommand)
			worker.getConn <- req
		case req := <-subscribeChan:
			if worker, ok := workers[req.prefix]; ok {
				worker.subscribeProgress <- req.topic
			}
		case req := <-isReadyChan:
			worker := ensurePresent(workers, req.info, proxyCommand)
			worker.isReadyChan <- req
		}
	}
}

func getBackendConnection(info *PathInfo) net.Conn {
	replyChan := make(chan net.Conn)
	getConnChan <- getConnReq{info, replyChan}
	return <-replyChan
}
