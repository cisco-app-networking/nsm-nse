package config

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.ligato.io/vpp-agent/v3/proto/ligato/vpp"
	interfaces "go.ligato.io/vpp-agent/v3/proto/ligato/vpp/interfaces"
	l3 "go.ligato.io/vpp-agent/v3/proto/ligato/vpp/l3"

	"github.com/cisco-app-networking/nsm-nse/pkg/universal-cnf/config/mocks"
	"github.com/networkservicemesh/networkservicemesh/controlplane/api/connection"
	"github.com/networkservicemesh/networkservicemesh/controlplane/api/connectioncontext"
)

var (
	interfaces1 = []*interfaces.Interface{
		{
			IpAddresses: []string{"192.168.22.2/30"},
		},
	}

	interfaces2 = []*interfaces.Interface{
		{
			IpAddresses: []string{"192.168.22.1/30", "192.168.22.0/30"},
		},
	}

	interfaces3 = []*interfaces.Interface{
		{
			IpAddresses: []string{"192.168.22.2/30", "192.168.22.1/30", "192.168.22.0/30"},
		},
	}

	routes1 = []*l3.Route{
		{
			NextHopAddr: "192.168.22.1",
			DstNetwork:  "192.168.0.0/16",
		},
	}

	routes2 = []*l3.Route{
		{
			NextHopAddr: "192.168.22.2",
			DstNetwork:  "192.168.0.0/16",
		},
	}

	conn1 = &connection.Connection{
		Id:             "2",
		NetworkService: "ucnf",
		Mechanism: &connection.Mechanism{
			Cls:  "LOCAL",
			Type: "MEMIF",
			Parameters: map[string]string{
				"description": "NSM Endpoint",
				"name":        "nsm2DoJzgpRd",
				"netnsInode":  "4026534215",
				"socketfile":  "nsm2DoJzgpRd/memif.sock",
			},
		},
		Context: &connectioncontext.ConnectionContext{
			IpContext: &connectioncontext.IPContext{
				SrcIpAddr:     "192.168.22.1/30",
				DstIpAddr:     "192.168.22.2/30",
				SrcIpRequired: true,
				DstIpRequired: true,
				DstRoutes: []*connectioncontext.Route{
					{Prefix: "192.168.0.0/16"},
				},
			},
		},
		Labels: map[string]string{
			"namespace": "nsm-system",
			"podName":   "helloworld-ucnf-6454c88c5f-pw98x",
		},
		Path: &connection.Path{
			PathSegments: []*connection.PathSegment{
				{
					Name: "kind-2-control-plane",
				},
			},
		},
	}

	//Changed DstIpAddr
	conn2 = &connection.Connection{
		Id:             "2",
		NetworkService: "ucnf",
		Mechanism: &connection.Mechanism{
			Cls:  "LOCAL",
			Type: "MEMIF",
			Parameters: map[string]string{
				"description": "NSM Endpoint",
				"name":        "nsm2DoJzgpRd",
				"netnsInode":  "4026534215",
				"socketfile":  "nsm2DoJzgpRd/memif.sock",
			},
		},
		Context: &connectioncontext.ConnectionContext{
			IpContext: &connectioncontext.IPContext{
				SrcIpAddr:     "192.168.22.1/30",
				DstIpAddr:     "192.168.22.3/30",
				SrcIpRequired: true,
				DstIpRequired: true,
				DstRoutes: []*connectioncontext.Route{
					{Prefix: "192.168.0.0/16"},
				},
			},
		},
		Labels: map[string]string{
			"namespace": "nsm-system",
			"podName":   "helloworld-ucnf-6454c88c5f-pw98x",
		},
		Path: &connection.Path{
			PathSegments: []*connection.PathSegment{
				{
					Name: "kind-2-control-plane",
				},
			},
		},
	}

	//With SrcRoutes
	conn3 = &connection.Connection{
		Id:             "2",
		NetworkService: "ucnf",
		Mechanism: &connection.Mechanism{
			Cls:  "LOCAL",
			Type: "MEMIF",
			Parameters: map[string]string{
				"description": "NSM Endpoint",
				"name":        "nsm2DoJzgpRd",
				"netnsInode":  "4026534215",
				"socketfile":  "nsm2DoJzgpRd/memif.sock",
			},
		},
		Context: &connectioncontext.ConnectionContext{
			IpContext: &connectioncontext.IPContext{
				SrcIpAddr:     "192.168.22.1/30",
				DstIpAddr:     "192.168.22.2/30",
				SrcIpRequired: true,
				DstIpRequired: true,
				DstRoutes: []*connectioncontext.Route{
					{Prefix: "192.168.0.0/16"},
				},
				SrcRoutes: []*connectioncontext.Route{
					{Prefix: "192.168.0.0/16"},
				},
			},
		},
		Labels: map[string]string{
			"namespace": "nsm-system",
			"podName":   "helloworld-ucnf-6454c88c5f-pw98x",
		},
		Path: &connection.Path{
			PathSegments: []*connection.PathSegment{
				{
					Name: "kind-2-control-plane",
				},
			},
		},
	}
)

func TestRemoveClientInterface(t *testing.T) {
	for testName, c := range map[string]struct {
		interfaces    []*interfaces.Interface
		conn          *connection.Connection
		routes        []*l3.Route
		expectedError error
	}{
		"Remove from nil interface": {
			nil,
			conn1,
			routes1,
			fmt.Errorf("client interface with dstIpAddr %s not found", conn1.Context.IpContext.DstIpAddr),
		},
		"Remove from empty interface": {
			[]*interfaces.Interface{},
			conn1,
			routes1,
			fmt.Errorf("client interface with dstIpAddr %s not found", conn1.Context.IpContext.DstIpAddr),
		},
		"Remove from one interface": {
			interfaces1,
			conn1,
			routes1,
			nil,
		},
		"Remove from multiple interfaces": {
			interfaces3,
			conn1,
			routes1,
			nil,
		},
		"Unmatched NextHopAddr": {
			interfaces1,
			conn1,
			routes2,
			nil,
		},
		"Route prefix matches DstNetwork": {
			interfaces1,
			conn3,
			routes1,
			nil,
		},
		"Unmatched connection dstIpAddr": {
			interfaces1,
			conn2,
			routes1,
			fmt.Errorf("client interface with dstIpAddr %s not found", conn1.Context.IpContext.DstIpAddr),
		},
		"Unfound dstIpAddr in interfaces": {
			interfaces2,
			conn1,
			routes1,
			fmt.Errorf("client interface with dstIpAddr %s not found", conn1.Context.IpContext.DstIpAddr),
		},
	} {
		t.Logf("Running test case: %s", testName)

		ucnfBackend := &mocks.UniversalCNFBackend{}
		uce := &UniversalCNFEndpoint{
			backend:  ucnfBackend,
			dpConfig: &vpp.ConfigData{Interfaces: c.interfaces, Routes: c.routes},
		}

		removeConfig, err := uce.removeClientInterface(c.conn)
		if c.expectedError != nil {
			assert.NotNil(t, err)
		} else {
			assert.Nil(t, err)
			assert.NotNil(t, removeConfig)
			assert.Equal(t, 1, len(removeConfig.Interfaces))
		}
	}
}
