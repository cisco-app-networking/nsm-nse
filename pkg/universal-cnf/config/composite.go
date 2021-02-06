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

package config

import (
	"context"
	"fmt"
	"github.com/cisco-app-networking/nsm-nse/pkg/nseconfig"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/networkservicemesh/networkservicemesh/controlplane/api/connection"
	"github.com/networkservicemesh/networkservicemesh/controlplane/api/connectioncontext"
	"github.com/networkservicemesh/networkservicemesh/controlplane/api/networkservice"
	"github.com/networkservicemesh/networkservicemesh/sdk/endpoint"
	"github.com/sirupsen/logrus"
	"go.ligato.io/vpp-agent/v3/proto/ligato/vpp"
	"go.ligato.io/vpp-agent/v3/proto/ligato/vpp/interfaces"
	l3 "go.ligato.io/vpp-agent/v3/proto/ligato/vpp/l3"
	"net"
	"os"
)

// UniversalCNFEndpoint is a Universal CNF Endpoint composite implementation
type UniversalCNFEndpoint struct {
	//endpoint.BaseCompositeEndpoint
	endpoint *nseconfig.Endpoint
	backend  UniversalCNFBackend
	dpConfig *vpp.ConfigData
}

// Request implements the request handler
func (uce *UniversalCNFEndpoint) Request(ctx context.Context,
	request *networkservice.NetworkServiceRequest) (*connection.Connection, error) {
	conn := request.GetConnection()

	if uce.dpConfig == nil {
		uce.dpConfig = uce.backend.NewDPConfig()
	}

	passThrough, ok := os.LookupEnv("PASS_THROUGH")
	if ok && passThrough == "true" {
		ifName := uce.backend.BuildVppIfName(uce.endpoint.PassThrough.Ifname, uce.endpoint.Name, conn)
		// this memif is a master since it connects with the client pod
		if err := uce.backend.CreateL2Memif(uce.dpConfig, ifName, conn, true); err != nil {
			logrus.Errorf("Failed to process: %+v", uce.endpoint)
			return nil, err
		}
	} else {
		if err := uce.backend.ProcessEndpoint(uce.dpConfig, uce.endpoint.Name, uce.endpoint.VL3.Ifname, conn); err != nil {
			logrus.Errorf("Failed to process: %+v", uce.endpoint)
			return nil, err
		}
	}

	if err := uce.backend.ProcessDPConfig(uce.dpConfig, true); err != nil {
		logrus.Errorf("Error processing dpconfig: %+v", uce.dpConfig)
		return nil, err
	}

	if endpoint.Next(ctx) != nil {
		return endpoint.Next(ctx).Request(ctx, request)
	}

	return request.GetConnection(), nil
}

// Removes the client interfaces from the dpConfig and
// Returns a new *vpp.ConfigData which contains the removed interfaces.
func (uce *UniversalCNFEndpoint) removeClientInterface(connection *connection.Connection) (*vpp.ConfigData, error) {
	dstIpAddr := connection.Context.IpContext.DstIpAddr

	// find the interface index in the dpConfig.Interface slice
	index := -1
	for i, inter := range uce.dpConfig.Interfaces {
		if index != -1 {
			break
		}
		for _, ipAddr := range inter.IpAddresses {
			if ipAddr == dstIpAddr {
				index = i
				break
			}
		}
	}

	if index == -1 {
		return nil, fmt.Errorf("client interface with dstIpAddr %s not found", dstIpAddr)
	}

	// Get a reference to the removed interface.
	inter := uce.dpConfig.Interfaces[index]

	// Remove the interface from the actual config.
	uce.dpConfig.Interfaces = append(uce.dpConfig.Interfaces[:index], uce.dpConfig.Interfaces[index+1:]...)

	// Create a new configuration used in the vpp to perform the removal.
	removeConfig := uce.backend.NewDPConfig()

	// Append the interface that has to be removed.
	removeConfig.Interfaces = append(removeConfig.Interfaces, inter)

	srcIP, _, _ := net.ParseCIDR(connection.GetContext().GetIpContext().GetSrcIpAddr())

	// Create a new Routes slice for the routes that are kept in the dpConfig
	// Create a removedRoutes slice which is going to be passed to the vpp
	var removedRoutes, newRoutes []*l3.Route
	srcRoutes := connection.GetContext().GetIpContext().GetSrcRoutes()
	for _, route := range uce.dpConfig.Routes {
		if route.NextHopAddr == srcIP.String() {
			found := false
			for _, r := range srcRoutes {
				if route.DstNetwork == r.Prefix {
					found = true
				}
			}
			if found {
				removedRoutes = append(removedRoutes, route)
			} else {
				newRoutes = append(newRoutes, route)
			}
		} else {
			newRoutes = append(newRoutes, route)
		}
	}

	// Updating the static routes configuration
	removeConfig.Routes = removedRoutes
	uce.dpConfig.Routes = newRoutes

	return removeConfig, nil
}


