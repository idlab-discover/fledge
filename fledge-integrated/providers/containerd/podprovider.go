package containerd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"regexp"
	"strconv"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/remotecommand"

	"fledge/fledge-integrated/vkube"

	"k8s.io/apimachinery/pkg/api/resource"
)

var reInsideWhtsp = regexp.MustCompile(`\s+`)

type ContainerdProvider struct {
}

func NewContainerdProvider() (*ContainerdProvider, error) {
	if CheckProviderRequirements() {
		var provider ContainerdProvider

		return &provider, nil
	} else {
		return nil, errors.New("Containerd not found")
	}
}

func CheckProviderRequirements() bool {
	//check if ctr binary is present, or maybe just a flag in config for now
	return true
}

func (p *ContainerdProvider) PodsChanged() bool {
	pChanged := vkube.Cri.PodsChanged()
	fmt.Printf("PodsChanged check: %t", pChanged)
	return pChanged
}

func (p *ContainerdProvider) ResetChanges() {
	vkube.Cri.ResetFlags()
}

func (p *ContainerdProvider) CreatePod(ctx context.Context, pod *v1.Pod) error {
	fmt.Println("CreatePod")

	json, _ := json.Marshal(pod)
	fmt.Println(string(json))

	name := pod.ObjectMeta.Name
	namespace := pod.ObjectMeta.Namespace

	fmt.Printf("Creating pod namespace %s name %s\n", namespace, name)

	containers := pod.Spec.Containers
	restartPolicy := pod.Spec.RestartPolicy
	useHostnetwork := pod.Spec.HostNetwork

	fmt.Printf("Creating pod num containers %d restart policy %s use host network %t\n", len(containers), restartPolicy, useHostnetwork)

	vkube.Cri.DeployPod(pod)
	return nil
}

func (p *ContainerdProvider) UpdatePod(ctx context.Context, pod *v1.Pod) error {
	name := pod.ObjectMeta.Name
	namespace := pod.ObjectMeta.Namespace

	fmt.Printf("Updating pod namespace %s name %s\n", namespace, name)

	vkube.Cri.UpdatePod(pod)
	return nil
}

func (p *ContainerdProvider) DeletePod(ctx context.Context, pod *v1.Pod) error {
	name := pod.ObjectMeta.Name
	namespace := pod.ObjectMeta.Namespace

	fmt.Printf("Deleting pod namespace %s name %s\n", namespace, name)

	vkube.Cri.DeletePod(pod)
	return nil
}

func (p *ContainerdProvider) GetPod(ctx context.Context, namespace, name string) (*v1.Pod, error) {
	pod, found := vkube.Cri.GetPod(namespace, name)

	if found {
		return pod, nil
	} else {
		//TODO FIX THIS BASED ON WHAT THE WEB PROVIDER DID
		return nil, nil
	}
}

// GetContainerLogs returns the logs of a container running in a pod by name.
func (p *ContainerdProvider) GetContainerLogs(ctx context.Context, namespace, podName, containerName string, tail int) (string, error) {
	reader := vkube.Cri.FetchContainerLogs(namespace, podName, containerName, strconv.Itoa(tail), true)
	if reader != nil {
		buf := new(bytes.Buffer)
		buf.ReadFrom(*reader)
		logs := buf.String()
		//fmt.Fprintf(w, "%s", logs)
		return logs, nil
	} else {
		return "", nil
	}
}

// Get full pod name as defined in the provider context
// TODO: Implementation
/*func (p *ContainerdProvider) GetPodFullName(namespace string, pod string) string {
	//TODO check how web provider did it
	return ""
}*/

// ExecInContainer executes a command in a container in the pod, copying data
// between in/out/err and the container's stdin/stdout/stderr.
// TODO: Implementation
func (p *ContainerdProvider) ExecInContainer(name string, uid types.UID, container string, cmd []string, in io.Reader, out, err io.WriteCloser, tty bool, resize <-chan remotecommand.TerminalSize, timeout time.Duration) error {
	log.Printf("receive ExecInContainer %q\n", container)
	return nil
}

// GetPodStatus retrieves the status of a given pod by name.
func (p *ContainerdProvider) GetPodStatus(ctx context.Context, namespace, name string) (*v1.PodStatus, error) {
	pod, found := vkube.Cri.GetPod(namespace, name)

	if found {
		return &pod.Status, nil
	} else {
		return nil, nil
	}
}

// GetPods retrieves a list of all pods scheduled to run.
func (p *ContainerdProvider) GetPods(ctx context.Context) ([]*v1.Pod, error) {
	podSpecs := vkube.Cri.GetPods()
	//again, might wanna update some values first tho
	pods := []*v1.Pod{}
	for _, value := range podSpecs {
		//cri.UpdatePodStatus(value.ObjectMeta.Namespace, value)
		pods = append(pods, value)
	}
	return pods, nil
}

//don't know enough go to write this in a decent way yet
//then again, there's probably no decent way to write this in go
//table flip
func GetContainerResources() map[v1.ResourceName]*resource.Quantity {
	cpuRequest, _ := resource.ParseQuantity("0")
	memRequest, _ := resource.ParseQuantity("0")
	storRequest, _ := resource.ParseQuantity("0")
	cpuLimit, _ := resource.ParseQuantity("0")
	memLimit, _ := resource.ParseQuantity("0")

	podSpecs := vkube.Cri.GetPods()
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
