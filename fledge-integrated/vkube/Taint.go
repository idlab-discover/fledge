package vkube

import (
	"os"

	//	"github.com/cpuguy83/strongerrors"

	//	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
)

// Default taint values
const (
	DefaultTaintEffect = corev1.TaintEffectNoSchedule
	DefaultTaintKey    = "virtual-kubelet.io/provider"
)

func getEnv(key, defaultValue string) string {
	value, found := os.LookupEnv(key)
	if found {
		return value
	}
	return defaultValue
}

// getTaint creates a taint using the provided key/value.
// Taint effect is read from the environment
// The taint key/value may be overwritten by the environment.
func GetTaint(provider, key, value string) (*corev1.Taint, error) {
	if key == "" {
		key = DefaultTaintKey
		value = provider
	}

	var effect corev1.TaintEffect = corev1.TaintEffectNoSchedule

	return &corev1.Taint{
		Key:    key,
		Value:  value,
		Effect: effect,
	}, nil
}
