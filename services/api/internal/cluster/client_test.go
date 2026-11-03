package cluster

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
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

func TestSecurityIssuesIncludesStandardsBackedPodRules(t *testing.T) {
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

	expectedRules := map[string]bool{
		"privileged-container":       false,
		"missing-resource-settings":  false,
		"default-service-account":    false,
		"allow-privilege-escalation": false,
		"run-as-non-root":            false,
		"missing-network-policy":     false,
	}
	for _, issue := range report.Issues {
		if _, expected := expectedRules[issue.RuleID]; expected {
			expectedRules[issue.RuleID] = true
			if len(issue.References) == 0 {
				t.Fatalf("expected %s to include a standards reference", issue.RuleID)
			}
			if issue.CVSS == "" {
				t.Fatalf("expected %s to explain its CVSS applicability", issue.RuleID)
			}
		}
	}
	for ruleID, found := range expectedRules {
		if !found {
			t.Fatalf("expected %s to be reported", ruleID)
		}
	}
}

func TestSecurityIssuesFindsExposureAndRBACCases(t *testing.T) {
	falseValue := false
	trueValue := true
	client := NewClientWithClientset(fake.NewSimpleClientset(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "workload"},
			Spec: corev1.PodSpec{
				ServiceAccountName: "app-reader",
				SecurityContext:    &corev1.PodSecurityContext{RunAsNonRoot: &trueValue},
				Volumes: []corev1.Volume{{
					Name:         "node-logs",
					VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/var/log"}},
				}},
				Containers: []corev1.Container{{
					Name: "app",
					SecurityContext: &corev1.SecurityContext{
						AllowPrivilegeEscalation: &falseValue,
						RunAsNonRoot:             &trueValue,
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m"), corev1.ResourceMemory: resource.MustParse("128Mi")},
						Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("256Mi")},
					},
				}},
			},
		},
		&networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "default-deny"}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "public-api"}, Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeNodePort}},
		&networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "public-web"}},
		&rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{Name: "demo-wildcard"},
			Rules:      []rbacv1.PolicyRule{{APIGroups: []string{"*"}, Resources: []string{"*"}, Verbs: []string{"*"}}},
		},
		&rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "demo-wildcard-binding"},
			RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: "demo-wildcard"},
			Subjects:   []rbacv1.Subject{{Kind: rbacv1.ServiceAccountKind, Namespace: "demo", Name: "app-reader"}},
		},
		&rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "demo-admin-binding"},
			RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: "cluster-admin"},
			Subjects:   []rbacv1.Subject{{Kind: rbacv1.ServiceAccountKind, Namespace: "demo", Name: "admin-agent"}},
		},
		&rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "secret-reader"},
			Rules:      []rbacv1.PolicyRule{{APIGroups: []string{""}, Resources: []string{"secrets"}, Verbs: []string{"get", "list"}}},
		},
		&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Namespace: "demo", Name: "secret-reader-binding"},
			RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "Role", Name: "secret-reader"},
			Subjects:   []rbacv1.Subject{{Kind: rbacv1.ServiceAccountKind, Name: "secret-reader"}},
		},
	))

	report, err := client.SecurityIssues(context.Background())
	if err != nil {
		t.Fatalf("SecurityIssues returned an error: %v", err)
	}

	expectedRules := map[string]bool{
		"hostpath-volume":               false,
		"service-exposure-review":       false,
		"ingress-exposure-review":       false,
		"rbac-wildcard-permission":      false,
		"cluster-admin-service-account": false,
		"rbac-broad-secret-access":      false,
	}
	for _, issue := range report.Issues {
		if _, expected := expectedRules[issue.RuleID]; expected {
			expectedRules[issue.RuleID] = true
		}
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
