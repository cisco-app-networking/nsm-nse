// Copyright 2019 Cisco Systems, Inc.
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

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/networkservicemesh/networkservicemesh/controlplane/api/networkservice"
	"github.com/networkservicemesh/networkservicemesh/pkg/tools"
	"github.com/networkservicemesh/networkservicemesh/sdk/common"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"

	"github.com/cisco-app-networking/nsm-nse/pkg/metrics"
	"github.com/cisco-app-networking/nsm-nse/pkg/nseconfig"
	"github.com/cisco-app-networking/nsm-nse/pkg/universal-cnf/ucnf"
	"github.com/cisco-app-networking/nsm-nse/pkg/universal-cnf/vppagent"
)

const (
	metricsPortEnv     = "METRICS_PORT"
	metricsPath        = "/metrics"
	metricsPortDefault = "2112"
)

const (
	defaultConfigPath   = "/etc/universal-cnf/config.yaml"
	defaultPluginModule = ""
)

// Flags holds the command line flags as supplied with the binary invocation
type Flags struct {
	ConfigPath string
	Verify     bool
}

type fnGetNseName func() string

// Process will parse the command line flags and init the structure members
func (mf *Flags) Process() {
	flag.StringVar(&mf.ConfigPath, "file", defaultConfigPath, " full path to the configuration file")
	flag.BoolVar(&mf.Verify, "verify", false, "only verify the configuration, don't run")
	flag.Parse()
}

type passThroughCompositeEndpoint struct {
}

func (e passThroughCompositeEndpoint) AddCompositeEndpoints(nsConfig *common.NSConfiguration,
	ucnfEndpoint *nseconfig.Endpoint) *[]networkservice.NetworkServiceServer {
	logrus.WithFields(logrus.Fields{
		"prefixPool":         nsConfig.IPAddress,
		"nsConfig.IPAddress": nsConfig.IPAddress,
	}).Infof("Creating pass-through IPAM endpoint")

	var nsRemoteIpList []string
	nsRemoteIpListStr, ok := os.LookupEnv("NSM_REMOTE_NS_IP_LIST")
	if ok {
		nsRemoteIpList = strings.Split(nsRemoteIpListStr, ",")
	}

	compositeEndpoints := []networkservice.NetworkServiceServer{
		newPassThroughComposite(nsConfig, nsConfig.IPAddress, &vppagent.UniversalCNFVPPAgentBackend{}, nsRemoteIpList,
			func() string {
				return ucnfEndpoint.NseName
			}, ucnfEndpoint.PassThrough.IPAM.DefaultPrefixPool, ucnfEndpoint.Labels, ucnfEndpoint.PassThrough.Ifname),
	}

	return &compositeEndpoints
}

func InitializeMetrics() {
	metricsPort := os.Getenv(metricsPortEnv)
	if metricsPort == "" {
		metricsPort = metricsPortDefault
	}
	addr := fmt.Sprintf("0.0.0.0:%v", metricsPort)
	logrus.WithField("path", metricsPath).Infof("Serving metrics on: %v", addr)
	metrics.ServeMetrics(addr, metricsPath)
}

// exported the symbol named "CompositeEndpointPlugin"
func main() {
	// Capture signals to cleanup before exiting
	logrus.Info("starting pass-through endpoint")
	c := tools.NewOSSignalChannel()

	logrus.SetOutput(os.Stdout)
	logrus.SetLevel(logrus.TraceLevel)

	mainFlags := &Flags{}
	mainFlags.Process()

	InitializeMetrics()

	// Capture signals to cleanup before exiting
	prometheus.NewBuildInfoCollector()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	passThrough := passThroughCompositeEndpoint{}
	ucnfNse := ucnf.NewUcnfNse(mainFlags.ConfigPath, mainFlags.Verify, &vppagent.UniversalCNFVPPAgentBackend{},
		passThrough, ctx)
	logrus.Info("pass-through endpoint started")

	defer ucnfNse.Cleanup()
	<-c
}
