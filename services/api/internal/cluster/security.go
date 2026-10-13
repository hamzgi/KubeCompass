package cluster

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SecurityReport contains explainable, read-only checks. The rules intentionally
// describe a setting and its impact instead of changing any cluster resource.
type SecurityReport struct {
	Issues     []SecurityIssue `json:"issues"`
	ObservedAt time.Time       `json:"observedAt"`
}

type SecurityIssue struct {
	ID             string            `json:"id"`
	RuleID         string            `json:"ruleId"`
	Severity       string            `json:"severity"`
	Title          string            `json:"title"`
	Resource       ResourceReference `json:"resource"`
	Summary        string            `json:"summary"`
	Cause          string            `json:"cause"`
	Impact         string            `json:"impact"`
	Recommendation string            `json:"recommendation"`
	ExampleYAML    string            `json:"exampleYaml"`
}

type ResourceReference struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Container string `json:"container,omitempty"`
}

// SecurityIssues checks application pods outside system namespaces. This keeps
// the first learning experience focused on workloads the cluster user controls.
func (c *Client) SecurityIssues(ctx context.Context) (SecurityReport, error) {
	podList, err := c.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return SecurityReport{}, fmt.Errorf("list pods for security checks: %w", err)
	}

	issues := make([]SecurityIssue, 0)
	for _, pod := range podList.Items {
		if isSystemNamespace(pod.Namespace) {
			continue
		}

		issues = append(issues, defaultServiceAccountIssue(pod)...)
		for _, container := range pod.Spec.Containers {
			if container.SecurityContext != nil && container.SecurityContext.Privileged != nil && *container.SecurityContext.Privileged {
				issues = append(issues, privilegedContainerIssue(pod, container.Name))
			}

			if missing := missingResourceSettings(container.Resources); len(missing) > 0 {
				issues = append(issues, missingResourcesIssue(pod, container.Name, missing))
			}
		}
	}

	sort.Slice(issues, func(i, j int) bool {
		leftRank := severityRank(issues[i].Severity)
		rightRank := severityRank(issues[j].Severity)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		if issues[i].Resource.Namespace != issues[j].Resource.Namespace {
			return issues[i].Resource.Namespace < issues[j].Resource.Namespace
		}
		return issues[i].ID < issues[j].ID
	})

	return SecurityReport{Issues: issues, ObservedAt: time.Now().UTC()}, nil
}

func defaultServiceAccountIssue(pod corev1.Pod) []SecurityIssue {
	serviceAccountName := pod.Spec.ServiceAccountName
	if serviceAccountName != "" && serviceAccountName != "default" {
		return nil
	}

	return []SecurityIssue{{
		ID:             fmt.Sprintf("default-service-account:%s/%s", pod.Namespace, pod.Name),
		RuleID:         "default-service-account",
		Severity:       "low",
		Title:          "기본 서비스 계정을 사용합니다",
		Resource:       ResourceReference{Kind: "Pod", Namespace: pod.Namespace, Name: pod.Name},
		Summary:        "이 Pod는 별도 계정 대신 default ServiceAccount를 사용하고 있습니다.",
		Cause:          "Pod의 serviceAccountName이 비어 있거나 default로 설정되어 있습니다.",
		Impact:         "나중에 기본 계정에 권한이 추가되면 의도하지 않은 앱도 같은 권한을 가질 수 있습니다.",
		Recommendation: "앱 전용 ServiceAccount를 만들고 필요한 권한만 Role과 RoleBinding으로 부여합니다.",
		ExampleYAML:    "apiVersion: v1\nkind: ServiceAccount\nmetadata:\n  name: app-reader\n---\nspec:\n  serviceAccountName: app-reader",
	}}
}

func privilegedContainerIssue(pod corev1.Pod, containerName string) SecurityIssue {
	return SecurityIssue{
		ID:             fmt.Sprintf("privileged-container:%s/%s/%s", pod.Namespace, pod.Name, containerName),
		RuleID:         "privileged-container",
		Severity:       "high",
		Title:          "권한이 너무 높은 컨테이너입니다",
		Resource:       ResourceReference{Kind: "Pod", Namespace: pod.Namespace, Name: pod.Name, Container: containerName},
		Summary:        "컨테이너가 privileged 모드로 실행되고 있습니다.",
		Cause:          "securityContext.privileged가 true로 설정되어 있습니다.",
		Impact:         "컨테이너가 노드의 장치와 커널 기능에 더 넓게 접근할 수 있어, 침해 시 영향 범위가 커질 수 있습니다.",
		Recommendation: "특별한 이유가 없다면 privileged를 제거하고 필요한 Linux capability만 개별적으로 허용합니다.",
		ExampleYAML:    "securityContext:\n  privileged: false\n  allowPrivilegeEscalation: false",
	}
}

func missingResourcesIssue(pod corev1.Pod, containerName string, missing []string) SecurityIssue {
	return SecurityIssue{
		ID:             fmt.Sprintf("missing-resource-settings:%s/%s/%s", pod.Namespace, pod.Name, containerName),
		RuleID:         "missing-resource-settings",
		Severity:       "medium",
		Title:          "CPU·메모리 기준이 비어 있습니다",
		Resource:       ResourceReference{Kind: "Pod", Namespace: pod.Namespace, Name: pod.Name, Container: containerName},
		Summary:        fmt.Sprintf("%s 설정이 없습니다.", strings.Join(missing, ", ")),
		Cause:          "컨테이너 resources에 CPU·메모리 request 또는 limit가 모두 설정되지 않았습니다.",
		Impact:         "한 앱이 많은 자원을 사용하면 같은 노드의 다른 앱이 느려지거나, 메모리 부족으로 종료될 수 있습니다.",
		Recommendation: "실제 사용량을 관찰한 뒤 CPU와 메모리의 request·limit를 함께 설정합니다.",
		ExampleYAML:    "resources:\n  requests:\n    cpu: 100m\n    memory: 128Mi\n  limits:\n    cpu: 500m\n    memory: 256Mi",
	}
}

func missingResourceSettings(resources corev1.ResourceRequirements) []string {
	checks := []struct {
		name   string
		values corev1.ResourceList
		key    corev1.ResourceName
	}{
		{name: "CPU request", values: resources.Requests, key: corev1.ResourceCPU},
		{name: "메모리 request", values: resources.Requests, key: corev1.ResourceMemory},
		{name: "CPU limit", values: resources.Limits, key: corev1.ResourceCPU},
		{name: "메모리 limit", values: resources.Limits, key: corev1.ResourceMemory},
	}

	missing := make([]string, 0, len(checks))
	for _, check := range checks {
		if _, exists := check.values[check.key]; !exists {
			missing = append(missing, check.name)
		}
	}
	return missing
}

func severityRank(severity string) int {
	switch severity {
	case "high":
		return 0
	case "medium":
		return 1
	default:
		return 2
	}
}
