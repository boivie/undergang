package app

import (
	"errors"
	"io"
	"io/ioutil"
	"net"
	"os"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/crypto/ssh"
)

// Backend represents a tunnel to an app reached by a SSH tunnel
type Backend interface {
	ID() int
	Start()
	IsReady() bool
	Connect() net.Conn
	GetInfo() PathInfo
	Subscribe(chan progressCmd)
	GetLogger() *logrus.Entry
}

type backendStruct struct {
	id                int
	info              PathInfo
	log               *logrus.Entry
	subscribeProgress chan chan progressCmd
	getConn           chan chan net.Conn
	progress          chan progressCmd
	start             chan bool
	isReady           bool
	sshConfig         *ssh.ClientConfig
}

func (b *backendStruct) ID() int {
	return b.id
}

func (b *backendStruct) Start() {
	b.start <- true
}

func (b *backendStruct) IsReady() bool {
	return b.isReady
}

func (b *backendStruct) Connect() net.Conn {
	reply := make(chan net.Conn, 1)
	b.getConn <- reply
	return <-reply
}

func (b *backendStruct) Subscribe(sub chan progressCmd) {
	b.subscribeProgress <- sub
}

func (b *backendStruct) GetInfo() PathInfo {
	return b.info
}

func (b *backendStruct) GetLogger() *logrus.Entry {
	return b.log
}

const maxRetriesServer = 15 * 60
const maxRetriesClient = (10 * 60 / 5)

func generateKeepalive(client *ssh.Client) {
	go func() {
		t := time.NewTicker(2 * time.Second)
		defer t.Stop()
		for {
			<-t.C
			_, _, err := client.Conn.SendRequest("keepalive@openssh.com", true, nil)
			if err != nil {
				return
			}
		}
	}()
}

func (b *backendStruct) isProvisioned() bool {
	return b.info.Provisioning == nil || b.info.Provisioning.Status != "started"
}

func (b *backendStruct) waitProvisioned() error {
	if !b.isProvisioned() {
		start := time.Now()
		b.progress <- progressCmd{"wait_provisioning_start", nil}
		for {
			newInfo := doLookup(b.info.Host, b.info.Prefix)
			if newInfo == nil {
				// TODO: Retry?
				return errors.New("Failed to get info from backend")
			}
			b.info = *newInfo
			if b.isProvisioned() {
				break
			}
			b.log.Info("Provisioning - retry...")
			time.Sleep(5 * time.Second)
		}
		BackendProvisioningDuration.Observe(time.Since(start).Seconds())
		b.log.Info("Provisioning completed")
		b.progress <- progressCmd{"wait_provisioning_end", nil}
	}
	return nil
}

func (b *backendStruct) failed(reason string, err error) {
	b.log.Warnf("ENTER FAILED STATE, due to %s: %v", reason, err)
	BackendFailure.With(prometheus.Labels{"reason": reason}).Inc()
	// lame duck mode.
	UnregisterBackend(b.id)
	for {
		reply := <-b.getConn
		reply <- nil
	}
}

