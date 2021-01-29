package main

import (
	"context"
	"os"
	"sync"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/networkservicemesh/networkservicemesh/controlplane/api/connection"
	"github.com/networkservicemesh/networkservicemesh/controlplane/api/connection/mechanisms/memif"
	"github.com/networkservicemesh/networkservicemesh/controlplane/api/connectioncontext"
	"github.com/networkservicemesh/networkservicemesh/controlplane/api/networkservice"
	"github.com/networkservicemesh/networkservicemesh/controlplane/api/registry"
	"github.com/networkservicemesh/networkservicemesh/pkg/tools"
	"github.com/networkservicemesh/networkservicemesh/sdk/client"
	"github.com/networkservicemesh/networkservicemesh/sdk/common"
	"github.com/networkservicemesh/networkservicemesh/sdk/endpoint"
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
	LABEL_NSESOURCE = "vl3Nse/nseSource/endpointName"
)

type vL3PeerState int

const (
	PEER_STATE_NOTCONN vL3PeerState = iota
	PEER_STATE_CONN
	PEER_STATE_CONNERR
	PEER_STATE_CONN_INPROG
	PEER_STATE_CONN_RX
)

type vL3NsePeer struct {
	sync.RWMutex
	endpointName              string
	networkServiceManagerName string
	state                     vL3PeerState
	connHdl                   *connection.Connection
	connErr                   error
	excludedPrefixes          []string
	remoteIp                  string
}

type vL3ConnectComposite struct {
	sync.RWMutex
	//endpoint.BaseCompositeEndpoint
	myEndpointName     string
	nsConfig           *common.NSConfiguration
	defaultRouteIpCidr string
	remoteNsIpList     []string
	vL3NetCidr         string
	vl3NsePeers        map[string]*vL3NsePeer
	nsRegGrpcClient    *grpc.ClientConn
	nsDiscoveryClient  registry.NetworkServiceDiscoveryClient
	//nsClient networkservice.NetworkServiceClient
	nsmClient      *client.NsmClient
	backend        config.UniversalCNFBackend
	myNseNameFunc  fnGetNseName
	connDomain     string
	nseControlAddr string
}

func (peer *vL3NsePeer) setPeerState(state vL3PeerState) {
	peer.Lock()
	defer peer.Unlock()
	peer.state = state
}

func (peer *vL3NsePeer) getPeerState() vL3PeerState {
	peer.Lock()
	defer peer.Unlock()
	return peer.state
}
func (peer *vL3NsePeer) setPeerConnHdl(connHdl *connection.Connection, connErr error) {
	peer.Lock()
	defer peer.Unlock()
	peer.connHdl = connHdl
	peer.connErr = connErr
}

func (vxc *vL3ConnectComposite) getPeer(endpointName string) *vL3NsePeer {
	vxc.Lock()
	defer vxc.Unlock()
	peer, ok := vxc.vl3NsePeers[endpointName]
	if !ok {
		return nil
	}
	return peer
}

func (vxc *vL3ConnectComposite) addPeer(endpointName, networkServiceManagerName, remoteIp string) *vL3NsePeer {
	vxc.Lock()
	defer vxc.Unlock()
	_, ok := vxc.vl3NsePeers[endpointName]
	if !ok {
		vxc.vl3NsePeers[endpointName] = &vL3NsePeer{
			endpointName:              endpointName,
			networkServiceManagerName: networkServiceManagerName,
			state:                     PEER_STATE_NOTCONN,
			remoteIp:                  remoteIp,
		}
	}
	return vxc.vl3NsePeers[endpointName]
}
func (vxc *vL3ConnectComposite) SetMyNseName(request *networkservice.NetworkServiceRequest) {
	vxc.Lock()
	defer vxc.Unlock()
	if vxc.myEndpointName == "" {
		nseName := vxc.myNseNameFunc()
		logrus.Infof("Setting vL3connect composite endpoint name to \"%s\"--req contains \"%s\"", nseName, request.GetConnection().GetNetworkServiceEndpointName())
		if request.GetConnection().GetNetworkServiceEndpointName() != "" {
			vxc.myEndpointName = request.GetConnection().GetNetworkServiceEndpointName()
		} else {
			vxc.myEndpointName = nseName
		}
	}
}

func (vxc *vL3ConnectComposite) GetMyNseName() string {
	vxc.Lock()
	defer vxc.Unlock()
	return vxc.myEndpointName
}

