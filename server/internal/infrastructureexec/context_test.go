package infrastructureexec

import (
	"context"
	"reflect"
	"testing"
)

func TestSelectContextDoesNotRequireCurrentContext(t *testing.T) {
	for _, test := range []struct {
		name, config, requested, selected string
		choices                           int
		fail                              bool
	}{
		{name: "single connection without default", config: `{"contexts":[{"name":"manager-admin","context":{"cluster":"logs"}}]}`, selected: "manager-admin"},
		{name: "multiple connections need a choice", config: `{"current-context":"prod","contexts":[{"name":"prod","context":{"cluster":"production"}},{"name":"qa","context":{"cluster":"test"}}]}`, choices: 2},
		{name: "explicit choice", config: `{"contexts":[{"name":"prod"},{"name":"qa"}]}`, requested: "qa", selected: "qa"},
		{name: "removed choice", config: `{"contexts":[{"name":"prod"}]}`, requested: "removed", fail: true},
		{name: "ordinary kubectl connection", config: `{"contexts":[]}`},
		{name: "malformed configuration", config: `not JSON`, fail: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			runner := fakeRunner{run: func(args []string) ([]byte, error) {
				if !reflect.DeepEqual(args, []string{"kubectl", "config", "view", "--output=json"}) {
					t.Fatalf("unexpected configuration read: %v", args)
				}
				return []byte(test.config), nil
			}}
			selected, options, err := SelectContext(context.Background(), runner, test.requested)
			if (err != nil) != test.fail || selected != test.selected || len(options) != test.choices {
				t.Fatalf("selected=%q options=%v error=%v", selected, options, err)
			}
		})
	}
}

func TestKubectlOmitsAnEmptyContextForDefaultConnections(t *testing.T) {
	for _, args := range [][]string{Kubectl("", "get", "pods"), Kube("", "logs", "get", "deployments")} {
		for _, arg := range args {
			if arg == "--context" || arg == "" {
				t.Fatalf("empty context was passed to kubectl: %v", args)
			}
		}
	}
	if !reflect.DeepEqual(Kubectl("selected", "get", "pods"), []string{"kubectl", "--context", "selected", "get", "pods"}) {
		t.Fatal("explicitly selected connection was not preserved")
	}
}
