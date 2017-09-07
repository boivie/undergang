package app

import (
	"io"
	"io/ioutil"
	"net"
	"time"

	"github.com/Sirupsen/logrus"

	"golang.org/x/crypto/ssh"
)

const MAX_RETRIES_SERVER = 60 * 60
const MAX_RETRIES_CLIENT = 60 * 60

type GetServerReq struct {
	reply chan *ssh.Client
}

type ConnectionDone struct {
	client *ssh.Client
	err    error
}

func dialSSH(info *SSHTunnel, config *ssh.ClientConfig, proxyCommand string) (*ssh.Client, error) {
	var conn net.Conn
	var err error

	if proxyCommand == "" {
		conn, err = net.DialTimeout(`tcp`, info.Address, 10*time.Second)
	} else {
		conn, err = connectProxy(proxyCommand, info.Address)
	}
	if err != nil {
		return nil, err
	}
	c, chans, reqs, err := ssh.NewClientConn(conn, info.Address, config)
	if err != nil {
		return nil, err
	}
	return ssh.NewClient(c, chans, reqs), nil
}

func acceptAllHostKeys(hostname string, remote net.Addr, key ssh.PublicKey) error {
	return nil
}

func connectSSH(info PathInfo, resp chan<- *ssh.Client, progress chan<- ProgressCmd) {
	var err error
	log := logrus.New().WithFields(logrus.Fields{
		"type": "ssh-server-conn",
		"host": info.Host,
		"path": info.Prefix,
	})

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
		HostKeyCallback: acceptAllHostKeys,
	}

	currentRetriesServer := 0
	var sshClientConn *ssh.Client

reconnect:
	log.Printf("SSH-connecting to %s\n", info.SSHTunnel.Address)
	for {
		progress <- ProgressCmd{"connection_try", nil}
		if sshClientConn, err = dialSSH(info.SSHTunnel, config, proxyCommand); err == nil {
			break
		}

		currentRetriesServer++
		log.Printf("SSH Connection failed %s: %s\n", info.SSHTunnel.Address, err.Error())

		if currentRetriesServer < (MAX_RETRIES_SERVER / 1) {
			log.Println(`Retry...`)
			progress <- ProgressCmd{"connection_retry", nil}
			time.Sleep(1 * time.Second)
		} else {
			log.Println(`SSH connection limit reached. Aborting`)
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
			ssh.ECHO: 0,
		}

		if err := session.RequestPty("xterm", 80, 40, modes); err != nil {
			log.Fatalf("request for pseudo terminal failed: %s", err)
		}

		session.Start(info.SSHTunnel.Run.Command)
		time.Sleep(500 * time.Millisecond)
	}
	log.Printf("SSH-connection OK. Waiting for %s to be ready...\n", info.Backend.Address)

	progress <- ProgressCmd{"waiting_backend", nil}
	currentRetriesClient := 0
	for {
		log.Printf("Trying to connect to %s...\n", info.Backend.Address)
		var conn net.Conn
		if conn, err = sshClientConn.Dial("tcp", info.Backend.Address); err == nil {
			log.Printf("Connected to %s successfully!\n", info.Backend.Address)
			conn.Close()
			break
		} else if err == io.EOF {
			log.Printf("Disconnected from SSH server while connecting to %s: %v - re-connecting SSH\n", info.Backend.Address, err)
			time.Sleep(1 * time.Second)
			goto reconnect
		} else if err, ok := err.(net.Error); ok && err.Timeout() {
			log.Printf("Timeout connecting to %s: %v - re-connecting SSH\n", info.Backend.Address, err)
			time.Sleep(1 * time.Second)
			goto reconnect
		}
		currentRetriesClient++

		if currentRetriesClient < (MAX_RETRIES_CLIENT / 5) {
			log.Printf("Failed to connect to %s - %v, retrying...\n", info.Backend.Address, err)
			progress <- ProgressCmd{"waiting_backend_retry", nil}
			time.Sleep(5 * time.Second)
		} else {
			log.Printf("Connection limit to %s reached. Aborting.\n", info.Backend.Address)
			progress <- ProgressCmd{"waiting_backend_timeout", "Connection retry limit reached"}
			resp <- nil
			return
		}
	}

	progress <- ProgressCmd{"connection_success", nil}
	resp <- sshClientConn
}