func (vxc *vL3ConnectComposite) processPeerRequest(vl3SrcEndpointName string, request *networkservice.NetworkServiceRequest, incoming *connection.Connection) error {
	logrus.Infof("vL3ConnectComposite received connection request from vL3 NSE %s", vl3SrcEndpointName)
	go func() {
		metrics.ReceivedConnRequests.Inc()
	}()
	peer := vxc.addPeer(vl3SrcEndpointName, request.GetConnection().GetSourceNetworkServiceManagerName(), "")
	peer.Lock()
	defer peer.Unlock()
	logrus.WithFields(logrus.Fields{
		"endpointName":              peer.endpointName,
		"networkServiceManagerName": peer.networkServiceManagerName,
		"prior_state":               peer.state,
		"new_state":                 PEER_STATE_CONN_RX,
	}).Infof("vL3ConnectComposite vl3 NSE peer %s added", vl3SrcEndpointName)
	peer.excludedPrefixes = removeDuplicates(append(peer.excludedPrefixes, incoming.Context.IpContext.ExcludedPrefixes...))
	incoming.Context.IpContext.ExcludedPrefixes = peer.excludedPrefixes
	peer.connHdl = request.GetConnection()

	/* tell my peer to route to me for my vL3NetCidr */
	mySubnetRoute := connectioncontext.Route{
		Prefix: vxc.vL3NetCidr,
	}
	incoming.Context.IpContext.DstRoutes = append(incoming.Context.IpContext.DstRoutes, &mySubnetRoute)
	peer.state = PEER_STATE_CONN_RX
	return nil
}

func (vxc *vL3ConnectComposite) Request(ctx context.Context,
	request *networkservice.NetworkServiceRequest) (*connection.Connection, error) {
	logger := logrus.New() // endpoint.Log(ctx)
	conn := request.GetConnection()
	logger.WithFields(logrus.Fields{
		"endpointName":              conn.GetNetworkServiceEndpointName(),
		"networkServiceManagerName": conn.GetSourceNetworkServiceManagerName(),
	}).Infof("vL3ConnectComposite Request handler")
	//var err error
	/* NOTE: for IPAM we assume there's no IPAM endpoint in the composite endpoint list */
	/* -we are taking care of that here in this handler */
	/*incoming, err := vxc.GetNext().Request(ctx, request)
	if err != nil {
		logrus.Error(err)
		return nil, err
	}*/
	logger.Infof("DEBUGGING -- NS request: %+v", request)
	logger.Infof("DEBUGGING -- conn: %+v,  conn.labels:%+v", conn, conn.GetLabels())

	if vl3SrcEndpointName, ok := conn.GetLabels()[LABEL_NSESOURCE]; ok {
		// request is from another vl3 NSE
		conn.Labels[config.PEER_NAME] = vl3SrcEndpointName
		_ = vxc.processPeerRequest(vl3SrcEndpointName, request, request.Connection)

	} else {
		/* set NSC route to this NSE for full vL3 CIDR */
		nscVL3Route := connectioncontext.Route{
			Prefix: vxc.defaultRouteIpCidr,
		}
		request.Connection.Context.IpContext.DstRoutes = append(request.Connection.Context.IpContext.DstRoutes, &nscVL3Route)

		vxc.SetMyNseName(request)
		logger.Infof("vL3ConnectComposite serviceRegistry.DiscoveryClient")
		if vxc.nsDiscoveryClient == nil {
			logger.Error("nsDiscoveryClient is nil")
		} else {
			/* Find all NSEs registered as the same type as this one */
			req := &registry.FindNetworkServiceRequest{
				NetworkServiceName: conn.GetNetworkService(),
			}
			logger.Infof("vL3ConnectComposite FindNetworkService for NS=%s", conn.GetNetworkService())
			response, err := vxc.nsDiscoveryClient.FindNetworkService(context.Background(), req)
			if err != nil {
				logger.Error(err)
				go func() {
					metrics.FailedFindNetworkService.Inc()
				}()
			} else {
				logger.Infof("vL3ConnectComposite found network service; processing endpoints")
				go vxc.processNsEndpoints(context.TODO(), response, "", conn)
			}
			vxc.nsmClient.Configuration.ClientNetworkService = req.NetworkServiceName
			logger.Infof("vL3ConnectComposite check remotes for endpoints")
			for _, remoteIp := range vxc.remoteNsIpList {
				req.NetworkServiceName = req.NetworkServiceName + "@" + remoteIp
				logger.Infof("vL3ConnectComposite querying remote NS %s", req.NetworkServiceName)
				response, err := vxc.nsDiscoveryClient.FindNetworkService(context.Background(), req)
				if err != nil {
					logger.Error(err)
					go func() {
						metrics.FailedFindNetworkService.Inc()
					}()
				} else {
					logger.Infof("vL3ConnectComposite found network service; processing endpoints from remote %s", remoteIp)
					go vxc.processNsEndpoints(context.TODO(), response, remoteIp, conn)
				}
			}
		}
	}

	err := ValidateInLabels(conn.Labels)
	if err != nil {
		logger.Errorf("vL3 workload params not in labels: %v", err)
	} else {
		serviceRegistry, registryClient, err := NewServiceRegistry(vxc.nseControlAddr, ctx)
		if err != nil {
			logger.Error(err)
		} else {
			err = serviceRegistry.RegisterWorkload(ctx, conn.Labels, vxc.connDomain,
				processWorkloadIps(conn.Context.IpContext.SrcIpAddr, ";"), config.GetEndpointName())
			if err != nil {
				logger.Error(err)
			}
			registryClient.Stop()
		}
	}

	logger.Infof("vL3ConnectComposite request done")
	//return incoming, nil
	if endpoint.Next(ctx) != nil {
		return endpoint.Next(ctx).Request(ctx, request)
	}
	return conn, nil
}

