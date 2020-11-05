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

// the mock implementer of the interface UniversalCNFBackend
type MockUcnfBackendProcessDPConfig struct {
}

/*
these are the implementations of the ucnf endpoint backend interface, and these mock functions are used for testing
the ProcessDPconfig function call
*/
func (m *MockUcnfBackendProcessDPConfig) NewDPConfig() *vpp.ConfigData {
	return &vpp.ConfigData{}
}

func (m *MockUcnfBackendProcessDPConfig) NewUniversalCNFBackend() error {
	return nil
}

func (m *MockUcnfBackendProcessDPConfig) ProcessClient(dpconfig interface{}, ifName string, conn *connection.Connection) error {
	return nil
}

func (m *MockUcnfBackendProcessDPConfig) ProcessEndpoint(dpconfig interface{}, serviceName, ifName string, conn *connection.Connection) error {
	return nil

}

func (m *MockUcnfBackendProcessDPConfig) ProcessDPConfig(dpconfig interface{}, update bool) error {
	return fmt.Errorf("failed to run ProcessDPConfig() method")
}

// mock implementor
type MockUcnfBackendReturnNil struct {
}

/*
these are the mock functions that will return nil only, which means that the Request() and Close() functions will
run till the end without getting errors from ProcessEndpoint and ProcessDPConfig function calls
*/
func (m *MockUcnfBackendReturnNil) NewDPConfig() *vpp.ConfigData {
	return &vpp.ConfigData{}
}

func (m *MockUcnfBackendReturnNil) NewUniversalCNFBackend() error {
	return nil
}

func (m *MockUcnfBackendReturnNil) ProcessClient(dpconfig interface{}, ifName string, conn *connection.Connection) error {
	return nil
}

func (m *MockUcnfBackendReturnNil) ProcessEndpoint(dpconfig interface{}, serviceName, ifName string, conn *connection.Connection) error {
	return nil
}

func (m *MockUcnfBackendReturnNil) ProcessDPConfig(dpconfig interface{}, update bool) error {
	return nil
}

// mock implementor
type MockUcnfBackendProcessEndpoint struct {
}

/*
these are the implementations of the ucnf endpoint backend interface
mock functions are used for testingthe ProcessEndpoint function call
*/
func (m *MockUcnfBackendProcessEndpoint) NewDPConfig() *vpp.ConfigData {
	return &vpp.ConfigData{}
}

func (m *MockUcnfBackendProcessEndpoint) NewUniversalCNFBackend() error {
	return nil
}

func (m *MockUcnfBackendProcessEndpoint) ProcessClient(dpconfig interface{}, ifName string, conn *connection.Connection) error {
	return nil
}

func (m *MockUcnfBackendProcessEndpoint) ProcessEndpoint(dpconfig interface{}, serviceName, ifName string, conn *connection.Connection) error {
	return fmt.Errorf("failed to run ProcessEndpoint function")
}

func (m *MockUcnfBackendProcessEndpoint) ProcessDPConfig(dpconfig interface{}, update bool) error {
	return nil
}

func createUcnfEndpoint() (e *nseconfig.Endpoint) {
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

func createConnection() (conn *connection.Connection) {
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
func createDpConfig() (dpConfig *vpp.ConfigData) {
	dpConfig = &vpp.ConfigData{
		Interfaces: []*vpp_interfaces.Interface{},
	}
	return
}

func initUcnfEndpoint() (ucnfe UniversalCNFEndpoint) {
	ucnfe = UniversalCNFEndpoint{
		endpoint: createUcnfEndpoint(),
		backend:  &MockUcnfBackendReturnNil{},
		dpConfig: createDpConfig(),
	}
	return
}

func TestMain(m *testing.M) {
	runTests := m.Run()
	os.Exit(runTests)
}

/*
this test ensures that the Request function can get return values fromProcessEndpoint function
and ProcessDPConfig function
*/
func TestRequest(t *testing.T) {
	g := NewWithT(t)
	ctx := context.TODO()
	r := &networkservice.NetworkServiceRequest{}
	ucnfe := initUcnfEndpoint()

	logrus.Println("all functions return nil")
	_, err := ucnfe.Request(ctx, r)
	g.Expect(err).ShouldNot(HaveOccurred(), fmt.Sprintf("ERROR : %v", err))

	// Deploy backend that contains mock functions returning err
	ucnfe.backend = &MockUcnfBackendProcessEndpoint{}
	logrus.Println("ProcessEndpoint function should return err")
	_, err = ucnfe.Request(ctx, r)
	g.Expect(err).Should(HaveOccurred(), fmt.Sprintf("ERROR: %v", err))

	ucnfe.backend = &MockUcnfBackendProcessDPConfig{}
	logrus.Println("ProcessDPConfig function should return err")
	_, err = ucnfe.Request(ctx, r)
	g.Expect(err).Should(HaveOccurred(), fmt.Sprintf("ERROR: %v", err))

}

/*
this test ensures that the Close function can get return values of ProcessDPConfig function
*/
func TestClose(t *testing.T) {
	g := NewWithT(t)
	ctx := context.TODO()
	ucnfe := initUcnfEndpoint()
	conn := createConnection()

	logrus.Println("all functions return nil")
	_, err := ucnfe.Close(ctx, conn)
	g.Expect(err).ShouldNot(HaveOccurred(), fmt.Sprintf("ERROR : %v", err))

	logrus.Println("ProcessDPconfig should return err")
	ucnfe.backend = &MockUcnfBackendProcessDPConfig{}
	_, err = ucnfe.Close(ctx, conn)
	g.Expect(err).Should(HaveOccurred(), fmt.Sprintf("ERROR : %v", err))

}
