package infrastructureexec

import (
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func TestHostKeyDiscoveryDoesNotAuthenticateAndSavedIdentityRemainsPinned(t *testing.T) {
	signer := testSSHKey(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	var authAttempts atomic.Int32
	config := &ssh.ServerConfig{
		AuthLogCallback: func(_ ssh.ConnMetadata, _ string, _ error) { authAttempts.Add(1) },
		PasswordCallback: func(metadata ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			if metadata.User() == "deploy" && string(password) == "test-password" {
				return nil, nil
			}
			return nil, errors.New("incorrect test password")
		},
	}
	config.AddHostKey(signer)
	go func() {
		for {
			socket, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer socket.Close()
				connection, channels, requests, err := ssh.NewServerConn(socket, config)
				if err != nil {
					return
				}
				defer connection.Close()
				go ssh.DiscardRequests(requests)
				for channel := range channels {
					channel.Reject(ssh.Prohibited, "no commands in identity test")
				}
			}()
		}
	}()
	host, portText, _ := net.SplitHostPort(listener.Addr().String())
	port, _ := strconv.Atoi(portText)
	access := Access{Host: host, Port: port, Username: "deploy", Password: "test-password"}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	prepared, review, err := PrepareHostKey(ctx, access, "", false)
	if err != nil || review == nil {
		t.Fatalf("read host identity: %v", err)
	}
	if prepared.HostKey != "" || authAttempts.Load() != 0 {
		t.Fatal("discovery trusted the server or attempted authentication before confirmation")
	}
	if review.Fingerprint != ssh.FingerprintSHA256(signer.PublicKey()) || !strings.HasPrefix(review.Key, "ssh-ed25519 ") {
		t.Fatal("discovery did not return the observed public identity")
	}
	prepared, again, err := PrepareHostKey(ctx, access, review.Key, false)
	if err != nil || again != nil || prepared.HostKey != review.Key {
		t.Fatalf("saved identity was not reused: %v", err)
	}
	runner, err := Dial(ctx, prepared)
	if err != nil {
		t.Fatal(err)
	}
	runner.Close()
	beforeMismatch := authAttempts.Load()
	if beforeMismatch == 0 {
		t.Fatal("confirmed identity did not allow authentication")
	}
	wrongKey := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(testSSHKey(t).PublicKey())))
	prepared, again, err = PrepareHostKey(ctx, access, wrongKey, false)
	if err != nil || again != nil {
		t.Fatalf("saved pin should be checked by SSH: %v", err)
	}
	if runner, err := Dial(ctx, prepared); !errors.Is(err, ErrHostKeyChanged) {
		if runner != nil {
			runner.Close()
		}
		t.Fatalf("changed server identity was not rejected: %v", err)
	}
	if authAttempts.Load() != beforeMismatch {
		t.Fatal("changed server identity received authentication")
	}
	_, rotated, err := PrepareHostKey(ctx, access, wrongKey, true)
	if err != nil || rotated == nil || rotated.Fingerprint != review.Fingerprint || rotated.PreviousFingerprint != HostKeyFingerprint(wrongKey) {
		t.Fatalf("explicit recheck lost old and new identities: %v", err)
	}
	if authAttempts.Load() != beforeMismatch {
		t.Fatal("explicit identity recheck attempted authentication")
	}
}

func TestHostKeyDiscoveryHonorsCancellation(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		socket, err := listener.Accept()
		if err == nil {
			defer socket.Close()
			buffer := make([]byte, 1024)
			for {
				if _, err := socket.Read(buffer); err != nil {
					return
				}
			}
		}
	}()
	host, portText, _ := net.SplitHostPort(listener.Addr().String())
	port, _ := strconv.Atoi(portText)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	if _, err := ReadHostKey(ctx, host, port); err == nil {
		t.Fatal("incomplete SSH handshake was accepted")
	}
	if time.Since(started) > time.Second {
		t.Fatal("identity discovery ignored its cancellation deadline")
	}
}
