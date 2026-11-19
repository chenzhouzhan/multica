package transwarp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"gopkg.in/yaml.v3"
)

// WorkloadReference contains identity only; service manifests may contain secrets.
type WorkloadReference struct {
	Kind      string
	Namespace string
	Name      string
	UID       string
}

type managerText string

func (c *Client) Service(ctx context.Context, id int64) (Service, error) {
	var raw rawService
	if id <= 0 {
		return Service{}, errors.New("service_id is required")
	}
	if err := c.get(ctx, fmt.Sprintf("/api/services/%d", id), nil, &raw); err != nil {
		return Service{}, err
	}
	if raw.ID != id {
		return Service{}, fmt.Errorf("%w: Manager returned a different service", ErrResponse)
	}
	return raw.project(), nil
}

func (c *Client) ServiceWorkloads(ctx context.Context, serviceID int64) ([]WorkloadReference, error) {
	base := fmt.Sprintf("/api/services/%d/tosObjects", serviceID)
	var kinds []string
	if err := c.get(ctx, base+"/kinds", nil, &kinds); err != nil {
		return nil, err
	}
	result := []WorkloadReference{}
	seen := map[string]bool{}
	for _, kind := range kinds {
		if kind != "DEPLOYMENT" && kind != "STATEFULSET" && kind != "DAEMONSET" {
			continue
		}
		var objects []struct {
			Name string `json:"name"`
		}
		if err := c.get(ctx, base, url.Values{"kind": {kind}}, &objects); err != nil {
			return nil, err
		}
		if len(objects)+len(result) > 128 {
			return nil, errors.New("service has too many Kubernetes workloads")
		}
		for _, object := range objects {
			if object.Name == "" {
				return nil, fmt.Errorf("%w: Manager returned an unnamed workload", ErrResponse)
			}
			var text managerText
			if err := c.get(ctx, base+"/config", url.Values{"kind": {kind}, "name": {object.Name}}, &text); err != nil {
				return nil, err
			}
			var manifest struct {
				Kind     string `yaml:"kind"`
				Metadata struct {
					Name      string `yaml:"name"`
					Namespace string `yaml:"namespace"`
					UID       string `yaml:"uid"`
				} `yaml:"metadata"`
			}
			if err := yaml.Unmarshal([]byte(text), &manifest); err != nil || !strings.EqualFold(manifest.Kind, kind) || manifest.Metadata.Name != object.Name || manifest.Metadata.Namespace == "" {
				return nil, fmt.Errorf("%w: Manager returned a workload manifest without matching kind, name and namespace", ErrResponse)
			}
			key := kind + "/" + manifest.Metadata.Namespace + "/" + object.Name
			if seen[key] {
				continue
			}
			seen[key] = true
			result = append(result, WorkloadReference{Kind: manifest.Kind, Namespace: manifest.Metadata.Namespace, Name: object.Name, UID: manifest.Metadata.UID})
		}
	}
	return result, nil
}

func decodeManagerText(data []byte) string {
	var value string
	if json.Unmarshal(data, &value) == nil {
		return value
	}
	return string(data)
}
