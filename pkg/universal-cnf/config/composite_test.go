package config

import (
	"fmt"
	"os"
	"testing"

	"github.com/cisco-app-networking/nsm-nse/pkg/nseconfig"
	"github.com/networkservicemesh/networkservicemesh/controlplane/api/connection"
	"github.com/networkservicemesh/networkservicemesh/controlplane/api/connectioncontext"
	"github.com/networkservicemesh/networkservicemesh/controlplane/api/networkservice"
	"github.com/sirupsen/logrus"
	"go.ligato.io/vpp-agent/v3/proto/ligato/vpp"
	"golang.org/x/net/context"

	. "github.com/onsi/gomega"

	vpp_interfaces "go.ligato.io/vpp-agent/v3/proto/ligato/vpp/interfaces"
)

const (
	// fields of type nseconfig.Endpoint
	ucnfeName                    string = ""
	nseName                      string = ""
	nseControlName               string = ""
	nseControlAddress            string = ""
	nseControlAccessToken        string = ""
	nseControlConnectivityDomain string = ""
	vl3Ifname                    string = ""
	ipamDefaultPrefixPool        string = ""
	ipamServerAddress            string = ""
	ipamPrefixLength             int    = 0

	// fields of type connection.Connection
	connId             string = ""
	connNetworkService string = "uncf"
	machanismCls       string = ""
	mechanismType      string = "MEMIF"
	paraDescription    string = ""
	paraName           string = "test"
	paraNetnsInode     string = ""
	paraSocketfile            = paraName + "/memif.sock"
	srcIP              string = "192.168.22.2/30"
	dstIP              string = "192.168.22.1/30"
	srcRequired        bool   = true
	dstRequired        bool   = true
	dstRoutePrefix     string = "192.168.0.0/16"
	labelsNamespace    string = "default"
	ucnfPodName        string = "helloworld-ucnf-"
	labelsPodName             = ucnfPodName + "test"
	controlPlaneName   string = "-control-plane"
	clusterName        string = "kind-2"
	pathSegmentName           = clusterName + controlPlaneName
)

var (
	// fields of type nseconfig.Endpoint
	ucnfeLabels    = map[string]string{}
	ipamRoute      = []string{""}
	vl3NameServers = []string{""}
	vl3DNSZones    = []string{""}
)

// mock implementor
type MockUcnfBackend struct {
	newDPConfig            func() *vpp.ConfigData
	newUniversalCNFBackEnd func() error
	processClient          func(dpconfig interface{}, ifName string, conn *connection.Connection) error
	processDPConfig        func(dpconfig interface{}, update bool) error
	processEndpoints       func(dpconfig interface{}, serviceName, ifName string, conn *connection.Connection) error
}

/*
mock functions returning the defined overriding functions otherwise will return nil by default
*/
func (m *MockUcnfBackend) NewDPConfig() *vpp.ConfigData {
	if m.newDPConfig == nil {
		return &vpp.ConfigData{}
	}
	return m.newDPConfig()
}

func (m *MockUcnfBackend) NewUniversalCNFBackend() error {
	if m.newUniversalCNFBackEnd == nil {
		return nil
	}
	return m.newUniversalCNFBackEnd()
}

func (m *MockUcnfBackend) ProcessClient(dpconfig interface{}, ifName string, conn *connection.Connection) error {
	if m.processClient == nil {
		return nil
	}
	return m.processClient(dpconfig, ifName, conn)
}

func (m *MockUcnfBackend) ProcessEndpoint(dpconfig interface{}, serviceName, ifName string, conn *connection.Connection) error {
	if m.processEndpoints == nil {
		return nil
	}
	return m.processEndpoints(dpconfig, serviceName, ifName, conn)
}

func (m *MockUcnfBackend) ProcessDPConfig(dpconfig interface{}, update bool) error {
	if m.processDPConfig == nil {
		return nil
	}
	return m.processDPConfig(dpconfig, update)
}

func createNseConfigEndpoint() (e *nseconfig.Endpoint) {
	e = &nseconfig.Endpoint{
		Name:    ucnfeName,
		Labels:  ucnfeLabels,
		NseName: nseName,
		NseControl: &nseconfig.NseControl{
			Name:               nseControlName,
			Address:            nseControlAddress,
			AccessToken:        nseControlAccessToken,
			ConnectivityDomain: nseControlConnectivityDomain,
		},
		VL3: nseconfig.VL3{
			IPAM: nseconfig.IPAM{
				DefaultPrefixPool: ipamDefaultPrefixPool,
				PrefixLength:      ipamPrefixLength,
				Routes:            ipamRoute,
				ServerAddress:     ipamServerAddress,
			},
			Ifname:      vl3Ifname,
			NameServers: vl3NameServers,
			DNSZones:    vl3DNSZones,
		},
	}
	return
}

