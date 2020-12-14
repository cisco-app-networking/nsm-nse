package nseconfig

import (
	"cisco-app-networking.github.io/networkservicemesh/controlplane/api/connection/mechanisms/memif"
	"cisco-app-networking.github.io/networkservicemesh/sdk/common"
)

type NSConfigurationConverter interface {
	ToNSConfiguration() *common.NSConfiguration
}

func (e *Endpoint) ToNSConfiguration() *common.NSConfiguration {
	configuration := &common.NSConfiguration{
		EndpointNetworkService: e.Name,
		EndpointLabels:         e.Labels.String(),
		MechanismType:          memif.MECHANISM,
		IPAddress:              e.VL3.IPAM.DefaultPrefixPool,
		Routes:                 e.VL3.IPAM.Routes,
		NscInterfaceName:       e.VL3.Ifname,
	}

	// takes the rest of configuration from env if env is set accordingly
	return configuration.FromEnv()
}
