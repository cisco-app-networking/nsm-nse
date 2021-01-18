package integration

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/golang/protobuf/proto"
	. "github.com/onsi/gomega"
	"go.ligato.io/cn-infra/v2/health/statuscheck/model/status"
	"go.ligato.io/cn-infra/v2/logging"
	"go.ligato.io/vpp-agent/v3/client"
	"go.ligato.io/vpp-agent/v3/client/remoteclient"
	"go.ligato.io/vpp-agent/v3/cmd/agentctl/api/types"
	"go.ligato.io/vpp-agent/v3/pkg/models"
	"go.ligato.io/vpp-agent/v3/plugins/kvscheduler/api"
	vpp_interfaces "go.ligato.io/vpp-agent/v3/proto/ligato/vpp/interfaces"

	agentctl "go.ligato.io/vpp-agent/v3/cmd/agentctl/client"
	"go.ligato.io/vpp-agent/v3/plugins/linux/ifplugin/linuxcalls"
	"go.ligato.io/vpp-agent/v3/proto/ligato/kvscheduler"
	ns "go.ligato.io/vpp-agent/v3/proto/ligato/linux/namespace"
	"go.ligato.io/vpp-agent/v3/tests/e2e"
)

// TestContext holds context of a running test.
type TestContext struct {
	t  *testing.T
	tc *e2e.TestCtx
}

// Setup sets up test environment.
func Setup(t *testing.T) *TestContext {
	if testing.Short() {
		t.Skip("skipping e2e tests in short mode")
	}

	tc := e2e.Setup(t)
	t.Cleanup(tc.Teardown)

	return &TestContext{
		tc: tc,
		t:  t,
	}
}

func (t *TestContext) StartMicroservice(name string) *microservice {
	t.tc.StartMicroservice(name)
	return &microservice{
		ctx:  t,
		name: name,
	}
}

func (t *TestContext) SetupVPPAgent(name string, opts ...e2e.AgentOptModifier) *vppAgent {
	t.t.Helper()

	// prepare options
	options := e2e.DefaultAgentOpt()
	for _, optionsModifier := range opts {
		optionsModifier(options)
	}

	// start agent container
	a := t.tc.StartAgent(name, opts...) // not passing prepared options due to public visibility

	// interact with the agent using the client from agentctl
	c, err := agentctl.NewClient(a.IPAddress())
	Expect(err).To(Succeed(), "Agent APIClient error: %v", err)

	agent := &vppAgent{
		ctx:    t,
		name:   name,
		agent:  a,
		client: c,
		ping: func(addr string) error {
			return a.Ping(addr)
		},
	}

	// wait to agent to start properly
	Eventually(agent.checkAgentReady).Should(Succeed(), "Agent should have become ready")

	// run initial resync
	if !options.NoManualInitialResync {
		if _, err := syncAgent(c); err != nil {
			Expect(err).To(Succeed(), "Agent sync should succeed")
		}
	}

	return agent
}

type microservice struct {
	ctx  *TestContext
	name string
}

func (ms *microservice) ping(addr string) error {
	return ms.ctx.tc.PingFromMs(ms.name, addr)
}

func (ms *microservice) stop() {
	ms.ctx.tc.StopMicroservice(ms.name)
}

func (ms *microservice) label() string {
	return microserviceLabel(ms.name)
}

type Agent interface {
	Exec(cmd string, args ...string) (string, string, error)
	LinuxInterfaceHandler() linuxcalls.NetlinkAPI
}

type vppAgent struct {
	ctx    *TestContext
	name   string
	agent  Agent
	client agentctl.APIClient
	ping   func(targetAddr string) error
}

func (a *vppAgent) stop() {
	a.ctx.tc.StopAgent(a.name)
}

func (a *vppAgent) label() string {
	return a.name
}

func (a *vppAgent) APIClient() agentctl.APIClient {
	return a.client
}

func (a *vppAgent) pingFromVPP(addr string) error {
	a.ctx.t.Helper()

	// run ping on VPP using vppctl
	stdout, err := a.vppctl("ping", addr)
	if err != nil {
		return err
	}

	// parse output
	matches := vppPingRegexp.FindStringSubmatch(stdout)
	sent, recv, loss, err := parsePingOutput(stdout, matches)
	if err != nil {
		return err
	}
	a.ctx.t.Logf("VPP ping %s: sent=%d, received=%d, loss=%d%%",
		addr, sent, recv, loss)

	if sent == 0 || loss >= 50 {
		return fmt.Errorf("failed to ping '%s': %s", addr, matches[0])
	}
	return nil
}

var vppPingRegexp = regexp.MustCompile("Statistics: ([0-9]+) sent, ([0-9]+) received, ([0-9]+)% packet loss")

func parsePingOutput(output string, matches []string) (sent int, recv int, loss int, err error) {
	if len(matches) != 4 {
		err = fmt.Errorf("unexpected output from ping: %s", output)
		return
	}
	sent, err = strconv.Atoi(matches[1])
	if err != nil {
		err = fmt.Errorf("failed to parse the sent packet count: %v", err)
		return
	}
	recv, err = strconv.Atoi(matches[2])
	if err != nil {
		err = fmt.Errorf("failed to parse the received packet count: %v", err)
		return
	}
	loss, err = strconv.Atoi(matches[3])
	if err != nil {
		err = fmt.Errorf("failed to parse the loss percentage: %v", err)
		return
	}
	return
}