func dialSSH(info *configSSHTunnel, config *ssh.ClientConfig, proxyCommand string) (*ssh.Client, error) {
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

func (b *backendStruct) connectSSH() (client *ssh.Client, err error) {
	start := time.Now()
	b.progress <- progressCmd{"connection_start", nil}
	b.log.Info("Connecting to SSH server")
	for retry := 0; retry < maxRetriesServer; retry++ {
		b.progress <- progressCmd{"connection_try", nil}
		client, err = dialSSH(b.info.SSHTunnel, b.sshConfig, proxyCommand)
		if err == nil {
			BackendConnectSSHDuration.Observe(time.Since(start).Seconds())
			b.log.Infof("Connected to SSH server: %v, err %v", client, err)
			go generateKeepalive(client)
			b.progress <- progressCmd{"connection_established", nil}
			return
		}

		b.log.Warnf("SSH Connection failed: %v - retrying", err)
		b.progress <- progressCmd{"connection_retry", nil}
		time.Sleep(1 * time.Second)
	}
	b.log.Warnf("SSH Connection retry limit reached")
	b.progress <- progressCmd{"connection_failed", "Connection retry limit reached"}
	return nil, errors.New("SSH Connection retry limit reached")
}

func (b *backendStruct) reconnectSSH() (client *ssh.Client, err error) {
	b.progress <- progressCmd{"reconnection_start", nil}
	b.log.Info("Re-connecting to SSH server")
	client, err = dialSSH(b.info.SSHTunnel, b.sshConfig, proxyCommand)
	if err == nil {
		b.log.Infof("Re-connected to SSH server: %v, err %v", client, err)
		BackendReconnectSSH.Inc()
		go generateKeepalive(client)
		b.progress <- progressCmd{"reconnection_established", nil}
		return
	}

	b.log.Warnf("SSH Re-connection failed. Assuming host is down.")
	b.progress <- progressCmd{"reconnection_failed", "Re-connection failed"}
	return nil, err
}

func (b *backendStruct) bootstrap(client *ssh.Client) (err error) {
	if len(b.info.SSHTunnel.Bootstrap) == 0 && b.info.SSHTunnel.Run == nil {
		return
	}

	start := time.Now()

	type BootstrapStep struct {
		Description string `json:"description"`
		Status      string `json:"status"`
	}
	status := struct {
		Steps []BootstrapStep `json:"steps"`
	}{make([]BootstrapStep, 0)}

	for _, cmd := range b.info.SSHTunnel.Bootstrap {
		status.Steps = append(status.Steps, BootstrapStep{cmd.Description, ""})
	}

	var session *ssh.Session
	for idx, cmd := range b.info.SSHTunnel.Bootstrap {
		b.log.Infof("Started running bootstrap '%s'", cmd.Command)
		status.Steps[idx].Status = "started"
		b.progress <- progressCmd{"bootstrap_status", status}
		if session, err = client.NewSession(); err != nil {
			return
		}
		defer session.Close()
		session.Stdout = os.Stdout
		session.Stderr = os.Stderr
		session.Run(cmd.Command)
		status.Steps[idx].Status = "done"
		b.progress <- progressCmd{"bootstrap_status", status}
		b.log.Infof("Finished running bootstrap '%s'", cmd.Command)
	}

	if b.info.SSHTunnel.Run != nil {
		b.log.Info("Running command: '%s'", b.info.SSHTunnel.Run.Command)
		if session, err = client.NewSession(); err != nil {
			return
		}

		modes := ssh.TerminalModes{
			ssh.ECHO: 0,
		}

		if err = session.RequestPty("xterm", 80, 40, modes); err != nil {
			b.log.Warnf("request for pseudo terminal failed: %s", err)
			return
		}

		session.Start(b.info.SSHTunnel.Run.Command)
		time.Sleep(500 * time.Millisecond)
	}
	BackendBootstrapDuration.Observe(time.Since(start).Seconds())
	return
}

func acceptAllHostKeys(hostname string, remote net.Addr, key ssh.PublicKey) error {
	return nil
}

func (b *backendStruct) prepareSSH() (err error) {
	sshKey := []byte(b.info.SSHTunnel.SSHKeyContents)
	if b.info.SSHTunnel.SSHKeyFileName != "" {
		sshKey, err = ioutil.ReadFile(b.info.SSHTunnel.SSHKeyFileName)
		if err != nil {
			b.progress <- progressCmd{"connection_failed", "Failed to read SSH key"}
			return
		}
	}

	key, err := ssh.ParsePrivateKey(sshKey)
	if err != nil {
		b.progress <- progressCmd{"connection_failed", "Failed to parse SSH key"}
		return
	}

	b.sshConfig = &ssh.ClientConfig{
		User: b.info.SSHTunnel.Username,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(key),
		},
		HostKeyCallback: acceptAllHostKeys,
	}
	return
}

