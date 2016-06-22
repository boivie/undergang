package app

import (
	"io"
	"net"
	"os"
	"os/exec"
	"time"
)

type proxyConnection struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
}

func (p *proxyConnection) Read(b []byte) (n int, err error) {
	return p.stdout.Read(b)
}

func (p *proxyConnection) Write(b []byte) (n int, err error) {
	return p.stdin.Write(b)
}

func (p *proxyConnection) Close() error {
	return p.cmd.Process.Kill()
}

func (p *proxyConnection) LocalAddr() net.Addr {
	return nil
}

func (p *proxyConnection) RemoteAddr() net.Addr {
	return nil
}

func (p *proxyConnection) SetDeadline(t time.Time) error {
	return nil
}

func (p *proxyConnection) SetReadDeadline(t time.Time) error {
	return nil
}

func (p *proxyConnection) SetWriteDeadline(t time.Time) error {
	return nil
}

func connectProxy(proxyCommand, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(proxyCommand, host, port)
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &proxyConnection{cmd, stdin, stdout}, nil
}
