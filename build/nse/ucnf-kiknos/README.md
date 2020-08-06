# Kiknos base VPN NSE

This replaces the VPP agent in the universal-cnf with the Kiknos VPP aio-agent

**Navigation**
* [Prerequisites](#prerequisites)
* [Scenarios](#scenarios)
    * [Direct connection between workloads and NSE](#direct-connection-between-workloads-and-nse)
    * [Using an Ingress Gateway as NSC](#using-an-ingress-gateway-as-nsc)
* [Deploy the environment](#deploy-the-environment)
    * [Using Make](#using-make)
    * [Using scripts + commands](#using-scripts--commands)
        * [Deploying the K8s clusters](#deploying-the-k8s-clusters)
        * [Deploy the NSM](#deploy-the-nsm)
        * [Client side deployment](#client-side-deployment)
        * [Deploy IPSec Gateway and Remote Clients](#deploy-ipsec-gateway-and-remote-clients)
* [Check connectivity between workloads (Scenario 1)](#check-connectivity-between-workloads-scenario-1)
* [Check connectivity to workloads through the Istio gateway (Scenario 2)](#check-connectivity-to-workloads-through-the-istio-gateway-scenario-2)
* [Dumping the state of the cluster](#dumping-the-state-of-the-cluster)
* [Cleanup](#cleanup)
* [Known Issues](#known-issues)


# Prerequisites
You first need to clone the [Network Service Mesh repo](https://github.com/networkservicemesh/networkservicemesh)
Please follow the instructions on where the NSM project should be in the [README.md](../../README.md)

- helm v2.16.3
- kubectl v1.18.2

##### Kind deployment
- kind v0.7.0 

##### AWS deployment
- aws-cli v2.0.11
- eksctl v0.18.0
- python >= 2.7

 
 # Scenarios
 ## Direct connection between workloads and NSE
 In this case, the NSE connects directly to the workloads (pods) deployed on the cluster. 
 With this configuration, all the workloads can be accessed from outside the cluster using the IP address provided by 
 the NSM through a secure IP Sec tunnel.  
 
 ## Using an Ingress Gateway as NSC
 Expose the workloads through an [Istio](https://istio.io/) Ingress Gateway. In this case the Ingress Gateway will act 
 as a network service client. With this configuration, it is easy to expose workloads through a common entry point. This 
 allows for the case where you want all workloads to be exposed through a single IP address and differentiate between them.
 This case provides better configurability of the k8s cluster, but adds a layer of complexity with the introduction of Istio.

# Deploy the environment

## Using Make

* Deploy kind environment:
```bash
make deploy-kiknos-clients CLUSTER=kiknos-demo-1
make deploy-kiknos-start-vpn BUILD_IMAGE=false DEPLOY_ISTIO=false CLUSTER=kiknos-demo-2 CLUSTER_REF=kiknos-demo-1 
```

* Deploy on aws:
```bash
make deploy-kiknos-clients AWS=true CLUSTER=kiknos-demo-1
make deploy-kiknos-start-vpn AWS=true BUILD_IMAGE=false DEPLOY_ISTIO=false CLUSTER=kiknos-demo-2 CLUSTER_REF=kiknos-demo-1
```

* Deploy ASAv with istio
```bash
make deploy-asa AWS=true CLUSTER=kiknos-demo-asa
```

By default, most of the deployment scripts depend on `k8s-ucnf-kiknos-save` rule. 
This rule builds a docker image. To control the registry and tag use `ORG` and `TAG` options. 
If you know that the image is already present you can skip this step with `BUILD_IMAGE=false` as in example above. 

Makefile consists of the following rules: 
* *provide-image*: Builds docker image and pushes or executes a `kind-load`.
* *docker-push*: Pushes the docker image to the registry specified in `ORG`. 
* *create-cluster*: Creates a cluster whether kind or aws (if set `AWS=true`). use `CLUSTER` variable for cluster name.
* *helm-init*: Deploys Tiller for helm version 2.x.
* *helm-install-nsm*: Installs NSM.
* *deploy-kiknos*: Deploys the kiknos NSEs.
* *deploy-istio*: Installs Istio. Used for the VPN client cluster.
* *deploy-kiknos-clients*: Deploys worker pods as NSC in both clusters.
    * When `DEPLOY_ISTIO=true` Deploys the Istio ingress gateway as a NSC in the VPN client cluster.
    * When `DEPLOY_ISTIO=true` Deploys two pods that will be exposed through an Istio virtual service to the ingress gateway.
* *deploy-kiknos-start-vpn*: Starts kiknos ipsec.
>Note: Only the AWS target will perform the following steps
* *deploy-asa*: Deploys an ASAv and Ubuntu EC2 instances to act as Kiknos clients. The Ubuntu client needs to be manually configured.

>Note: Make sure to check the deletion of items in AWS as they can sometimes be redeployed

Makefile options:

- `CLUSTER` - Set the cluster name - (Default: `kiknos-demo-1`)
- `ORG` - Set the org of new built image - (Default: `tiswanso`)
- `TAG` - Set the tag of new built image - (Default: `kiknos`)
- `AWS_KEY_PAIR` - AWS Key Pair for connecting over SSH - (Default: `kiknos-asa`)
- `CLUSTER_REF` - Reference cluster required when deploying the second cluster in
    order to be able to take some configurations such as remote IP address when configuring kiknos NSE or the AWS VPC - (Default: *None*)
- `VPP_AGENT` - Parent image for the NSE - (Default: `ciscolabs/kiknos:latest`)
- `FORWARDING_PLANE` - Set a default forwarding plane - (Default: `vpp`)
- `NETWORK_SERVICE` - Set a default network service for Example clients - (Default: `hello-world`)
- `BUILD_IMAGE` - Set whether to build the image or not - (Default: `true`)
- `PROVISION_MODE` - Set the mode to provision the built image. Default "push"
    one of "push" or "kind-load"
    not relevant if $BUILD_IMAGE is not true - (Default: `push`)
- `DEPLOY_ISTIO` - Set whether to deploy istio gateway or not - (Default: `true`)
- `AWS` - Create aws cluster - (Default: `false`)
- `SUBNET_IP` - IP for the remote ASA subnet (without the mask, ex: 192.168.254.0) - (Default: `192.168.254.0`)

## Using scripts + commands

### Deploying the K8s clusters

1. KinD:
    ```bash
   
    # First cluster, which will act as the IPSec client
    kind create cluster --name kiknos-demo-1
    kubectl config rename-context kind-kiknos-demo-1 kiknos-demo-1
   
    # Second cluster, on which will act as an IPSec gateway
    kind create cluster --name kiknos-demo-2
    kubectl config rename-context kind-kiknos-demo-1 kiknos-demo-2
    ```

1. AWS:
    
    Exec:
    ```bash
    # First cluster, which will act as the IPSec client
    python scripts/ucnf-kiknos/pyaws/create_cluster.py --name kiknos-demo-1 --open-sg
    aws eks update-kubeconfig --name=kiknos-demo-1 --alias kiknos-demo-1
   
    # Second cluster, on which will act as an IPSec gateway
    python scripts/ucnf-kiknos/pyaws/create_cluster.py --name kiknos-demo-2 --ref kiknos-demo-1 --open-sg
    aws eks update-kubeconfig --name=kiknos-demo-2 --alias kiknos-demo-2
    ```
   Help:
   ```bash
   python scripts/ucnf-kiknos/pyaws/create_cluster.py --help
   usage: create_cluster.py [-h] --name NAME [--region REGION] [--ref REF]
                            [--cidr CIDR] [--test] [--open-sg]
   
   Utility for dealing with AWS clusters
   
   optional arguments:
     -h, --help       show this help message and exit
     --name NAME      Member cluster name to create config for.
     --region REGION  Member cluster region
     --ref REF        Reference cluster name (client cluster will use reference
                      clusters vpc when is created)
     --cidr CIDR      Client cluster name to create config yaml for.
     --test           Dump generated config
     --open-sg        Open all ports and all ips for SecurityGroups

   ```
       
### Deploy the NSM

```bash
# Move to the NSM directory and execute the install scripts:
cd ${GOPATH}/src/github.com/networkservicemesh/networkservicemesh
./scripts/helm-init-wrapper.sh
./scripts/helm-nsm-install.sh --chart nsm \
		--container_repo networkservicemesh \
		--container_tag master \
		--forwarding_plane vpp \
		--insecure true \
		--networkservice hello-world \
		--enable_prometheus true \
		--enable_metric_collection true \
		--nsm_namespace nsm-system \
		--spire_enabled false
# Move back to the NSE repo
cd ${GOPATH}/src/github.com/cisco-app-networking/nsm-nse

```
   
### Client side deployment

1. Kiknos NSE
    
    Exec:
    ```bash
    ./scripts/ucnf-kiknos/deploy_kiknos.sh --cluster=kiknos-demo-1 --subnet-ip=192.168.254.0 --org=tiswanso --tag=kiknos --service-name=hello-world
    ```
   Help:
   ```bash
   ./scripts/ucnf-kiknos/deploy_kiknos_nse.sh --help
   deploy_kiknos_nse.sh - Deploy the Kiknos NSE. All properties can also be provided through env variables
   
   NOTE: The defaults will change to the env values for the ones set.
   
   Usage: deploy_kiknos_nse.sh [options...]
   Options:
     --cluster             Cluster name (context)                                              env var: CLUSTER          - (Default: )
     --cluster-ref         Reference to pair cluster name (context)                            env var: CLUSTER_REF      - (Default: )
     --nse-org             Docker image org                                                    env var: NSE_ORG          - (Default: mmatache)
     --nse-tag             Docker image tag                                                    env var: NSE_TAG          - (Default: kiknos)
     --pull-policy         Pull policy for the NSE image                                       env var: PULL_POLICY      - (Default: IfNotPresent)
     --service-name        NSM service                                                         env var: SERVICE_NAME     - (Default: hello-world)
     --delete              Delete NSE                                                          env var: DELETE           - (Default: false)
     --subnet-ip           IP for the remote subnet (without the mask, ex: 192.168.254.0)      env var: SUBNET_IP        - (Default: 192.168.254.0)
     --help -h             Help
   ```
1. Client pods:
    
    1. With Istio:
        
        Exec:
        ```bash
        # Deploys the istio components
        ./scripts/ucnf-kiknos/deploy_istio.sh --cluster=kiknos-demo-1
        # Deploys the client pods
        ./scripts/ucnf-kiknos/deploy_clients.sh --cluster=kiknos-demo-1 --service-name=hello-world --istio-client
        ```
        Help:
        ```bash
        ./scripts/ucnf-kiknos/deploy_istio.sh --help
        deploy_istio.sh - Deploy an Istio service mesh. All properties can also be provided through env variables
        
        NOTE: The defaults will change to the env values for the ones set.
        
        Usage: deploy_istio.sh [options...]
        Options:
          --cluster     Cluster name            env var: CLUSTER    - (Default: )
          --help -h     Help
        ```
        ```bash
        ./scripts/ucnf-kiknos/deploy_clients.sh --help
        ~/go/src/github.com/cisco-app-networking ~/go/src/github.com/cisco-app-networking/nsm-nse
        deploy_clients.sh - Deploy NSM Clients. All properties can also be provided through env variables
        
        NOTE: The defaults will change to the env values for the ones set.
        
        Usage: deploy_clients.sh [options...]
        Options:
          --cluster             Cluster name                                                        env var: CLUSTER         - (Default: )
          --service-name        NSM service                                                         env var: SERVICE_NAME    - (Default: hello-world)
          --istio-client        If an istio client should be deployed instead of a regular client   env var: ISTIO_CLIENT    - (Default: false)
          --delete
          --help -h             Help
        ```
       
    1. Without Istio:
    
        Exec:
        ```bash
        # Deploys the client pods
        ./scripts/ucnf-kiknos/deploy_clients.sh --cluster=kiknos-demo-1 --service-name=hello-world
        ```
        Help:
        ```bash
        ./scripts/ucnf-kiknos/deploy_clients.sh --help
        ~/go/src/github.com/cisco-app-networking ~/go/src/github.com/cisco-app-networking/nsm-nse
        deploy_clients.sh - Deploy NSM Clients. All properties can also be provided through env variables
        
        NOTE: The defaults will change to the env values for the ones set.
        
        Usage: deploy_clients.sh [options...]
        Options:
          --cluster             Cluster name                                                        env var: CLUSTER         - (Default: )
          --service-name        NSM service                                                         env var: SERVICE_NAME    - (Default: hello-world)
          --istio-client        If an istio client should be deployed instead of a regular client   env var: ISTIO_CLIENT    - (Default: false)
          --delete
          --help -h             Help
        ```
       
    

### Deploy IPSec Gateway and Remote Clients:
    
1. Kiknos:

    Exec:
    ```bash
    # Deploy Kiknos NSE to act as a gateway
    ./scripts/ucnf-kiknos/deploy_kiknos.sh --cluster=kiknos-demo-2 --org=tiswanso --tag=kiknos --cluster-ref=kiknos-demo-1 --service-name=hello-world
    # Deploys the client pods
    ./scripts/ucnf-kiknos/deploy_clients.sh --cluster=kiknos-demo-2 --service-name=hello-world
    ```
    Help:
    ```bash
    ./scripts/ucnf-kiknos/deploy_kiknos_nse.sh --help
    deploy_kiknos_nse.sh - Deploy the Kiknos NSE. All properties can also be provided through env variables
    
    NOTE: The defaults will change to the env values for the ones set.
    
    Usage: deploy_kiknos_nse.sh [options...]
    Options:
    --cluster             Cluster name (context)                                              env var: CLUSTER          - (Default: )
    --cluster-ref         Reference to pair cluster name (context)                            env var: CLUSTER_REF      - (Default: )
    --nse-org             Docker image org                                                    env var: NSE_ORG          - (Default: mmatache)
    --nse-tag             Docker image tag                                                    env var: NSE_TAG          - (Default: kiknos)
    --pull-policy         Pull policy for the NSE image                                       env var: PULL_POLICY      - (Default: IfNotPresent)
    --service-name        NSM service                                                         env var: SERVICE_NAME     - (Default: hello-world)
    --delete              Delete NSE                                                          env var: DELETE           - (Default: false)
    --subnet-ip           IP for the remote subnet (without the mask, ex: 192.168.254.0)      env var: SUBNET_IP        - (Default: 192.168.254.0)
    --help -h             Help
    ```
1. ASA
   >:warning: Only works for AWS deployments
   
   An ASAv and an Ubuntu client are deployed in AWS.Then an additional IPSec tunnel gets created which connects to the Kiknos deployment.
   
   The ASA uses a [day0](./scripts/day0.txt) configuration in order to be able to create the IPSec tunnel.
           
   Exec:
   ```bash
   ./scripts/ucnf-kiknos/deploy_asa.sh --cluster-ref=kiknos-demo-1 --subnet-ip=192.168.254.0 --aws-key-pair=kiknos-asa
   ```
   Help:
   ```bash
   ./scripts/ucnf-kiknos/deploy_asa.sh --help
   deploy_asa.sh - Deploy ASAv to act as a Gateway and Ubuntu EC2 instance to act as a client.
                   All properties can also be provided through env variables
   
   NOTE: The defaults will change to the env values for the ones set.
   
   Usage: deploy_asa.sh [options...]
   Options:
     --cluster-ref         Reference to cluster to connect to                                  env var: CLUSTER_REF      - (Default: )
     --aws-key-pair        AWS Key Pair for connecting over SSH                                env var: AWS_KEY_PAIR     - (Default: kiknos-asa)
     --subnet-ip           IP for the remote ASA subnet (without the mask, ex: 192.168.254.0)  env var: SUBNET_IP        - (Default: 192.168.254.0)
     --help -h             Help
   ```
       
          
   

## Check connectivity between workloads (Scenario 1)
In order to check the connectivity between worker pods run the following make target:
```bash
./scripts/ucnf-kiknos/test_vpn_conn.sh --cluster1=kiknos-demo-1 --cluster2=kiknos-demo-2 
```
This will attempt to perform `curl` commands from the workers in the VPN Gateway cluster to the workers in the VPN client cluster directly.

Output:
```bash
CLUSTER1=kiknos-demo-1 CLUSTER2=kiknos-demo-2 /home/mihai/go/src/github.com/cisco-app-networking/nsm-nse/scripts/ucnf-kiknos/test_vpn_conn.sh
Detected pod with nsm interface ip: 172.31.22.5
Detected pod with nsm interface ip: 172.31.22.1
Hello version: v1, instance: helloworld-ucnf-client-7bd94648d-d2gbh 
Hello version: v1, instance: helloworld-ucnf-client-7bd94648d-nksbm
Hello version: v1, instance: helloworld-ucnf-client-7bd94648d-d2gbh
Hello version: v1, instance: helloworld-ucnf-client-7bd94648d-nksbm
```

## Check connectivity to workloads through the Istio gateway (Scenario 2)
In order to check the connectivity to workloads through the Istio gateway run the following make target:
```bash
./scripts/ucnf-kiknos/test_istio_vpn_conn.sh --cluster1=kiknos-demo-1 --cluster2=kiknos-demo-2
```
This will attempt to perform `curl` commands from the workers in the VPN Gateway cluster to the Istio ingress gateway.
This allows a user to connect to different services by keeping the same IP address for each call and differentiating 
between workloads by specifying different ports or URL paths. 

In our current example, we deploy 2 services, and expose each service twice. In all cases, the request goes through the
IP address of the Ingress gateway that was supplied by the NSE.

Output:
```bash
./scripts/ucnf-kiknos/test_istio_vpn_conn.sh --cluster1=kiknos-demo-1 --cluster2=kiknos-demo-2 
Detected pod with nsm interface ip: 172.31.22.9
------------------------- Source pod/helloworld-ucnf-client-d7d79cf54-dzs4m -------------------------
Connecting to: http://172.31.22.9/hello
no healthy upstreamConnecting to: http://172.31.22.9/hello-v2
no healthy upstreamConnecting to: http://172.31.22.9:8000/hello
no healthy upstreamConnecting to: http://172.31.22.9:8000/hello-v2
no healthy upstream-----------------------------------------------------------------------------------------------------
------------------------- Source pod/helloworld-ucnf-client-d7d79cf54-qx786 -------------------------
Connecting to: http://172.31.22.9/hello
no healthy upstreamConnecting to: http://172.31.22.9/hello-v2
no healthy upstreamConnecting to: http://172.31.22.9:8000/hello
no healthy upstreamConnecting to: http://172.31.22.9:8000/hello-v2
no healthy upstream-----------------------------------------------------------------------------------------------------

```


## Dumping the state of the cluster
In order to dump the clusters state run the following make target:

```bash
make kiknos-dump-clusters-state
```

Alternatively run the script:
```bash
./scripts/ucnf-kiknos/dump_clusters_state.sh
```

This will give you information regarding:
- state of the meaningful pods in the system
- state of the NSM provisioned interfaces
 

# Cleanup
The following commands will delete the kind clusters.

```bash
make clean CLUSTER=kiknos-demo-1
make clean CLUSTER=kiknos-demo-2
```

If on AWS:

```bash
make clean AWS=true CLUSTER=kiknos-demo-1
make clean AWS=true CLUSTER=kiknos-demo-2
```

# Known issues
These issues require further investigation:

1. If the NSE restarts it loses all the interfaces  - in case of test failure dump the state of the clusters and check 
for restarts on the VPN pods in the default namespace
