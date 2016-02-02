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
	prefix              string
	info                PathInfo
	subscribeProgress   chan chan ProgressCmd
	progressChan        chan ProgressCmd
	getConn             chan chan net.Conn
	isReadyChan         chan chan bool
	getServerChan       chan GetServerReq
	reconnectServerChan chan chan *ssh.Client
}

func (b *backendStruct)IsReady() bool {
	reply := make(chan bool, 1)
	b.isReadyChan <- reply
	return <-reply
}

func (b *backendStruct)Connect() net.Conn {
	reply := make(chan net.Conn, 1)
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
	var isProvisioned bool
	provisioningDone := make(chan *PathInfo)

	go progressBroker(b.progressChan, b.subscribeProgress)
	go b.sshServerConnector()
	go b.sshClientConnector()
	go waitProvisioning(&b.info, provisioningDone, b.progressChan)

	for {
		select {
		case newInfo := <-provisioningDone:
			if newInfo != nil && !isProvisioned {
				b.info = *newInfo
				isProvisioned = true
			}
		case reply := <-b.isReadyChan:
			if !isProvisioned {
				reply <- false
			} else {
				myReply := make(chan *ssh.Client, 1)
				b.getServerChan <- GetServerReq{reply: myReply, returnDirectly:true}
				client := <-myReply
				reply <- client != nil
			}
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
		make(chan GetServerReq),
		make(chan chan *ssh.Client),
	}

	go self.monitor()

	return &self
}
