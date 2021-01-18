package integration

import (
	"path"
	"testing"

	. "github.com/onsi/gomega"
	vpp_l2 "go.ligato.io/vpp-agent/v3/proto/ligato/vpp/l2"

	"go.ligato.io/vpp-agent/v3/proto/ligato/kvscheduler"
	linux_interfaces "go.ligato.io/vpp-agent/v3/proto/ligato/linux/interfaces"
	linux_namespace "go.ligato.io/vpp-agent/v3/proto/ligato/linux/namespace"
	vpp_interfaces "go.ligato.io/vpp-agent/v3/proto/ligato/vpp/interfaces"
)

// TestInterfaceAfPacket connects VPP with a microservice via AF-PACKET + VETH interfaces
func TestInterfaceAfPacket(t *testing.T) {
	// Setup test environment
	test := Setup(t)

	agent := test.SetupVPPAgent("agent")
	ms := test.StartMicroservice("microservice")

	// Prepare configuration
	const (
		veth1Name  = "linux-veth1"
		veth2Name  = "linux-veth2"
		afPacketIP = "192.168.1.1"
		veth2IP    = "192.168.1.2"
	)
	afPacket := &vpp_interfaces.Interface{
		Name:        "vpp-afpacket",
		Type:        vpp_interfaces.Interface_AF_PACKET,
		Enabled:     true,
		IpAddresses: []string{afPacketIP + ("/30")},
		Link: &vpp_interfaces.Interface_Afpacket{
			Afpacket: &vpp_interfaces.AfpacketLink{
				HostIfName: "veth1",
			},
		},
	}
	veth1 := &linux_interfaces.Interface{
		Name:       veth1Name,
		Type:       linux_interfaces.Interface_VETH,
		Enabled:    true,
		HostIfName: "veth1",
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
		HostIfName:  "veth2",
		IpAddresses: []string{veth2IP + ("/30")},
		Link: &linux_interfaces.Interface_Veth{
			Veth: &linux_interfaces.VethLink{
				PeerIfName: veth1Name,
			},
		},
		Namespace: &linux_namespace.NetNamespace{
			Type:      linux_namespace.NetNamespace_MICROSERVICE,
			Reference: ms.label(),
		},
	}

	// Apply configuration
	Expect(agent.configUpdate(afPacket, veth1, veth2)).To(Succeed())

	// Assert expected state
	afPacketState := func() kvscheduler.ValueState {
		return agent.valueStateOf(afPacket)
	}
	Eventually(afPacketState).Should(Equal(kvscheduler.ValueState_CONFIGURED))
	Expect(agent.valueStateOf(veth1)).To(Equal(kvscheduler.ValueState_CONFIGURED))
	Expect(agent.valueStateOf(veth2)).To(Equal(kvscheduler.ValueState_CONFIGURED))
	Expect(agent.configInSync()).To(BeTrue())

	// Run pings from VPP to/from microservice
	Expect(agent.pingFromVPP(veth2IP)).To(Succeed())
	Expect(ms.ping(afPacketIP)).To(Succeed())
}

