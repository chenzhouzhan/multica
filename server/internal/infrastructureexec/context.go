package infrastructureexec

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

type KubernetesContext struct {
	Name    string `json:"name"`
	Cluster string `json:"cluster"`
}

func Kubectl(contextName string, args ...string) []string {
	command := []string{"kubectl"}
	if contextName != "" {
		command = append(command, "--context", contextName)
	}
	return append(command, args...)
}

// A single connection does not need a configured current-context. Multiple
// connections require an explicit choice rather than guessing a Manager mapping.
func SelectContext(ctx context.Context, runner Runner, requested string) (string, []KubernetesContext, error) {
	data, err := runner.Run(ctx, "kubectl", "config", "view", "--output=json")
	if err != nil {
		return "", nil, fmt.Errorf("read Kubernetes connections for the SSH user: %w", err)
	}
	var config struct {
		Contexts []struct {
			Name    string `json:"name"`
			Context struct {
				Cluster string `json:"cluster"`
			} `json:"context"`
		} `json:"contexts"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return "", nil, errors.New("kubectl returned invalid connection configuration")
	}
	options := []KubernetesContext{}
	for _, item := range config.Contexts {
		if item.Name != "" {
			options = append(options, KubernetesContext{Name: item.Name, Cluster: item.Context.Cluster})
		}
	}
	if requested != "" {
		for _, option := range options {
			if option.Name == requested {
				return requested, nil, nil
			}
		}
		return "", nil, errors.New("the selected Kubernetes connection no longer exists; detect connections again")
	}
	if len(options) == 1 {
		return options[0].Name, nil, nil
	}
	if len(options) > 1 {
		return "", options, nil
	}
	// A kubectl wrapper may supply its connection without named contexts.
	// The caller must verify that the ordinary kubectl command can reach the API.
	return "", nil, nil
}
