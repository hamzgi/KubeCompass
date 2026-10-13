package cluster

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// Topology is a beginner-friendly snapshot of the application resources in a
// cluster. System namespaces are deliberately omitted from the initial view so
// that the map starts with the workloads a user is most likely to own.
type Topology struct {
	Nodes      []TopologyNode `json:"nodes"`
	Edges      []TopologyEdge `json:"edges"`
	ObservedAt time.Time      `json:"observedAt"`
}

type TopologyNode struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	Namespace   string `json:"namespace,omitempty"`
	Description string `json:"description"`
	Status      string `json:"status,omitempty"`
}

type TopologyEdge struct {
	ID           string `json:"id"`
	Source       string `json:"source"`
	Target       string `json:"target"`
	Relationship string `json:"relationship"`
}

// Topology reads Namespace, Pod, Service, and ServiceAccount metadata only.
// It never requests Secret objects or their values.
func (c *Client) Topology(ctx context.Context) (Topology, error) {
	namespaceList, err := c.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return Topology{}, fmt.Errorf("list namespaces: %w", err)
	}

	podList, err := c.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return Topology{}, fmt.Errorf("list pods: %w", err)
	}

	serviceList, err := c.clientset.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return Topology{}, fmt.Errorf("list services: %w", err)
	}

	serviceAccountList, err := c.clientset.CoreV1().ServiceAccounts("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return Topology{}, fmt.Errorf("list service accounts: %w", err)
	}

	visibleNamespaces := make(map[string]bool)
	nodesByID := make(map[string]TopologyNode)
	edgesByID := make(map[string]TopologyEdge)

	addNode := func(node TopologyNode) {
		nodesByID[node.ID] = node
	}
	addEdge := func(edge TopologyEdge) {
		edgesByID[edge.ID] = edge
	}

	for _, namespace := range namespaceList.Items {
		if isSystemNamespace(namespace.Name) {
			continue
		}

		visibleNamespaces[namespace.Name] = true
		addNode(TopologyNode{
			ID:          namespaceNodeID(namespace.Name),
			Kind:        "Namespace",
			Name:        namespace.Name,
			Description: "리소스를 나누는 작업 공간",
		})
	}

	for _, serviceAccount := range serviceAccountList.Items {
		if !visibleNamespaces[serviceAccount.Namespace] {
			continue
		}

		addNode(TopologyNode{
			ID:          serviceAccountNodeID(serviceAccount.Namespace, serviceAccount.Name),
			Kind:        "ServiceAccount",
			Name:        serviceAccount.Name,
			Namespace:   serviceAccount.Namespace,
			Description: "앱이 Kubernetes API를 사용할 때 쓰는 계정",
		})
	}

	visiblePods := make([]corev1.Pod, 0)
	for _, pod := range podList.Items {
		if !visibleNamespaces[pod.Namespace] {
			continue
		}

		visiblePods = append(visiblePods, pod)
		podID := podNodeID(pod.Namespace, pod.Name)
		addNode(TopologyNode{
			ID:          podID,
			Kind:        "Pod",
			Name:        pod.Name,
			Namespace:   pod.Namespace,
			Description: "실행 중인 앱",
			Status:      string(pod.Status.Phase),
		})
		addEdge(TopologyEdge{
			ID:           fmt.Sprintf("contains:%s:%s", namespaceNodeID(pod.Namespace), podID),
			Source:       namespaceNodeID(pod.Namespace),
			Target:       podID,
			Relationship: "포함",
		})

		serviceAccountName := pod.Spec.ServiceAccountName
		if serviceAccountName == "" {
			serviceAccountName = "default"
		}
		serviceAccountID := serviceAccountNodeID(pod.Namespace, serviceAccountName)
		if _, exists := nodesByID[serviceAccountID]; !exists {
			addNode(TopologyNode{
				ID:          serviceAccountID,
				Kind:        "ServiceAccount",
				Name:        serviceAccountName,
				Namespace:   pod.Namespace,
				Description: "앱이 Kubernetes API를 사용할 때 쓰는 계정",
			})
		}
		addEdge(TopologyEdge{
			ID:           fmt.Sprintf("uses:%s:%s", podID, serviceAccountID),
			Source:       podID,
			Target:       serviceAccountID,
			Relationship: "계정 사용",
		})
	}

	for _, service := range serviceList.Items {
		if !visibleNamespaces[service.Namespace] {
			continue
		}

		serviceID := serviceNodeID(service.Namespace, service.Name)
		addNode(TopologyNode{
			ID:          serviceID,
			Kind:        "Service",
			Name:        service.Name,
			Namespace:   service.Namespace,
			Description: "앱으로 연결되는 내부 접속 주소",
		})
		addEdge(TopologyEdge{
			ID:           fmt.Sprintf("contains:%s:%s", namespaceNodeID(service.Namespace), serviceID),
			Source:       namespaceNodeID(service.Namespace),
			Target:       serviceID,
			Relationship: "포함",
		})

		if len(service.Spec.Selector) == 0 {
			continue
		}

		selector := labels.SelectorFromSet(service.Spec.Selector)
		for _, pod := range visiblePods {
			if pod.Namespace != service.Namespace || !selector.Matches(labels.Set(pod.Labels)) {
				continue
			}

			podID := podNodeID(pod.Namespace, pod.Name)
			addEdge(TopologyEdge{
				ID:           fmt.Sprintf("routes:%s:%s", serviceID, podID),
				Source:       serviceID,
				Target:       podID,
				Relationship: "트래픽 전달",
			})
		}
	}

	nodes := make([]TopologyNode, 0, len(nodesByID))
	for _, node := range nodesByID {
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Kind != nodes[j].Kind {
			return nodes[i].Kind < nodes[j].Kind
		}
		if nodes[i].Namespace != nodes[j].Namespace {
			return nodes[i].Namespace < nodes[j].Namespace
		}
		return nodes[i].Name < nodes[j].Name
	})

	edges := make([]TopologyEdge, 0, len(edgesByID))
	for _, edge := range edgesByID {
		edges = append(edges, edge)
	}
	sort.Slice(edges, func(i, j int) bool {
		return edges[i].ID < edges[j].ID
	})

	return Topology{
		Nodes:      nodes,
		Edges:      edges,
		ObservedAt: time.Now().UTC(),
	}, nil
}

func isSystemNamespace(name string) bool {
	return strings.HasPrefix(name, "kube-") || name == "kubernetes-dashboard"
}

func namespaceNodeID(name string) string {
	return fmt.Sprintf("namespace:%s", name)
}

func podNodeID(namespace, name string) string {
	return fmt.Sprintf("pod:%s/%s", namespace, name)
}

func serviceNodeID(namespace, name string) string {
	return fmt.Sprintf("service:%s/%s", namespace, name)
}

func serviceAccountNodeID(namespace, name string) string {
	return fmt.Sprintf("serviceaccount:%s/%s", namespace, name)
}
