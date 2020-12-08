package vppagent

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.ligato.io/vpp-agent/v3/proto/ligato/vpp"
	interfaces "go.ligato.io/vpp-agent/v3/proto/ligato/vpp/interfaces"
	vppl3 "go.ligato.io/vpp-agent/v3/proto/ligato/vpp/l3"

	"github.com/networkservicemesh/networkservicemesh/controlplane/api/connection"
	"github.com/networkservicemesh/networkservicemesh/controlplane/api/connectioncontext"
	"github.com/networkservicemesh/networkservicemesh/sdk/common"
)

const (
	dstIpAddrClient = "192.168.22.2"
	dstIpRouteClient =  "192.168.0.0/16"
	ifName = "endpoint0"
	mechanismType = "MEMIF"
	podName = "helloworld-ucnf-78956df47b-r5fj4"
	srcIpAddrEndpoint = "192.168.22.1"
	srcIpRouteEndpoint = "192.168.0.1/16"
	serviceName = "ucnf"
	workspaceEnv = "workspace"
)

func TestBuildVppIfNameNSCPeer(t *testing.T) {

	b := UniversalCNFVPPAgentBackend{}
	conn := &connection.Connection{
		Labels: map[string]string {
			"podName": podName,
		},
	}

	expected := podName
	got := b.buildVppIfName("", "", conn)

	assert.Equal(t, expected, got)
}

func TestBuildVppIfNameVL3NSEPeer(t *testing.T) {

	b := UniversalCNFVPPAgentBackend{}
	conn := &connection.Connection{
		Labels: map[string]string {
			"ucnf/peerName": podName,
		},
	}

	expected := podName
	got := b.buildVppIfName("", "", conn)

	assert.Equal(t, expected, got)
}

func TestBuildVppIfNameDefault(t *testing.T) {

	b := UniversalCNFVPPAgentBackend{
		EndpointIfID: map[string]int {},
	}

	conn := &connection.Connection{}

	expected := ifName + "/0"
	got := b.buildVppIfName(ifName, serviceName, conn)

	assert.Equal(t, expected, got)
}

func TestProcessEndpoint(t *testing.T) {

	b := UniversalCNFVPPAgentBackend{}
	vppconfig := &vpp.ConfigData{}
	conn := &connection.Connection{
		Context: &connectioncontext.ConnectionContext{
			IpContext: &connectioncontext.IPContext{
				SrcIpAddr: srcIpAddrEndpoint + "/30",
				SrcIpRequired: true,
				DstIpRequired: true,
				SrcRoutes: []*connectioncontext.Route{
					&connectioncontext.Route{Prefix: srcIpRouteEndpoint},
				},
			},
		},
		Labels: map[string]string {
			"podName": podName,
		},
		Mechanism: &connection.Mechanism{
			Type: mechanismType,
		},
	}

	os.Setenv(common.WorkspaceEnv, workspaceEnv)

	b.ProcessEndpoint(vppconfig, serviceName, ifName, conn)

	//make sure the expected interface has been added to vppconfig
	assert.NotNil(t, vppconfig)
	assert.NotNil(t, vppconfig.Interfaces)
	assert.Equal(t, 1, len(vppconfig.Interfaces))

	expectedIfName := b.buildVppIfName(ifName, serviceName, conn)
	iface := vppconfig.Interfaces[0]
	assert.Equal(t, expectedIfName, iface.Name)
	assert.Equal(t, interfaces.Interface_MEMIF, iface.GetType())
	assert.True(t, iface.Enabled)
	assert.NotNil(t, iface.IpAddresses)
	assert.NotNil(t, iface.Link)

	memif := iface.GetMemif()
	assert.NotNil(t, memif)
	assert.True(t, memif.Master)
	assert.Equal(t, workspaceEnv, memif.SocketFilename)
	assert.Equal(t, uint32(512), memif.RingSize)

	assert.NotNil(t, iface.RxModes)
	assert.Equal(t, 1, len(iface.RxModes))
	rxmode := iface.RxModes[0]
	assert.Equal(t, interfaces.Interface_RxMode_INTERRUPT, rxmode.Mode)
	assert.True(t, rxmode.DefaultMode)

	assert.NotNil(t, vppconfig.Routes)
	assert.Equal(t, 1, len(vppconfig.Routes))

	route := vppconfig.Routes[0]
	assert.Equal(t, vppl3.Route_INTER_VRF, route.Type)
	assert.Equal(t, srcIpRouteEndpoint, route.DstNetwork)
	assert.Equal(t, srcIpAddrEndpoint, route.NextHopAddr)
}

func TestProcessClient(t *testing.T) {

	b := UniversalCNFVPPAgentBackend{}
	vppconfig := &vpp.ConfigData{}
	conn := &connection.Connection{
		Context: &connectioncontext.ConnectionContext{
			IpContext: &connectioncontext.IPContext{
				DstIpAddr: dstIpAddrClient +"/30",
				SrcIpRequired: true,
				DstIpRequired: true,
				DstRoutes: []*connectioncontext.Route{
					&connectioncontext.Route{Prefix: dstIpRouteClient},
				},
			},
		},
		Mechanism: &connection.Mechanism{
			Type: mechanismType,
		},
	}

	os.Setenv(common.WorkspaceEnv, workspaceEnv)

	b.ProcessClient(vppconfig, ifName, conn)

	assert.NotNil(t, vppconfig)
	assert.NotNil(t, vppconfig.Interfaces)
	assert.Equal(t, 1, len(vppconfig.Interfaces))
	iface := vppconfig.Interfaces[0]
	assert.Equal(t, ifName, iface.Name)
	assert.Equal(t, interfaces.Interface_MEMIF, iface.GetType())
	assert.True(t, iface.Enabled)
	assert.NotNil(t, iface.IpAddresses)
	assert.NotNil(t, iface.Link)

	memif := iface.GetMemif()
	assert.NotNil(t, memif)
	assert.False(t, memif.Master)
	assert.Equal(t, workspaceEnv, memif.SocketFilename)
	assert.Equal(t, uint32(512), memif.RingSize)

	assert.NotNil(t, vppconfig.Routes)
	assert.Equal(t, 1, len(vppconfig.Routes))

	route := vppconfig.Routes[0]
	assert.Equal(t, vppl3.Route_INTER_VRF, route.Type)
	assert.Equal(t, dstIpRouteClient, route.DstNetwork)
	assert.Equal(t, dstIpAddrClient, route.NextHopAddr)
}
