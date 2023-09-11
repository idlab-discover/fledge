package fledgeprovider

import (
	"context"
	"fledge/fledge-integrated/config"
	"fledge/fledge-integrated/manager"
	"fledge/fledge-integrated/providers"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/remotecommand"
)

var reInsideWhtsp = regexp.MustCompile(`\s+`)

type FledgeProviderConfig struct {
	//ConfigPath      string
	NodeName              string
	OperatingSystem       string
	InternalIP            string
	DaemonPort            int32
	ResourceManager       *manager.ResourceManager
	AvailablePodProviders map[string]*providers.PodProvider
	BridgeDevice          string
}

type FledgeProvider struct {
	config              FledgeProviderConfig
	endpoint            *url.URL
	client              *http.Client
	lastMemoryPressure  bool
	lastStoragePressure bool
	lastStorageFull     bool
}

func NewFledgeProvider(cfg FledgeProviderConfig) (*FledgeProvider, error) {
	var provider FledgeProvider

	provider.config = cfg

	//force a node update first time
	provider.lastMemoryPressure = true
	provider.lastStorageFull = true
	provider.lastStoragePressure = true

	return &provider, nil
}

func (p *FledgeProvider) Architecture() string {
	arch, err := manager.ExecCmdBash("uname -p")
	if err != nil {
		fmt.Println("Failed to fetch CPU architecture")
		return ""
	}
	return arch
}

func (p *FledgeProvider) Capacity(ctx context.Context) v1.ResourceList {
	resources := v1.ResourceList{} //make(map[v1.ResourceName]string)
	cpu, _ := resource.ParseQuantity(manager.CpuCores())
	resources[v1.ResourceCPU] = cpu
	mem, _ := resource.ParseQuantity(manager.TotalMemory() + "Mi")
	resources[v1.ResourceMemory] = mem
	stor, _ := resource.ParseQuantity(manager.TotalStorage() + "i")
	resources[v1.ResourceStorage] = stor
	pods, _ := resource.ParseQuantity("2")
	resources[v1.ResourcePods] = pods

	contResources := GetContainerResources()

	resources[v1.ResourceRequestsCPU] = *contResources[v1.ResourceRequestsCPU]
	resources[v1.ResourceRequestsMemory] = *contResources[v1.ResourceRequestsMemory]
	resources[v1.ResourceRequestsStorage] = *contResources[v1.ResourceRequestsStorage]
	resources[v1.ResourceLimitsCPU] = *contResources[v1.ResourceLimitsCPU]
	resources[v1.ResourceLimitsMemory] = *contResources[v1.ResourceLimitsCPU]

	if manager.HasOpenCLCaps() {
		q, _ := resource.ParseQuantity("1")
		resources["device/openclgpu"] = q
	}
	if manager.HasCudaCaps() {
		q, _ := resource.ParseQuantity("1")
		resources["device/cudagpu"] = q
	}

	return resources
	//return nil
}

// NodeConditions returns a list of conditions (Ready, OutOfDisk, etc), for updates to the node status
func (p *FledgeProvider) NodeConditions(ctx context.Context) []v1.NodeCondition {
	conditionReady := v1.NodeCondition{Type: v1.NodeReady, Status: v1.ConditionTrue, LastHeartbeatTime: metav1.Now(), LastTransitionTime: metav1.Now(), Reason: "Started", Message: "Rocket ranger, ready to rock it"}

	var memPressure v1.ConditionStatus
	p.lastMemoryPressure = manager.IsMemoryPressure()
	if p.lastMemoryPressure {
		memPressure = v1.ConditionTrue
	} else {
		memPressure = v1.ConditionFalse
	}
	conditionMemPressure := v1.NodeCondition{Type: v1.NodeMemoryPressure, Status: memPressure, LastHeartbeatTime: metav1.Now(), LastTransitionTime: metav1.Now(), Reason: "Memory pressure", Message: "We're giving 'er all she's got captain"}

	var storagePressure v1.ConditionStatus
	p.lastStoragePressure = manager.IsStoragePressure()
	if p.lastStoragePressure {
		storagePressure = v1.ConditionTrue
	} else {
		storagePressure = v1.ConditionFalse
	}
	conditionStoragePressure := v1.NodeCondition{Type: v1.NodeDiskPressure, Status: storagePressure, LastHeartbeatTime: metav1.Now(), LastTransitionTime: metav1.Now(), Reason: "Storage pressure", Message: "She won't take it much longer"}

	//add more conditions later, with info from cmd
	conditions := []v1.NodeCondition{conditionReady, conditionMemPressure, conditionStoragePressure} //, conditionStorageFull}
	return conditions
}

