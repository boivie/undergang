package app

import (
	"golang.org/x/crypto/ssh"
	"log"
	"time"
	"io/ioutil"
)

const MAX_RETRIES_SERVER = 16

type GetServerReq struct {
	reply          chan *ssh.Client
	returnDirectly bool
}

type ConnectionDone struct {
	client *ssh.Client
	err    error
}

const (
	SSH_SERVER_DISCONNECTED = iota
	SSH_SERVER_CONNECTING = iota
	SSH_SERVER_CONNECTED = iota
)

func (b *backendStruct)sshServerConnector() {
	var client *ssh.Client
	state := SSH_SERVER_DISCONNECTED
	waitq := make([]chan *ssh.Client, 0)

	connectionDone := make(chan *ssh.Client)
	for {
		select {
		case req := <-b.getServerChan:
			if req.returnDirectly || client != nil {
				req.reply <- client
			} else {
				waitq = append(waitq, req.reply)
			}
			if client == nil && state == SSH_SERVER_DISCONNECTED && b.info.SSHTunnel != nil {
				state = SSH_SERVER_CONNECTING
				go connectSSH(b.info, connectionDone, b.progressChan)
			}
		case c := <-connectionDone:
			client = c
			if c != nil {
				state = SSH_SERVER_CONNECTED
				for _, reply := range waitq {
					reply <- c
				}
				waitq = nil
			} else {
				state = SSH_SERVER_DISCONNECTED
			}
		case reply := <-b.reconnectServerChan:
			waitq = append(waitq, reply)
			if state != SSH_SERVER_CONNECTING {
				client = nil
				state = SSH_SERVER_CONNECTING
				go connectSSH(b.info, connectionDone, b.progressChan)
			}
		}
	}
}

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

func connectSSH(info PathInfo, resp chan <- *ssh.Client, progress chan <- ProgressCmd) {
	var err error
	log.Printf("SSH-connecting to %s\n", info.SSHTunnel.Address)

	progress <- ProgressCmd{"connection_start", nil}
	sshKey := []byte(info.SSHTunnel.SSHKeyContents)
	if info.SSHTunnel.SSHKeyFileName != "" {
		sshKey, err = ioutil.ReadFile(info.SSHTunnel.SSHKeyFileName)
		if err != nil {
			progress <- ProgressCmd{"connection_failed", "Failed to read SSH key"}
			resp <- nil
			return
		}
	}

	key, err := ssh.ParsePrivateKey(sshKey)
	if err != nil {
		progress <- ProgressCmd{"connection_failed", "Failed to parse SSH key"}
		resp <- nil
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
			progress <- ProgressCmd{"connection_failed", "Connection retry limit reached"}
			resp <- nil
			return
		}
	}
	progress <- ProgressCmd{"connection_established", nil}

	runBootstrap(sshClientConn, info, progress)

	if info.SSHTunnel.Run != nil {
		session, _ := sshClientConn.NewSession()

		modes := ssh.TerminalModes{
			ssh.ECHO:          0,
		}

		if err := session.RequestPty("xterm", 80, 40, modes); err != nil {
			log.Fatalf("request for pseudo terminal failed: %s", err)
		}

		session.Start(info.SSHTunnel.Run.Command)
		time.Sleep(500 * time.Millisecond)
	}
	log.Printf("SSH-connection OK\n")
	progress <- ProgressCmd{"connection_success", nil}
	resp <- sshClientConn
}
