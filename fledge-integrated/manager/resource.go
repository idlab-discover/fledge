package manager

import (
	"context"
	"sync"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	corev1listers "k8s.io/client-go/listers/core/v1"

	"fledge/fledge-integrated/log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ResourceManager acts as a passthrough to a cache (lister) for pods assigned to the current node.
// It is also a passthrough to a cache (lister) for Kubernetes secrets and config maps.
type ResourceManager struct {
	sync.RWMutex

	podLister corev1listers.PodLister
	k8sClient *kubernetes.Clientset
}

// NewResourceManager returns a ResourceManager with the internal maps initialized.
func NewResourceManager(podLister corev1listers.PodLister, client *kubernetes.Clientset) (*ResourceManager, error) {
	rm := ResourceManager{
		podLister: podLister,
		k8sClient: client,
	}
	return &rm, nil
}

// GetPods returns a list of all known pods assigned to this virtual node.
func (rm *ResourceManager) GetPods() []*v1.Pod {
	l, err := rm.podLister.List(labels.Everything())
	if err == nil {
		return l
	}
	log.L.Errorf("failed to fetch pods from lister: %v", err)
	return make([]*v1.Pod, 0)
}

// GetConfigMap retrieves the specified config map from the cache.
func (rm *ResourceManager) GetConfigMap(ctx context.Context, name, namespace string) (*v1.ConfigMap, error) {
	return rm.k8sClient.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{}) //.configMapLister.ConfigMaps(namespace).Get(name)
}

// GetSecret retrieves the specified secret from Kubernetes.
func (rm *ResourceManager) GetSecret(ctx context.Context, name, namespace string) (*v1.Secret, error) {
	return rm.k8sClient.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{}) //.secretLister.Secrets(namespace).Get(name)
}