func (uce *UniversalCNFEndpoint) removeL2xConnInterface(conn *connection.Connection) (*vpp.ConfigData, error) {
	var slaveIfName string
	var clientIfToRm, endpointIfToRm *vpp_interfaces.Interface
	var rmInx = -1

	slaveIfName = conn.GetLabels()[connection.PodNameKey]

	for i, vppIf := range(uce.dpConfig.Interfaces) {
		if slaveIfName == vppIf.GetName(){
			endpointIfToRm = PassThroughMemifs.VppIfs[slaveIfName]
			clientIfToRm = uce.dpConfig.Interfaces[i]
			rmInx = i

			PassThroughMemifs.Lock()
			delete(PassThroughMemifs.VppIfs, slaveIfName)// delete the corresponding entry in the map
			PassThroughMemifs.Unlock()
		}
	}

	if rmInx == -1 || clientIfToRm == nil || endpointIfToRm == nil {
		return nil, fmt.Errorf("Failed to find the L2 cross connected interface pair")
	}

	// Remove the interface from the actual config.
	uce.dpConfig.Interfaces = append(uce.dpConfig.Interfaces[:rmInx], uce.dpConfig.Interfaces[rmInx+1:]...)

	// Create a new configuration used in the vpp to perform the removal.
	removeConfig := uce.backend.NewDPConfig()
	removeConfig.Interfaces = append(removeConfig.Interfaces, clientIfToRm, endpointIfToRm)

	return removeConfig, nil
}

// Close implements the close handler
func (uce *UniversalCNFEndpoint) Close(ctx context.Context, connection *connection.Connection) (*empty.Empty, error) {
	logrus.Infof("Universal CNF DeleteConnection: %v", connection)

	var removeConfig *vpp.ConfigData
	var err error

	passThrough, ok := os.LookupEnv("PASS_THROUGH")
	if ok && passThrough == "true" {
		removeConfig, err = uce.removeL2xConnInterface(connection)
	} else {
		removeConfig, err = uce.removeClientInterface(connection)
	}
	if err != nil && endpoint.Next(ctx) != nil {
		logrus.Info(err)
		return endpoint.Next(ctx).Close(ctx, connection)
	}

	// Remove the interfaces from the vpp agent
	if err := uce.backend.ProcessDPConfig(removeConfig, false); err != nil {
		logrus.Errorf("Error processing dpconfig: %+v", uce.dpConfig)
		return nil, err
	}

	if endpoint.Next(ctx) != nil {
		return endpoint.Next(ctx).Close(ctx, connection)
	}

	return &empty.Empty{}, nil
}

// Name returns the composite name
func (uce *UniversalCNFEndpoint) Name() string {
	return "Universal CNF"
}

// NewUniversalCNFEndpoint creates a MonitorEndpoint
func NewUniversalCNFEndpoint(backend UniversalCNFBackend, e *nseconfig.Endpoint) *UniversalCNFEndpoint {
	self := &UniversalCNFEndpoint{
		backend:  backend,
		endpoint: e,
	}

	return self
}

func makeRouteMutator(routes []string) endpoint.ConnectionMutator {
	return func(ctx context.Context, c *connection.Connection) error {
		for _, r := range routes {
			c.GetContext().GetIpContext().DstRoutes = append(c.GetContext().GetIpContext().DstRoutes, &connectioncontext.Route{
				Prefix: r,
			})
		}
		return nil
	}
}

func makeDnsMutator(dnsZones, nameservers []string) endpoint.ConnectionMutator {
	return func(ctx context.Context, c *connection.Connection) error {
		logrus.Infof("Universal CNF DNS composite endpoint: %v", c)
		dnsConfig := &connectioncontext.DNSConfig{}
		for _, zone := range dnsZones {
			dnsConfig.SearchDomains = append(dnsConfig.SearchDomains, zone)
		}
		for _, server := range nameservers {
			dnsConfig.DnsServerIps = append(dnsConfig.DnsServerIps, server)
		}
		logrus.Infof("Universal CNF DNS composite endpoint adding DNSConfig: %v", dnsConfig)
		if c.GetContext().GetDnsContext() == nil {
			c.GetContext().DnsContext = &connectioncontext.DNSContext{}
		}
		c.GetContext().DnsContext.Configs = append(c.GetContext().DnsContext.Configs,
			dnsConfig)
		return nil
	}
}
