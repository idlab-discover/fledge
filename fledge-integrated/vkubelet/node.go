package vkubelet

import (
	"context"
	"fmt"
	"strings"

	"fledge/fledge-integrated/log"
	"fledge/fledge-integrated/manager"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	vkVersion = strings.Join([]string{"v1.15.1"}, "-") //, "vka", "1"}, "-")
)

// registerNode registers this virtual node with the Kubernetes API.
func (s *Server) registerNode(ctx context.Context) error {
	taints := make([]corev1.Taint, 0)

	if s.taint != nil {
		taints = append(taints, *s.taint)
	}

	computeCUDA := "false"
	computeOpenCL := "false"
	computeOpenCLVers := manager.OpenCLVersSupport()

	if manager.HasCudaCaps() {
		computeCUDA = "true"
	}
	if manager.HasOpenCLCaps() {
		computeOpenCL = "true"
	}

	fmt.Printf("Node compute caps OpenCL %s %s CUDA %s", computeOpenCL, computeOpenCLVers, computeCUDA)

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: strings.ToLower(s.nodeName),
			Labels: map[string]string{
				"type":                   "virtual-kubelet",
				"kubernetes.io/role":     "agent",
				"beta.kubernetes.io/os":  strings.ToLower(s.nodeProvider.OperatingSystem()),
				"kubernetes.io/hostname": s.nodeName,
				"alpha.service-controller.kubernetes.io/exclude-balancer": "true",
				"computeCUDA":   computeCUDA,
				"computeOpenCL": computeOpenCL,
				"openCLVersion": computeOpenCLVers,
			},
		},
		Spec: corev1.NodeSpec{
			Taints: taints,
		},
		Status: corev1.NodeStatus{
			NodeInfo: corev1.NodeSystemInfo{
				OperatingSystem: s.nodeProvider.OperatingSystem(),
				Architecture:    s.nodeProvider.Architecture(),
				KubeletVersion:  vkVersion,
			},
			Capacity:        s.nodeProvider.Capacity(ctx),
			Allocatable:     s.nodeProvider.Capacity(ctx),
			Conditions:      s.nodeProvider.NodeConditions(ctx),
			Addresses:       s.nodeProvider.NodeAddresses(ctx),
			DaemonEndpoints: *s.nodeProvider.NodeDaemonEndpoints(ctx),
		},
	}

	if _, err := s.k8sClient.CoreV1().Nodes().Create(ctx, node, metav1.CreateOptions{}); err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	log.G(ctx).Info("Registered node")

	return nil
}

// updateNode updates the node status within Kubernetes with updated NodeConditions.
func (s *Server) updateNode(ctx context.Context) {
	opts := metav1.GetOptions{}
	n, err := s.k8sClient.CoreV1().Nodes().Get(ctx, s.nodeName, opts)
	if err != nil && !errors.IsNotFound(err) {
		log.G(ctx).WithError(err).Error("Failed to retrieve node")
		return
	}

	if errors.IsNotFound(err) {
		if err = s.registerNode(ctx); err != nil {
			log.G(ctx).WithError(err).Error("Failed to register node")
		} else {
			//span.Annotate(nil, "Registered node in k8s")
		}
		return
	}

	n.ResourceVersion = "" // Blank out resource version to prevent object has been modified error
	n.Status.Conditions = s.nodeProvider.NodeConditions(ctx)

	capacity := s.nodeProvider.Capacity(ctx)
	n.Status.Capacity = capacity
	n.Status.Allocatable = capacity

	n.Status.Addresses = s.nodeProvider.NodeAddresses(ctx)

	n, err = s.k8sClient.CoreV1().Nodes().UpdateStatus(ctx, n, metav1.UpdateOptions{})
	if err != nil {
		log.G(ctx).WithError(err).Error("Failed to update node")
		return
	}
}

type taintsStringer []corev1.Taint

func (t taintsStringer) String() string {
	var s string
	for _, taint := range t {
		if s == "" {
			s = taint.Key + "=" + taint.Value + ":" + string(taint.Effect)
		} else {
			s += ", " + taint.Key + "=" + taint.Value + ":" + string(taint.Effect)
		}
	}
	return s
}
