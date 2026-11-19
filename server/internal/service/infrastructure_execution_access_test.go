package service

import (
	"crypto/ed25519"
	"encoding/json"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/infrastructureexec"
	"golang.org/x/crypto/ssh"
)

func TestExecutionAccessInfoDefaultsToPasswordAndExposesOnlyPublicIdentity(t *testing.T) {
	if accessInfo(infrastructureexec.Access{}, false).AuthType != "password" {
		t.Fatal("new execution access should default to password authentication")
	}
	key, err := ssh.NewPublicKey(ed25519.PublicKey(make([]byte, ed25519.PublicKeySize)))
	if err != nil {
		t.Fatal(err)
	}
	info := accessInfo(infrastructureexec.Access{Host: "manager.test", Password: "test-secret-password", HostKey: string(ssh.MarshalAuthorizedKey(key))}, true)
	encoded, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	if info.HostKeyFingerprint != ssh.FingerprintSHA256(key) || info.AuthType != "password" {
		t.Fatal("public server identity was not projected")
	}
	if strings.Contains(string(encoded), "test-secret-password") || strings.Contains(string(encoded), "private_key") {
		t.Fatal("execution credentials were returned with public identity")
	}
}
