package app

import (
	"golang.org/x/crypto/ssh"
	"io"
	"log"
	"net"
)

func drainChildWaitq(waitq []chan net.Conn, address string, client *ssh.Client) ([]chan net.Conn, bool) {
	for len(waitq) > 0 {
		reply := waitq[0]
		conn, err := client.Dial("tcp", address)
		if err != nil {
			if err == io.EOF {
				// Disconnected from the SSH server.
				return waitq, true
			} else if err, ok := err.(net.Error); ok && err.Timeout() {
				log.Print(err)
				log.Printf("Timeout. Does this mean that we should recycle the connection? %v", address)
				return waitq, true
			} else {
				log.Print(err)
				log.Printf("Failed to connect to backend server at %s\n", address)
			}
		}
		reply <- conn

		waitq = waitq[1:]
	}
	return waitq, false
}
