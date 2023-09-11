package providers

const (
	OperatingSystemLinux  = "Linux"
	OperatingSystemUbuntu = "Ubuntu"
)

type OperatingSystems map[string]bool

var (
	ValidOperatingSystems = OperatingSystems{
		OperatingSystemLinux:  true,
		OperatingSystemUbuntu: true,
	}
)

func (o OperatingSystems) Names() []string {
	keys := make([]string, 0, len(o))
	for k := range o {
		keys = append(keys, k)
	}
	return keys
}
