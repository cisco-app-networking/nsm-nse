package main

import (
	"context"
	"errors"
	"github.com/cisco-app-networking/nsm-nse/pkg/nseconfig"
	"github.com/networkservicemesh/networkservicemesh/controlplane/api/connection/mechanisms/memif"
	"github.com/networkservicemesh/networkservicemesh/sdk/endpoint"
	"os"
	"sync"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/networkservicemesh/networkservicemesh/controlplane/api/connection"
	"github.com/networkservicemesh/networkservicemesh/controlplane/api/connectioncontext"
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
	NSCLIENT_PORT   = "5001"
)

type passThroughCmposite struct {
	sync.RWMutex
	myEndpointName   			   string
	nsConfig           			   *common.NSConfiguration
	defaultRouteIpCidr 			   string
	remoteNsIpList                 []string
	passThroughNetCidr 			   string
	myEndpointLabels			   nseconfig.Labels
	nsRegGrpcClient    			   *grpc.ClientConn
	nsDiscoveryClient  			   registry.NetworkServiceDiscoveryClient
	nsmClient          			   *client.NsmClient
	backend            			   config.UniversalCNFBackend
	myNseNameFunc      			   fnGetNseName
}


func removeDuplicates(elements []string) []string {
	encountered := map[string]bool{}
	result := []string{}

	for v := range elements {
		if !encountered[elements[v]] {
			encountered[elements[v]] = true
			result = append(result, elements[v])
		}
	}
	return result
}


func (ptxc *passThroughCmposite) Request(ctx context.Context,
	request *networkservice.NetworkServiceRequest) (*connection.Connection, error) {
	// 1. Create discoveryService instance
	// 2. Call FindNetworkService() to get the matches under the NetworkService Definition
	//		2.a Parse the response from matches
	// 3. Make a NS Request to find the next pod in the chain

	logger := logrus.New()
	conn := request.GetConnection()
	logger.WithFields(logrus.Fields{
		"endpointName":              conn.GetNetworkServiceEndpointName(),
		"networkServiceManagerName": conn.GetSourceNetworkServiceManagerName(),
	}).Infof("passThroughCmposite Request handler")

	logger.Infof("DEBUGGING -- ptxc:%+v", ptxc)
	logger.Infof("DEBUGGING -- ptxc.:%+v", ptxc.nsmClient)
	logger.Infof("DEBUGGING -- initial NS request conn: %+v", conn)

	// set NSC route to this NSE for full pass-through CIDR
	nscPassThrRoute := connectioncontext.Route {
		Prefix: ptxc.defaultRouteIpCidr,
	}
	request.Connection.Context.IpContext.DstRoutes = append(request.Connection.Context.IpContext.DstRoutes, &nscPassThrRoute)
	logger.Infof("DEBUGGING -- The nsc -> pass-through-nse route is: %s, DstRoutes is: %+v", nscPassThrRoute.Prefix,
		request.Connection.Context.IpContext.DstRoutes)

	ptxc.SetMyNseName(request)
	logger.Infof("DEBUGGING -- after SetMyNseName(): %+v", ptxc)

	logger.Infof("passThroughCmposite serviceRegistry.DiscoveryClient")


	if ptxc.nsDiscoveryClient == nil {
		logger.Error("nsDiscoveryClient is nil")
	} else {
		req := &registry.FindNetworkServiceRequest{
			NetworkServiceName: conn.GetNetworkService(),
		}

		logger.Infof("passThroughCmposite FindNetworkService for NS=%s", conn.GetNetworkService())
		response, err := ptxc.nsDiscoveryClient.FindNetworkService(context.Background(), req)
		if err != nil {
			logger.Error(err)
			go func() {
				metrics.FailedFindNetworkService.Inc()
			}()
		} else {
			/* handle the local case */
			logger.Infof("passThroughCmposite found network service; processing endpoints")
			dpConfig := &vpp.ConfigData{}
			go ptxc.processNsEndpoints(context.TODO(), dpConfig, response, "")
		}
		ptxc.nsmClient.Configuration.ClientNetworkService = req.NetworkServiceName
		logger.Infof("DEBUGGING -- main routine")

		/* handle the remote case */
		//logger.Infof("passThroughCmposite check remotes for endpoints")
		//for _, remoteIP := range ptxc.remoteNsIpList {
		//	req.NetworkServiceName = req.NetworkServiceName + "@" + remoteIP
		//	logger.Infof("passThroughCmposite querying remote ns %s", req.NetworkServiceName)
		//	response, err := ptxc.nsDiscoveryClient.FindNetworkService(context.Background(), req)
		//	if err != nil {
		//		logger.Error(err)
		//		go func() {
		//			metrics.FailedFindNetworkService.Inc()
		//		}()
		//	} else {
		//		logger.Infof("passThroughCmposite found network serviec; processing endpoints from remote %s", remoteIP)
		//		logger.Infof("DEBUGGING -- response:%+v", response)
		//		//
		//	}
		//}

		// TODO: need to handle the vl3NSE -> passThroughNSE -> NSC case
	}

	logger.Infof("passThroughCmposite request done")

	if endpoint.Next(ctx) != nil {
		return endpoint.Next(ctx).Request(ctx, request)
	}

	return conn, nil
}