func TestInterfaceAfPacketVNF(t *testing.T) {
	// Setup test environment
	test := Setup(t)

	clientService := test.StartMicroservice("client")
	appService := test.StartMicroservice("app")
	vswitchAgent := test.SetupVPPAgent("vswitch")
	vnfAgent := test.SetupVPPAgent("vnf")

	// Prepare configuration
	const (
		VETH1a          = "VETH_1a"
		VETH1b          = "VETH_1b"
		VETH2a          = "VETH_2a"
		VETH2b          = "VETH_2b"
		clientServiceIP = "192.168.1.1"
		appServiceIP    = "192.168.1.2"
	)
	var (
		memifSocket = path.Join(defaultTestShareDir, "memif.sock")
	)

	// Configure vswitch agent
	{
		afpacket1 := &vpp_interfaces.Interface{
			Name:    "AFPACKET_1",
			Type:    vpp_interfaces.Interface_AF_PACKET,
			Enabled: true,
			Link: &vpp_interfaces.Interface_Afpacket{
				Afpacket: &vpp_interfaces.AfpacketLink{
					HostIfName: "veth1a",
				},
			},
		}
		veth1a := &linux_interfaces.Interface{
			Name:       VETH1a,
			Type:       linux_interfaces.Interface_VETH,
			Enabled:    true,
			HostIfName: "veth1a",
			Link: &linux_interfaces.Interface_Veth{
				Veth: &linux_interfaces.VethLink{
					PeerIfName: VETH1b,
				},
			},
		}
		veth1b := &linux_interfaces.Interface{
			Name:        VETH1b,
			Type:        linux_interfaces.Interface_VETH,
			Enabled:     true,
			HostIfName:  "veth1b",
			IpAddresses: []string{clientServiceIP + "/30"},
			Link: &linux_interfaces.Interface_Veth{
				Veth: &linux_interfaces.VethLink{
					PeerIfName: VETH1a,
				},
			},
			Namespace: &linux_namespace.NetNamespace{
				Type:      linux_namespace.NetNamespace_MICROSERVICE,
				Reference: clientService.label(),
			},
		}
		afpacket2 := &vpp_interfaces.Interface{
			Name:    "AFPACKET_2",
			Type:    vpp_interfaces.Interface_AF_PACKET,
			Enabled: true,
			Link: &vpp_interfaces.Interface_Afpacket{
				Afpacket: &vpp_interfaces.AfpacketLink{
					HostIfName: "veth2a",
				},
			},
		}
		veth2a := &linux_interfaces.Interface{
			Name:       VETH2a,
			Type:       linux_interfaces.Interface_VETH,
			Enabled:    true,
			HostIfName: "veth2a",
			Link: &linux_interfaces.Interface_Veth{
				Veth: &linux_interfaces.VethLink{
					PeerIfName: VETH2b,
				},
			},
		}
		veth2b := &linux_interfaces.Interface{
			Name:        VETH2b,
			Type:        linux_interfaces.Interface_VETH,
			Enabled:     true,
			HostIfName:  "veth2b",
			IpAddresses: []string{appServiceIP + "/30"},
			Link: &linux_interfaces.Interface_Veth{
				Veth: &linux_interfaces.VethLink{
					PeerIfName: VETH2a,
				},
			},
			Namespace: &linux_namespace.NetNamespace{
				Type:      linux_namespace.NetNamespace_MICROSERVICE,
				Reference: appService.label(),
			},
		}
		memif1 := &vpp_interfaces.Interface{
			Name:    "MEMIF_1",
			Type:    vpp_interfaces.Interface_MEMIF,
			Enabled: true,
			Mtu:     1500,
			Link: &vpp_interfaces.Interface_Memif{
				Memif: &vpp_interfaces.MemifLink{
					Id:             1,
					Master:         true,
					SocketFilename: memifSocket,
				},
			},
		}
		memif2 := &vpp_interfaces.Interface{
			Name:    "MEMIF_2",
			Type:    vpp_interfaces.Interface_MEMIF,
			Enabled: true,
			Mtu:     1500,
			Link: &vpp_interfaces.Interface_Memif{
				Memif: &vpp_interfaces.MemifLink{
					Id:             2,
					Master:         true,
					SocketFilename: memifSocket,
				},
			},
		}
		xconn1 := &vpp_l2.XConnectPair{
			ReceiveInterface:  afpacket1.Name,
			TransmitInterface: memif1.Name,
		}
		xconn2 := &vpp_l2.XConnectPair{
			ReceiveInterface:  afpacket2.Name,
			TransmitInterface: memif2.Name,
		}
		xconn3 := &vpp_l2.XConnectPair{
			ReceiveInterface:  memif1.Name,
			TransmitInterface: afpacket1.Name,
		}
		xconn4 := &vpp_l2.XConnectPair{
			ReceiveInterface:  memif2.Name,
			TransmitInterface: afpacket2.Name,
		}
		bd := &vpp_l2.BridgeDomain{
			Name:                "INTERNAL_SWITCH",
			Flood:               true,
			Forward:             true,
			Learn:               true,
			UnknownUnicastFlood: true,
		}

		// Apply configuration
		Expect(vswitchAgent.configUpdate(
			afpacket1, veth1a, veth1b,
			afpacket2, veth2a, veth2b,
			memif1, memif2, bd,
			xconn1, xconn2, xconn3, xconn4,
		)).To(Succeed())

		agentConfigured := func() bool {
			return vswitchAgent.valueIsConfigured(afpacket1) && vswitchAgent.valueIsConfigured(afpacket2)
		}
		Eventually(agentConfigured).Should(BeTrue(), "Agent should be configured")
		Expect(vswitchAgent.configInSync()).To(BeTrue())
	}

	// Configure vnf agent
	{
		memif1 := &vpp_interfaces.Interface{
			Name:    "MEMIF_1x",
			Type:    vpp_interfaces.Interface_MEMIF,
			Enabled: true,
			Mtu:     1500,
			Link: &vpp_interfaces.Interface_Memif{
				Memif: &vpp_interfaces.MemifLink{
					Id:             1,
					SocketFilename: memifSocket,
				},
			},
		}
		memif2 := &vpp_interfaces.Interface{
			Name:    "MEMIF_2x",
			Type:    vpp_interfaces.Interface_MEMIF,
			Enabled: true,
			Mtu:     1500,
			Link: &vpp_interfaces.Interface_Memif{
				Memif: &vpp_interfaces.MemifLink{
					Id:             2,
					SocketFilename: memifSocket,
				},
			},
		}
		xconn1 := &vpp_l2.XConnectPair{
			ReceiveInterface:  memif1.Name,
			TransmitInterface: memif2.Name,
		}
		xconn2 := &vpp_l2.XConnectPair{
			ReceiveInterface:  memif2.Name,
			TransmitInterface: memif1.Name,
		}

		// Configure agent
		Expect(vnfAgent.configUpdate(memif1, memif2, xconn1, xconn2)).To(Succeed())
		agentConfigured := func() bool {
			return vnfAgent.valueIsConfigured(memif1) && vnfAgent.valueIsConfigured(memif2)
		}
		Eventually(agentConfigured).Should(BeTrue())

		// Wait until interfaces are UP
		interfacesUp := func() bool {
			return vnfAgent.interfaceIsUp(memif1) && vnfAgent.interfaceIsUp(memif2)
		}
		Eventually(interfacesUp).Should(BeTrue())

		Expect(vnfAgent.configInSync()).To(BeTrue())
	}

	// Run ping from client to app
	Expect(clientService.ping(appServiceIP)).To(Succeed(), "Client should be able to ping app")
	Expect(appService.ping(clientServiceIP)).To(Succeed(), "App should be able to ping client")
}
