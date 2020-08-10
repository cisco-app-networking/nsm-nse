# Kiknos VPN Endpoint NSE
This folder contains a Helm chart for the Kiknos VPN Endpoint NSE. This document describes the most
important parts that may be needed in order to tweak the NSE deployment.


## Important Helm options
The default values for Helm options present in [values.yaml](values.yaml) file can be generally preserved,
except the following options that usually have to be tweaked for each deployment:

| Helm Option                          | Example Value                      | Description |
| ------------------------------------ | ---------------------------------- | ----------- |
| `strongswan.network.localSubnet`     | `172.31.23.0/24`                   | Local IP subnet exposed via the IPSec tunnel to the remote peers. It can be a /32 NAT subnet (e.g. `1.2.3.4/32`) in case that NAT is enabled. |
| `strongswan.network.remoteSubnets`   | `{172.31.23.0/24,172.31.100.0/24}` | Array of remote IP subnets accessible via IPSec tunnels. This usually matches to an array of the local subnets of individual remote IPSec peers. |
| `strongswan.network.remoteAddr`      | `81.82.123.124`                    | (optional) IP address of the remote IPSec peer. Required only if this NSE acts as an IPSec client initiating a connection to the remote gateway/peer. Leave empty if this NSE acts as an IPSec gateway. |
| `strongswan.secrets.ikePreSharedKey` | `Vpp123`                           | Pre-shared-key used to authenticate with remote IPSec peers. |
| `nse.localSubnet`                    | `172.31.23.0/24`                   | Local NSE's IPAM prefix pool (subnet used to address the NSM links between the gateway and NSM-enabled pods). If NAT is *not* enabled, this is equal to `strongswan.network.localSubnet` |
| `nse.natIP`                          | `1.2.3.4`                          | (optional) If set, this enables source NAT on the IPSec interfaces. All traffic going from the cluster to the remote peers will be NATed to this IP. If enabled, `strongswan.network.localSubnet` needs to be set to `<this-IP>/32`. |


## StrongSwan configuration files
There are two StrongSwan configuration files embedded in this helm chart. Most often they don't have to be
touched, since they are parametrized using the aforementioned helm options, but the parts that are not exposed
via the helm options can still be tweaked if needed:

- [strongswan-cfg.yaml](templates/strongswan-cfg.yaml): contains generic StrongSwan configuration,
 such as the count of StrongSwan threads or timeout values,
- [responder-cfg.yaml](templates/responder-cfg.yaml): contains IPSec connection configuration,
 such as authentication and encryption algorithms.


## VPP startup configuration file
VPP startup configuration generally does not require any tweaks, but it is located in
[vpp-cfg.yaml](templates/vpp-cfg.yaml) file in case that any changes are required.


## NAT-enabled deployment
Source NAT on IPSec tunnel interfaces can be enabled with the helm option `nse.natIP`. Its value should contain
the requested outside NAT IP address, e.g. `nse.natIP=1.2.3.4`.

Enabling NAT results into the following configuration on the NSE VPP:

- each NSC-facing interface is configured as "NAT inside"
- each IPSec (IPIP) tunnel interface is configured as "NAT outside"

```
vpp# sh nat44 interfaces
NAT44 interfaces:
 memif1/0 in
 memif2/0 in
 memif3/0 in
 ipip0 out
```

NAT pool is configured to the NAT IP provided in the `nse.natIP` helm option:

```
vpp# sh nat44 addresses
NAT44 pool addresses:
1.2.3.4
  tenant VRF: 0
  0 busy other ports
  0 busy udp ports
  0 busy tcp ports
  0 busy icmp ports
```

At this point, every connection from inside (NSCs) to outside (remote IPSec peers) is source-NATed to the
configured NAT IP. Connections from outside to inside are not allowed (not to NAT IP nor NSC IPs).
To allow outside-to-inside connections, NSC has to request forwarding of a specific TCP/UDP port to them.
They can do it with lables in `networkservicemesh.io` annotations, e.g.
`ns.networkservicemesh.io: nsm-service-name?nat-port-forward-tcp=80` to forward TCP port 80, or
`ns.networkservicemesh.io: nsm-service-name?nat-port-forward-udp=53` to port forward UDP port 53.

That renders into a static nat44 mapping configuration on VPP, e.g.:

```
vpp# sh nat44 static mappings
NAT44 static mappings:
 tcp local 172.31.22.9:80 external 1.2.3.4:80 vrf 0  out2in-only
```

In our setup, the Istio NSC requests port forwarding on port 80 with
`ns.networkservicemesh.io: {{ .Values.nsm.serviceName }}?nat-port-forward-tcp=80`,
which means that remote IPSec peers can access the Istio ingress gateway running in the cluster at `http://1.2.3.4:80`.
