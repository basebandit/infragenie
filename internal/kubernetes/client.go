package kubernetes

import (
	"context"
	"fmt"
	"strings"

	"github.com/basebandit/infragenie/pkg/models"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Client struct {
	clientset     kubernetes.Interface
	dynamicClient dynamic.Interface
}

func NewClient(kubeConfig string) (*Client, error) {
	var config *rest.Config
	var err error

	if kubeConfig == "" {
		// Use in-cluster config
		config, err = rest.InClusterConfig()
	} else {
		// Use kubeconfig file
		config, err = clientcmd.BuildConfigFromFlags("", kubeConfig)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes config: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	return &Client{
		clientset:     clientset,
		dynamicClient: dynamicClient,
	}, nil
}

func (c *Client) ApplyResource(ctx context.Context, resource *models.KubernetesResource) error {
	// convert to unstructured object
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": resource.APIVersion,
			"kind":       resource.Kind,
			"metadata":   resource.Metadata,
			"spec":       resource.Spec,
		},
	}

	// Determin Group Version Resource (GVR)
	gvr, err := c.getGVR(resource.APIVersion, resource.Kind)
	if err != nil {
		return fmt.Errorf("failed to determine GVR: %w", err)
	}

	namespace := "default"
	if ns, ok := resource.Metadata["namespace"].(string); ok {
		namespace = ns
	}

	// Apply resource
	_, err = c.dynamicClient.Resource(gvr).Namespace(namespace).Create(ctx, obj, metav1.CreateOptions{})

	if err != nil {
		// if resource exists, update it
		if strings.Contains(err.Error(), "already exists") {
			_, err = c.dynamicClient.Resource(gvr).Namespace(namespace).Update(ctx, obj, metav1.UpdateOptions{})
		}
	}

	return err
}

func (c *Client) getGVR(apiVersion, kind string) (schema.GroupVersionResource, error) {
	// Map common Kubernetes resources to their GVR
	gvrMap := map[string]map[string]schema.GroupVersionResource{
		"v1": {
			"Pod":       {Group: "", Version: "v1", Resource: "pods"},
			"Service":   {Group: "", Version: "v1", Resource: "services"},
			"ConfigMap": {Group: "", Version: "v1", Resource: "configmaps"},
			"Secret":    {Group: "", Version: "v1", Resource: "secrets"},
			"Namespace": {Group: "", Version: "v1", Resource: "namespaces"},
		},
		"apps/v1": {
			"Deployment":  {Group: "apps", Version: "v1", Resource: "deployments"},
			"StatefulSet": {Group: "apps", Version: "v1", Resource: "statefulsets"},
			"DaemonSet":   {Group: "apps", Version: "v1", Resource: "daemonsets"},
		},
		"networking.k8s.io/v1": {
			"NetworkPolicy": {Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies"},
			"Ingress":       {Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"},
		},
		"rbac.authorization.k8s.io/v1": {
			"Role":               {Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "roles"},
			"RoleBinding":        {Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "rolebindings"},
			"ClusterRole":        {Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles"},
			"ClusterRoleBinding": {Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterrolebindings"},
		},
	}

	if kindMap, exists := gvrMap[apiVersion]; exists {
		if gvr, exists := kindMap[kind]; exists {
			return gvr, nil
		}
	}

	return schema.GroupVersionResource{}, fmt.Errorf("unknown resource: %s/%s", apiVersion, kind)
}

func (c *Client) GetPods(ctx context.Context, namespace string) ([]models.KubernetesResource, error) {
	pods, err := c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var resources []models.KubernetesResource
	for _, pod := range pods.Items {
		resource := models.KubernetesResource{
			APIVersion: pod.APIVersion,
			Kind:       pod.Kind,
			Metadata: map[string]interface{}{
				"name":      pod.Name,
				"namespace": pod.Namespace,
				"labels":    pod.Labels,
			},
			Spec: map[string]interface{}{
				"containers": pod.Spec.Containers,
			},
			Status: map[string]interface{}{
				"phase": pod.Status.Phase,
			},
		}
		resources = append(resources, resource)
	}

	return resources, nil
}
