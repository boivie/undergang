package app

import (
	"golang.org/x/crypto/ssh"
	"log"
	"time"
	"io/ioutil"
	"net"
	"errors"
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

type ConnectionDone struct {
	updatedInfo *PathInfo
	client      *ssh.Client
	err         error
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

func connectSSH(info *PathInfo, resp chan <- ConnectionDone, progress chan <- ProgressCmd, proxyCommand string) {
	var err error

	progress <- ProgressCmd{"connection_start", nil}
	sshKey := []byte(info.SSHTunnel.SSHKeyContents)
	if info.SSHTunnel.SSHKeyFileName != "" {
		sshKey, err = ioutil.ReadFile(info.SSHTunnel.SSHKeyFileName)
		if err != nil {
			resp <- ConnectionDone{err: errors.New("Failed to read SSH key")}
			return
		}
	}

	key, err := ssh.ParsePrivateKey(sshKey)
	if err != nil {
		resp <- ConnectionDone{err: errors.New("Failed to parse SSH key")}
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
			resp <- ConnectionDone{err: errors.New("Connection retry limit reached")}
			return
		}
	}
	progress <- ProgressCmd{"connection_established", nil}

	runBootstrap(sshClientConn, info, progress)

	if info.SSHTunnel.Run != nil {
		session, _ := sshClientConn.NewSession()
		defer session.Close()
		session.Start(info.SSHTunnel.Run.Command)
		time.Sleep(500 * time.Millisecond)
	}
	resp <- ConnectionDone{updatedInfo: info, client: sshClientConn}
}

type PrefixWorker struct {
	getConn           chan getConnReq
	subscribeProgress chan chan ProgressCmd
	isReadyChan       chan isReadyReq
}

func progressBroker(progressChan <- chan ProgressCmd, subscribeChan <- chan chan ProgressCmd) {
	progress := make([]ProgressCmd, 0)
	subscribers := make([]chan ProgressCmd, 0)
	for {
		select {
		case msg := <-progressChan:
			progress = append(progress, msg)
			for _, sub := range subscribers {
				sub <- msg
			}
		case q := <-subscribeChan:
		// Send all old progress first
			for _, p := range progress {
				q <- p
			}
			subscribers = append(subscribers, q)
		}
	}
}

func (w *PrefixWorker) run(info *PathInfo, proxyCommand string) {
	var client    *ssh.Client
	waitq := make([]chan net.Conn, 0)

	progressChan := make(chan ProgressCmd)
	connectionDoneChan := make(chan ConnectionDone)
	go progressBroker(progressChan, w.subscribeProgress)
	go connectSSH(info, connectionDoneChan, progressChan, proxyCommand)
	for {
		select {
		case req := <-w.getConn:
			if client != nil {
				c, _ := client.Dial("tcp", info.Backend.Address)
				req.Reply <- c
			} else {
				waitq = append(waitq, req.Reply)
			}
		case req := <-w.isReadyChan:
			req.Reply <- client != nil
		case msg := <-connectionDoneChan:
			client = msg.client
			if client != nil {
				progressChan <- ProgressCmd{"connection_success", nil}
				info = msg.updatedInfo
				for _, reply := range waitq {
					c, _ := client.Dial("tcp", info.Backend.Address)
					reply <- c
				}
				waitq = nil
			} else {
				progressChan <- ProgressCmd{"connection_failed", msg.err.Error()}
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