func (b *backendStruct) connectionCreator(client *ssh.Client, onError chan error) {
	putBack := func(reply chan net.Conn) {
		select {
		case b.getConn <- reply:
		default:
			reply <- nil
		}
	}

	for {
		reply := <-b.getConn
		conn, err := client.Dial("tcp", b.info.Backend.Address)
		if err != nil {
			if err == io.EOF {
				// Disconnected from the SSH server.
				putBack(reply)
				onError <- err
				return
			} else if err2, ok := err.(net.Error); ok && err2.Timeout() {
				putBack(reply)
				onError <- err2
				return
			} else {
				conn = nil
			}
		}
		reply <- conn
	}
}

func (b *backendStruct) waitBackend(client *ssh.Client) (err error) {
	b.progress <- progressCmd{"waiting_backend", nil}
	for retries := 0; retries < maxRetriesClient; retries++ {
		b.log.Info("Waiting for backend to be ready...")
		var conn net.Conn
		if conn, err = client.Dial("tcp", b.info.Backend.Address); err == nil {
			defer conn.Close()
			b.log.Info("Backend is ready.")
			b.progress <- progressCmd{"connection_success", nil}
			return
		} else if err == io.EOF {
			b.log.Warnf("Disconnected from SSH server while connecting to %s: %v - re-connecting SSH", b.info.Backend.Address, err)
			return
		} else if err2, ok := err.(net.Error); ok && err2.Timeout() {
			b.log.Warnf("Timeout connecting to %s: %v - re-connecting SSH", b.info.Backend.Address, err)
			return
		}

		b.log.Warnf("Backend not ready yet. (%v)", err)
		b.progress <- progressCmd{"waiting_backend_retry", nil}
		time.Sleep(5 * time.Second)
	}
	b.log.Warn("Waiting backend retry limit reached. Aborting.")
	b.progress <- progressCmd{"waiting_backend_timeout", "Connection retry limit reached"}
	err = errors.New("Backend retry limit reached")
	return
}

func (b *backendStruct) waitUntilStarted() {
	<-b.start
	BackendsStarted.Inc()
	b.log.Info("Woke up")
	// Just a goroutine that eats up all future start calls.
	go func() {
		for range b.start {
		}
	}()
}

func (b *backendStruct) monitor() {
	var client *ssh.Client
	var err error

	// Don't connect until we get our initial connection attempt.
	b.waitUntilStarted()

	if err = b.waitProvisioned(); err != nil {
		b.failed("provisioning", err)
	}

	b.log = b.log.WithFields(logrus.Fields{
		"ssh_host": b.info.SSHTunnel.Address,
		"backend":  b.info.Backend.Address,
	})

	if err = b.prepareSSH(); err != nil {
		b.failed("prepare_ssh", err)
	}

	if client, err = b.connectSSH(); err != nil {
		b.failed("connect_ssh", err)
	}

	if err = b.bootstrap(client); err != nil {
		b.failed("bootstrap", err)
	}

	if err = b.waitBackend(client); err != nil {
		b.failed("wait_backend_ready", err)
	}
	b.isReady = true

	connectionError := make(chan error)
	for {
		go b.connectionCreator(client, connectionError)
		err = <-connectionError
		b.log.Warnf("Connection error: %v - reconnecting", err)
		if client, err = b.reconnectSSH(); err != nil {
			b.failed("reconnect_ssh", err)
		}
	}
}

// NewBackend instantiates a new backend
func NewBackend(id int, info PathInfo) Backend {
	log := logrus.New().WithFields(logrus.Fields{
		"type":       "backend",
		"backend_id": id,
		"host":       info.Host,
		"path":       info.Prefix,
	})
	log.Logger = logrus.StandardLogger()

	self := backendStruct{
		id,
		info,
		log,
		make(chan chan progressCmd),
		make(chan chan net.Conn, 1000),
		make(chan progressCmd),
		make(chan bool),
		false,
		nil,
	}
	go progressBroker(self.progress, self.subscribeProgress)
	go self.monitor()

	return &self
}
