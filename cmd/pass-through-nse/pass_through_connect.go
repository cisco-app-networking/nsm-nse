package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/cisco-app-networking/nsm-nse/pkg/nseconfig"
	"github.com/networkservicemesh/networkservicemesh/controlplane/api/connection/mechanisms/memif"
	"github.com/networkservicemesh/networkservicemesh/sdk/endpoint"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/networkservicemesh/networkservicemesh/controlplane/api/connection"
	"github.com/networkservicemesh/networkservicemesh/controlplane/api/networkservice"
	"github.com/networkservicemesh/networkservicemesh/controlplane/api/registry"
	"github.com/networkservicemesh/networkservicemesh/pkg/tools"
	"github.com/networkservicemesh/networkservicemesh/sdk/client"
	"github.com/networkservicemesh/networkservicemesh/sdk/common"
	"github.com/sirupsen/logrus"
	"go.ligato.io/vpp-agent/v3/proto/ligato/vpp"
	"google.golang.org/grpc"

	"github.com/cisco-app-networking/nsm-nse/pkg/metrics"
	"github.com/cisco-app-networking/nsm-nse/pkg/universal-cnf/config"
)

const (
	NSREGISTRY_ADDR = "nsmgr.nsm-system"
	NSREGISTRY_PORT = "5000"
)

type passThroughComposite struct {
	sync.RWMutex
	myEndpointName    string
	nsConfig          *common.NSConfiguration
	remoteNsIpList    []string
	myEndpointLabels  nseconfig.Labels
	nsRegGrpcClient   *grpc.ClientConn
	nsDiscoveryClient registry.NetworkServiceDiscoveryClient
	nsmClient         *client.NsmClient
	backend           config.UniversalCNFBackend
	myNseNameFunc     fnGetNseName
	defaultIfName     string
	endpointIfID      map[string]int
}

func (ptxc *passThroughComposite) Request(ctx context.Context, request *networkservice.NetworkServiceRequest) (*connection.Connection, error) {

	logger := logrus.New()
	conn := request.GetConnection()
	logger.WithFields(
		logrus.Fields{
			"endpointName":              conn.GetNetworkServiceEndpointName(),
			"networkServiceManagerName": conn.GetSourceNetworkServiceManagerName(),
		},
	).Infof("passThroughComposite Request handler")

	ptxc.setMyNseName(request)

	if ptxc.nsDiscoveryClient == nil {
		logger.Error("nsDiscoveryClient is nil")
	} else {
		req := &registry.FindNetworkServiceRequest{
			NetworkServiceName: conn.GetNetworkService(),
		}

		ptxc.nsmClient.Configuration.ClientNetworkService = req.NetworkServiceName

		response, err := ptxc.nsDiscoveryClient.FindNetworkService(context.Background(), req)
		if err != nil {
			logger.Error(err)
			go func() {
				metrics.FailedFindNetworkService.Inc()
			}()
		} else {
			logger.Infof("passThroughComposite found network service for NS=%s", conn.GetNetworkService())

			if chainConn, connErr := ptxc.processNsEndpoints(context.TODO(), response, "", logger); connErr != nil {
				logrus.Error(connErr)
			} else {
				// Pass the IpContext received from the chained endpoint connection to the client request
				request.Connection.Context.IpContext = chainConn.Context.IpContext
			}
		}
	}

	/* Check for remote NS? */

	logger.Infof("passThroughComposite request done")

	if endpoint.Next(ctx) != nil {
		return endpoint.Next(ctx).Request(ctx, request)
	}

	return conn, nil
}

func (ptxc *passThroughComposite) Close(ctx context.Context, conn *connection.Connection) (*empty.Empty, error) {
	// remove from connections
	logrus.Infof("passThrough DeleteConnection: %+v", conn)

	if endpoint.Next(ctx) != nil {
		return endpoint.Next(ctx).Close(ctx, conn)
	}
	return &empty.Empty{}, nil
}

//processNsEndpoints finds the label of the chain endpoint and sends a NS request to connect with it
func (ptxc *passThroughComposite) processNsEndpoints(
	ctx context.Context,
	response *registry.FindNetworkServiceResponse,
	remoteIP string,
	logger *logrus.Logger,
) (*connection.Connection, error) {
	logger.Infof("Processing NS endpoints...")

	chainLabelKey, chainLabelVal, err := ptxc.findChainEndpointLabel(response, logger)

	if err != nil {
		return nil, err
	}

	//routes := []string{ptxc.passThroughNetCidr}
	chainConn, connErr := ptxc.createChainConnection(ctx, response, logger, remoteIP, chainLabelKey, chainLabelVal)

	if connErr != nil {
		return nil, connErr
	}

	return chainConn, nil
}

