package integration_test

import (
	"testing"

	. "github.com/onsi/gomega"

	. "github.com/cisco-app-networking/nsm-nse/test/integration"
)

func TestAgentInSync(t *testing.T) {
	test := Setup(t)

	agent := test.SetupVPPAgent("agent")
	Expect(agent.ConfigInSync()).To(BeTrue())
}

func TestStartStopMicroservice(t *testing.T) {
	ctx := Setup(t)

	agent := ctx.SetupVPPAgent("agent")
	ms := ctx.SetupMicroservice("microservice1")

	msAvailable := func() bool {
		return agent.CheckServiceAvailable(ms)
	}
	Eventually(msAvailable).Should(BeTrue())

	ms.Stop()
	Eventually(msAvailable).Should(BeFalse())
}

func TestStartStopAgent(t *testing.T) {
	ctx := Setup(t)

	agent1 := ctx.SetupVPPAgent("agent1")
	agent2 := ctx.SetupVPPAgent("agent2")

	agent2Available := func() bool {
		return agent1.CheckServiceAvailable(agent2)
	}
	Eventually(agent2Available).Should(BeTrue())

	agent2.Stop()
	Eventually(agent2Available).Should(BeFalse())
}
