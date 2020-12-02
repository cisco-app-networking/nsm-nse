package main

import (
	"context"
	"github.com/cisco-app-networking/nsm-nse/pkg/nseconfig"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/networkservicemesh/networkservicemesh/controlplane/api/connection"
	"github.com/networkservicemesh/networkservicemesh/controlplane/api/networkservice"
	"github.com/networkservicemesh/networkservicemesh/sdk/endpoint"
	"github.com/sirupsen/logrus"
)

type AwsTgwConnector struct {
	TransitGatewayID 	string
	TransitGatewayName 	string
}

func newAwsTgwConnector(ate *nseconfig.AwsTgwEndpoint) *AwsTgwConnector {
	logrus.Infof("newAwsTgwConnector()")

	if ate != nil {
		newAwsTgwConnector := &AwsTgwConnector{
			TransitGatewayID:   ate.TransitGatewayID,
			TransitGatewayName: ate.TransitGatewayName,
		}

		logrus.Infof("newAwsTgwConnector returning object: %v", newAwsTgwConnector)
		return newAwsTgwConnector
	}

	logrus.Infof("newAwsTgwConnector returning")

	return nil
}

func (atc *AwsTgwConnector) Close(ctx context.Context, conn *connection.Connection) (*empty.Empty, error) {

	logrus.Infof("AWS TGW Connector Close()")

	if endpoint.Next(ctx) != nil {
		return endpoint.Next(ctx).Close(ctx, conn)
	}
	return &empty.Empty{}, nil
}

func (atc *AwsTgwConnector) Request(ctx context.Context,
	request *networkservice.NetworkServiceRequest) (*connection.Connection, error) {

	logrus.Infof("AWS TGW Connector Request()")

	if endpoint.Next(ctx) != nil {
		return endpoint.Next(ctx).Request(ctx, request)
	}

	return nil, nil
}