func (a *vppAgent) configSet(items ...proto.Message) error {
	c := genericClient(a)
	return c.ResyncConfig(items...)
}

func (a *vppAgent) configUpdate(items ...proto.Message) error {
	c := genericClient(a)
	req := c.ChangeRequest()
	req.Update(items...)
	return req.Send(context.TODO())
}

func (a *vppAgent) configDelete(items ...proto.Message) error {
	c := genericClient(a)
	req := c.ChangeRequest()
	req.Delete(items...)
	return req.Send(context.TODO())
}

func (a *vppAgent) configChangeRequest(fn func(req client.ChangeRequest)) error {
	c := genericClient(a)
	req := c.ChangeRequest()
	fn(req)
	return req.Send(context.TODO())
}

func genericClient(a *vppAgent) client.GenericClient {
	conn, err := a.APIClient().GRPCConn()
	if err != nil {
		Expect(err).To(Succeed())
	}
	c, err := remoteclient.NewClientGRPC(conn)
	if err != nil {
		Expect(err).To(Succeed())
	}
	return c
}

func (a *vppAgent) exec(cmd string, args ...string) (string, error) {
	a.ctx.t.Helper()

	stdout, _, err := a.agent.Exec(cmd, args...)
	if err != nil {
		return "", fmt.Errorf("exec error: %v", err)
	}
	return stdout, nil
}

func (a *vppAgent) vppctl(action string, args ...string) (string, error) {
	a.ctx.t.Helper()

	cmd := append([]string{"-s", "127.0.0.1:5002", action}, args...)

	stdout, err := a.exec("vppctl", cmd...)
	if err != nil {
		return "", fmt.Errorf("execute `vppctl %s` error: %v", strings.Join(cmd, " "), err)
	}

	return stdout, nil
}

func (a *vppAgent) interfaceIsUp(iface *vpp_interfaces.Interface) bool {
	return a.getValueStateByKey(vpp_interfaces.LinkStateKey(iface.Name, true), "") == kvscheduler.ValueState_OBTAINED
}

func (a *vppAgent) valueIsConfigured(value proto.Message) bool {
	key := models.Key(value)
	return a.getValueStateByKey(key, "") == kvscheduler.ValueState_CONFIGURED
}

func (a *vppAgent) valueIsPending(value proto.Message) bool {
	key := models.Key(value)
	return a.getValueStateByKey(key, "") == kvscheduler.ValueState_PENDING
}

func (a *vppAgent) valueStateOf(value proto.Message) kvscheduler.ValueState {
	key := models.Key(value)
	return a.getValueStateByKey(key, "")
}

func (a *vppAgent) GetValueStateByKey(key string) kvscheduler.ValueState {
	return a.getValueStateByKey(key, "")
}

func (a *vppAgent) GetDerivedValueState(baseValue proto.Message, derivedKey string) kvscheduler.ValueState {
	key := models.Key(baseValue)
	return a.getValueStateByKey(key, derivedKey)
}

func (a *vppAgent) getValueStateByKey(key, derivedKey string) kvscheduler.ValueState {
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

// syncAgent runs downstream resync and returns the list of executed operations.
func syncAgent(client agentctl.APIClient) (executed api.RecordedTxnOps, err error) {
	txn, err := client.SchedulerResync(context.Background(), types.SchedulerResyncOptions{
		Retry: true,
	})
	if err != nil {
		Expect(err).To(Succeed())
		return nil, fmt.Errorf("resync request has failed: %w", err)
	}
	if txn.Start.IsZero() {
		return nil, fmt.Errorf("resync returned empty transaction record: %v", txn)
	}
	return txn.Executed, nil
}

func (a *vppAgent) configInSync() bool {
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

func (a *vppAgent) checkAgentReady() error {
	c := a.APIClient()
	agentStatus, err := c.Status(context.Background())
	if err != nil {
		return fmt.Errorf("query to get agent status failed: %v", err)
	}
	agent, ok := agentStatus.PluginStatus["VPPAgent"]
	if !ok {
		return fmt.Errorf("agent status missing")
	}
	if agent.State != status.OperationalState_OK {
		return fmt.Errorf("agent status: %v", agent.State.String())
	}
	return nil
}

type service interface {
	label() string
}

func (a *vppAgent) checkServiceAvailable(svc service) bool {
	msKey := ns.MicroserviceKey(svc.label())
	state := a.ctx.tc.GetValueStateByKey(msKey)
	return state == kvscheduler.ValueState_OBTAINED
}

// TODO: switch to e2e.MsNamePrefix when it gets exported
const defaultTestShareDir = "/test-share"
const msNamePrefix = "e2e-test-ms-"

func microserviceLabel(msName string) string {
	return msNamePrefix + msName
}