func (p *FledgeProvider) ConditionsChanged() bool {
	if manager.IsMemoryPressure() != p.lastMemoryPressure {
		return true
	}
	if manager.IsStoragePressure() != p.lastStoragePressure {
		return true
	}
	if manager.IsStorageFull() != p.lastStorageFull {
		return true
	}
	return false
}

// NodeAddresses returns a list of addresses for the node status
// within Kubernetes.
func (p *FledgeProvider) NodeAddresses(ctx context.Context) []v1.NodeAddress {
	nodenameStr, _ := manager.ExecCmdBash("hostname")
	p.config.NodeName = strings.TrimSuffix(nodenameStr, "\n")
	addresshost := v1.NodeAddress{Type: v1.NodeHostName, Address: p.config.NodeName}
	addressip := v1.NodeAddress{Type: v1.NodeInternalIP, Address: config.Cfg.DeviceIP}
	addresses := []v1.NodeAddress{addresshost, addressip}
	return addresses
}

func (p *FledgeProvider) AddressesChanged() bool {
	nodenameStr, _ := manager.ExecCmdBash("hostname")
	nodename := strings.TrimSuffix(nodenameStr, "\n")
	return p.config.NodeName != nodename
}

//merge those from the available podproviders
func GetContainerResources() map[v1.ResourceName]*resource.Quantity {
	//TODO
	return make(map[v1.ResourceName]*resource.Quantity)
}

// OperatingSystem returns the operating system for this provider.
func (p *FledgeProvider) OperatingSystem() string {
	return p.config.OperatingSystem
}

func (p *FledgeProvider) NodeChanged() bool {
	aChanged := p.AddressesChanged()
	pChanged := p.ConditionsChanged()

	fmt.Printf("NodeChanged check: addresses changed %t conditions changed %t\n", aChanged, pChanged)

	return aChanged || pChanged
}

func (p *FledgeProvider) CreatePod(ctx context.Context, pod *v1.Pod) error {
	//TODO
	return nil
}

func (p *FledgeProvider) UpdatePod(ctx context.Context, pod *v1.Pod) error {
	//TODO
	return nil
}

func (p *FledgeProvider) DeletePod(ctx context.Context, pod *v1.Pod) error {
	//TODO
	return nil
}

func (p *FledgeProvider) GetPod(ctx context.Context, namespace, name string) (*v1.Pod, error) {
	//TODO
	return nil, nil
}

func (p *FledgeProvider) GetContainerLogs(ctx context.Context, namespace, podName, containerName string, tail int) (string, error) {
	//TODO
	return "", nil
}

func (p *FledgeProvider) ExecInContainer(name string, uid types.UID, container string, cmd []string, in io.Reader, out, err io.WriteCloser, tty bool, resize <-chan remotecommand.TerminalSize, timeout time.Duration) error {
	//TODO
	return nil
}

// GetPodStatus retrieves the status of a given pod by name.
func (p *FledgeProvider) GetPodStatus(ctx context.Context, namespace, name string) (*v1.PodStatus, error) {
	//TODO
	return nil, nil
}

// GetPods retrieves a list of all pods scheduled to run.
func (p *FledgeProvider) GetPods(ctx context.Context) ([]*v1.Pod, error) {
	//TODO
	return nil, nil
}

func (p *FledgeProvider) PodsChanged() bool {
	return false
}

func (p *FledgeProvider) ResetChanges() {

}

func (p *FledgeProvider) NodeDaemonEndpoints(ctx context.Context) *v1.NodeDaemonEndpoints {
	return &v1.NodeDaemonEndpoints{
		KubeletEndpoint: v1.DaemonEndpoint{
			Port: p.config.DaemonPort,
		},
	}
}

