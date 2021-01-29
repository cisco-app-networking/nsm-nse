package integration

import (
	"context"
	"fmt"
	"regexp"
	"strconv"

	"go.ligato.io/cn-infra/v2/health/statuscheck/model/status"
	"go.ligato.io/vpp-agent/v3/client"
	"go.ligato.io/vpp-agent/v3/client/remoteclient"
	"go.ligato.io/vpp-agent/v3/cmd/agentctl/api/types"
	agent "go.ligato.io/vpp-agent/v3/cmd/agentctl/client"
	"go.ligato.io/vpp-agent/v3/plugins/kvscheduler/api"
)

var (
	reVppPing = regexp.MustCompile("Statistics: ([0-9]+) sent, ([0-9]+) received, ([0-9]+)% packet loss")
)

func parseVppPingOutput(output string) (sent int, recv int, loss int, err error) {
	matches := reVppPing.FindStringSubmatch(output)
	if len(matches) != 4 {
		err = fmt.Errorf("unexpected output from ping: %s", output)
		return
	}
	if sent, err = strconv.Atoi(matches[1]); err != nil {
		err = fmt.Errorf("failed to parse the sent packet count: %v", err)
		return
	}
	if recv, err = strconv.Atoi(matches[2]); err != nil {
		err = fmt.Errorf("failed to parse the received packet count: %v", err)
		return
	}
	if loss, err = strconv.Atoi(matches[3]); err != nil {
		err = fmt.Errorf("failed to parse the loss percentage: %v", err)
		return
	}
	return
}

// syncAgent runs downstream resync and returns the list of executed operations.
func syncAgent(c agent.APIClient) (executed api.RecordedTxnOps, err error) {
	txn, err := c.SchedulerResync(context.Background(), types.SchedulerResyncOptions{
		Retry: true,
	})
	if err != nil {
		return nil, fmt.Errorf("resync request has failed: %w", err)
	}
	if txn.Start.IsZero() {
		return nil, fmt.Errorf("resync returned empty txn record: %v", txn)
	}
	return txn.Executed, nil
}

func newGenericClient(client agent.APIClient) (client.GenericClient, error) {
	conn, err := client.GRPCConn()
	if err != nil {
		return nil, err
	}
	c, err := remoteclient.NewClientGRPC(conn)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func checkAgentReady(client agent.APIClient) error {
	agentStatus, err := client.Status(context.Background())
	if err != nil {
		return fmt.Errorf("query to get agent status failed: %v", err)
	}
	a, ok := agentStatus.PluginStatus["VPPAgent"]
	if !ok {
		return fmt.Errorf("agent status missing")
	}
	if a.State != status.OperationalState_OK {
		return fmt.Errorf("agent status: %v", a.State.String())
	}
	return nil
}
