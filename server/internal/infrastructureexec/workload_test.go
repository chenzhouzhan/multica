package infrastructureexec

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

type fakeRunner struct {
	run func([]string) ([]byte, error)
}

func (f fakeRunner) Run(_ context.Context, args ...string) ([]byte, error) { return f.run(args) }
func (f fakeRunner) Close() error                                          { return nil }

func fixtureWorkload(t *testing.T) Workload {
	t.Helper()
	var workload Workload
	err := json.Unmarshal([]byte(`{"kind":"Deployment","metadata":{"name":"worker-a","namespace":"qa","uid":"workload-a","generation":7},"spec":{"replicas":1,"selector":{"matchLabels":{"app":"shared"}},"template":{"metadata":{"annotations":{"keep":"yes"}},"spec":{"containers":[{"name":"worker","image":"registry.example/worker:nonstandard-tag"},{"name":"sidecar","image":"registry.example/sidecar:stable"}]}}},"status":{"observedGeneration":7}}`), &workload)
	if err != nil {
		t.Fatal(err)
	}
	return workload
}

func TestImagePatchProtectsIdentityAndTouchesOnlySelectedContainers(t *testing.T) {
	w := fixtureWorkload(t)
	target := w.Targets()[0]
	patch, err := ImagePatch(w, []PatchItem{{Target: target, Image: "registry.example/worker@sha256:new"}}, "")
	if err != nil {
		t.Fatal(err)
	}
	var operations []map[string]any
	if err := json.Unmarshal(patch, &operations); err != nil {
		t.Fatal(err)
	}
	if operations[0]["path"] != "/metadata/uid" || operations[1]["path"] != "/metadata/generation" {
		t.Fatalf("missing atomic identity checks: %s", patch)
	}
	for _, operation := range operations {
		if strings.Contains(operation["path"].(string), "/containers/1") {
			t.Fatalf("unselected sidecar was patched: %s", patch)
		}
	}
	changed := w
	changed.Metadata.Generation++
	if _, err := ImagePatch(changed, []PatchItem{{Target: target}}, ""); err == nil {
		t.Fatal("another writer's generation was accepted")
	}
	changed = w
	changed.Metadata.UID = "replacement"
	if _, err := ImagePatch(changed, []PatchItem{{Target: target}}, ""); err == nil {
		t.Fatal("recreated workload was accepted")
	}
	patch, err = ImagePatch(w, []PatchItem{{Target: target, Image: "registry.example/worker@sha256:new"}}, "operation-1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(patch), `"keep":"yes"`) || !strings.Contains(string(patch), `"multica.ai/restartedAt":"operation-1"`) {
		t.Fatalf("restart lost template annotations: %s", patch)
	}
}

func TestReadWorkloadDoesNotAcceptAnotherNamespaceOrUIDlessResource(t *testing.T) {
	for _, response := range []string{
		`{"kind":"Deployment","metadata":{"name":"worker-a","namespace":"other","uid":"a","generation":1}}`,
		`{"kind":"Deployment","metadata":{"name":"worker-a","namespace":"qa","generation":1}}`,
	} {
		_, err := ReadWorkload(context.Background(), fakeRunner{run: func([]string) ([]byte, error) { return []byte(response), nil }}, "context-a", "Deployment", "qa", "worker-a")
		if err == nil {
			t.Fatal("foreign workload identity accepted")
		}
	}
}

