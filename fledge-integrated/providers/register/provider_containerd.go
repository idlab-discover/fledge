//go:build !no_web_provider
// +build !no_web_provider

package register

import (
	"fledge/fledge-integrated/providers"
	containerd "fledge/fledge-integrated/providers/containerd"
)

func init() {
	register("containerd", initContainerd)
}

func initContainerd(cfg PodInitConfig) (providers.PodProvider, error) {
	return containerd.NewContainerdProvider()
}