func (vxc *vL3ConnectComposite) Close(ctx context.Context, conn *connection.Connection) (*empty.Empty, error) {
	// remove from connections
	logrus.Infof("vL3 DeleteConnection: %v", conn)
	err := ValidateInLabels(conn.Labels)
	if err != nil {
		logrus.Errorf("vL3 workload params not in labels: %v", err)
	} else {
		logrus.WithFields(logrus.Fields{
			"SrcIP": processWorkloadIps(conn.Context.IpContext.SrcIpAddr, ";"),
		}).Infof("vL3 Removing workload instance")
		serviceRegistry, registryClient, err := NewServiceRegistry(vxc.nseControlAddr, ctx)
		if err != nil {
			logrus.Error(err)
		} else {
			err = serviceRegistry.RemoveWorkload(ctx, conn.Labels, vxc.connDomain,
				processWorkloadIps(conn.Context.IpContext.SrcIpAddr, ";"), config.GetEndpointName())
			if err != nil {
				logrus.Error(err)
			}
			registryClient.Stop()
		}
	}

	if endpoint.Next(ctx) != nil {
		return endpoint.Next(ctx).Close(ctx, conn)
	}
	return &empty.Empty{}, nil
}

// Name returns the composite name
func (vxc *vL3ConnectComposite) Name() string {
	return "vL3 NSE"
}

func (vxc *vL3ConnectComposite) processNsEndpoints(ctx context.Context, response *registry.FindNetworkServiceResponse,
	remoteIp string, conn *connection.Connection) error {
	/* TODO: For NSs with multiple endpoint types how do we know their type?
	   - do we need to match the name portion?  labels?
	*/
	// just create a new logger for this go thread
	logger := logrus.New()
	for _, nsEndpoint := range response.GetNetworkServiceEndpoints() {
		// only vL3 NSEs have a label of "nsepod.name"
		_, ok := nsEndpoint.GetLabels()[POD_NAME]

		// treat all other vL3 NSEs besides myself as peers
		if ok {
			if nsEndpoint.GetName() != vxc.GetMyNseName() {
				logger.Infof("Found vL3 service %s peer %s", nsEndpoint.NetworkServiceName,
					nsEndpoint.GetName())
				peer := vxc.addPeer(nsEndpoint.GetName(), nsEndpoint.NetworkServiceManagerName, remoteIp)
				peer.Lock()
				//peer.excludedPrefixes = removeDuplicates(append(peer.excludedPrefixes, incoming.Context.IpContext.ExcludedPrefixes...))
				err := vxc.ConnectPeerEndpoint(ctx, peer, logger)
				if err != nil {
					logger.WithFields(logrus.Fields{
						"peerEndpoint": nsEndpoint.GetName(),
					}).Errorf("Failed to connect to vL3 Peer")
				} else {
					if peer.connHdl != nil {
						logger.WithFields(logrus.Fields{
							"peerEndpoint":         nsEndpoint.GetName(),
							"srcIP":                peer.connHdl.Context.IpContext.SrcIpAddr,
							"ConnExcludedPrefixes": peer.connHdl.Context.IpContext.ExcludedPrefixes,
							"peerExcludedPrefixes": peer.excludedPrefixes,
							"peer.DstRoutes":       peer.connHdl.Context.IpContext.DstRoutes,
						}).Infof("Connected to vL3 Peer")
					} else {
						logger.WithFields(logrus.Fields{
							"peerEndpoint":         nsEndpoint.GetName(),
							"peerExcludedPrefixes": peer.excludedPrefixes,
						}).Infof("Connected to vL3 Peer but connhdl == nil")
					}
				}
				peer.Unlock()
			} else {
				logger.Infof("Found my vL3 service %s instance endpoint name: %s", nsEndpoint.NetworkServiceName,
					nsEndpoint.GetName())
			}
		}
	}
	return nil
}

