package e2e_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"go.ligato.io/vpp-agent/v3/proto/ligato/kvscheduler"
	ns "go.ligato.io/vpp-agent/v3/proto/ligato/linux/namespace"
	"go.ligato.io/vpp-agent/v3/tests/e2e"
)

const msNamePrefix = "e2e-test-"

func TestAgentInSync(t *testing.T) {
	ctx := e2e.Setup(t)
	defer ctx.Teardown()

	Expect(ctx.AgentInSync()).To(BeTrue())
}

func TestStartStopMicroservice(t *testing.T) {
	ctx := e2e.Setup(t)
	defer ctx.Teardown()

	const msName = "microservice1"
	key := ns.MicroserviceKey(msNamePrefix + msName)
	msState := func() kvscheduler.ValueState {
		return ctx.GetValueStateByKey(key)
	}

	ctx.StartMicroservice(msName)
	Eventually(msState).Should(Equal(kvscheduler.ValueState_OBTAINED))
	ctx.StopMicroservice(msName)
	Eventually(msState).Should(Equal(kvscheduler.ValueState_NONEXISTENT))
}