func (ptxc *passThroughCmposite) Close(ctx context.Context, conn *connection.Connection) (*empty.Empty, error) {
	// remove from connections
	logrus.Infof("passThrough DeleteConnection: %", conn)

	if endpoint.Next(ctx) != nil {
		return endpoint.Next(ctx).Close(ctx, conn)
	}
	return &empty.Empty{}, nil
}

//processNsEndpoints finds the next NSE in the chain and sends a NS request to connect with the chaining NSE
func (ptxc *passThroughCmposite) processNsEndpoints(ctx context.Context, dpConfig interface{},
	response *registry.FindNetworkServiceResponse, remoteIP string) (error) {
	// create a new logger for this go thread
	logger := logrus.New()
	logger.Infof("DEBUGGING -- the ptxc is:%+v", ptxc)

	// Find the matches entries under the NetworkService definition
	// matches: []*Match, Match: struct{SourceSelector, Routes}
	matches := response.GetNetworkService().GetMatches()

	var curKey, curVal, desKey, desVal string

	// First find out the current pod label
	// Assuming multiple labels in a pod is allowed
	logger.Infof("DEBUGGING -- the response is: %+v", response)
	logger.Infof("DEBUGGING -- the corresponding networkservice is: %+v", response.GetNetworkService())
	logger.Infof("DEBUGGING -- matches for the NS are:%+v", matches)
	for _, match := range matches {
		if match.GetSourceSelector() == nil {
			// GetRoutes(): []*Destination, Destination: {DestinationSelector, weight}
			for _, route := range match.GetRoutes() {
				for k, v := range route.DestinationSelector {
					curKey = k
					curVal = v
				}
			}
		}
	}

	logger.Infof("DEBUGGING -- curKey is: %s, curVal is: %s", curKey, curVal)

	// Use the curKey and curVal to find the next NSE in the chain
	for _, match := range matches {
		sourceSelector := match.GetSourceSelector()
		if sourceSelector != nil {
			if val, ok := sourceSelector[curKey]; ok && val == curVal {
				for k, v := range match.GetRoutes()[0].DestinationSelector {
					desKey = k
					desVal = v
				}
			}
		}
	}

	if desKey == "" || desVal =="" {
		return errors.New("Unable to find the label for the next NSE in the chain")
	}

	logger.Infof("DEBUGGING -- desKey is: %s, desVal is: %s", desKey, desVal)

	// Then use the desKey and desVal to find the next NSE in the chain and initiate an NS Request to the next NSE
	logger.Infof("The endpoints under the current network service are: %+v", response.GetNetworkServiceEndpoints)
	for _, e := range response.GetNetworkServiceEndpoints() {
		logger.Infof("DEBUGGING -- The current endpoitn is: %+v", e)
		for eLabelKey, eLabelVal := range e.Labels {
			if eLabelKey == desKey && eLabelVal == desVal {
				logger.Infof("The chained NSE is found: %s", e.GetName())

				chainConn, connErr := ptxc.performChainConnectRequest(ctx, e, dpConfig, logger, remoteIP)
				if connErr != nil {
					logger.Error("Error perform chain connect request: %s", connErr)
					return connErr
				}

				logger.Infof("DEBUGGING -- the chained connection is: %+v", chainConn)

				logger.Infof("DEBUGGING -- done with ProcessClient(), now to ProcessDPConfig: %+v", dpConfig)

				if err := ptxc.backend.ProcessDPConfig(dpConfig, true); err != nil {
					logger.Errorf("endpoint %s Erroor processing dpconfig: %+v -- %v", e.GetName(), dpConfig, err)
					return err
				}

				logger.Infof("DEBUGGING -- done with ProcessDPConfig(), dpConfig: %+v", dpConfig)

				logger.Infof("Done with connect to chained NSE")
				return nil
			}
		}
	}
	return errors.New("Unable to find the next NSE in the chain")
}

