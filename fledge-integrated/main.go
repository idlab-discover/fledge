package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	kubeinformers "k8s.io/client-go/informers"

	"k8s.io/client-go/kubernetes"

	"math"
	"net/http"
	"os"
	"os/signal"

	"k8s.io/client-go/rest"

	"fledge/fledge-integrated/config"
	"fledge/fledge-integrated/log"
	"fledge/fledge-integrated/manager"
	"fledge/fledge-integrated/providers"
	"fledge/fledge-integrated/providers/fledgeprovider"
	"fledge/fledge-integrated/providers/register"
	"fledge/fledge-integrated/vkube"
	"fledge/fledge-integrated/vkubelet"
	"strings"
	"syscall"
	"time"
)

func main() {
	argsWithoutProg := os.Args[1:]
	cfgFile := "defaultconfig.json"
	if len(argsWithoutProg) > 0 {
		cfgFile = argsWithoutProg[0]
	}

	//fmt.Printf("Loading config file %s\n", cfgFile)
	config.LoadConfig(cfgFile)
	/*if config.Cfg.Runtime == "containerd" {
		//fmt.Println("Created containerd runtime interface")
		vkube.Cri = (&vkube.ContainerdRuntimeInterface{}).Init()
	} else {
		vkube.Cri = nil //(&DockerRuntimeInterface{}).Init()
	}*/

	vkube.StartTime = time.Now()

	//vkubelet router is replaced by starting the virtual kubelet thing with the "iotedge" provider
	startVirtualKubelet()
}

func createVirtualKubelet(cfg vkubelet.Config) *vkubelet.Server {
	k8snil := k8sClient == nil
	fmt.Printf("CreateVirtualKubelet %t", k8snil)
	vk := vkubelet.New(cfg)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		rootContextCancel()
		//prof.Stop()
	}()

	return vk
}

func getKubeClient() (*kubernetes.Clientset, error) {
	host, port := os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT")
	fmt.Printf("GetKubeClient: creating config with host %s port %s\n", host, port)

	config, err := rest.InClusterConfig()
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}
	cfgJson, _ := json.Marshal(config)
	fmt.Printf("GetKubeClient: Config created %s\n", string(cfgJson))
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	clientsetnil := clientset == nil
	fmt.Printf("GetKubeClient: Clientset nil %t\n", clientsetnil)

	if err != nil {
		return clientset, err
	}

	//fmt.Println("GetKubeClient: Clientset created")
	return clientset, nil
}

func startVirtualKubelet() {
	if config.Cfg.VkubeServiceURL != "" {
		//fmt.Println("StartVirtualKubelet")
		parts := strings.Split(config.Cfg.DeviceName, ".")

		namelen := int(math.Min(55, float64(len(parts[0]))))
		config.Cfg.ShortDeviceName = config.Cfg.DeviceName[:namelen]

		cfg := initVkubeletConfig()
		//err := StartVKubelet(config.shortDeviceName, config.DeviceIP, config.ServicePort, config.KubeletPort)
		vk := createVirtualKubelet(cfg)

		go func() {
			//fmt.Println("Creating mux and attaching routes to provider")
			mux := http.NewServeMux()
			vkubelet.AttachAllRoutes(cfg.NodeProvider, mux)
			http.ListenAndServe(":"+config.Cfg.KubeletPort, mux)
		}()

		go func() {
			node, err := k8sClient.CoreV1().Nodes().Get(rootContext, config.Cfg.ShortDeviceName, metav1.GetOptions{}) //(*nodeLister).Get(config.Cfg.ShortDeviceName)
			for node == nil {
				node, err = k8sClient.CoreV1().Nodes().Get(rootContext, config.Cfg.ShortDeviceName, metav1.GetOptions{}) //(*nodeLister).Get(config.Cfg.ShortDeviceName)
				time.Sleep(500 * time.Millisecond)
			}

			podCidr := node.Spec.PodCIDR

			if err != nil {
				fmt.Println(err.Error())
			} else {
				fmt.Println("Virtual kubelet started")
				fmt.Printf("Pod subnet %s", podCidr)
				createHostCNI(podCidr)
			}
		}()
		if err := vk.Run(rootContext); err != nil && errors.Cause(err) != context.Canceled {
			log.G(rootContext).Fatal(err)
		}
	}
}

func createHostCNI(cidrSubnet string) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println(err)
		}
	}()

	cmd := fmt.Sprintf("ip address show dev %s | grep -E -o '[0-9\\.]{7,15}/'", config.Cfg.ExternalInterface)
	tunaddr, _ := manager.ExecCmdBash(cmd)

	tunaddrlen := len(tunaddr)
	fmt.Printf("Got tun/tap addr %s len %d\n", tunaddr, tunaddrlen)
	subnetPts := strings.Split(cidrSubnet, "/")

	ipPts := strings.Split(subnetPts[0], ".")

	tunPts := strings.Split(tunaddr[0:len(tunaddr)-2], ".")
	subnetIpPts := strings.Split(subnetPts[0], ".")

	//so, obviously some assumptions are made here that need to be cleared up for production grade code
	//the first parameter assumes a pod subnet (cluster wide) of /16, which isn't always the case
	//however, getting the entire subnet from Kubernetes would mean another call just to get that, so for now it's not worth it
	//the last parameter also assumes that the vpn server is on the .1 ip address of the subnet, which may not be the case
	//this is something that's not really easy to work out without knowing how vpn will be deployed in production, so left it like this for now
	cmd = fmt.Sprintf("sh -x ./startcni.sh %s %s %s %s", ipPts[0]+"."+ipPts[1]+".0.0", subnetPts[1], subnetIpPts[0]+"."+subnetIpPts[1]+"."+subnetIpPts[2]+".1", tunPts[0]+"."+tunPts[1]+"."+tunPts[2]+".1")

	fmt.Printf("Attempting CNI initialization %s", cmd)
	output, _ := manager.ExecCmdBash(cmd)
	fmt.Println(output)

	vkube.InitContainerNetworking(subnetPts[0], subnetPts[1])
}

