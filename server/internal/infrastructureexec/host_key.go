package infrastructureexec

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

var ErrHostKeyChanged = errors.New("SSH server fingerprint changed; re-read and verify the server fingerprint before reconnecting")

type HostKeyReview struct {
	Key                 string
	Fingerprint         string
	PreviousFingerprint string
}

func HostKeyFingerprint(value string) string {
	key, _, _, _, err := ssh.ParseAuthorizedKey([]byte(value))
	if err != nil {
		return ""
	}
	return ssh.FingerprintSHA256(key)
}

// The host-key callback aborts key exchange before user authentication. A
// discovery request never sends the configured password or private key.
func ReadHostKey(ctx context.Context, host string, port int) (HostKeyReview, error) {
	address := net.JoinHostPort(host, strconv.Itoa(port))
	conn, err := (&net.Dialer{Timeout: 15 * time.Second}).DialContext(ctx, "tcp", address)
	if err != nil {
		return HostKeyReview{}, fmt.Errorf("read SSH server fingerprint: %w", err)
	}
	defer conn.Close()
	deadline := time.Now().Add(15 * time.Second)
	if value, ok := ctx.Deadline(); ok && value.Before(deadline) {
		deadline = value
	}
	_ = conn.SetDeadline(deadline)
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	var observed ssh.PublicKey
	_, _, _, _ = ssh.NewClientConn(conn, address, &ssh.ClientConfig{User: "multica-host-key-probe", HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error {
		observed = key
		return errors.New("host key observed; authentication was not requested")
	}})
	if observed == nil {
		return HostKeyReview{}, errors.New("could not read the SSH server fingerprint")
	}
	return HostKeyReview{Key: strings.TrimSpace(string(ssh.MarshalAuthorizedKey(observed))), Fingerprint: ssh.FingerprintSHA256(observed)}, nil
}

func PrepareHostKey(ctx context.Context, access Access, trustedKey string, forceRead bool) (Access, *HostKeyReview, error) {
	if forceRead {
		access.HostKey = ""
	} else if access.HostKey == "" {
		access.HostKey = trustedKey
	}
	if access.HostKey != "" {
		if HostKeyFingerprint(access.HostKey) == "" {
			return access, nil, errors.New("SSH host_key must be a trusted OpenSSH public key")
		}
		return access, nil, nil
	}
	review, err := ReadHostKey(ctx, access.Host, access.Port)
	if err != nil {
		return access, nil, err
	}
	review.PreviousFingerprint = HostKeyFingerprint(trustedKey)
	return access, &review, nil
}