func TestVerificationUsesControllerOwnershipReadinessAndRuntimeDigest(t *testing.T) {
	w := fixtureWorkload(t)
	digest := "sha256:" + strings.Repeat("a", 64)
	other := "sha256:" + strings.Repeat("b", 64)
	pinned := "registry.example/worker@" + digest
	for _, tc := range []struct {
		name, actual, owner string
		ready               bool
		want                bool
	}{
		{"owned ready content", digest, "rs-a", true, true},
		{"ready old image", other, "rs-a", true, false},
		{"not ready", digest, "rs-a", false, false},
		{"another service shares selector", digest, "rs-b", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runner := fakeRunner{run: func(args []string) ([]byte, error) {
				if contains(args, "replicasets") {
					return []byte(`{"items":[{"metadata":{"uid":"rs-a","ownerReferences":[{"uid":"workload-a","controller":true}]}},{"metadata":{"uid":"rs-b","ownerReferences":[{"uid":"workload-b","controller":true}]}}]}`), nil
				}
				if !contains(args, "pods") {
					return nil, errors.New("unexpected command")
				}
				status := "False"
				if tc.ready {
					status = "True"
				}
				return json.Marshal(map[string]any{"items": []any{map[string]any{"metadata": map[string]any{"name": "worker-pod", "ownerReferences": []any{map[string]any{"uid": tc.owner, "controller": true}}}, "spec": map[string]any{"containers": []any{map[string]any{"name": "worker", "image": pinned}}}, "status": map[string]any{"phase": "Running", "conditions": []any{map[string]any{"type": "Ready", "status": status}}, "containerStatuses": []any{map[string]any{"name": "worker", "imageID": "docker-pullable://registry.example/worker@" + tc.actual, "ready": tc.ready}}}}}})
			}}
			err := Verify(context.Background(), runner, "ctx", w, []ExpectedImage{{Container: "worker", Image: pinned, Digests: []string{digest}}})
			if (err == nil) != tc.want {
				t.Fatalf("verified=%v want=%v err=%v", err == nil, tc.want, err)
			}
		})
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func TestResolveImageKeepsArbitraryTagsAndPinsThePulledDigest(t *testing.T) {
	rootDigest := "sha256:" + strings.Repeat("a", 64)
	child := "sha256:" + strings.Repeat("b", 64)
	config := "sha256:" + strings.Repeat("c", 64)
	source := "harbor.company.example/team/worker:Li-debug_42"
	var calls [][]string
	runner := fakeRunner{run: func(args []string) ([]byte, error) {
		calls = append(calls, append([]string{}, args...))
		if args[1] == "pull" {
			return []byte("Digest: " + rootDigest + "\nStatus: Downloaded newer image\n"), nil
		}
		return []byte(`[{"Descriptor":{"digest":"` + child + `"},"SchemaV2Manifest":{"config":{"digest":"` + config + `"}}}]`), nil
	}}
	image, err := ResolveImage(context.Background(), runner, source)
	if err != nil {
		t.Fatal(err)
	}
	if image.Pinned != "harbor.company.example/team/worker@"+rootDigest || image.Source != source {
		t.Fatalf("wrong pinned reference: %+v", image)
	}
	if !reflect.DeepEqual(calls[0], []string{"docker", "pull", source}) {
		t.Fatalf("source tag changed: %v", calls)
	}
	if len(image.Digests) != 3 {
		t.Fatalf("multi-platform digests omitted: %+v", image)
	}
	for _, value := range []string{"worker:latest", "registry.example/team/app;touch /tmp/file", "https://registry.example/app:tag", "registry.example/app@sha256:invalid"} {
		if _, err := ParseImage(value); err == nil {
			t.Fatalf("invalid reference accepted: %q", value)
		}
	}
}

func TestShellCommandQuotesEveryArgument(t *testing.T) {
	command := ShellCommand("docker", "pull", "registry.example/team/app:tag'$(touch /tmp/should-not-exist)")
	if command != "'docker' 'pull' 'registry.example/team/app:tag'\\''$(touch /tmp/should-not-exist)'" {
		t.Fatalf("argument escaped incorrectly: %s", command)
	}
}

func TestManifestVerificationIncludesOCIImageConfigDigest(t *testing.T) {
	root := "sha256:" + strings.Repeat("a", 64)
	config := "sha256:" + strings.Repeat("c", 64)
	values, err := manifestDigests([]byte(`{"Descriptor":{"digest":"`+root+`"},"OCIManifest":{"config":{"digest":"`+config+`"}}}`), root)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(values, config) {
		t.Fatal("OCI runtime configuration digest was omitted")
	}
}