var k8sClient *kubernetes.Clientset

var rootContext, rootContextCancel = context.WithCancel(context.Background())

func initVkubeletConfig() vkubelet.Config {
	k8sClient, _ = getKubeClient()
	k8snil := k8sClient == nil
	fmt.Printf("k8sClient nil check %t\n", k8snil)

	availableProviders, err := checkAvailableProviders()
	if err != nil {
		panic(err)
	}

	brdInterface, err := determineBridgeInterface(availableProviders)

	operatingSystem, err := checkOperatingSystem()
	if err != nil {
		panic(err)
	}

	logLevel := "INFO"
	level, err := log.ParseLevel(logLevel)
	if err != nil {
		fmt.Printf("Log level %s not parsed correctly\n", logLevel)
	}

	logrus.SetLevel(level)

	logger := log.L.WithFields(logrus.Fields{
		"nodeProvider":    "fledge",
		"podProviders":    availableProviders,
		"operatingSystem": operatingSystem,
		"node":            config.Cfg.ShortDeviceName,
		"namespace":       corev1.NamespaceAll,
	})
	log.L = logger

	taint, err := vkube.GetTaint("fledge", "", "fledge")
	if err != nil {
		logger.WithError(err).Fatal("Error setting up desired kubernetes node taint")
	}

	resourceManager, err := createResourceManager()
	if err != nil {
		panic(err)
	}

	nodeConfig := fledgeprovider.FledgeProviderConfig{
		NodeName:              config.Cfg.ShortDeviceName,
		OperatingSystem:       operatingSystem,
		ResourceManager:       resourceManager,
		DaemonPort:            int32(8101),
		InternalIP:            config.Cfg.DeviceIP,
		BridgeDevice:          brdInterface,
		AvailablePodProviders: availableProviders,
	}

	jsonStr, _ := json.Marshal(nodeConfig)
	fmt.Printf("Created provider config %s\n", string(jsonStr))

	nodeProvider, err := fledgeprovider.NewFledgeProvider(nodeConfig)
	if err != nil {
		fmt.Println("Error intializing resource manager")
		fmt.Println(err.Error())
		panic(err)
	}

	return vkubelet.Config{
		Client:          k8sClient,
		Namespace:       corev1.NamespaceAll,
		NodeName:        config.Cfg.ShortDeviceName,
		Taint:           taint,
		NodeProvider:    nodeProvider,
		ResourceManager: resourceManager,
		PodSyncWorkers:  2,
		//PodInformer:     podInformer,
	}
}

func checkAvailableProviders() (map[string]*providers.PodProvider, error) {
	podProviderNames := []string{"containerd", "osv"}

	//check which ones are actually present
	availableProviders := make(map[string]*providers.PodProvider)
	for _, name := range podProviderNames {
		prov, err := register.GetPodProvider(name)
		if err == nil {
			fmt.Printf("Provider loaded %s\n", name)
			availableProviders[name] = &prov
		} else {
			fmt.Printf("Provider not present %s\n", name)
			fmt.Println(err.Error())
		}
	}

	if len(availableProviders) == 0 {
		return nil, errors.New("No pod providers detected")
	}
	return availableProviders, nil
}

func checkOperatingSystem() (string, error) {
	operatingSystem, err := manager.ExecCmdBash("lsb_release -is")
	if err != nil {
		fmt.Println("Error checking operating system, OS check fail")
		return "", err
	}

	fmt.Printf("InitVkubeletConfig %s\n", operatingSystem)
	// Validate operating system.
	ok, _ := providers.ValidOperatingSystems[operatingSystem]
	if !ok {
		return "", errors.New(fmt.Sprintf("OS check %t\n", ok))
	}
	return operatingSystem, nil
}

func createResourceManager() (*manager.ResourceManager, error) {
	kubeSharedInformerFactoryResync := 1 * time.Minute
	kubeNamespace := corev1.NamespaceAll
	// Create a shared informer factory for Kubernetes pods in the current namespace (if specified) and scheduled to the current node.
	podInformerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(k8sClient, kubeSharedInformerFactoryResync, kubeinformers.WithNamespace(kubeNamespace), kubeinformers.WithTweakListOptions(func(options *metav1.ListOptions) {
		options.FieldSelector = fields.OneTermEqualSelector("spec.nodeName", config.Cfg.ShortDeviceName).String()
	}))
	// Create a pod informer so we can pass its lister to the resource manager.
	podInformer := podInformerFactory.Core().V1().Pods()

	vkube.K8sClient = k8sClient

	// Create a new instance of the resource manager that uses the listers above for pods, secrets and config maps.
	resourceManager, err := manager.NewResourceManager(podInformer.Lister(), k8sClient)
	if err != nil {
		return nil, err
	}

	// Start the shared informer factory for pods.
	go podInformerFactory.Start(rootContext.Done())

	return resourceManager, nil
}
