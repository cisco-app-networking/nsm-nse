package main

import (
	"fmt"
	"github.com/cisco-app-networking/nsm-nse/pkg/nseconfig"
	"github.com/cisco-app-networking/nsm-nse/pkg/universal-cnf/config"
	"github.com/networkservicemesh/networkservicemesh/sdk/common"
	"github.com/stretchr/testify/assert"
	"testing"
)


func TestAddCompositeEndpoints(t *testing.T) {
	vce := vL3CompositeEndpoint{}
	nsConfig := &common.NSConfiguration{
		IPAddress: "10.0.0.1",
	}

	ucnfEndpoint := &nseconfig.Endpoint{
		VL3: nseconfig.VL3{
			IPAM: nseconfig.IPAM{
				DefaultPrefixPool: "192.168.0.0/24",
				ServerAddress: "192.168.0.1",
			},
		},
		NseControl: &nseconfig.NseControl{},
		AwsTgwEndpoint: nil,
	}

	newVL3ConnectComposite = func(configuration *common.NSConfiguration, vL3NetCidr string, backend config.UniversalCNFBackend, remoteIpList []string, getNseName fnGetNseName, defaultCdPrefix, nseControlAddr, connDomain string) *vL3ConnectComposite {
		return nil
	}

	got := vce.AddCompositeEndpoints(nsConfig, ucnfEndpoint)

	assert.NotNil(t, got)
	assert.Equal(t, 1, len(*got))

	compositeEndpoints := *got
	typeofEndpoint := fmt.Sprintf("%T", compositeEndpoints[0])
	assert.NotEqual(t, "*main.AwsTgwConnector", typeofEndpoint)
}

func TestAddCompositeEndpointsAws(t *testing.T) {
	vce := vL3CompositeEndpoint{}
	nsConfig := &common.NSConfiguration{
		IPAddress: "10.0.0.1",
	}

	ucnfEndpoint := &nseconfig.Endpoint{
		VL3: nseconfig.VL3{
			IPAM: nseconfig.IPAM{
				DefaultPrefixPool: "192.168.0.0/24",
				ServerAddress: "192.168.0.1",
			},
		},
		NseControl: &nseconfig.NseControl{},
		AwsTgwEndpoint: &nseconfig.AwsTgwEndpoint{
			TransitGatewayID: "1",
			TransitGatewayName: "test",
		},
	}

	newVL3ConnectComposite = func(configuration *common.NSConfiguration, vL3NetCidr string, backend config.UniversalCNFBackend, remoteIpList []string, getNseName fnGetNseName, defaultCdPrefix, nseControlAddr, connDomain string) *vL3ConnectComposite {
		return nil
	}

	got := vce.AddCompositeEndpoints(nsConfig, ucnfEndpoint)

	assert.NotNil(t, got)
	assert.Equal(t, len(*got), 2)

	compositeEndpoints := *got
	awsTgwEndpoint := compositeEndpoints[1]
	typeOfEndpoint := fmt.Sprintf("%T", awsTgwEndpoint)

	assert.Equal(t, "*main.AwsTgwConnector", typeOfEndpoint )
}