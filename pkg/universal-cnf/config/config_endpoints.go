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
	"net"
	"os"
	"strconv"

	"github.com/cisco-app-networking/nsm-nse/pkg/nseconfig"
	"github.com/networkservicemesh/networkservicemesh/controlplane/api/connection/mechanisms/memif"
	"github.com/networkservicemesh/networkservicemesh/controlplane/api/networkservice"

	"github.com/networkservicemesh/networkservicemesh/sdk/common"
	"github.com/networkservicemesh/networkservicemesh/sdk/endpoint"
	"github.com/sirupsen/logrus"
)

// SingleEndpoint keeps the state of a single endpoint instance
type SingleEndpoint struct {
	NSConfiguration *common.NSConfiguration
	NSComposite     networkservice.NetworkServiceServer
	Endpoint        *nseconfig.Endpoint
	Cleanup         func()
}

// ProcessEndpoints keeps the state of the running network service endpoints
type ProcessEndpoints struct {
	Endpoints []*SingleEndpoint
}

type CompositeEndpointAddons interface {
	AddCompositeEndpoints(*common.NSConfiguration, *nseconfig.Endpoint) *[]networkservice.NetworkServiceServer
}

func buildIpPrefixFromLocal(defaultPrefixPool string) string {
	nsPodIp, ok := os.LookupEnv("NSE_POD_IP")
	if !ok {
		nsPodIp = "2.2.20.0" // needs to be set to make sense
	}
	ipamUseNsPodOctet := false
	nseUniqueOctet, ok := os.LookupEnv("NSE_IPAM_UNIQUE_OCTET")
	if !ok {
		ipamUseNsPodOctet = true
	}
	prefixPool := defaultPrefixPool
	logrus.Infof("IPAM local calc--default prefix pool IP %s",
		defaultPrefixPool)
	// Find the 3rd octet of the pod IP
	if defaultPrefixPool != "" {
		prefixPoolIP, defaultPrefixIPNet, err := net.ParseCIDR(defaultPrefixPool)
		if err != nil {
			logrus.Errorf("Failed to parse configured prefix pool IP")
			prefixPoolIP = net.ParseIP("1.1.0.0")
		}
		defaultPrefixMaskOnes, _ := defaultPrefixIPNet.Mask.Size()
		logrus.Infof("IPAM local calc--default prefix pool IP %s (len = %d)",
			defaultPrefixPool, defaultPrefixMaskOnes)
		// this default IPAM pool logic assumes ipv4
		if defaultPrefixMaskOnes <= 16 {
			var ipamUniqueOctet int
			if ipamUseNsPodOctet {
				podIP := net.ParseIP(nsPodIp)
				if podIP == nil {
					logrus.Errorf("Failed to parse configured pod IP")
					ipamUniqueOctet = 0
				} else {
					ipamUniqueOctet = int(podIP.To4()[2])
				}
			} else {
				ipamUniqueOctet, _ = strconv.Atoi(nseUniqueOctet)
			}
			prefixPool = fmt.Sprintf("%d.%d.%d.%d/24",
				prefixPoolIP.To4()[0],
				prefixPoolIP.To4()[1],
				ipamUniqueOctet,
				0)
		}
	}
	return prefixPool
}

