package app

import (
	"golang.org/x/crypto/ssh"
	"net"
)

type backend interface {
	IsReady() bool
	Connect() net.Conn
	GetInfo() PathInfo
	Subscribe(chan ProgressCmd)
}

const (
	SSH_SERVER_DISCONNECTED = iota
	SSH_SERVER_CONNECTING   = iota
	SSH_SERVER_CONNECTED    = iota
)

type backendStruct struct {
	info                PathInfo
	subscribeProgress   chan chan ProgressCmd
	progressChan        chan ProgressCmd
	getConn             chan chan net.Conn
	isReadyChan         chan chan bool
	getServerChan       chan GetServerReq
	reconnectServerChan chan chan *ssh.Client
}

func (b *backendStruct) IsReady() bool {
	reply := make(chan bool, 1)
	b.isReadyChan <- reply
	return <-reply
}

func (b *backendStruct) Connect() net.Conn {
	reply := make(chan net.Conn, 1)
	b.getConn <- reply
	return <-reply
}

func (b *backendStruct) Subscribe(sub chan ProgressCmd) {
	b.subscribeProgress <- sub
}

func (b *backendStruct) GetInfo() PathInfo {
	return b.info
}

func (b *backendStruct) monitor() {
	isProvisioned := isProvisioned(&b.info)
	provisioningDone := make(chan *PathInfo)

	go progressBroker(b.progressChan, b.subscribeProgress)
	if !isProvisioned {
		go waitProvisioning(&b.info, provisioningDone, b.progressChan)
	}

	clientWaitQ := make([]chan net.Conn, 0)

	clientConnectionDone := make(chan *ssh.Client, 100)

	var client *ssh.Client
	state := SSH_SERVER_DISCONNECTED
	serverWaitQ := make([]chan *ssh.Client, 0)

	wd := watchdog(b)

	connectionDone := make(chan *ssh.Client)

	for {
		select {
		case newInfo := <-provisioningDone:
			if newInfo != nil {
				b.info = *newInfo
				isProvisioned = true

				// Trigger a "connect to SSH"
				myReply := make(chan *ssh.Client, 1)
				b.getServerChan <- GetServerReq{reply: myReply, returnDirectly: true}
				<-myReply
			}
		case reply := <-b.isReadyChan:
			if !isProvisioned {
				reply <- false
			} else {
				myReply := make(chan *ssh.Client, 1)
				b.getServerChan <- GetServerReq{reply: myReply, returnDirectly: true}
				client := <-myReply
				reply <- client != nil
			}
		case reply := <-b.getConn:
			clientWaitQ = append(clientWaitQ, reply)
			b.getServerChan <- GetServerReq{reply: clientConnectionDone}
		case client := <-clientConnectionDone:
			if client != nil {
				var disconnected bool
				if clientWaitQ, disconnected = drainChildWaitq(clientWaitQ, b.info.Backend.Address, client); disconnected {
					b.reconnectServerChan <- clientConnectionDone
				}
			}
		case req := <-b.getServerChan:
			if req.returnDirectly || client != nil {
				req.reply <- client
			} else {
				serverWaitQ = append(serverWaitQ, req.reply)
			}
			if client == nil && state == SSH_SERVER_DISCONNECTED && b.info.SSHTunnel != nil {
				state = SSH_SERVER_CONNECTING
				go connectSSH(b.info, connectionDone, b.progressChan)
			}
		case c := <-connectionDone:
			client = c
			if c != nil {
				state = SSH_SERVER_CONNECTED
				for _, reply := range serverWaitQ {
					reply <- c
				}
				serverWaitQ = nil
			} else {
				state = SSH_SERVER_DISCONNECTED
			}
		case reply := <-b.reconnectServerChan:
			serverWaitQ = append(serverWaitQ, reply)
			if state != SSH_SERVER_CONNECTING {
				client = nil
				state = SSH_SERVER_CONNECTING
				go connectSSH(b.info, connectionDone, b.progressChan)
			}

		case bark := <-wd:
			bark <- true
		}
	}
}

func NewBackend(info PathInfo) backend {
	self := backendStruct{
		info,
		make(chan chan ProgressCmd),
		make(chan ProgressCmd),
		make(chan chan net.Conn),
		make(chan chan bool),
		make(chan GetServerReq),
		make(chan chan *ssh.Client),
	}

	go self.monitor()

	return &self
}
