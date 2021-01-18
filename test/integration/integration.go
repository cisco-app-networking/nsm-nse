package integration

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/golang/protobuf/proto"
	. "github.com/onsi/gomega"
	"go.ligato.io/cn-infra/v2/logging"
	"go.ligato.io/vpp-agent/v3/client"
	"go.ligato.io/vpp-agent/v3/cmd/agentctl/api/types"
	"go.ligato.io/vpp-agent/v3/pkg/models"
	vpp_interfaces "go.ligato.io/vpp-agent/v3/proto/ligato/vpp/interfaces"

	agent "go.ligato.io/vpp-agent/v3/cmd/agentctl/client"
	"go.ligato.io/vpp-agent/v3/plugins/linux/ifplugin/linuxcalls"
	"go.ligato.io/vpp-agent/v3/proto/ligato/kvscheduler"
	ns "go.ligato.io/vpp-agent/v3/proto/ligato/linux/namespace"
	"go.ligato.io/vpp-agent/v3/tests/e2e"
)

// Setup sets up test environment and returns TestContext.
func Setup(t *testing.T) *TestContext {
	if testing.Short() {
		t.Skip("skipping integration tests in short mode")
	}

	tc := e2e.Setup(t)
	t.Cleanup(tc.Teardown)

	return &TestContext{
		tc: tc,
		t:  t,
	}
}

// TestContext holds context of a running test.
type TestContext struct {
	t  *testing.T
	tc *e2e.TestCtx
}

func (t *TestContext) T() *testing.T {
	return t.t
}

// SetupMicroservice starts a container with busybox image to be used in a test.
func (t *TestContext) SetupMicroservice(name string) *TestMicroservice {
	t.t.Helper()

	t.tc.StartMicroservice(name)

	return &TestMicroservice{
		t:    t,
		name: name,
	}
}

// SetupVPPAgent starts a container with VPP and Agent to be used in a test.
func (t *TestContext) SetupVPPAgent(name string) *TestVPPAgent {
	t.t.Helper()

	// start agent container
	ta := t.tc.StartAgent(name)

	// interact with the agent using the client
	c, err := agent.NewClient(ta.IPAddress())
	Expect(err).To(Succeed(), "Agent client error: %v", err)

	a := &TestVPPAgent{
		t:       t,
		name:    name,
		agent:   ta,
		client:  c,
		cliAddr: "127.0.0.1:5002",
		pingFn: func(addr string) error {
			return ta.Ping(addr)
		},
	}

	// wait to agent to start properly
	Eventually(a.checkReady).Should(Succeed(), "Agent should become ready")

	// run initial resync
	if _, err := syncAgent(c); err != nil {
		Expect(err).To(Succeed(), "Agent initial sync should succeed")
	}

	return a
}

func (t *TestContext) ShareDir() string {
	return defaultTestShareDir
}

type TestMicroservice struct {
	t    *TestContext
	name string
}

func (ms *TestMicroservice) Label() string {
	return microserviceLabel(ms.name)
}

func (ms *TestMicroservice) Stop() {
	ms.t.t.Helper()

	ms.t.tc.StopMicroservice(ms.name)
}

func (ms *TestMicroservice) Ping(addr string) error {
	ms.t.t.Helper()

	return ms.t.tc.PingFromMs(ms.name, addr)
}

type vppAgent interface {
	Exec(cmd string, args ...string) (string, string, error)
	LinuxInterfaceHandler() linuxcalls.NetlinkAPI
}

type Service interface {
	Label() string
}

type TestVPPAgent struct {
	t       *TestContext
	name    string
	cliAddr string
	agent   vppAgent
	client  agent.APIClient
	generic client.GenericClient
	pingFn  func(addr string) error
}

func (a *TestVPPAgent) Label() string {
	return a.name
}

func (a *TestVPPAgent) Stop() {
	a.t.t.Helper()

	a.t.tc.StopAgent(a.name)
}

func (a *TestVPPAgent) PingFromHost(addr string) error {
	a.t.t.Helper()

	return a.pingFn(addr)
}

func (a *TestVPPAgent) Exec(cmd string, args ...string) (string, error) {
	a.t.t.Helper()

	stdout, _, err := a.agent.Exec(cmd, args...)
	if err != nil {
		return "", fmt.Errorf("exec error: %v", err)
	}

	return stdout, nil
}

func (a *TestVPPAgent) VppCLI(action string, args ...string) (string, error) {
	a.t.t.Helper()

	cmd := append([]string{"-s", a.cliAddr, action}, args...)

	stdout, err := a.Exec("vppctl", cmd...)
	if err != nil {
		return "", fmt.Errorf("execute `vppctl %s` error: %v", strings.Join(cmd, " "), err)
	}

	return stdout, nil
}

