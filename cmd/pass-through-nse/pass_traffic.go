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

type chainNSEState int

const (
	POD_NAME     = "podName"
)

const (
	CHAIN_STATE_NOTCONN chainNSEState = iota
	CHAIN_STATE_CONN
	CHAIN_STATE_CONNERR
	CHAIN_STATE_CONN_INPROG
	//CHAIN_STATE_CONN_RX
)

type chainNSE struct {
	sync.RWMutex
	endpointName		   		string
	networkServiceManagerName	string
	state						chainNSEState
	connHdl						*connection.Connection
	connErr						error
	remoteIP					string
}

type passThroughComposite struct {
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
	backend            			config.UniversalCNFBackend
	myNseNameFunc      			   fnGetNseName
	chain					  		*chainNSE // assume each passthrough NSE has only 1 chain NSE
	defaultIfName					string
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


func (ptxc *passThroughComposite) Request(ctx context.Context,
	request *networkservice.NetworkServiceRequest) (*connection.Connection, error) {

	logger := logrus.New()
	conn := request.GetConnection()
	logger.WithFields(logrus.Fields{
		"endpointName":              conn.GetNetworkServiceEndpointName(),
		"networkServiceManagerName": conn.GetSourceNetworkServiceManagerName(),
	}).Infof("passThroughComposite Request handler")

	logger.Infof("DEBUGGING -- ptxc:%+v", ptxc)
	logger.Infof("DEBUGGING -- ptxc.nsmClient:%+v", ptxc.nsmClient)
	logger.Infof("DEBUGGING -- initial NS request conn: %+v", conn)

	// set NSC route to this NSE for full pass-through CIDR
	nscPassThrRoute := connectioncontext.Route {
		Prefix: ptxc.defaultRouteIpCidr,
	}
	request.Connection.Context.IpContext.DstRoutes = append(request.Connection.Context.IpContext.DstRoutes, &nscPassThrRoute)
	logger.Infof("DEBUGGING -- The nsc -> pass-through-nse route is: %s, DstRoutes is: %+v", nscPassThrRoute.Prefix,
		request.Connection.Context.IpContext.DstRoutes)

	ptxc.SetMyNseName(request)

	logger.Infof("passThroughComposite serviceRegistry.DiscoveryClient")


	if ptxc.nsDiscoveryClient == nil {
		logger.Error("nsDiscoveryClient is nil")
	} else {
		req := &registry.FindNetworkServiceRequest{
			NetworkServiceName: conn.GetNetworkService(),
		}

		logger.Infof("passThroughComposite FindNetworkService for NS=%s", conn.GetNetworkService())
		response, err := ptxc.nsDiscoveryClient.FindNetworkService(context.Background(), req)
		if err != nil {
			logger.Error(err)
			go func() {
				metrics.FailedFindNetworkService.Inc()
			}()
		} else {
			/* handle the local case */
			logger.Infof("passThroughComposite found network service; processing endpoints")
			ptxc.nsmClient.Configuration.ClientNetworkService = req.NetworkServiceName
			ptxc.processNsEndpoints(context.TODO(), request, response, "", logger)
		}

		/* handle the remote case */
		//logger.Infof("passThroughComposite check remotes for endpoints")
		//for _, remoteIP := range ptxc.remoteNsIpList {
		//	req.NetworkServiceName = req.NetworkServiceName + "@" + remoteIP
		//	logger.Infof("passThroughComposite querying remote ns %s", req.NetworkServiceName)
		//	response, err := ptxc.nsDiscoveryClient.FindNetworkService(context.Background(), req)
		//	if err != nil {
		//		logger.Error(err)
		//		go func() {
		//			metrics.FailedFindNetworkService.Inc()
		//		}()
		//	} else {
		//		logger.Infof("passThroughComposite found network serviec; processing endpoints from remote %s", remoteIP)
		//		logger.Infof("DEBUGGING -- response:%+v", response)
		//		//
		//	}
		//}

	}

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

//processNsEndpoints finds the next NSE in the chain and sends a NS request to connect with the chaining NSE
func (ptxc *passThroughComposite) processNsEndpoints(ctx context.Context, request *networkservice.NetworkServiceRequest,
	response *registry.FindNetworkServiceResponse, remoteIP string, logger *logrus.Logger) (error) {

	// Find the matches entries under the NetworkService definition
	// matches: []*Match, Match: struct{SourceSelector, Routes}
	matches := response.GetNetworkService().GetMatches()

	var curKey, curVal, desKey, desVal string

	// First find out the current pod label that matches the DestinationSelector under matches.routes
	// Assuming multiple labels in a pod is allowed
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
		return errors.New("Unable to find the chain NSE")
	}

	logger.Infof("DEBUGGING -- desKey is: %s, desVal is: %s", desKey, desVal)

	// Then use the desKey and desVal to find the next NSE in the chain and initiate an NS Request to the next NSE
	logger.Infof("The endpoints under the current network service are: %+v", response.GetNetworkServiceEndpoints)

	for _, nsEndpoint := range response.GetNetworkServiceEndpoints() {
		logger.Infof("DEBUGGING -- The current endpoint is: %+v", nsEndpoint)
		for eLabelKey, eLabelVal := range nsEndpoint.Labels {
			if eLabelKey == desKey && eLabelVal == desVal {
				logger.Infof("The chain NSE is found: %s, now trying to connect...", nsEndpoint.GetName())

				chainNSE := ptxc.addChain(nsEndpoint.GetName(), nsEndpoint.GetNetworkServiceManagerName(), remoteIP)

				chainNSE.Lock()
				routes := []string{ptxc.passThroughNetCidr}

				connErr := ptxc.ConnectChainEndpoint(ctx, chainNSE, routes, logger)
				if connErr != nil {
					logger.Error("Error perform chain connect request: %s", connErr)
					return connErr
				} else {
					if chainNSE.connHdl != nil {
						logger.WithFields(logrus.Fields{
							"chainEndpoint":	nsEndpoint.GetName(),
							"srcIP":			chainNSE.connHdl.Context.IpContext.SrcIpAddr,
							"chain.DstRoutes":	chainNSE.connHdl.Context.IpContext.DstRoutes,
							"IpContext":		chainNSE.connHdl.Context.IpContext,
						}).Infof("Connected to passThrough chain")

						// Pass the IpContext to the client connection
						request.Connection.Context.IpContext = chainNSE.connHdl.Context.IpContext
					} else {
						logger.WithFields(logrus.Fields{
							"chainEndpoint":	nsEndpoint.GetName(),
						}).Infof("Connected to passThrough chain but connection handler is nil")
					}
				}

				chainNSE.Unlock()

				return nil
			}
		}
	}
	return errors.New("Unable to find the next NSE in the chain")
}

func (ptxc *passThroughComposite) ConnectChainEndpoint(ctx context.Context, chain *chainNSE, routes []string,
	logger logrus.FieldLogger) (error) {
	/* expect to be called with chain.Lock() */
	logger.WithFields(logrus.Fields{
		"endpointName":              chain.endpointName,
		"networkServiceManagerName": chain.networkServiceManagerName,
		"state":                     chain.state,
	}).Info("passThrough ConnectChainEndpoint")

	state := chain.state

	switch state {
	case CHAIN_STATE_NOTCONN:
		logger.WithFields(logrus.Fields{
			"endpointName":			chain.endpointName,
			"networkServiceManagerName": chain.networkServiceManagerName,
		}).Info("requesting chain connection")
		return ptxc.createChainConnection(ctx, chain, routes, logger)
	case CHAIN_STATE_CONN:
		logger.WithFields(logrus.Fields{
			"endpointName":	chain.endpointName,
			"networkServiceManagerName": chain.networkServiceManagerName,
		}).Info("chain connection already established")
	case CHAIN_STATE_CONN_INPROG:
		logger.WithFields(logrus.Fields{
			"endpointName":	chain.endpointName,
			"networkServiceManagerName": chain.networkServiceManagerName,
		}).Info("chain connection in progress")
	default:
		logger.WithFields(logrus.Fields{
			"endpointName":              chain.endpointName,
			"networkServiceManagerName": chain.networkServiceManagerName,
		}).Info("chain connection state unknown")

	}
	return nil
}

func(ptxc *passThroughComposite) createChainConnection(ctx context.Context, chain *chainNSE, routes []string,
	logger logrus.FieldLogger) (error) {
	/* expect to be called with chain.Lock() */
	if chain.state == CHAIN_STATE_CONN || chain.state == CHAIN_STATE_CONN_INPROG {
		logger.WithFields(logrus.Fields{
			"chain.Endpoint": chain.endpointName,
		}).Infof("Already connected to chain endpoint")
	}
	chain.state = CHAIN_STATE_CONN_INPROG
	logger.WithFields(logrus.Fields{
		"chain.Endpoint": chain.endpointName,
	}).Infof("Performing conenct to chain endpoint")

	dpConfig := &vpp.ConfigData{}
	chain.connHdl, chain.connErr = ptxc.performChainConnectionRequest(ctx, chain, routes, dpConfig, logger)

	if chain.connErr != nil {
		logger.WithFields(logrus.Fields{
			"chain.Endpoint:": chain.endpointName,
		}).Errorf("NSE chain connection failed - %v", chain.connErr)
		chain.state = CHAIN_STATE_CONNERR
		return chain.connErr
	}

	if chain.connErr = ptxc.backend.ProcessDPConfig(dpConfig, true); chain.connErr != nil {
		logger.Errorf("endpoint %s Error processing dpconfig: %+v -- %v", chain.endpointName, dpConfig, chain.connErr)
		chain.state = CHAIN_STATE_CONNERR
		return chain.connErr
	}

	chain.state = CHAIN_STATE_CONN
	logger.WithFields(logrus.Fields{
		"chain.Endpoint": chain.endpointName,
	}).Info("Done with connect to chain")

	return nil

}

// performChainConnectionRequest
func (ptxc *passThroughComposite) performChainConnectionRequest(ctx context.Context, chain *chainNSE,
	routes []string, dpconfig interface{}, logger logrus.FieldLogger) (*connection.Connection, error){
	/* expect to be called with chain.Lock() */
	go func() {
		metrics.PerormedConnRequests.Inc()
	}()

	//ifName := ptxc.defaultIfName

	ifName := chain.endpointName
	// the memif.sock is created here
	conn, err := ptxc.nsmClient.ConnectToEndpoint(ctx, chain.remoteIP, chain.endpointName, chain.networkServiceManagerName,
		ifName, memif.MECHANISM, "VPP interface " + ifName, routes)
	if err != nil {
		logger.Errorf("Error creating %s: %v", ifName, err)
		return nil, err
	}
	logrus.Infof("The chain connection is: %+v", conn)

	// TODO: need to somehow store the memif that connects to the chainNSE

	err = ptxc.backend.ProcessMemif(dpconfig, ifName, conn, false)
	if err != nil {
		logger.Errorf("Error processing %s: %v", ifName, err)
		return nil, err
	}
	return conn, nil
}

// newPassThroughComposite creates a new passThrough composite
func newPassThroughComposite(configuration *common.NSConfiguration, passThroughNetCidr string,
	backend config.UniversalCNFBackend, remoteIpList []string, getNseName fnGetNseName,
	defaultCdPrefix string, labels nseconfig.Labels, ifName string) *passThroughComposite {

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
		chain:						nil,
		defaultIfName:				ifName,
	}

	logrus.Infof("DEBUGGING -- newPassThroughComposite returning: %+v", newPassThroughComposite)

	return newPassThroughComposite
}

// addChain() adds the chain NSE to the passThroughComposite
// this assumes each passthrough NSE has only one chain NSE
func (ptxc *passThroughComposite) addChain(endpointName, networkServiceManagerName, remoteIp string) *chainNSE {
	ptxc.Lock()
	defer ptxc.Unlock()
	if ptxc.chain == nil {
		ptxc.chain = &chainNSE{
			endpointName: endpointName,
			networkServiceManagerName: networkServiceManagerName,
			state: CHAIN_STATE_NOTCONN,
			remoteIP: remoteIp,
		}
	}
	return ptxc.chain
}

// SetMyNseName() is a helper function that sets the passThrough composite endpoint name
func (ptxc *passThroughComposite) SetMyNseName(request *networkservice.NetworkServiceRequest){
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

func (ptxc *passThroughComposite) GetMyNseName() string {
	ptxc.Lock()
	ptxc.Unlock()
	return ptxc.myEndpointName
}