// findChainEndpointLabel finds the label for the chain endpoint
func (ptxc *passThroughComposite) findChainEndpointLabel(
	response *registry.FindNetworkServiceResponse,
	logger *logrus.Logger,
) (string, string, error) {
	var chainLabelKey, chainLabelVal string

	// There must be >= 2 matches under the networkservice to perform NSE chaining:
	// 1 match for current endpoint, the other match(es) for the chaining NSE
	matches := response.GetNetworkService().GetMatches()
	if len(matches) < 2 {
		return "", "", fmt.Errorf("Failed to find chain NSE: current networkservice has %d match", len(matches))
	}

	logger.Infof("The matches under current networkservice are: %v", matches)
	// An endpoint has at least 1 label for it to be found by NSMgr
	for labelKey, labelVal := range ptxc.myEndpointLabels {
		for _, match := range matches {
			// First find the match that has the same label as the current endpoint
			if val, ok := match.SourceSelector[labelKey]; ok && val == labelVal {
				// There should be only one route and one destination since one client only connects to one endpoint under the same networkservice
				routesLen := len(match.GetRoutes())
				if routesLen != 1 {
					return "", "", fmt.Errorf("Failed to find chain NSE label -- found %d routes under current match", routesLen)
				}

				destinationLen := len(match.GetRoutes()[0].GetDestinationSelector())
				if destinationLen != 1 {
					return "", "", fmt.Errorf(
						"Failed to find chain NSE label -- found %d destination selectors under current match",
						destinationLen,
					)
				}

				// Get the destination label key and value
				for chainLabelKey, chainLabelVal = range match.GetRoutes()[0].GetDestinationSelector() {
				}
				break
			}
		}
	}
	if chainLabelKey == "" || chainLabelVal == "" {
		return "", "", fmt.Errorf("Failed to find chain NSE label: the chain NSE label is empty")
	}
	return chainLabelKey, chainLabelVal, nil
}

// createChainConnection first finds the chain endpoint using the label, then creates connection with it
func (ptxc *passThroughComposite) createChainConnection(
	ctx context.Context,
	response *registry.FindNetworkServiceResponse,
	logger *logrus.Logger,
	remoteIP,
	chainLabelKey,
	chainLabelVal string,
) (*connection.Connection, error) {

	var chainEndpoint *registry.NetworkServiceEndpoint

	// Loop through all the endpoints under the current networkservice to find the endpoint that matches the chain endpoint label
	for _, nsEndpoint := range response.GetNetworkServiceEndpoints() {
		for eLabelKey, eLabelVal := range nsEndpoint.Labels {
			if eLabelKey == chainLabelKey && eLabelVal == chainLabelVal {
				chainEndpoint = nsEndpoint
			}
		}
	}

	if chainEndpoint == nil {
		return nil, fmt.Errorf("Unable to find the chain NSE under this networkservice")
	}

	logger.Infof("The chain NSE is found -- %s, now trying to connect...", chainEndpoint.GetName())

	dpConfig := &vpp.ConfigData{}
	conn, connErr := ptxc.performChainConnectionRequest(ctx, chainEndpoint, dpConfig, logger, remoteIP)

	if connErr != nil {
		logger.Errorf("NSE chain connection failed -- %v", connErr)
		return nil, connErr
	}

	if connErr = ptxc.backend.ProcessDPConfig(dpConfig, true); connErr != nil {
		logger.Errorf("Endpoint %s processing dpconfig: %+v with error -- %v", chainEndpoint.GetName(), dpConfig, connErr)
		return nil, connErr
	}

	logger.Infof("Succefully connected to the chain endpoint -- %s", chainEndpoint.GetName())

	return conn, nil

}

