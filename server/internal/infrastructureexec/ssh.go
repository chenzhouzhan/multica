// Package infrastructureexec executes bounded Docker and Kubernetes operations on a configured node.
package infrastructureexec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type Access struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Username   string `json:"username"`
	HostKey    string `json:"host_key"`
	Context    string `json:"context"`
	PrivateKey string `json:"private_key"`
	Password   string `json:"password"`
}

func (a Access) ValidateCredentials() error {
	if strings.TrimSpace(a.Host) == "" || strings.ContainsAny(a.Host, "/\\\x00\r\n ") || a.Port < 1 || a.Port > 65535 || strings.TrimSpace(a.Username) == "" || len(a.Context) > 256 {
		return errors.New("SSH host, port and username are required; the Kubernetes connection name must be at most 256 bytes")
	}
	if a.PrivateKey != "" {
		if a.Password != "" {
			return errors.New("choose one SSH authentication method")
		}
		if _, err := ssh.ParsePrivateKey([]byte(a.PrivateKey)); err != nil {
			return errors.New("SSH private_key must be an unencrypted OpenSSH or PEM private key")
		}
	} else if a.Password == "" {
		return errors.New("an SSH private key or password is required")
	}
	return nil
}

func (a Access) Validate() error {
	if err := a.ValidateCredentials(); err != nil {
		return err
	}
	if _, _, _, _, err := ssh.ParseAuthorizedKey([]byte(a.HostKey)); err != nil {
		return errors.New("SSH host_key must be a trusted OpenSSH public key")
	}
	return nil
}

type Runner interface {
	Run(context.Context, ...string) ([]byte, error)
	Close() error
}

type Dialer func(context.Context, Access) (Runner, error)

type sshRunner struct{ client *ssh.Client }

func Dial(ctx context.Context, access Access) (Runner, error) {
	if err := access.Validate(); err != nil {
		return nil, err
	}
	key, _, _, _, _ := ssh.ParseAuthorizedKey([]byte(access.HostKey))
	var auth ssh.AuthMethod
	if access.PrivateKey != "" {
		signer, _ := ssh.ParsePrivateKey([]byte(access.PrivateKey))
		auth = ssh.PublicKeys(signer)
	} else {
		auth = ssh.Password(access.Password)
	}
	address := net.JoinHostPort(access.Host, strconv.Itoa(access.Port))
	conn, err := (&net.Dialer{Timeout: 15 * time.Second}).DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, fmt.Errorf("connect to execution node: %w", err)
	}
	deadline := time.Now().Add(15 * time.Second)
	if value, ok := ctx.Deadline(); ok && value.Before(deadline) {
		deadline = value
	}
	_ = conn.SetDeadline(deadline)
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	verify := ssh.FixedHostKey(key)
	connection, channels, requests, err := ssh.NewClientConn(conn, address, &ssh.ClientConfig{User: access.Username, Auth: []ssh.AuthMethod{auth}, HostKeyCallback: func(hostname string, remote net.Addr, observed ssh.PublicKey) error {
		if err := verify(hostname, remote, observed); err != nil {
			return ErrHostKeyChanged
		}
		return nil
	}})
	if err != nil {
		conn.Close()
		if errors.Is(err, ErrHostKeyChanged) {
			return nil, ErrHostKeyChanged
		}
		return nil, errors.New("SSH authentication or host key verification failed")
	}
	_ = conn.SetDeadline(time.Time{})
	return &sshRunner{ssh.NewClient(connection, channels, requests)}, nil
}

func (r *sshRunner) Close() error { return r.client.Close() }

func ShellCommand(args ...string) string {
	quoted := make([]string, len(args))
	for i, arg := range args {
		quoted[i] = "'" + strings.ReplaceAll(arg, "'", "'\\''") + "'"
	}
	return strings.Join(quoted, " ")
}

func executionCommand(args ...string) (string, error) {
	if len(args) == 0 || (args[0] != "kubectl" && args[0] != "docker") {
		return "", errors.New("unsupported infrastructure command")
	}
	// Load the same user configuration as an SSH login. Keep the allowlisted
	// program unquoted so Bash expands aliases; every argument stays literal.
	command := args[0]
	if len(args) > 1 {
		command += " " + ShellCommand(args[1:]...)
	}
	// Keep startup diagnostics from an interactive shell without a terminal out
	// of command errors, then restore the command's stderr before execution.
	return ShellCommand("bash", "-lic", "exec 2>&3 3>&-; "+command) + " 3>&2 2>/dev/null", nil
}

type tailWriter struct {
	data []byte
	max  int
}

func (w *tailWriter) Write(data []byte) (int, error) {
	count := len(data)
	if len(data) >= w.max {
		w.data = append(w.data[:0], data[len(data)-w.max:]...)
	} else {
		if extra := len(w.data) + len(data) - w.max; extra > 0 {
			w.data = w.data[extra:]
		}
		w.data = append(w.data, data...)
	}
	return count, nil
}

func (r *sshRunner) Run(ctx context.Context, args ...string) ([]byte, error) {
	command, err := executionCommand(args...)
	if err != nil {
		return nil, err
	}
	session, err := r.client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()
	stop := context.AfterFunc(ctx, func() { _ = r.client.Close() })
	defer stop()
	stdout, stderr := &tailWriter{max: 16 << 20}, &tailWriter{max: 8192}
	session.Stdout, session.Stderr = stdout, stderr
	err = session.Run(command)
	if err != nil {
		message := strings.TrimSpace(string(bytes.ToValidUTF8(stderr.data, []byte("?"))))
		if message != "" {
			return stdout.data, fmt.Errorf("%w: %s", err, message)
		}
		return stdout.data, err
	}
	return stdout.data, nil
}

func DefiniteFailure(err error) bool {
	var exit *ssh.ExitError
	return errors.As(err, &exit)
}
