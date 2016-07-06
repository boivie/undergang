package app

import (
	"github.com/Sirupsen/logrus"
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
	SSH_SERVER_PROVISIONING = iota
	SSH_SERVER_PROVISIONED  = iota
	SSH_SERVER_CONNECTING   = iota
	SSH_SERVER_CONNECTED    = iota
	SSH_SERVER_FAILED       = iota
)

type backendStruct struct {
	info              PathInfo
	subscribeProgress chan chan ProgressCmd
	getConn           chan chan net.Conn
	isReadyChan       chan chan bool
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
	log := logrus.New().WithFields(logrus.Fields{
		"type": "backend-monitor",
		"host": b.info.Host,
		"path": b.info.Prefix,
	})
	log.Logger = logrus.StandardLogger()
	log.Debug("Starting up")

	progressChan := make(chan ProgressCmd)

	go progressBroker(progressChan, b.subscribeProgress)

	provisioningDone := make(chan *PathInfo)

	state := SSH_SERVER_PROVISIONED
	if !isProvisioned(&b.info) {
		state = SSH_SERVER_PROVISIONING
		go waitProvisioning(&b.info, provisioningDone, progressChan)
	}

	clientWaitQ := make([]chan net.Conn, 0)

	drainClientConnChan := make(chan bool, 100)
	serverConnectionDone := make(chan *ssh.Client)

	var client *ssh.Client

	wd := watchdog(b)

	for {
		select {
		case newInfo := <-provisioningDone:
			log.Debugf("Provisioning done")
			if newInfo != nil {
				b.info = *newInfo
				state = SSH_SERVER_CONNECTING
				go connectSSH(b.info, serverConnectionDone, progressChan)
			} else {
				state = SSH_SERVER_FAILED
			}
		case reply := <-b.isReadyChan:
			if state == SSH_SERVER_PROVISIONED {
				// Kick-start it.
				state = SSH_SERVER_CONNECTING
				go connectSSH(b.info, serverConnectionDone, progressChan)
			}
			reply <- (state == SSH_SERVER_CONNECTED)
		case reply := <-b.getConn:
			clientWaitQ = append(clientWaitQ, reply)
			if client != nil {
				drainClientConnChan <- true
			} else {
				if state == SSH_SERVER_PROVISIONED && b.info.SSHTunnel != nil {
					log.Debugf("Get first connection - connecting to SSH")
					state = SSH_SERVER_CONNECTING
					go connectSSH(b.info, serverConnectionDone, progressChan)
				}
			}
		case <-drainClientConnChan:
			if client != nil {
				var disconnected bool
				if clientWaitQ, disconnected = drainChildWaitq(clientWaitQ, b.info.Backend.Address, client); disconnected {
					// Disconnected!
					client = nil
					state = SSH_SERVER_CONNECTING
					go connectSSH(b.info, serverConnectionDone, progressChan)
				}
			} else {
				log.Printf("got clientConnectionDone but no client")
			}
		case c := <-serverConnectionDone:
			if c != nil {
				state = SSH_SERVER_CONNECTED
				client = c
				drainClientConnChan <- true
			} else if client == nil {
				state = SSH_SERVER_PROVISIONED
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
		make(chan chan net.Conn),
		make(chan chan bool),
	}

	go self.monitor()

	return &self
}
