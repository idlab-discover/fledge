package osv

import (
	"context"
	"errors"
	"fledge/fledge-integrated/vkube"
	"fmt"
	"io"
	"log"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/remotecommand"
)

type OSvProvider struct {
}

func NewOSvProvider() (*OSvProvider, error) {
	if CheckProviderRequirements() {
		var provider OSvProvider
		return &provider, nil
	} else {
		return nil, errors.New("Containerd not found")
	}
}

func CheckProviderRequirements() bool {
	//check for qemu-kvm (uhh how to determine binary name?) and check scripts/run.py path from config
	//alternatively, include scripts/run.py in FLEDGE? maybe later
	return true
}

func (p *OSvProvider) PodsChanged() bool {
	return false
}

func (p *OSvProvider) ResetChanges() {

}

// CreatePod accepts a Pod definition and forwards the call to the web endpoint
func (p *OSvProvider) CreatePod(ctx context.Context, pod *v1.Pod) error {
	fmt.Println("CreatePod")

	return nil
}

// UpdatePod accepts a Pod definition and forwards the call to the web endpoint
func (p *OSvProvider) UpdatePod(ctx context.Context, pod *v1.Pod) error {
	name := pod.ObjectMeta.Name
	namespace := pod.ObjectMeta.Namespace

	fmt.Printf("Updating pod namespace %s name %s\n", namespace, name)

	vkube.Cri.UpdatePod(pod)
	return nil
}

// DeletePod accepts a Pod definition and forwards the call to the web endpoint
func (p *OSvProvider) DeletePod(ctx context.Context, pod *v1.Pod) error {
	name := pod.ObjectMeta.Name
	namespace := pod.ObjectMeta.Namespace

	fmt.Printf("Deleting pod namespace %s name %s\n", namespace, name)

	vkube.Cri.DeletePod(pod)
	return nil
}

// GetPod returns a pod by name that is being managed by the web server
func (p *OSvProvider) GetPod(ctx context.Context, namespace, name string) (*v1.Pod, error) {
	return nil, nil
}

// GetContainerLogs returns the logs of a container running in a pod by name.
func (p *OSvProvider) GetContainerLogs(ctx context.Context, namespace, podName, containerName string, tail int) (string, error) {
	return "", nil
}

// Get full pod name as defined in the provider context
// TODO: Implementation
func (p *OSvProvider) GetPodFullName(namespace string, pod string) string {
	//TODO check how web provider did it
	return ""
}

// ExecInContainer executes a command in a container in the pod, copying data
// between in/out/err and the container's stdin/stdout/stderr.
// TODO: Implementation
func (p *OSvProvider) ExecInContainer(name string, uid types.UID, container string, cmd []string, in io.Reader, out, err io.WriteCloser, tty bool, resize <-chan remotecommand.TerminalSize, timeout time.Duration) error {
	log.Printf("receive ExecInContainer %q\n", container)
	return nil
}

// GetPodStatus retrieves the status of a given pod by name.
func (p *OSvProvider) GetPodStatus(ctx context.Context, namespace, name string) (*v1.PodStatus, error) {
	return nil, nil
}

// GetPods retrieves a list of all pods scheduled to run.
func (p *OSvProvider) GetPods(ctx context.Context) ([]*v1.Pod, error) {
	return nil, nil
}

func GetContainerResources() map[v1.ResourceName]*resource.Quantity {
	cpuRequest, _ := resource.ParseQuantity("0")
	memRequest, _ := resource.ParseQuantity("0")
	storRequest, _ := resource.ParseQuantity("0")
	cpuLimit, _ := resource.ParseQuantity("0")
	memLimit, _ := resource.ParseQuantity("0")

	podSpecs := []*v1.Pod{} //vkube.Cri.GetPods()
	fmt.Printf("Podspecs count %d\n", len(podSpecs))
	//iterate pods => podspec => iterate containers => resourcerequirements
	for _, pod := range podSpecs {
		fmt.Printf("Tallying podspec %s\n", pod.ObjectMeta.Name)
		for _, container := range pod.Spec.Containers {
			fmt.Printf("Tallying container %s\n", container.Name)
			if container.Resources.Requests != nil {
				val := container.Resources.Requests.Cpu()
				if val != nil {
					cpuRequest.Add(*val)
				}
				val = container.Resources.Requests.Memory()
				if val != nil {
					memRequest.Add(*val)
				}
				sval, err := container.Resources.Requests[v1.ResourceStorage]
				if !err {
					storRequest.Add(sval)
				}
				val = container.Resources.Limits.Cpu()
				if val != nil {
					cpuLimit.Add(*val)
				}
				val = container.Resources.Limits.Memory()
				if val != nil {
					memLimit.Add(*val)
				}
			}
		}
	}

	resources := make(map[v1.ResourceName]*resource.Quantity)
	resources[v1.ResourceRequestsCPU] = &cpuRequest
	resources[v1.ResourceRequestsMemory] = &memRequest
	resources[v1.ResourceRequestsStorage] = &storRequest
	resources[v1.ResourceLimitsCPU] = &cpuLimit
	resources[v1.ResourceLimitsMemory] = &memLimit
	fmt.Println("Resources used")
	fmt.Println(resources)

	return resources
}