func (vxc *vL3ConnectComposite) createPeerConnectionRequest(ctx context.Context, peer *vL3NsePeer, routes []string, logger logrus.FieldLogger) error {
	/* expected to be called with peer.Lock() */
	if peer.state == PEER_STATE_CONN || peer.state == PEER_STATE_CONN_INPROG {
		logger.WithFields(logrus.Fields{
			"peer.Endpoint": peer.endpointName,
		}).Infof("Already connected to peer")
		return peer.connErr
	}
	peer.state = PEER_STATE_CONN_INPROG
	logger.WithFields(logrus.Fields{
		"peer.Endpoint": peer.endpointName,
	}).Infof("Performing connect to peer")
	dpconfig := &vpp.ConfigData{}
	peer.connHdl, peer.connErr = vxc.performPeerConnectRequest(ctx, peer, routes, dpconfig, logger)
	if peer.connErr != nil {
		logger.WithFields(logrus.Fields{
			"peer.Endpoint": peer.endpointName,
		}).Errorf("NSE peer connection failed - %v", peer.connErr)
		peer.state = PEER_STATE_CONNERR
		return peer.connErr
	}

	if peer.connErr = vxc.backend.ProcessDPConfig(dpconfig, true); peer.connErr != nil {
		logger.Errorf("endpoint %s Error processing dpconfig: %+v -- %v", peer.endpointName, dpconfig, peer.connErr)
		peer.state = PEER_STATE_CONNERR
		return peer.connErr
	}

	peer.state = PEER_STATE_CONN
	logger.WithFields(logrus.Fields{
		"peer.Endpoint": peer.endpointName,
	}).Infof("Done with connect to peer")
	return nil
}

func (vxc *vL3ConnectComposite) performPeerConnectRequest(ctx context.Context, peer *vL3NsePeer, routes []string, dpconfig interface{}, logger logrus.FieldLogger) (*connection.Connection, error) {
	/* expected to be called with peer.Lock() */
	go func() {
		metrics.PerormedConnRequests.Inc()
	}()
	ifName := peer.endpointName
	vxc.nsmClient.ClientLabels[LABEL_NSESOURCE] = vxc.GetMyNseName()
	conn, err := vxc.nsmClient.ConnectToEndpoint(ctx, peer.remoteIp, peer.endpointName, peer.networkServiceManagerName, ifName, memif.MECHANISM, "VPP interface "+ifName, routes)
	if err != nil {
		logger.Errorf("Error creating %s: %v", ifName, err)
		return nil, err
	}

	err = vxc.backend.ProcessClient(dpconfig, ifName, conn)

	return conn, nil
}

