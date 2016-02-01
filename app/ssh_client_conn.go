package app
import (
	"golang.org/x/crypto/ssh"
	"net"
)

func (w *backendStruct) sshClientConnector(serverConnectionChan <- chan *ssh.Client) {
	var client    *ssh.Client
	waitq := make([]chan net.Conn, 0)

	for {
		select {
		case reply := <-w.getConn:
			if client != nil {
				c, _ := client.Dial("tcp", w.info.Backend.Address)
				reply <- c
			} else {
				waitq = append(waitq, reply)
			}
		case c := <-serverConnectionChan:
			client = c
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
