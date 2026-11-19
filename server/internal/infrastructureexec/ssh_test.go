package infrastructureexec

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net"
	"strconv"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func testSSHKey(t *testing.T) ssh.Signer {
	t.Helper()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(private)
	if err != nil {
		t.Fatal(err)
	}
	return signer
}

func TestSSHExecutionVerifiesTheHostAndDoesNotExecuteUnapprovedCommands(t *testing.T) {
	signer := testSSHKey(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	commands := make(chan string, 2)
	serverConfig := &ssh.ServerConfig{PasswordCallback: func(metadata ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
		if metadata.User() == "deploy" && string(password) == "test-password" {
			return nil, nil
		}
		return nil, errors.New("incorrect test credentials")
	}}
	serverConfig.AddHostKey(signer)
	go func() {
		for {
			socket, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				connection, channels, requests, err := ssh.NewServerConn(socket, serverConfig)
				if err != nil {
					socket.Close()
					return
				}
				defer connection.Close()
				go ssh.DiscardRequests(requests)
				for next := range channels {
					if next.ChannelType() != "session" {
						next.Reject(ssh.UnknownChannelType, "session required")
						continue
					}
					channel, requests, err := next.Accept()
					if err != nil {
						return
					}
					for request := range requests {
						if request.Type != "exec" {
							request.Reply(false, nil)
							continue
						}
						var payload struct{ Command string }
						if ssh.Unmarshal(request.Payload, &payload) != nil {
							channel.Close()
							return
						}
						commands <- payload.Command
						request.Reply(true, nil)
						channel.Write([]byte("verified test response"))
						channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
						channel.Close()
						break
					}
				}
			}()
		}
	}()
	host, portText, _ := net.SplitHostPort(listener.Addr().String())
	port, _ := strconv.Atoi(portText)
	access := Access{Host: host, Port: port, Username: "deploy", HostKey: string(ssh.MarshalAuthorizedKey(signer.PublicKey())), Context: "test", Password: "test-password"}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	runner, err := Dial(ctx, access)
	if err != nil {
		t.Fatal(err)
	}
	defer runner.Close()
	response, err := runner.Run(ctx, "docker", "pull", "harbor.example/team/app:Li-debug_42")
	if err != nil || string(response) != "verified test response" {
		t.Fatalf("SSH execution failed: %v", err)
	}
	select {
	case command := <-commands:
		if command != ShellCommand("bash", "-lic", "exec 2>&3 3>&-; docker 'pull' 'harbor.example/team/app:Li-debug_42'")+" 3>&2 2>/dev/null" {
			t.Fatalf("unexpected remote command: %s", command)
		}
	case <-ctx.Done():
		t.Fatal("remote command was not received")
	}
	if _, err := runner.Run(ctx, "sh", "-c", "anything"); err == nil {
		t.Fatal("arbitrary command was accepted")
	}
	access.HostKey = string(ssh.MarshalAuthorizedKey(testSSHKey(t).PublicKey()))
	if unexpected, err := Dial(ctx, access); err == nil {
		unexpected.Close()
		t.Fatal("untrusted host key was accepted")
	}
	select {
	case <-commands:
		t.Fatal("rejected request reached remote execution")
	default:
	}
}