//var totalNanoCores uint64

// GetStatsSummary returns a stats summary of the virtual node
/*func (p *ContainerdProvider) GetStatsSummary(context.Context) (*stats.Summary, error) {
	//we can delete the other service perhaps?
	nodenameStr, _ := manager.ExecCmdBash("hostname")
	nodename := strings.TrimSuffix(nodenameStr, "\n")

	//CPU STUFF, REFACTOR TO METHOD
	cpuStatsStr, _ := manager.ExecCmdBash("mpstat 1 1 | grep 'all'")

	nProc, _ := manager.ExecCmdBash("nproc")
	numCpus, _ := strconv.Atoi(strings.Trim(nProc, "\n"))

	cpuStatsLines := strings.Split(cpuStatsStr, "\n")
	//cpuStatsStr = strings.TrimSuffix(cpuStatsStr, "\n")
	cpuCats := strings.Split(reInsideWhtsp.ReplaceAllString(cpuStatsLines[0], " "), " ")
	cpuIdle, _ := strconv.ParseFloat(cpuCats[len(cpuCats)-1], 64)

	cpuNanos := uint64((100-cpuIdle)*10000000) * uint64(numCpus) //pct is already 10^2, so * 10^7, then * cores.

	//TODO: take time into account here (cpuNanos * seconds passed since last check)
	totalNanoCores += cpuNanos

	cpuStats := stats.CPUStats{
		Time:                 metav1.Now(),
		UsageNanoCores:       &cpuNanos,
		UsageCoreNanoSeconds: &totalNanoCores,
	}

	//MEM STUFF, REFACTOR TO METHOD
	memStatsStr, _ := manager.ExecCmdBash("free | grep 'Mem:'")
	cats := strings.Split(reInsideWhtsp.ReplaceAllString(memStatsStr, " "), " ")
	memFree, _ := strconv.ParseUint(cats[6], 10, 64)
	memSize, _ := strconv.ParseUint(cats[1], 10, 64)

	memStatsStr, _ = manager.ExecCmdBash("free | grep '+'")
	//bailout for older free versions, in which case this is more accurate for "available" memory
	if memStatsStr != "" {
		cats := strings.Split(reInsideWhtsp.ReplaceAllString(memStatsStr, " "), " ")
		memFree, _ = strconv.ParseUint(cats[2], 10, 64)
	}

	memUsed := memSize - memFree

	memStats := stats.MemoryStats{
		Time:            metav1.Now(),
		UsageBytes:      &memUsed,
		AvailableBytes:  &memFree,
		WorkingSetBytes: &memUsed,
	}

	//NETWORK STUFF, REFACTOR TO METHOD

	//ifnames: / # ip a | grep -o -E '[0-9]: [a-z0-9]*: '

	ifacesStr, _ := manager.ExecCmdBash("ip a | grep -o -E '[0-9]{1,2}: [a-z0-9]*: ' | grep -o -E '[a-z0-9]{2,}'")
	ifaces := strings.Split(ifacesStr, "\n")

	//ifstats: ifconfig enp1s0f0 | grep 'bytes'
	//      RX bytes:726654708 (692.9 MiB)  TX bytes:456250038 (435.1 MiB)

	ifacesStats := []stats.InterfaceStats{}
	for _, iface := range ifaces {
		ifaceStatsStr, _ := manager.ExecCmdBash("ifconfig " + iface + "| grep 'bytes'")
		fmt.Println(ifaceStatsStr)
		//TODO from here on
	}

	netStats := stats.NetworkStats{
		Time:       metav1.Now(),
		Interfaces: ifacesStats,
	}

	nodeStats := stats.NodeStats{
		NodeName:  nodename,
		StartTime: metav1.NewTime(vkube.StartTime),
		CPU:       &cpuStats,
		Memory:    &memStats,
		Network:   &netStats,
		//Fs: ,
		//Runtime: ,
		//Rlimit: ,
	}

	summary := stats.Summary{
		Node: nodeStats,
	}
	return &summary, nil
}*/
