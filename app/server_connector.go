package app

import (
	"golang.org/x/crypto/ssh"
	"log"
	"time"
	"io/ioutil"
	"errors"
)

const MAX_RETRIES_SERVER = 16
var proxyCommand string

type ConnectionDone struct {
	client *ssh.Client
	err    error
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

func connectSSH(info PathInfo, resp chan <- ConnectionDone, progress chan <- ProgressCmd) {
	var err error

	progress <- ProgressCmd{"connection_start", nil}
	sshKey := []byte(info.SSHTunnel.SSHKeyContents)
	if info.SSHTunnel.SSHKeyFileName != "" {
		sshKey, err = ioutil.ReadFile(info.SSHTunnel.SSHKeyFileName)
		if err != nil {
			progress <- ProgressCmd{"connection_failed", nil}
			resp <- ConnectionDone{err: errors.New("Failed to read SSH key")}
			return
		}
	}

	key, err := ssh.ParsePrivateKey(sshKey)
	if err != nil {
		progress <- ProgressCmd{"connection_failed", nil}
		resp <- ConnectionDone{err: errors.New("Failed to parse SSH key")}
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
			progress <- ProgressCmd{"connection_failed", nil}
			resp <- ConnectionDone{err: errors.New("Connection retry limit reached")}
			return
		}
	}
	progress <- ProgressCmd{"connection_established", nil}

	runBootstrap(sshClientConn, info, progress)

	if info.SSHTunnel.Run != nil {
		session, _ := sshClientConn.NewSession()
		defer session.Close()
		session.Start(info.SSHTunnel.Run.Command)
		time.Sleep(500 * time.Millisecond)
	}
	progress <- ProgressCmd{"connection_success", nil}
	resp <- ConnectionDone{client: sshClientConn}
}
