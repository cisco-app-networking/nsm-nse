package e2e_test

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"

	"go.ligato.io/vpp-agent/v3/proto/ligato/kvscheduler"
	linux_interfaces "go.ligato.io/vpp-agent/v3/proto/ligato/linux/interfaces"
	linux_namespace "go.ligato.io/vpp-agent/v3/proto/ligato/linux/namespace"
	vpp_interfaces "go.ligato.io/vpp-agent/v3/proto/ligato/vpp/interfaces"
	"go.ligato.io/vpp-agent/v3/tests/e2e"
)

// TestInterfaceAfPacket connects VPP with a microservice via AF-PACKET + VETH interfaces
func TestInterfaceAfPacket(t *testing.T) {
	// 0. Setup e2e test
	ctx := e2e.Setup(t)
	defer ctx.Teardown()

	// 1. Prepare configuration data
	const (
		afPacketName  = "vpp-afpacket"
		veth1Name     = "linux-veth1"
		veth2Name     = "linux-veth2"
		veth1Hostname = "veth1"
		veth2Hostname = "veth2"
		afPacketIP    = "192.168.1.1"
		veth2IP       = "192.168.1.2"
		netMask       = "/30"
		msName        = "microservice1"
	)
	afPacket := &vpp_interfaces.Interface{
		Name:        afPacketName,
		Type:        vpp_interfaces.Interface_AF_PACKET,
		Enabled:     true,
		IpAddresses: []string{afPacketIP + netMask},
		Link: &vpp_interfaces.Interface_Afpacket{
			Afpacket: &vpp_interfaces.AfpacketLink{
				HostIfName: veth1Hostname,
			},
		},
	}
	veth1 := &linux_interfaces.Interface{
		Name:       veth1Name,
		Type:       linux_interfaces.Interface_VETH,
		Enabled:    true,
		HostIfName: veth1Hostname,
		Link: &linux_interfaces.Interface_Veth{
			Veth: &linux_interfaces.VethLink{
				PeerIfName: veth2Name,
			},
		},
	}
	veth2 := &linux_interfaces.Interface{
		Name:        veth2Name,
		Type:        linux_interfaces.Interface_VETH,
		Enabled:     true,
		HostIfName:  veth2Hostname,
		IpAddresses: []string{veth2IP + netMask},
		Link: &linux_interfaces.Interface_Veth{
			Veth: &linux_interfaces.VethLink{
				PeerIfName: veth1Name,
			},
		},
		Namespace: &linux_namespace.NetNamespace{
			Type:      linux_namespace.NetNamespace_MICROSERVICE,
			Reference: msNamePrefix + msName,
		},
	}

	// 2. Start microservice
	ctx.StartMicroservice(msName)

	// 3. Apply configuration
	req := ctx.GenericClient().ChangeRequest()
	err := req.Update(
		afPacket,
		veth1,
		veth2,
	).Send(context.Background())
	Expect(err).ToNot(HaveOccurred())

	// 4. Assert expected state
	Eventually(ctx.GetValueStateClb(afPacket)).Should(Equal(kvscheduler.ValueState_CONFIGURED))
	Expect(ctx.GetValueState(veth1)).To(Equal(kvscheduler.ValueState_CONFIGURED))
	Expect(ctx.GetValueState(veth2)).To(Equal(kvscheduler.ValueState_CONFIGURED))
	Expect(ctx.AgentInSync()).To(BeTrue())

	// 5. Run pings from VPP to/from microservice
	Expect(ctx.PingFromVPP(veth2IP)).To(Succeed())
	Expect(ctx.PingFromMs(msName, afPacketIP)).To(Succeed())
}