// NewProcessEndpoints returns a new ProcessInitCommands struct
func NewProcessEndpoints(backend UniversalCNFBackend, endpoints []*nseconfig.Endpoint, nsconfig *common.NSConfiguration, ceAddons CompositeEndpointAddons, ctx context.Context) *ProcessEndpoints {
	result := &ProcessEndpoints{}

	for _, e := range endpoints {
		endpointLabels := e.Labels
		endpointLabels[PodName] = GetEndpointName()

		configuration := &common.NSConfiguration{
			NsmServerSocket:        nsconfig.NsmServerSocket,
			NsmClientSocket:        nsconfig.NsmClientSocket,
			Workspace:              nsconfig.Workspace,
			EndpointNetworkService: e.Name,
			ClientNetworkService:   nsconfig.ClientNetworkService,
			EndpointLabels:         labelStringFromMap(endpointLabels),
			ClientLabels:           nsconfig.ClientLabels,
			MechanismType:          memif.MECHANISM,
			IPAddress:              "",
			Routes:                 nil,
		}
		if e.VL3.IPAM.ServerAddress != "" {
			var err error
			ipamService, err := NewIpamService(ctx, e.VL3.IPAM.ServerAddress)
			if err != nil {
				logrus.Warningf("Unable to connect to IPAM Service %v",err)
			} else {
				configuration.IPAddress, err = ipamService.AllocateSubnet(e)
				if err != nil {
					logrus.Warningf("Unable to allocate subnet from IPAM Service %v", err)
					configuration.IPAddress = ""
				} else {
					logrus.Infof("Obtained subnet from IPAM Service: %s", configuration.IPAddress)
				}
			}
		}
		if configuration.IPAddress == "" {
			// central ipam server address is not set so attempt a local calculation of IPAM subnet
			configuration.IPAddress = buildIpPrefixFromLocal(e.VL3.IPAM.DefaultPrefixPool)
			logrus.Infof("Using locally calculated subnet for IPAM: %s", configuration.IPAddress)
		}
		// Build the list of composites
		compositeEndpoints := []networkservice.NetworkServiceServer{
			endpoint.NewMonitorEndpoint(configuration),
			endpoint.NewConnectionEndpoint(configuration),
		}

		if configuration.IPAddress != "" {
			compositeEndpoints = append(compositeEndpoints, endpoint.NewIpamEndpoint(&common.NSConfiguration{
				NsmServerSocket:        nsconfig.NsmServerSocket,
				NsmClientSocket:        nsconfig.NsmClientSocket,
				Workspace:              nsconfig.Workspace,
				EndpointNetworkService: nsconfig.EndpointNetworkService,
				ClientNetworkService:   nsconfig.ClientNetworkService,
				EndpointLabels:         nsconfig.EndpointLabels,
				ClientLabels:           nsconfig.ClientLabels,
				MechanismType:          nsconfig.MechanismType,
				IPAddress:              configuration.IPAddress,
				Routes:                 nil,
			}))
		}
		// Invoke any additional composite endpoint constructors via the add-on interface
		addCompositeEndpoints := ceAddons.AddCompositeEndpoints(configuration, e)
		if addCompositeEndpoints != nil {
			compositeEndpoints = append(compositeEndpoints, *addCompositeEndpoints...)
		}

		if len(e.VL3.IPAM.Routes) > 0 {
			routeAddr := makeRouteMutator(e.VL3.IPAM.Routes)
			compositeEndpoints = append(compositeEndpoints, endpoint.NewCustomFuncEndpoint("route", routeAddr))
		}
		if len(e.VL3.NameServers) > 0 {
			dnsServers := makeDnsMutator(e.VL3.DNSZones, e.VL3.NameServers)
			compositeEndpoints = append(compositeEndpoints, endpoint.NewCustomFuncEndpoint("dns", dnsServers))
		}

		compositeEndpoints = append(compositeEndpoints, NewUniversalCNFEndpoint(backend, e))
		// Compose the Endpoint
		composite := endpoint.NewCompositeEndpoint(compositeEndpoints...)

		result.Endpoints = append(result.Endpoints, &SingleEndpoint{
			NSConfiguration: configuration,
			NSComposite:     composite,
			Endpoint:        e,
		})
	}

	return result
}

// Process iterates over the init commands and applies them
func (pe *ProcessEndpoints) Process() error {
	for _, e := range pe.Endpoints {
		nsEndpoint, err := endpoint.NewNSMEndpoint(context.TODO(), e.NSConfiguration, e.NSComposite)
		if err != nil {
			logrus.Fatalf("%v", err)
			return err
		}

		_ = nsEndpoint.Start()
		e.Endpoint.NseName = nsEndpoint.GetName()
		logrus.Infof("Started endpoint %s", nsEndpoint.GetName())
		e.Cleanup = func() { _ = nsEndpoint.Delete() }
	}

	return nil
}

// Cleanup - cleans up before exit
func (pe *ProcessEndpoints) Cleanup() {
	for _, e := range pe.Endpoints {
		e.Cleanup()
	}
}
