// Copyright 2019 VMware, Inc.
// SPDX-License-Identifier: Apache-2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package vppagent

import (
	"fmt"
	"net"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/networkservicemesh/networkservicemesh/controlplane/api/connection"
	"github.com/networkservicemesh/networkservicemesh/controlplane/api/connection/mechanisms/memif"
	"github.com/sirupsen/logrus"
	"go.ligato.io/vpp-agent/v3/proto/ligato/vpp"
	interfaces "go.ligato.io/vpp-agent/v3/proto/ligato/vpp/interfaces"
	vpp_l3 "go.ligato.io/vpp-agent/v3/proto/ligato/vpp/l3"
	vpp_nat "go.ligato.io/vpp-agent/v3/proto/ligato/vpp/nat"
)

// UniversalCNFVPPAgentBackend is the VPP CNF backend struct
type UniversalCNFVPPAgentBackend struct {
	EndpointIfID map[string]int
}

// NewDPConfig returns a plain DPConfig struct
func (b *UniversalCNFVPPAgentBackend) NewDPConfig() *vpp.ConfigData {
	return &vpp.ConfigData{}
}

// NewUniversalCNFBackend initializes the VPP CNF backend
func (b *UniversalCNFVPPAgentBackend) NewUniversalCNFBackend() error {
	b.EndpointIfID = make(map[string]int)

	if err := ResetVppAgent(); err != nil {
		logrus.Fatalf("Error resetting vpp: %v", err)
	}

	return nil
}

// ProcessClient runs the client code for VPP CNF
func (b *UniversalCNFVPPAgentBackend) ProcessClient(
	dpconfig interface{}, ifName string, conn *connection.Connection) error {
	vppconfig, ok := dpconfig.(*vpp.ConfigData)
	if !ok {
		return fmt.Errorf("unable to convert dpconfig to vppconfig	")
	}

	srcIP := conn.GetContext().GetIpContext().GetSrcIpAddr()
	dstIP, _, _ := net.ParseCIDR(conn.GetContext().GetIpContext().GetDstIpAddr())
	socketFilename := path.Join(getBaseDir(), memif.ToMechanism(conn.GetMechanism()).GetSocketFilename())

	ipAddresses := []string{}
	if len(srcIP) > net.IPv4len {
		ipAddresses = append(ipAddresses, srcIP)
	}

	vppconfig.Interfaces = append(vppconfig.Interfaces,
		&interfaces.Interface{
			Name:        ifName,
			Type:        interfaces.Interface_MEMIF,
			Enabled:     true,
			IpAddresses: ipAddresses,
			Link: &interfaces.Interface_Memif{
				Memif: &interfaces.MemifLink{
					Master:         false, // The client is not the master in MEMIF
					SocketFilename: socketFilename,
					RingSize:       512,
				},
			},
		})

	// Process static routes
	for _, route := range conn.GetContext().GetIpContext().GetDstRoutes() {
		route := &vpp.Route{
			Type:        vpp_l3.Route_INTER_VRF,
			DstNetwork:  route.Prefix,
			NextHopAddr: dstIP.String(),
		}
		vppconfig.Routes = append(vppconfig.Routes, route)
	}

	return nil
}

