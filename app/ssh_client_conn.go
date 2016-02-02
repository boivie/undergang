package app
import (
	"golang.org/x/crypto/ssh"
	"net"
	"io"
	"log"
)

func drainChildWaitq(waitq []chan net.Conn, address string, client *ssh.Client) ([]chan net.Conn, bool) {
	for len(waitq) > 0 {
		reply := waitq[0]
		conn, err := client.Dial("tcp", address)
		if err != nil && err == io.EOF {
			// Disconnected from the SSH server.
			return waitq, true
		} else if err != nil {
			log.Printf("Failed to connect to backend server at %s\n", address)
		}
		reply <- conn

		waitq = waitq[1:]
	}
	return waitq, false
}

func (w *backendStruct) sshClientConnector() {
	waitq := make([]chan net.Conn, 0)

	connectionDone := make(chan *ssh.Client, 100)
	for {
		select {
		case reply := <-w.getConn:
			waitq = append(waitq, reply)
			w.getServerChan <- GetServerReq{reply:connectionDone}
		case client := <-connectionDone:
			if client != nil {
				var disconnected bool
				if waitq, disconnected = drainChildWaitq(waitq, w.info.Backend.Address, client); disconnected {
					w.reconnectServerChan <- connectionDone
				}
			}
		}
	}
}
