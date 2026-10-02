package cluster

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Summary is the smallest cluster snapshot needed by the first dashboard screen.
// It deliberately contains counts and metadata only: Secret values are never read.
type Summary struct {
	Namespaces int       `json:"namespaces"`
	Pods       int       `json:"pods"`
	Services   int       `json:"services"`
	ObservedAt time.Time `json:"observedAt"`
}

// Client is a read-only Kubernetes API client.
type Client struct {
	clientset kubernetes.Interface
}

// NewClient uses in-cluster authentication when the API runs in Kubernetes and
// falls back to KUBECONFIG or the user's standard kubeconfig for local development.
func NewClient() (*Client, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			home, homeErr := os.UserHomeDir()
			if homeErr != nil {
				return nil, fmt.Errorf("resolve kubeconfig path: %w", homeErr)
			}
			kubeconfig = filepath.Join(home, ".kube", "config")
		}

		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("load Kubernetes configuration: %w", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("create Kubernetes client: %w", err)
	}

	return &Client{clientset: clientset}, nil
}

// Summary reads only the resources shown on the initial dashboard.
func (c *Client) Summary(ctx context.Context) (Summary, error) {
	namespaces, err := c.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return Summary{}, fmt.Errorf("list namespaces: %w", err)
	}

	pods, err := c.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return Summary{}, fmt.Errorf("list pods: %w", err)
	}

	services, err := c.clientset.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return Summary{}, fmt.Errorf("list services: %w", err)
	}

	return Summary{
		Namespaces: len(namespaces.Items),
		Pods:       len(pods.Items),
		Services:   len(services.Items),
		ObservedAt: time.Now().UTC(),
	}, nil
}
