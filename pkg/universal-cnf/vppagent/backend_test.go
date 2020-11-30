package vppagent

import (
	"os"
	"testing"
	"github.com/stretchr/testify/assert"

	"go.ligato.io/vpp-agent/v3/proto/ligato/vpp"
	"github.com/networkservicemesh/networkservicemesh/controlplane/api/connection"
	connectioncontext "github.com/networkservicemesh/networkservicemesh/controlplane/api/connectioncontext"
	"github.com/networkservicemesh/networkservicemesh/sdk/common"
	interfaces "go.ligato.io/vpp-agent/v3/proto/ligato/vpp/interfaces"
	vpp_l3 "go.ligato.io/vpp-agent/v3/proto/ligato/vpp/l3"
)

func TestBuildVppIfNameNSCPeer(t *testing.T) {

	b := UniversalCNFVPPAgentBackend{}
	podName := "helloworld-ucnf-78956df47b-r5fj4"
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
	podName := "helloworld-ucnf-78956df47b-r5fj4"
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

	serviceName := "ucnf"
	defaultIfName := "endpoint0"

	b := UniversalCNFVPPAgentBackend{
		EndpointIfID: map[string]int {},
	}

	conn := &connection.Connection{}

	expected := defaultIfName + "/0"
	got := b.buildVppIfName(defaultIfName, serviceName, conn)

	assert.Equal(t, expected, got)
}

func TestProcessEndpoint(t *testing.T) {

	const (
		srcIpAddr = "192.168.22.1"
		srcIpRoute = "192.168.0.1/16"
		workspaceEnv = "workspace"
	)

	podName := "helloworld-ucnf-78956df47b-r5fj4"
	b := UniversalCNFVPPAgentBackend{}
	vppconfig := &vpp.ConfigData{}
	serviceName := "ucnf"
	ifName := "endpoint0"
	conn := &connection.Connection{
		Context: &connectioncontext.ConnectionContext{
			IpContext: &connectioncontext.IPContext{
				SrcIpAddr: srcIpAddr + "/30",
				SrcIpRequired: true,
				DstIpRequired: true,
				SrcRoutes: []*connectioncontext.Route{
					&connectioncontext.Route{Prefix: srcIpRoute},
				},
			},
		},
		Labels: map[string]string {
			"podName": podName,
		},
		Mechanism: &connection.Mechanism{
			Type: "MEMIF",
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
	assert.Equal(t, vpp_l3.Route_INTER_VRF, route.Type)
	assert.Equal(t, srcIpRoute, route.DstNetwork)
	assert.Equal(t, srcIpAddr, route.NextHopAddr)
}

func TestProcessClient(t *testing.T) {
	const (
		dstIpAddr = "192.168.22.2"
		dstIpRoute =  "192.168.0.0/16"
		workspaceEnv = "workspace"
	)

	b := UniversalCNFVPPAgentBackend{}
	vppconfig := &vpp.ConfigData{}
	ifName := "endpoint0"
	conn := &connection.Connection{
		Context: &connectioncontext.ConnectionContext{
			IpContext: &connectioncontext.IPContext{
				DstIpAddr: dstIpAddr +"/30",
				SrcIpRequired: true,
				DstIpRequired: true,
				DstRoutes: []*connectioncontext.Route{
					&connectioncontext.Route{Prefix: dstIpRoute},
				},
			},
		},
		Mechanism: &connection.Mechanism{
			Type: "MEMIF",
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
	assert.Equal(t, vpp_l3.Route_INTER_VRF, route.Type)
	assert.Equal(t, dstIpRoute, route.DstNetwork)
	assert.Equal(t, dstIpAddr, route.NextHopAddr)
}
