package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cisco-app-networking/nsm-nse/pkg/nseconfig"
	"github.com/networkservicemesh/networkservicemesh/controlplane/api/connection"
	"github.com/networkservicemesh/networkservicemesh/controlplane/api/networkservice"
)

func TestNewAwsTgwConnector(t *testing.T) {
	const (
		TransitGatewayID = "1"
		TransitGatewayName = "test"
	)

	ate := nseconfig.AwsTgwEndpoint{
		TransitGatewayID: TransitGatewayID,
		TransitGatewayName: TransitGatewayName,
	}

	expected := &AwsTgwConnector{
		TransitGatewayID: TransitGatewayID,
		TransitGatewayName: TransitGatewayName,
	}

	got := newAwsTgwConnector(&ate)

	assert.Equal(t, expected, got)
}

func TestNewAwsTgwConnectorNil(t *testing.T) {

	got := newAwsTgwConnector(nil)

	assert.Nil(t, got)
}

func TestAwsTgwConnector_Request(t *testing.T) {
	atc := AwsTgwConnector{}
	ctx := context.Background()
	request := &networkservice.NetworkServiceRequest{}

	_, err := atc.Request(ctx, request)

	assert.Nil(t, err)
}

func TestAwsTgwConnector_Close(t *testing.T) {
	atc := AwsTgwConnector{}
	ctx := context.Background()
	conn := &connection.Connection{}

	_, err := atc.Close(ctx, conn)

	assert.Nil(t, err)
}