func (vxc *vL3ConnectComposite) ConnectPeerEndpoint(ctx context.Context, peer *vL3NsePeer, logger logrus.FieldLogger) error {
	/* expected to be called with peer.Lock() */
	// build connection object
	// perform remote networkservice request
	state := peer.state
	logger.WithFields(logrus.Fields{
		"endpointName":              peer.endpointName,
		"networkServiceManagerName": peer.networkServiceManagerName,
		"state":                     state,
	}).Info("newVL3Connect ConnectPeerEndpoint")

	switch state {
	case PEER_STATE_NOTCONN:
		// TODO do connection request
		logger.WithFields(logrus.Fields{
			"endpointName":              peer.endpointName,
			"networkServiceManagerName": peer.networkServiceManagerName,
		}).Info("request remote connection")
		routes := []string{vxc.vL3NetCidr}
		return vxc.createPeerConnectionRequest(ctx, peer, routes, logger)
	case PEER_STATE_CONN:
		logger.WithFields(logrus.Fields{
			"endpointName":              peer.endpointName,
			"networkServiceManagerName": peer.networkServiceManagerName,
		}).Info("remote connection already established")
	case PEER_STATE_CONNERR:
		logger.WithFields(logrus.Fields{
			"endpointName":              peer.endpointName,
			"networkServiceManagerName": peer.networkServiceManagerName,
		}).Info("remote connection attempted prior and errored")
	case PEER_STATE_CONN_INPROG:
		logger.WithFields(logrus.Fields{
			"endpointName":              peer.endpointName,
			"networkServiceManagerName": peer.networkServiceManagerName,
		}).Info("remote connection in progress")
	case PEER_STATE_CONN_RX:
		logger.WithFields(logrus.Fields{
			"endpointName":              peer.endpointName,
			"networkServiceManagerName": peer.networkServiceManagerName,
		}).Info("remote connection already established--rx from peer")
	default:
		logger.WithFields(logrus.Fields{
			"endpointName":              peer.endpointName,
			"networkServiceManagerName": peer.networkServiceManagerName,
		}).Info("remote connection state unknown")
	}
	return nil
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

// newVL3ConnectComposite creates a new VL3 composite
func newVL3ConnectComposite(configuration *common.NSConfiguration, vL3NetCidr string, backend config.UniversalCNFBackend, remoteIpList []string, getNseName fnGetNseName, defaultCdPrefix, nseControlAddr, connDomain string) *vL3ConnectComposite {
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

	logrus.Infof("newVL3ConnectComposite")

	var nsDiscoveryClient registry.NetworkServiceDiscoveryClient

	/*
		regAddr := net.ParseIP(nsRegAddr)
		if regAddr == nil {
			regAddrList, err := net.LookupHost(nsRegAddr)
			if err != nil {
				logrus.Errorf("nsmConnection registry address resolution Error: %v", err)
			} else {
				logrus.Infof("newVL3ConnectComposite: resolved %s to %v", nsRegAddr, regAddrList)
				for _, regAddrVal := range regAddrList {
					if regAddr = net.ParseIP(regAddrVal); regAddr != nil {
						logrus.Infof("newVL3ConnectComposite: NSregistry using IP %s", regAddrVal)
						break
					}
				}
			}
		}
		regPort, _ := strconv.Atoi(nsRegPort)
		nsRegGrpcClient, err := tools.SocketOperationCheck(&net.TCPAddr{IP: regAddr, Port: regPort})
	*/
	nsRegGrpcClient, err := tools.DialTCP(nsRegAddr + ":" + nsRegPort)
	if err != nil {
		logrus.Errorf("nsmRegistryConnection GRPC Client Socket Error: %v", err)
		//return nil
	} else {
		logrus.Infof("newVL3ConnectComposite socket operation ok... create networkDiscoveryClient")
		nsDiscoveryClient = registry.NewNetworkServiceDiscoveryClient(nsRegGrpcClient)
		if nsDiscoveryClient == nil {
			logrus.Errorf("newVL3ConnectComposite networkDiscoveryClient nil")
		} else {
			logrus.Infof("newVL3ConnectComposite networkDiscoveryClient ok")
		}
	}

	// create remote_networkservice API connection

	//var nsClient networkservice.NetworkServiceClient
	/*
		nsGrpcClient, err := tools.DialTCP(nsRegAddr + ":" + nsPort)
		if err != nil {
			logrus.Errorf("nsmConnection GRPC Client Socket Error: %v", err)
			//return nil
		} else {
			logrus.Infof("newVL3ConnectComposite socket operation ok... create network-service client")
			nsClient = networkservice.NewNetworkServiceClient(nsGrpcClient)
			logrus.Infof("newVL3ConnectComposite network-service client ok")
		}
	*/
	// Call the NS Client initiation
	/* nsConfig := &common.NSConfiguration{
		ClientNetworkService:   configuration.EndpointNetworkService,
		ClientLabels: "",
		Routes:            configuration.Routes,
	} */
	nsConfig := configuration
	nsConfig.ClientLabels = ""
	var nsmClient *client.NsmClient
	nsmClient, err = client.NewNSMClient(context.TODO(), nsConfig)
	if err != nil {
		logrus.Errorf("Unable to create the NSM client %v", err)
	}
	/*
		nsmConn, err := common.NewNSMConnection(context.TODO(), configuration)
		if err != nil {
			logrus.Errorf("nsmConnection Client Connection Error: %v", err)
		} else {
			nsClient = nsmConn.NsClient
		}
	*/

	newVL3ConnectComposite := &vL3ConnectComposite{
		nsConfig:           configuration,
		remoteNsIpList:     remoteIpList,
		vL3NetCidr:         vL3NetCidr,
		myEndpointName:     "",
		vl3NsePeers:        make(map[string]*vL3NsePeer),
		nsRegGrpcClient:    nsRegGrpcClient,
		nsDiscoveryClient:  nsDiscoveryClient,
		nsmClient:          nsmClient,
		backend:            backend,
		myNseNameFunc:      getNseName,
		defaultRouteIpCidr: defaultCdPrefix,
		nseControlAddr:     nseControlAddr,
		connDomain:         connDomain,
	}

	logrus.Infof("newVL3ConnectComposite returning")

	return newVL3ConnectComposite
}
