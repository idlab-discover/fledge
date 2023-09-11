package register

import (
	"fledge/fledge-integrated/manager"
	"fledge/fledge-integrated/providers"

	"github.com/cpuguy83/strongerrors"
	"github.com/pkg/errors"
)

var providerInits = make(map[string]initFunc)

// InitConfig is the config passed to initialize a registered provider.
type InitConfig struct {
	//ConfigPath      string
	NodeName        string
	OperatingSystem string
	InternalIP      string
	DaemonPort      int32
	ResourceManager *manager.ResourceManager
}

type PodInitConfig struct {
}

type initFunc func(PodInitConfig) (providers.PodProvider, error)

// GetProvider gets the provider specified by the given name
func GetPodProvider(name string) (providers.PodProvider, error) {
	f, ok := providerInits[name]
	if !ok {
		return nil, strongerrors.NotFound(errors.Errorf("provider not found: %s", name))
	}
	return f(PodInitConfig{})
}

func register(name string, f initFunc) {
	providerInits[name] = f
}
