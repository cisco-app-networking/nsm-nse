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
	"context"
	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	"github.com/networkservicemesh/networkservicemesh/pkg/tools"
	"github.com/opentracing/opentracing-go"
	"github.com/sirupsen/logrus"
	"go.ligato.io/vpp-agent/v3/proto/ligato/configurator"
	"go.ligato.io/vpp-agent/v3/proto/ligato/vpp"
	vpp_l2 "go.ligato.io/vpp-agent/v3/proto/ligato/vpp/l2"
	"google.golang.org/grpc"
	"time"
)

const (
	defaultVPPAgentEndpoint = "localhost:9113"
)

var (
	endpoinfIfName string
	clientIfName string
)

// ResetVppAgent resets the VPP instance settings to nil
func ResetVppAgent() error {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	if err := tools.WaitForPortAvailable(ctx, "tcp", defaultVPPAgentEndpoint, 100*time.Millisecond); err != nil {
		return err
	}

	conn, err := grpc.Dial(defaultVPPAgentEndpoint, grpc.WithInsecure())
	if err != nil {
		logrus.Errorf("can't dial grpc server: %v", err)
		return err
	}

	defer func() { _ = conn.Close() }()

	client := configurator.NewConfiguratorServiceClient(conn)

	logrus.Infof("Resetting vppagent...")

	_, err = client.Update(context.Background(), &configurator.UpdateRequest{
		Update:     &configurator.Config{},
		FullResync: true,
	})
	if err != nil {
		logrus.Errorf("failed to reset vppagent: %s", err)
	}

	logrus.Infof("Finished resetting vppagent...")

	return nil
}

// SendVppConfigToVppAgent send the update to the VPP-Agent
func SendVppConfigToVppAgent(vppconfig *vpp.ConfigData, update bool) error {
	dataChange := &configurator.Config{
		VppConfig: vppconfig,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)

	defer cancel()

	if err := tools.WaitForPortAvailable(ctx, "tcp", defaultVPPAgentEndpoint, 100*time.Millisecond); err != nil {
		return err
	}

	tracer := opentracing.GlobalTracer()
	conn, err := grpc.Dial(defaultVPPAgentEndpoint, grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(
			otgrpc.OpenTracingClientInterceptor(tracer, otgrpc.LogPayloads())),
		grpc.WithStreamInterceptor(
			otgrpc.OpenTracingStreamClientInterceptor(tracer)))

	if err != nil {
		logrus.Errorf("can't dial grpc server: %v", err)
		return err
	}

	defer func() { _ = conn.Close() }()

	client := configurator.NewConfiguratorServiceClient(conn)

	logrus.Infof("Sending DataChange to vppagent: %v", dataChange)

	if update {
		if _, err = client.Update(ctx, &configurator.UpdateRequest{
			Update: dataChange,
		}); err != nil {
			logrus.Error(err)
			_, err = client.Delete(ctx, &configurator.DeleteRequest{
				Delete: dataChange,
			})
		} else {
			// Perform memif interfaces chaining
			if vppconfig.Interfaces[0].GetMemif().Master {
				clientIfName = vppconfig.Interfaces[0].Name
			} else {
				endpoinfIfName = vppconfig.Interfaces[0].Name
			}
			if endpoinfIfName != "" && clientIfName != "" {
				logrus.Infof("DEBUGGING -- clientIf:%+v, endpoinIf:%+v", clientIfName, endpoinfIfName)
				dataChange = &configurator.Config{
					VppConfig: &vpp.ConfigData{
						XconnectPairs: []*vpp_l2.XConnectPair{
							{
								ReceiveInterface:  clientIfName,
								TransmitInterface: endpoinfIfName,
							},
							{
								ReceiveInterface:  endpoinfIfName,
								TransmitInterface: clientIfName,
							},
						},
					},
				}

				if _, err = client.Update(ctx, &configurator.UpdateRequest{
					Update: dataChange,
				}); err != nil {
					logrus.Error(err)
					_, err = client.Delete(ctx, &configurator.DeleteRequest{
						Delete: dataChange,
					});
					if err != nil {
						logrus.Error(err)
					}
					return err
				}
			}
		}
	}
	return nil
}

			//endpointMemifName := os.Getenv(config.PassThroughMemifName)
			//var clientMemifName string
			//if len(vppconfig.Interfaces) > 0{
			//	clientMemifName = vppconfig.Interfaces[len(vppconfig.Interfaces) - 1].GetName()
			//}
			//
			//if endpointMemifName != "" && endpointMemifName != clientMemifName {
			//	logrus.Infof("DEBUGGING -- Now performing chaining of the two interfaces: %s, %s", endpointMemifName,
			//		clientMemifName)
			//
			//	dataChange = &configurator.Config{
			//		VppConfig: &vpp.ConfigData{
			//			XconnectPairs: []*vpp_l2.XConnectPair{
			//				{
			//					ReceiveInterface: endpointMemifName,
			//					TransmitInterface: clientMemifName,
			//				},
			//				{
			//					ReceiveInterface: endpointMemifName,
			//					TransmitInterface: clientMemifName,
			//				},
			//			},
			//		},
			//	}
			//
			//	if _, err = client.Update(ctx, &configurator.UpdateRequest{
			//		Update: dataChange,
			//	}); err != nil {
			//		logrus.Error(err)
			//		_, err = client.Delete(ctx, &configurator.DeleteRequest{
			//			Delete: dataChange,
			//		}); if err != nil {
			//			logrus.Error(err)
			//		}
			//	}
			//}
		//}
	//} else {
	//	_, err = client.Delete(ctx, &configurator.DeleteRequest{
	//		Delete: dataChange,
	//	})
	//}
	//return err
