package infrastructureexec

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var namePattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9.-]*[a-z0-9])?$`)

type Metadata struct {
	Name              string            `json:"name" yaml:"name"`
	Namespace         string            `json:"namespace" yaml:"namespace"`
	UID               string            `json:"uid" yaml:"uid"`
	Generation        int64             `json:"generation" yaml:"generation"`
	Annotations       map[string]string `json:"annotations" yaml:"annotations"`
	DeletionTimestamp *string           `json:"deletionTimestamp"`
	OwnerReferences   []struct {
		UID        string `json:"uid"`
		Controller bool   `json:"controller"`
	} `json:"ownerReferences"`
}

type Container struct {
	Name  string `json:"name" yaml:"name"`
	Image string `json:"image" yaml:"image"`
}

type Workload struct {
	Kind     string   `json:"kind" yaml:"kind"`
	Metadata Metadata `json:"metadata" yaml:"metadata"`
	Spec     struct {
		Replicas       *int64 `json:"replicas"`
		Paused         bool   `json:"paused"`
		UpdateStrategy struct {
			Type string `json:"type"`
		} `json:"updateStrategy"`
		Selector struct {
			MatchLabels      map[string]string `json:"matchLabels"`
			MatchExpressions []struct {
				Key      string   `json:"key"`
				Operator string   `json:"operator"`
				Values   []string `json:"values"`
			} `json:"matchExpressions"`
		} `json:"selector"`
		Template struct {
			Metadata Metadata `json:"metadata"`
			Spec     struct {
				Containers []Container `json:"containers"`
			} `json:"spec"`
		} `json:"template"`
	} `json:"spec"`
	Status struct {
		ObservedGeneration     int64 `json:"observedGeneration"`
		DesiredNumberScheduled int64 `json:"desiredNumberScheduled"`
	} `json:"status"`
}

type Target struct {
	Key           string `json:"key"`
	RoleType      string `json:"role_type"`
	RoleName      string `json:"role_name"`
	Kind          string `json:"kind"`
	Namespace     string `json:"namespace"`
	Workload      string `json:"workload"`
	UID           string `json:"uid"`
	Generation    int64  `json:"generation"`
	Container     string `json:"container"`
	Image         string `json:"image"`
	Replicas      int64  `json:"replicas"`
	BlockedReason string `json:"blocked_reason"`
}

func Resource(kind string) (string, error) {
	switch strings.ToLower(kind) {
	case "deployment":
		return "deployment", nil
	case "statefulset":
		return "statefulset", nil
	case "daemonset":
		return "daemonset", nil
	}
	return "", errors.New("only Deployment, StatefulSet and DaemonSet workloads support image publication")
}

func Kube(contextName, namespace string, args ...string) []string {
	return append(Kubectl(contextName, "--namespace", namespace, "--request-timeout=30s"), args...)
}

func ReadWorkload(ctx context.Context, runner Runner, kubeContext, kind, namespace, name string) (Workload, error) {
	resource, err := Resource(kind)
	if err != nil || !namePattern.MatchString(namespace) || !namePattern.MatchString(name) {
		return Workload{}, errors.New("Manager returned an invalid workload identity")
	}
	data, err := runner.Run(ctx, Kube(kubeContext, namespace, "get", resource, name, "-o", "json")...)
	if err != nil {
		return Workload{}, err
	}
	var workload Workload
	if err := json.Unmarshal(data, &workload); err != nil {
		return Workload{}, errors.New("execution node returned an invalid Kubernetes workload")
	}
	if workload.Metadata.UID == "" || workload.Metadata.Namespace != namespace || workload.Metadata.Name != name || !strings.EqualFold(workload.Kind, kind) || workload.Metadata.Generation <= 0 {
		return Workload{}, errors.New("Kubernetes workload identity did not match the Manager object")
	}
	return workload, nil
}

func (w Workload) Desired() int64 {
	if strings.EqualFold(w.Kind, "DaemonSet") {
		return w.Status.DesiredNumberScheduled
	}
	if w.Spec.Replicas == nil {
		return 1
	}
	return *w.Spec.Replicas
}

func (w Workload) Targets() []Target {
	result := make([]Target, 0, len(w.Spec.Template.Spec.Containers))
	for _, container := range w.Spec.Template.Spec.Containers {
		target := Target{Key: strings.ToLower(w.Kind) + "/" + w.Metadata.Namespace + "/" + w.Metadata.Name + "/" + container.Name, Kind: w.Kind, Namespace: w.Metadata.Namespace, Workload: w.Metadata.Name, UID: w.Metadata.UID, Generation: w.Metadata.Generation, Container: container.Name, Image: container.Image, Replicas: w.Desired(), RoleType: container.Name, RoleName: container.Name}
		switch {
		case w.Spec.Paused:
			target.BlockedReason = "Resume the paused workload before publishing images."
		case w.Spec.UpdateStrategy.Type == "OnDelete":
			target.BlockedReason = "OnDelete workloads require manual Pod replacement and are not supported by rolling image publication."
		case w.Desired() <= 0:
			target.BlockedReason = "Scale this workload above zero before publishing and verifying its Pods."
		}
		result = append(result, target)
	}
	return result
}

func (w Workload) Matches(target Target) bool {
	if w.Metadata.UID != target.UID || w.Metadata.Generation != target.Generation || w.Spec.Paused || w.Spec.UpdateStrategy.Type == "OnDelete" || w.Desired() <= 0 {
		return false
	}
	for _, container := range w.Spec.Template.Spec.Containers {
		if container.Name == target.Container {
			return container.Image == target.Image
		}
	}
	return false
}

type PatchItem struct {
	Target Target
	Image  string
}

func ImagePatch(workload Workload, items []PatchItem, restartID string) ([]byte, error) {
	patch := []map[string]any{{"op": "test", "path": "/metadata/uid", "value": workload.Metadata.UID}, {"op": "test", "path": "/metadata/generation", "value": workload.Metadata.Generation}}
	for _, item := range items {
		if !workload.Matches(item.Target) {
			return nil, errors.New("workload or container image changed since preview; read and confirm again")
		}
		for index, container := range workload.Spec.Template.Spec.Containers {
			if container.Name == item.Target.Container {
				prefix := "/spec/template/spec/containers/" + strconv.Itoa(index)
				patch = append(patch, map[string]any{"op": "test", "path": prefix + "/name", "value": container.Name}, map[string]any{"op": "test", "path": prefix + "/image", "value": container.Image})
				if item.Image != "" {
					patch = append(patch, map[string]any{"op": "replace", "path": prefix + "/image", "value": item.Image})
				}
			}
		}
	}
	if restartID != "" {
		annotations := map[string]string{}
		for key, value := range workload.Spec.Template.Metadata.Annotations {
			annotations[key] = value
		}
		annotations["multica.ai/restartedAt"] = restartID
		patch = append(patch, map[string]any{"op": "add", "path": "/spec/template/metadata/annotations", "value": annotations})
	}
	return json.Marshal(patch)
}

func (w Workload) selector() (string, error) {
	parts := []string{}
	for key, value := range w.Spec.Selector.MatchLabels {
		parts = append(parts, key+"="+value)
	}
	for _, expression := range w.Spec.Selector.MatchExpressions {
		switch expression.Operator {
		case "In", "NotIn":
			op := " in "
			if expression.Operator == "NotIn" {
				op = " notin "
			}
			parts = append(parts, expression.Key+op+"("+strings.Join(expression.Values, ",")+")")
		case "Exists":
			parts = append(parts, expression.Key)
		case "DoesNotExist":
			parts = append(parts, "!"+expression.Key)
		default:
			return "", errors.New("unsupported workload selector")
		}
	}
	if len(parts) == 0 {
		return "", errors.New("workload has no pod selector")
	}
	sort.Strings(parts)
	return strings.Join(parts, ","), nil
}

type ExpectedImage struct {
	Container string
	Image     string
	Digests   []string
}

func controlledBy(metadata Metadata, owners map[string]bool) bool {
	for _, owner := range metadata.OwnerReferences {
		if owner.Controller && owners[owner.UID] {
			return true
		}
	}
	return false
}

func Verify(ctx context.Context, runner Runner, kubeContext string, workload Workload, expected []ExpectedImage) error {
	if workload.Desired() <= 0 || workload.Status.ObservedGeneration < workload.Metadata.Generation {
		return errors.New("workload has no running replicas or its generation is not observed yet")
	}
	selector, err := workload.selector()
	if err != nil {
		return err
	}
	owners := map[string]bool{workload.Metadata.UID: true}
	if strings.EqualFold(workload.Kind, "Deployment") {
		data, err := runner.Run(ctx, Kube(kubeContext, workload.Metadata.Namespace, "get", "replicasets", "--selector", selector, "-o", "json")...)
		if err != nil {
			return err
		}
		var replicas struct{ Items []struct{ Metadata Metadata } }
		if err := json.Unmarshal(data, &replicas); err != nil {
			return errors.New("invalid ReplicaSet response")
		}
		for _, replica := range replicas.Items {
			if controlledBy(replica.Metadata, owners) {
				owners[replica.Metadata.UID] = true
			}
		}
		delete(owners, workload.Metadata.UID)
	}
	data, err := runner.Run(ctx, Kube(kubeContext, workload.Metadata.Namespace, "get", "pods", "--selector", selector, "-o", "json")...)
	if err != nil {
		return err
	}
	var pods struct {
		Items []struct {
			Metadata Metadata
			Spec     struct{ Containers []Container }
			Status   struct {
				Phase             string
				Conditions        []struct{ Type, Status string }
				ContainerStatuses []struct {
					Name    string
					ImageID string
					Ready   bool
				}
			}
		}
	}
	if err := json.Unmarshal(data, &pods); err != nil {
		return errors.New("invalid Pod response")
	}
	var count int64
	for _, pod := range pods.Items {
		if pod.Metadata.DeletionTimestamp != nil || !controlledBy(pod.Metadata, owners) {
			continue
		}
		ready := false
		for _, condition := range pod.Status.Conditions {
			if condition.Type == "Ready" && condition.Status == "True" {
				ready = true
			}
		}
		if !ready || pod.Status.Phase != "Running" {
			return fmt.Errorf("Pod %s is not running and ready", pod.Metadata.Name)
		}
		for _, image := range expected {
			matchedSpec, matchedStatus := false, false
			for _, container := range pod.Spec.Containers {
				if container.Name == image.Container && container.Image == image.Image {
					matchedSpec = true
				}
			}
			for _, status := range pod.Status.ContainerStatuses {
				if status.Name != image.Container || !status.Ready {
					continue
				}
				actual := digestPattern.FindString(status.ImageID)
				for _, accepted := range image.Digests {
					if actual == accepted && actual != "" {
						matchedStatus = true
					}
				}
			}
			if !matchedSpec || !matchedStatus {
				return fmt.Errorf("Pod %s container %s has not verified the pinned image digest", pod.Metadata.Name, image.Container)
			}
		}
		count++
	}
	if count != workload.Desired() {
		return fmt.Errorf("verified %d of %d desired Pods", count, workload.Desired())
	}
	return nil
}
