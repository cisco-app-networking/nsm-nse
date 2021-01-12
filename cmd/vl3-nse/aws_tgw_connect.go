package main

import (
	"context"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/sirupsen/logrus"

	"github.com/cisco-app-networking/nsm-nse/pkg/nseconfig"
	"github.com/networkservicemesh/networkservicemesh/controlplane/api/connection"
	"github.com/networkservicemesh/networkservicemesh/controlplane/api/networkservice"
	"github.com/networkservicemesh/networkservicemesh/sdk/endpoint"
)

type AwsTgwConnector struct {
	region                  	string
	ec2Svc 			        *ec2.EC2
	subnetID                	string
	transitGatewayAttachmentId	string
	transitGatewayID 		string
	transitGatewayName 		string
	vpcID                   	string
}

func newAwsTgwConnector(ate *nseconfig.AwsTgwEndpoint) *AwsTgwConnector {
	logrus.Infof("newAwsTgwConnector()")

	//if ate != nil {
		newAwsTgwConnector := &AwsTgwConnector{
			region:             "us-east-2",
			subnetID:           "subnet-04bda43175bbc7fc0",
			transitGatewayID:   "tgw-0bbdab8bc33b67ad1",
			vpcID:              "vpc-04d3edfae237d09d1",
		}

		logrus.Infof("newAwsTgwConnector returning object: %v", newAwsTgwConnector)
		return newAwsTgwConnector
	//}
	//
	//logrus.Errorf("newAwsTgwConnector(): got nil AwsTgwEndpoint, returning nil")
	//
	//return nil
}

func (atc *AwsTgwConnector) Close(ctx context.Context, conn *connection.Connection) (*empty.Empty, error) {

	logrus.Infof("AWS TGW Connector Close()")

	deleteInput := &ec2.DeleteTransitGatewayVpcAttachmentInput{
		TransitGatewayAttachmentId: &atc.transitGatewayAttachmentId,
	}

	output, err := atc.ec2Svc.DeleteTransitGatewayVpcAttachment(deleteInput)
	if err != nil {
		logrus.Errorf("Error", err)
	} else {
		logrus.Infof("Successfully deleted transit gateway attachment", output)
	}

	if endpoint.Next(ctx) != nil {
		return endpoint.Next(ctx).Close(ctx, conn)
	}
	return &empty.Empty{}, nil
}

func (atc *AwsTgwConnector) Request(ctx context.Context,
	request *networkservice.NetworkServiceRequest) (*connection.Connection, error) {

	logrus.Infof("AWS TGW Connector Request()")

	if atc != nil {
		logrus.Infof("atc.subnetID: %v", atc.subnetID)
	} else {
		logrus.Errorf("atc is nil")
	}

	if atc == nil {
		logrus.Errorf("AwsTgwConnector is nil")
		return nil, nil
	}

	sess, err := session.NewSession(&aws.Config{
		Region: aws.String("us-east-2")},
	)
	if err != nil {
		logrus.Errorf("Error getting AWS session")
		return nil, nil
	}

	// Create new EC2 client
	atc.ec2Svc = ec2.New(sess)

	input := &ec2.CreateTransitGatewayVpcAttachmentInput{
		SubnetIds: []*string{&atc.subnetID},
		TransitGatewayId: &atc.transitGatewayID,
		VpcId: &atc.vpcID,
	}

	response, err := atc.ec2Svc.CreateTransitGatewayVpcAttachment(input)
	if err != nil {
		logrus.Errorf("Error", err)
	} else {
		logrus.Infof("Successfully created AWS transit gateway vpc attachment", response)
		atc.transitGatewayAttachmentId = *response.TransitGatewayVpcAttachment.TransitGatewayAttachmentId
		logrus.Infof("Saved transit gateway attachment ID for later use: %v", atc.transitGatewayAttachmentId)
	}

	// TODO: close the session? or can it be saved and used again?

	if endpoint.Next(ctx) != nil {
		return endpoint.Next(ctx).Request(ctx, request)
	}

	return nil, nil
}
