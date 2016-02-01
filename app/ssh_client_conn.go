package app
import (
	"golang.org/x/crypto/ssh"
	"net"
)

func (w *backendStruct) sshClientConnector() {
	waitq := make([]chan net.Conn, 0)

	connectionDone := make(chan *ssh.Client)
	for {
		select {
		case reply := <-w.getConn:
			waitq = append(waitq, reply)
			w.getServerChan <- GetServerReq{reply:connectionDone}
		case client := <-connectionDone:
			if client != nil {
				for _, reply := range waitq {
					conn, _ := client.Dial("tcp", w.info.Backend.Address)
					reply <- conn
				}
				waitq = nil
			}
		}
	}
}
