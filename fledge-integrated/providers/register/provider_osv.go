//go:build !no_web_provider
// +build !no_web_provider

package register

import (
	"fledge/fledge-integrated/providers"
	osv "fledge/fledge-integrated/providers/osv"
)

func init() {
	register("osv", initOSV)
}

func initOSV(cfg PodInitConfig) (providers.PodProvider, error) {
	return osv.NewOSvProvider()
}
