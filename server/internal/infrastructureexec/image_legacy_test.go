package infrastructureexec

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestResolveImageSupportsDockerWithoutManifestCommand(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	imageID := "sha256:" + strings.Repeat("b", 64)
	pinned := "harbor.example/team/app@" + digest
	for _, source := range []string{"harbor.example/team/app:company-custom_42", pinned} {
		t.Run(source, func(t *testing.T) {
			var calls [][]string
			runner := fakeRunner{run: func(args []string) ([]byte, error) {
				calls = append(calls, append([]string{}, args...))
				switch {
				case args[1] == "pull":
					return []byte("Digest: " + digest + "\n"), nil
				case args[1] == "manifest":
					return nil, errors.New("Process exited with status 125: unknown flag: --verbose")
				case args[1] == "image" && args[2] == "inspect":
					if args[len(args)-1] != pinned {
						t.Fatalf("inspected a mutable tag: %q", args)
					}
					return []byte(`{"Id":"` + imageID + `","RepoDigests":["` + pinned + `"]}`), nil
				default:
					t.Fatalf("unexpected command: %q", args)
					return nil, nil
				}
			}}
			image, err := ResolveImage(context.Background(), runner, source)
			if err != nil {
				t.Fatal(err)
			}
			if image.Source != source || image.Pinned != pinned || image.Digest != digest || !reflect.DeepEqual(image.Digests, []string{digest, imageID}) {
				t.Fatalf("incorrect pinned identity: %+v", image)
			}
			pulledPinned := false
			for _, args := range calls {
				if args[1] == "pull" && args[2] == pinned {
					pulledPinned = true
				}
			}
			if !pulledPinned {
				t.Fatal("local inspection did not pull the immutable reference")
			}
		})
	}
}

func TestResolveImageRejectsInvalidLocalIdentityAndPreservesRegistryErrors(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	imageID := "sha256:" + strings.Repeat("b", 64)
	pinned := "harbor.example/team/app@" + digest
	for _, test := range []struct {
		name, manifestError, inspect string
		pullError                    bool
	}{
		{name: "registry denied", manifestError: "denied: requested access to the resource is denied"},
		{name: "invalid manifest", inspect: "invalid"},
		{name: "missing image ID", inspect: `{"RepoDigests":["` + pinned + `"]}`},
		{name: "invalid image ID", inspect: `{"Id":"sha256:broken","RepoDigests":["` + pinned + `"]}`},
		{name: "different repository", inspect: `{"Id":"` + imageID + `","RepoDigests":["harbor.example/other/app@` + digest + `"]}`},
		{name: "different digest", inspect: `{"Id":"` + imageID + `","RepoDigests":["harbor.example/team/app@` + imageID + `"]}`},
		{name: "pull failed", pullError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			runner := fakeRunner{run: func(args []string) ([]byte, error) {
				switch args[1] {
				case "manifest":
					if test.manifestError != "" {
						return nil, errors.New(test.manifestError)
					}
					return nil, errors.New("unknown flag: --verbose")
				case "pull":
					if test.manifestError != "" {
						t.Fatal("a registry error was treated as an unsupported CLI")
					}
					if test.pullError {
						return nil, errors.New("pull denied")
					}
					return []byte("Digest: " + digest), nil
				case "image":
					return []byte(test.inspect), nil
				default:
					t.Fatalf("unexpected command: %q", args)
					return nil, nil
				}
			}}
			if _, err := ResolveImage(context.Background(), runner, pinned); err == nil {
				t.Fatal("an unverified image identity was accepted")
			} else if test.manifestError != "" && !strings.Contains(err.Error(), test.manifestError) {
				t.Fatalf("registry error was hidden: %v", err)
			}
		})
	}
}