func (a *TestVPPAgent) PingFromVPP(addr string) error {
	a.t.t.Helper()

	// run ping on VPP using vppctl
	stdout, err := a.VppCLI("ping", addr)
	if err != nil {
		return err
	}

	sent, recv, loss, err := parseVppPingOutput(stdout)
	if err != nil {
		return err
	}
	a.t.t.Logf("VPP ping %s: sent=%d, received=%d, loss=%d%%",
		addr, sent, recv, loss)

	if sent == 0 || loss >= 50 {
		return fmt.Errorf("failed to ping '%s': %s", addr, stdout)
	}

	return nil
}

func (a *TestVPPAgent) APIClient() agent.APIClient {
	return a.client
}

func (a *TestVPPAgent) checkReady() error {
	a.t.t.Helper()

	return checkAgentReady(a.APIClient())
}

func (a *TestVPPAgent) GenericClient() client.GenericClient {
	a.t.t.Helper()

	if a.generic == nil {
		var err error
		a.generic, err = newGenericClient(a.APIClient())
		Expect(err).To(Succeed())
	}
	return a.generic
}

func (a *TestVPPAgent) ConfigReplaceWith(items ...proto.Message) error {
	return a.GenericClient().ResyncConfig(items...)
}

func (a *TestVPPAgent) ConfigUpdate(items ...proto.Message) error {
	req := a.GenericClient().ChangeRequest()
	req.Update(items...)
	return req.Send(context.TODO())
}

func (a *TestVPPAgent) ConfigDelete(items ...proto.Message) error {
	req := a.GenericClient().ChangeRequest()
	req.Delete(items...)
	return req.Send(context.TODO())
}

func (a *TestVPPAgent) CheckServiceAvailable(svc Service) bool {
	msKey := ns.MicroserviceKey(svc.Label())
	state := a.t.tc.GetValueStateByKey(msKey)
	return state == kvscheduler.ValueState_OBTAINED
}

func (a *TestVPPAgent) InterfaceIsUp(iface *vpp_interfaces.Interface) bool {
	return a.getValueStateByKey(vpp_interfaces.LinkStateKey(iface.Name, true), "") == kvscheduler.ValueState_OBTAINED
}

func (a *TestVPPAgent) ValueIsConfigured(value proto.Message) bool {
	key := models.Key(value)
	return a.getValueStateByKey(key, "") == kvscheduler.ValueState_CONFIGURED
}

func (a *TestVPPAgent) ValueIsPending(value proto.Message) bool {
	key := models.Key(value)
	return a.getValueStateByKey(key, "") == kvscheduler.ValueState_PENDING
}

func (a *TestVPPAgent) ValueStateOf(value proto.Message) kvscheduler.ValueState {
	key := models.Key(value)
	return a.getValueStateByKey(key, "")
}

func (a *TestVPPAgent) ValueStateByKey(key string) kvscheduler.ValueState {
	return a.getValueStateByKey(key, "")
}

func (a *TestVPPAgent) DerivedValueStateByKey(baseValue proto.Message, derivedKey string) kvscheduler.ValueState {
	key := models.Key(baseValue)
	return a.getValueStateByKey(key, derivedKey)
}

func (a *TestVPPAgent) getValueStateByKey(key, derivedKey string) kvscheduler.ValueState {
	values, err := a.client.SchedulerValues(context.Background(), types.SchedulerValuesOptions{
		Key: key,
	})
	if err != nil {
		e := fmt.Errorf("Request to obtain value status has failed: %w", err)
		Expect(e).To(BeNil())
	}
	if len(values) != 1 {
		e := fmt.Errorf("Expected single value status, got status for %d values", len(values))
		Expect(e).To(BeNil())
	}
	st := values[0]
	if st.GetValue().GetKey() != key {
		e := fmt.Errorf("Received value status for unexpected key: %v", st)
		Expect(e).To(BeNil())
	}
	if derivedKey != "" {
		for _, derVal := range st.DerivedValues {
			if derVal.Key == derivedKey {
				return derVal.State
			}
		}
		return kvscheduler.ValueState_NONEXISTENT
	}
	return st.GetValue().GetState()
}

func (a *TestVPPAgent) ConfigInSync() bool {
	ops, err := syncAgent(a.APIClient())
	if err != nil {
		logging.Warnf("sync agent error: %v", err)
		return false
	}
	for _, op := range ops {
		if !op.NOOP {
			return false
		}
	}
	return true
}

// TODO: replace these with configurable options
const defaultTestShareDir = "/test-share"
const msNamePrefix = "e2e-test-ms-"

func microserviceLabel(msName string) string {
	return msNamePrefix + msName
}