func createNsmConnection() (conn *connection.Connection) {
	conn = &connection.Connection{
		Id:             connId,
		NetworkService: connNetworkService,
		Mechanism: &connection.Mechanism{
			Cls:  machanismCls,
			Type: mechanismType,
			Parameters: map[string]string{
				"description": paraDescription,
				"name":        paraName,
				"netnsInode":  paraNetnsInode,
				"socketfile":  paraSocketfile,
			},
		},
		Context: &connectioncontext.ConnectionContext{
			IpContext: &connectioncontext.IPContext{
				SrcIpAddr:     srcIP,
				DstIpAddr:     dstIP,
				SrcIpRequired: srcRequired,
				DstIpRequired: dstRequired,
				DstRoutes: []*connectioncontext.Route{
					{Prefix: dstRoutePrefix},
				},
			},
		},
		Labels: map[string]string{
			"namespace": labelsNamespace,
			"podName":   labelsPodName,
		},
		Path: &connection.Path{
			PathSegments: []*connection.PathSegment{
				{Name: pathSegmentName},
			},
		},
	}
	return
}

/*
assigning an empty interface to dpConfig to prevent crashing, it cannot be a null pointer
*/
func createVppDpConfig() (dpConfig *vpp.ConfigData) {
	dpConfig = &vpp.ConfigData{
		Interfaces: []*vpp_interfaces.Interface{},
	}
	return
}

func initUcnfEndpoint() (ucnfe UniversalCNFEndpoint) {
	ucnfe = UniversalCNFEndpoint{
		endpoint: createNseConfigEndpoint(),
		backend:  &MockUcnfBackend{},
		dpConfig: createVppDpConfig(),
	}
	return
}

func TestMain(m *testing.M) {
	runTests := m.Run()
	os.Exit(runTests)
}

/*
description is the log describing what the test do
positiveTest is a flag to determine whether positive or negative tests
*/
type ucnfBackendTest struct {
	mockBackend  *MockUcnfBackend
	description  string
	positiveTest bool
}

/*
run either positive or negative tests depending on the flag
 */
func runTest(g *GomegaWithT, err error, flag bool){
	if !flag{
		g.Expect(err).Should(HaveOccurred(), fmt.Sprintf("ERROR : %v", err))
	} else {
		g.Expect(err).ShouldNot(HaveOccurred(), fmt.Sprintf("ERROR : %v", err))
	}
}

/*
this test ensures that the Request function can get return values fromProcessEndpoint function and ProcessDPConfig function
*/
func TestRequest(t *testing.T) {
	g := NewWithT(t)
	ctx := context.TODO()
	r := &networkservice.NetworkServiceRequest{}
	ucnfe := initUcnfEndpoint()

	tables := []ucnfBackendTest{
		{mockBackend: &MockUcnfBackend{processDPConfig: func(dpconfig interface{}, update bool) error {
			return nil
		},
		}, description: "method ProcessDPConfig()  should return nil", positiveTest: true},
		{mockBackend: &MockUcnfBackend{processEndpoints: func(dpconfig interface{}, serviceName, ifName string, conn *connection.Connection) error {
			return nil
		},
		}, description: "method ProcessEndpoints() should return nil", positiveTest: true},
		{mockBackend: &MockUcnfBackend{processDPConfig: func(dpconfig interface{}, update bool) error {
			return fmt.Errorf("failed to run ProcessDPConfig() method")
		},
		}, description: "method ProcessDPConfig() should return err", positiveTest: false},
		{mockBackend: &MockUcnfBackend{processEndpoints: func(dpconfig interface{}, serviceName, ifName string, conn *connection.Connection) error {
			return fmt.Errorf("failed to run ProcessEndpoint() method")
		},
		}, description: "method ProcessEndpoints() should return err", positiveTest: false},
	}

	for _, table := range tables {
		ucnfe.backend = table.mockBackend
		logrus.Printf(table.description)
		_, err := ucnfe.Request(ctx, r)
		runTest(g, err, table.positiveTest)
	}
}

/*
this test ensures that the Close function can get return values of ProcessDPConfig function
*/
func TestClose(t *testing.T) {
	g := NewWithT(t)
	ctx := context.TODO()
	ucnfe := initUcnfEndpoint()
	conn := createNsmConnection()

	tables := []ucnfBackendTest{
		{mockBackend: &MockUcnfBackend{processDPConfig: func(dpconfig interface{}, update bool) error {
			return nil
		},
		}, description: "method ProcessDPConfig() should return nil", positiveTest: true},
		{mockBackend: &MockUcnfBackend{processDPConfig: func(dpconfig interface{}, update bool) error {
			return fmt.Errorf("failed to run ProcessDPConfig() method")
		},
		}, description: "method ProcessDPConfig() should return err", positiveTest: false},
	}

	for _, table := range tables {
		ucnfe.backend = table.mockBackend
		logrus.Printf(table.description)
		_, err := ucnfe.Close(ctx, conn)
		runTest(g, err, table.positiveTest)
	}
}

