package integration

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestAgentInSync(t *testing.T) {
	ctx := Setup(t)

	agent := ctx.SetupVPPAgent("agent")
	Expect(agent.configInSync()).To(BeTrue())
}

func TestStartStopMicroservice(t *testing.T) {
	ctx := Setup(t)

	agent := ctx.SetupVPPAgent("agent")
	ms := ctx.StartMicroservice("microservice1")

	msAvailable := func() bool {
		return agent.checkServiceAvailable(ms)
	}
	Eventually(msAvailable).Should(BeTrue())

	ms.stop()
	Eventually(msAvailable).Should(BeFalse())
}

func TestStartStopAgent(t *testing.T) {
	ctx := Setup(t)

	agent1 := ctx.SetupVPPAgent("agent1")
	agent2 := ctx.SetupVPPAgent("agent2")

	agent2Available := func() bool {
		return agent1.checkServiceAvailable(agent2)
	}
	Eventually(agent2Available).Should(BeTrue())

	agent2.stop()
	Eventually(agent2Available).Should(BeFalse())
}
