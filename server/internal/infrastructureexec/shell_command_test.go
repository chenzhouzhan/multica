package infrastructureexec

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func TestExecutionCommandUsesUserAliasesAndPreservesArguments(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("Bash is required to exercise remote shell behavior")
	}
	dir := t.TempDir()
	write := func(name, content string) string {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0700); err != nil {
			t.Fatal(err)
		}
		return path
	}
	printer := write("print-args", "#!/bin/sh\nfor arg do\n  if [ \"$arg\" = '--fail' ]; then echo 'image-specific failure' >&2; exit 7; fi\ndone\nprintf '%s\\0' \"$@\"\n")
	write("kubectl", "#!/bin/sh\nexit 98\n")
	write("docker", "#!/bin/sh\nexit 98\n")
	rc := write("test.bashrc", "case $- in *i*) ;; *) return ;; esac\n"+
		"alias kubectl="+ShellCommand(ShellCommand(printer)+" -s https://127.0.0.1:6443 --certificate-authority=/srv/kubernetes/ca.pem --client-certificate=/srv/kubernetes/admin.pem --client-key=/srv/kubernetes/admin-key.pem")+"\n"+
		"alias docker="+ShellCommand(ShellCommand(printer)+" --docker-user-settings")+"\n")
	// Substitute only startup files, so the regression never reads the test
	// runner's account profiles or writes its shell history.
	write("bash", "#!/bin/sh\n[ \"$1\" = '-lic' ] || exit 97\necho 'shell startup warning' >&2\nexec "+ShellCommand(bash, "--noprofile", "--rcfile", rc, "-ic")+" \"$2\"\n")
	base := []string{"-s", "https://127.0.0.1:6443", "--certificate-authority=/srv/kubernetes/ca.pem", "--client-certificate=/srv/kubernetes/admin.pem", "--client-key=/srv/kubernetes/admin-key.pem"}
	marker := filepath.Join(dir, "must-not-exist")
	malicious := "tag'$(touch " + marker + ");\nnext"
	for _, test := range []struct {
		name string
		args []string
		want []string
	}{
		{"discover connections", []string{"kubectl", "config", "view", "--output=json"}, append(append([]string{}, base...), "config", "view", "--output=json")},
		{"reach API", []string{"kubectl", "--request-timeout=15s", "get", "--raw=/version"}, append(append([]string{}, base...), "--request-timeout=15s", "get", "--raw=/version")},
		{"literal arguments", []string{"kubectl", "get", "", "space and 'quotes'", malicious}, append(append([]string{}, base...), "get", "", "space and 'quotes'", malicious)},
		{"docker user settings", []string{"docker", "pull", malicious}, []string{"--docker-user-settings", "pull", malicious}},
	} {
		t.Run(test.name, func(t *testing.T) {
			command, err := executionCommand(test.args...)
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("/bin/sh", "-c", command)
			cmd.Env = []string{"PATH=" + dir + ":/usr/bin:/bin", "HOME=" + dir, "TERM=dumb", "HISTFILE=/dev/null"}
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			out, err := cmd.Output()
			if err != nil {
				t.Fatalf("run alias: %v: %s", err, &stderr)
			}
			parts := bytes.Split(bytes.TrimSuffix(out, []byte{0}), []byte{0})
			got := make([]string, len(parts))
			for i, part := range parts {
				got[i] = string(part)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("alias arguments: got %q, want %q", got, test.want)
			}
		})
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("a quoted argument was executed as shell code")
	}
	t.Run("exit status", func(t *testing.T) {
		command, err := executionCommand("kubectl", "--fail")
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("/bin/sh", "-c", command)
		cmd.Env = []string{"PATH=" + dir + ":/usr/bin:/bin", "HOME=" + dir, "TERM=dumb", "HISTFILE=/dev/null"}
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		var exit *exec.ExitError
		if err := cmd.Run(); !errors.As(err, &exit) || exit.ExitCode() != 7 {
			t.Fatalf("remote command exit status was lost: %v", err)
		}
		if stderr.String() != "image-specific failure\n" {
			t.Fatalf("command error was lost or mixed with shell startup output: %q", stderr.String())
		}
	})
}

func TestExecutionCommandRejectsUnsupportedPrograms(t *testing.T) {
	for _, args := range [][]string{nil, {"sh", "-c", "true"}, {"kubectl; echo injected"}, {"/usr/bin/kubectl"}} {
		if _, err := executionCommand(args...); err == nil {
			t.Fatalf("unsupported program accepted: %q", args)
		}
	}
}
