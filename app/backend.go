package app
import (
	"net"
	"golang.org/x/crypto/ssh"
)

type backend interface {
	IsReady() bool
	Connect() net.Conn
	GetInfo() PathInfo
	Subscribe(chan ProgressCmd)
}

type backendStruct struct {
	prefix            string
	info              PathInfo
	subscribeProgress chan chan ProgressCmd
	progressChan      chan ProgressCmd
	getConn           chan chan net.Conn
	isReadyChan       chan chan bool
}

func (b *backendStruct)IsReady() bool {
	reply := make(chan bool)
	b.isReadyChan <- reply
	return <-reply
}

func (b *backendStruct)Connect() net.Conn {
	reply := make(chan net.Conn)
	b.getConn <- reply
	return <-reply
}

func (b *backendStruct)Subscribe(sub chan ProgressCmd) {
	b.subscribeProgress <- sub
}

func (b *backendStruct)GetInfo() PathInfo {
	return b.info
}

func (b *backendStruct)monitor() {
	var client *ssh.Client
	var isProvisioned bool
	provisioningDone := make(chan bool)
	connectionDone := make(chan ConnectionDone)
	clientConnectionDone := make(chan *ssh.Client)

	go progressBroker(b.progressChan, b.subscribeProgress)
	go b.sshClientConnector(clientConnectionDone)
	go b.waitProvisioning(provisioningDone)

	// Are we provisioned yet? Wait until that is done.
	for {
		select {
		case provisioned := <-provisioningDone:
			if provisioned && !isProvisioned {
				isProvisioned = true
				go connectSSH(b.info, connectionDone, b.progressChan)
			}
		case reply := <-b.isReadyChan:
			reply <- client != nil
		case c := <-connectionDone:
			client = c.client
			clientConnectionDone <- client
		}
	}
}

func NewBackend(info PathInfo) backend {
	self := backendStruct{
		info.Prefix,
		info,
		make(chan chan ProgressCmd),
		make(chan ProgressCmd),
		make(chan chan net.Conn),
		make(chan chan bool),
	}

	go self.monitor()

	return &self
}