// ProcessEndpoint runs the endpoint code for VPP CNF
func (b *UniversalCNFVPPAgentBackend) ProcessEndpoint(
	dpconfig interface{}, serviceName, ifName string, conn *connection.Connection) error {
	vppconfig, ok := dpconfig.(*vpp.ConfigData)
	if !ok {
		return fmt.Errorf("unable to convert dpconfig to vppconfig	")
	}

	srcIP, _, _ := net.ParseCIDR(conn.GetContext().GetIpContext().GetSrcIpAddr())
	dstIP := conn.GetContext().GetIpContext().GetDstIpAddr()
	socketFilename := path.Join(getBaseDir(), memif.ToMechanism(conn.GetMechanism()).GetSocketFilename())

	ipAddresses := []string{}
	if len(dstIP) > net.IPv4len {
		ipAddresses = append(ipAddresses, dstIP)
	}

	endpointIfName := ifName + b.GetEndpointIfID(serviceName)
	rxModes := []*interfaces.Interface_RxMode{
		&interfaces.Interface_RxMode{
			Mode:        interfaces.Interface_RxMode_INTERRUPT,
			DefaultMode: true,
		},
	}

	vppconfig.Interfaces = append(vppconfig.Interfaces,
		&interfaces.Interface{
			Name:        endpointIfName,
			Type:        interfaces.Interface_MEMIF,
			Enabled:     true,
			IpAddresses: ipAddresses,
			Link: &interfaces.Interface_Memif{
				Memif: &interfaces.MemifLink{
					Master:         true, // The endpoint is always the master in MEMIF
					SocketFilename: socketFilename,
					RingSize:       512,
				},
			},
			RxModes: rxModes,
		})

	if err := os.MkdirAll(path.Dir(socketFilename), os.ModePerm); err != nil {
		return err
	}

	// Process static routes
	for _, route := range conn.GetContext().GetIpContext().GetSrcRoutes() {
		route := &vpp.Route{
			Type:        vpp_l3.Route_INTER_VRF,
			DstNetwork:  route.Prefix,
			NextHopAddr: srcIP.String(),
		}
		vppconfig.Routes = append(vppconfig.Routes, route)
	}

	// NAT configuration
	if natIP := os.Getenv("NSE_NAT_IP"); natIP != "" {
		// configure NAT pool (only once) - TODO: move pool config to some global init place?
		if b.EndpointIfID[serviceName] == 0 {
			natPool := &vpp_nat.Nat44AddressPool{FirstIp: natIP}
			vppconfig.Nat44Pools = append(vppconfig.Nat44Pools, natPool)
		}

		// enable NAT on the interface
		natIf := &vpp_nat.Nat44Interface{
			Name:      endpointIfName,
			NatInside: true,
		}
		vppconfig.Nat44Interfaces = append(vppconfig.Nat44Interfaces, natIf)

		// add static NAT mappings for port forward requests
		for k, v := range conn.Labels {
			if strings.HasPrefix(k, "nat-port-forward") {
				port, err := strconv.Atoi(v)
				if err != nil {
					logrus.Errorf("cannot convert port number %s: %v", v, err)
					continue
				}
				natMapping := &vpp_nat.DNat44{
					Label: k + "-to-" + srcIP.String(),
					StMappings: []*vpp_nat.DNat44_StaticMapping{
						{
							ExternalIp:   natIP,
							ExternalPort: uint32(port),
							LocalIps: []*vpp_nat.DNat44_StaticMapping_LocalIP{
								{
									LocalIp:   srcIP.String(),
									LocalPort: uint32(port),
								},
							},
						},
					},
				}
				if strings.Contains(k, "udp") {
					natMapping.StMappings[0].Protocol = vpp_nat.DNat44_UDP
				} else {
					natMapping.StMappings[0].Protocol = vpp_nat.DNat44_TCP
				}
				vppconfig.Dnat44S = append(vppconfig.Dnat44S, natMapping)
			}
		}
	}

	return nil
}

// GetEndpointIfID generates a new interface ID from the service name
func (b *UniversalCNFVPPAgentBackend) GetEndpointIfID(serviceName string) string {
	if _, ok := b.EndpointIfID[serviceName]; !ok {
		b.EndpointIfID[serviceName] = 0
	} else {
		b.EndpointIfID[serviceName]++
	}

	return "/" + strconv.Itoa(b.EndpointIfID[serviceName])
}

// ProcessDPConfig applies the VPP CNF configuration to VPP
func (b *UniversalCNFVPPAgentBackend) ProcessDPConfig(dpconfig interface{}, createOrUpdate bool) error {
	vppconfig, ok := dpconfig.(*vpp.ConfigData)
	if !ok {
		return fmt.Errorf("unable to convert dpconfig to vppconfig	")
	}

	if !createOrUpdate {
		logrus.Println("[COSMIN] ProcessDPConfig. dpconfig: interfaces")
		for _, inter := range vppconfig.Interfaces {
			logrus.Println("[COSMIN] interface:", inter)
		}
	}

	err := SendVppConfigToVppAgent(vppconfig, createOrUpdate)

	if err != nil {
		logrus.Errorf("Updating the VPP config failed with: %v", err)
	}

	return err
}