func (ptxc *passThroughCmposite) performChainConnectRequest(ctx context.Context, e *registry.NetworkServiceEndpoint,
	dpConfig interface{}, logger logrus.FieldLogger, remoteIP string) (*connection.Connection, error) {
	ifName := e.GetName()
	routes := []string{ptxc.passThroughNetCidr}

	//ptxc.nsmClient.ClientLabels[LABEL_NSESOURCE] = ptxc.GetMyNseName()
	conn, err := ptxc.nsmClient.ConnectToEndpoint(ctx, remoteIP, e.Name, e.NetworkServiceManagerName,
		ifName, memif.MECHANISM, "Chaining connection " + ifName, routes)
	if err != nil {
		logger.Errorf("Error creating %s: %v", ifName, err)
		return nil, err
	}

	logger.Infof("DEBUGGING -- to ProcessClient: %+v", dpConfig)

	err = ptxc.backend.ProcessClient(dpConfig, ifName, conn)
	if err != nil {
		logger.Errorf("Error processing %s: %v", ifName, err)
		return nil, err
	}
	return conn, nil
}

// newPassThroughConnectComposite creates a new pass-through composite
func newPassThroughConnectComposite(configuration *common.NSConfiguration, passThroughNetCidr string,
	backend config.UniversalCNFBackend, remoteIpList []string, getNseName fnGetNseName,
	defaultCdPrefix, nseControlAddr string, labels nseconfig.Labels) *passThroughCmposite {

	logrus.Infof("DEBUGGING -- newPassThroughConnectComposite()")

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
		//return nil
	} else {
		logrus.Infof("newPassThroughConnectComposite socket operation ok... create networkDiscoveryClient")
		nsDiscoveryClient = registry.NewNetworkServiceDiscoveryClient(nsRegGrpcClient)
		if nsDiscoveryClient == nil {
			logrus.Errorf("newPassThroughConnectComposite networkDiscoveryClient nil")
		} else {
			logrus.Infof("newPassThroughConnectComposite networkDiscoveryClient ok")
		}
	}

	nsConfig := configuration
	nsConfig.ClientLabels = ""
	var nsmClient *client.NsmClient
	nsmClient, err = client.NewNSMClient(context.TODO(), nsConfig)
	if err != nil {
		logrus.Errorf("Unable to create the NSM client %v", err)
	}

	newPassThroughConnectComposite := &passThroughCmposite{
		nsConfig:           		configuration,
		remoteNsIpList:     		remoteIpList,
		passThroughNetCidr:         passThroughNetCidr,
		myEndpointName:     		"",
		nsRegGrpcClient:    		nsRegGrpcClient,
		nsDiscoveryClient:  		nsDiscoveryClient,
		nsmClient:          		nsmClient,
		backend:            		backend,
		myNseNameFunc:      		getNseName,
		defaultRouteIpCidr: 		defaultCdPrefix,
		myEndpointLabels:			labels,
	}

	logrus.Infof("DEBUGGING -- newPassThroughConnectComposite returning: %+v", newPassThroughConnectComposite)

	return newPassThroughConnectComposite
}

// SetMyNseName() is a helper function that sets the passThrough composite endpoint name
func (ptxc *passThroughCmposite) SetMyNseName(request *networkservice.NetworkServiceRequest){
	ptxc.Lock()
	defer ptxc.Unlock()

	if ptxc.myEndpointName == "" {
		nseName := ptxc.myNseNameFunc()
		logrus.Infof("Setting passThrough composite endpoint name to \"%s\"--req contains \"%s\"", nseName,
			request.GetConnection().GetNetworkServiceEndpointName())
		if request.GetConnection().GetNetworkServiceEndpointName() != "" {
			ptxc.myEndpointName = request.GetConnection().GetNetworkServiceEndpointName()
		} else {
			ptxc.myEndpointName = nseName
		}
	}
}

func (ptxc *passThroughCmposite) GetMyNseName() string {
	ptxc.Lock()
	ptxc.Unlock()
	return ptxc.myEndpointName
}
