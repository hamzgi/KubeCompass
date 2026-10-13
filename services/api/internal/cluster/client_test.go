package cluster

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestTopologyConnectsServiceToMatchingPodAndServiceAccount(t *testing.T) {
	client := NewClientWithClientset(fake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "demo"}},
		&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "app-reader"}},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "web-1", Labels: map[string]string{"app": "web"}},
			Spec:       corev1.PodSpec{ServiceAccountName: "app-reader"},
			Status:     corev1.PodStatus{Phase: corev1.PodRunning},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "web"},
			Spec:       corev1.ServiceSpec{Selector: map[string]string{"app": "web"}},
		},
	))

	topology, err := client.Topology(context.Background())
	if err != nil {
		t.Fatalf("Topology returned an error: %v", err)
	}

	if !hasTopologyNode(topology.Nodes, "pod:demo/web-1") {
		t.Fatal("expected the Pod node to be present")
	}
	if !hasTopologyEdge(topology.Edges, "routes:service:demo/web:pod:demo/web-1") {
		t.Fatal("expected the Service to route traffic to the matching Pod")
	}
	if !hasTopologyEdge(topology.Edges, "uses:pod:demo/web-1:serviceaccount:demo/app-reader") {
		t.Fatal("expected the Pod to be connected to its ServiceAccount")
	}
}

func TestSecurityIssuesFindsTheInitialThreeRules(t *testing.T) {
	privileged := true
	client := NewClientWithClientset(fake.NewSimpleClientset(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "security-lab"},
			Spec: corev1.PodSpec{Containers: []corev1.Container{{
				Name:            "lab",
				SecurityContext: &corev1.SecurityContext{Privileged: &privileged},
			}}},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "healthy"},
			Spec: corev1.PodSpec{
				ServiceAccountName: "app-reader",
				Containers: []corev1.Container{{
					Name: "app",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m"), corev1.ResourceMemory: resource.MustParse("128Mi")},
						Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("500m"), corev1.ResourceMemory: resource.MustParse("256Mi")},
					},
				}},
			},
		},
	))

	report, err := client.SecurityIssues(context.Background())
	if err != nil {
		t.Fatalf("SecurityIssues returned an error: %v", err)
	}

	if len(report.Issues) != 3 {
		t.Fatalf("expected 3 issues for the insecure Pod, got %d", len(report.Issues))
	}

	expectedRules := map[string]bool{
		"privileged-container":      false,
		"missing-resource-settings": false,
		"default-service-account":   false,
	}
	for _, issue := range report.Issues {
		expectedRules[issue.RuleID] = true
	}
	for ruleID, found := range expectedRules {
		if !found {
			t.Fatalf("expected %s to be reported", ruleID)
		}
	}
}

func hasTopologyNode(nodes []TopologyNode, id string) bool {
	for _, node := range nodes {
		if node.ID == id {
			return true
		}
	}
	return false
}

func hasTopologyEdge(edges []TopologyEdge, id string) bool {
	for _, edge := range edges {
		if edge.ID == id {
			return true
		}
	}
	return false
}
