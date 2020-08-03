package nseconfig

import (
	"github.com/networkservicemesh/networkservicemesh/controlplane/api/connection/mechanisms/memif"
	"github.com/networkservicemesh/networkservicemesh/sdk/common"
)

type NSConfigurationConverter interface {
	ToNSConfiguration() *common.NSConfiguration
}

func (e *Endpoint) ToNSConfiguration() *common.NSConfiguration {
	configuration := &common.NSConfiguration{
		EndpointNetworkService: e.Name,
		EndpointLabels:         e.Labels.String(),
		MechanismType:          memif.MECHANISM,
		IPAddress:              e.VL3.WCMD.DefaultPrefixPool,
		Routes:                 e.VL3.WCMD.Routes,
		NscInterfaceName:       e.VL3.Ifname,
	}

	// takes the rest of configuration from env if env is set accordingly
	return configuration.FromEnv()
}