// performChainConnectionRequest sends the connection request to the chained endpoint and the request to create vpp interface for layer 2 MEMIF
func (ptxc *passThroughComposite) performChainConnectionRequest(
	ctx context.Context,
	nsEndpoint *registry.NetworkServiceEndpoint,
	dpconfig interface{},
	logger *logrus.Logger,
	remoteIP string,
) (*connection.Connection, error) {

	logger.Infof("Performing connect to the chain endpoint -- %s", nsEndpoint.GetName())

	go func() {
		metrics.PerormedConnRequests.Inc()
	}()

	ifName := ptxc.buildVppIfName(ptxc.defaultIfName, nsEndpoint.GetName())

	conn, err := ptxc.nsmClient.ConnectToEndpoint(
		ctx,
		remoteIP,
		nsEndpoint.GetName(),
		nsEndpoint.GetNetworkServiceManagerName(),
		ifName,
		memif.MECHANISM,
		"VPP interface "+ifName,
		ptxc.nsmClient.Configuration.Routes,
	)

	if err != nil {
		logger.Errorf("Error creating chain connection: %v", err)
		return nil, err
	}

	logrus.Infof("The chain connection is successfully built -- %+v", conn)

	// This is not a master interface because it is acting as a client to the chain endpoint
	memifMaster := false
	err = ptxc.backend.CreateL2Memif(dpconfig, ifName, conn, memifMaster)
	if err != nil {
		logger.Errorf("Error creating MEMIF for %s: %v", ifName, err)
		return nil, err
	}
	return conn, nil
}

// newPassThroughComposite creates a new passThrough composite
func newPassThroughComposite(
	configuration *common.NSConfiguration,
	backend config.UniversalCNFBackend,
	remoteIpList []string,
	getNseName fnGetNseName,
	labels nseconfig.Labels,
	ifName string,
) *passThroughComposite {

	nsRegAddr, ok := os.LookupEnv("NSREGISTRY_ADDR")
	if !ok {
		nsRegAddr = NSREGISTRY_ADDR
	}
	nsRegPort, ok := os.LookupEnv("NSREGISTRY_PORT")
	if !ok {
		nsRegPort = NSREGISTRY_PORT
	}

	// ensure the env variables are processed
	if configuration == nil {
		configuration = &common.NSConfiguration{}
		configuration.FromEnv()
	}

	var nsDiscoveryClient registry.NetworkServiceDiscoveryClient

	nsRegGrpcClient, err := tools.DialTCP(nsRegAddr + ":" + nsRegPort)
	if err != nil {
		logrus.Errorf("nsmRegistryConnection GRPC Client Socket Error: %v", err)
	} else {
		logrus.Infof("newPassThroughComposite socket operation ok... create networkDiscoveryClient")
		nsDiscoveryClient = registry.NewNetworkServiceDiscoveryClient(nsRegGrpcClient)
		if nsDiscoveryClient == nil {
			logrus.Errorf("newPassThroughComposite networkDiscoveryClient nil")
		} else {
			logrus.Infof("newPassThroughComposite networkDiscoveryClient ok")
		}
	}

	nsConfig := configuration
	nsConfig.ClientLabels = ""
	var nsmClient *client.NsmClient
	nsmClient, err = client.NewNSMClient(context.TODO(), nsConfig)
	if err != nil {
		logrus.Errorf("Unable to create the NSM client %v", err)
	}

	newPassThroughComposite := &passThroughComposite{
		nsConfig:          configuration,
		remoteNsIpList:    remoteIpList,
		myEndpointName:    "",
		nsRegGrpcClient:   nsRegGrpcClient,
		nsDiscoveryClient: nsDiscoveryClient,
		nsmClient:         nsmClient,
		backend:           backend,
		myNseNameFunc:     getNseName,
		myEndpointLabels:  labels,
		defaultIfName:     ifName,
		endpointIfID:      make(map[string]int),
	}

	return newPassThroughComposite
}

// setMyNseName is a helper function that sets the passThrough composite endpoint name
func (ptxc *passThroughComposite) setMyNseName(request *networkservice.NetworkServiceRequest) {
	ptxc.Lock()
	defer ptxc.Unlock()

	if ptxc.myEndpointName == "" {
		nseName := ptxc.myNseNameFunc()
		logrus.Infof(
			"Setting passThrough composite endpoint name from \"%s\" to \"%s\"",
			nseName,
			request.GetConnection().GetNetworkServiceEndpointName(),
		)
		if request.GetConnection().GetNetworkServiceEndpointName() != "" {
			ptxc.myEndpointName = request.GetConnection().GetNetworkServiceEndpointName()
		} else {
			ptxc.myEndpointName = nseName
		}
	}
}

// buildVppIfName constructs and returns the vpp interface name
func (ptxc *passThroughComposite) buildVppIfName(defaultIfName, serviceName string) string {
	if _, ok := ptxc.endpointIfID[serviceName]; !ok {
		ptxc.endpointIfID[serviceName] = 0
	} else {
		ptxc.endpointIfID[serviceName]++
	}

	return defaultIfName + "/" + strconv.Itoa(ptxc.endpointIfID[serviceName])
}
