#!/usr/bin/env bash

NSE_ORG=${NSE_ORG:-ciscoappnetworking}
NSE_TAG=${NSE_TAG:-ucnf-kiknos-vppagent}
PULL_POLICY=${PULL_POLICY:-IfNotPresent}
SERVICE_NAME=${SERVICE_NAME:-hello-world}
DELETE=${DELETE:-false}
OPERATION=${OPERATION:-apply}
SUBNET_IP=${SUBNET_IP:-192.168.254.0}
NSE_NAT_IP=${NSE_NAT_IP:-""}

function print_usage() {
    echo "$(basename "$0") - Deploy the Kiknos NSE. All properties can also be provided through env variables

NOTE: The defaults will change to the env values for the ones set.

Usage: $(basename "$0") [options...]
Options:
  --cluster             Cluster name (context)                                              env var: CLUSTER          - (Default: $CLUSTER)
  --cluster-ref         Reference to pair cluster name (context)                            env var: CLUSTER_REF      - (Default: $CLUSTER_REF)
  --nse-org             Docker image org                                                    env var: NSE_ORG          - (Default: $NSE_ORG)
  --nse-tag             Docker image tag                                                    env var: NSE_TAG          - (Default: $NSE_TAG)
  --pull-policy         Pull policy for the NSE image                                       env var: PULL_POLICY      - (Default: $PULL_POLICY)
  --service-name        NSM service                                                         env var: SERVICE_NAME     - (Default: $SERVICE_NAME)
  --delete              Delete NSE                                                          env var: DELETE           - (Default: $DELETE)
  --subnet-ip           IP for the remote subnet (without the mask, ex: 192.168.254.0)      env var: SUBNET_IP        - (Default: $SUBNET_IP)
  --nse-nat-ip          If set, enables source NAT on the IPSec interfaces to this IP.      env var: NSE_NAT_IP       - (Default: $NSE_NAT_IP)
  --help -h             Help
" >&2
}

for i in "$@"; do
  case $i in
  --nse-org=*)
    NSE_ORG="${i#*=}"
    ;;
  --nse-tag=*)
    NSE_TAG="${i#*=}"
    ;;
  --cluster=*)
    CLUSTER="${i#*=}"
    ;;
  --cluster-ref=*)
    CLUSTER_REF="${i#*=}"
    ;;
  --pull-policy=*)
    PULL_POLICY="${i#*=}"
    ;;
  --service-name=*)
    SERVICE_NAME="${i#*=}"
    ;;
  --subnet-ip=*)
    SUBNET_IP="${i#*=}"
    ;;
  --nse-nat-ip=*)
    NSE_NAT_IP="${i#*=}"
    ;;
  --delete)
    OPERATION=delete
    ;;
  -h | --help)
    print_usage
    exit 0
    ;;
  *)
    print_usage
    exit 1
    ;;
  esac
done

[[ -z "$CLUSTER" ]] && echo "env var: CLUSTER is required!" && print_usage && exit 1

pushd "$(dirname "${BASH_SOURCE[0]}")/../../" || exit 1

# Perform the given kubectl operation for the NSE
function performNSE() {
  local cluster=$1; shift
  local opts=$*
  echo "apply NSE into cluster: $cluster"
  helm template ./deployments/helm/ucnf-kiknos/kiknos_vpn_endpoint \
    --set org="$NSE_ORG" \
    --set tag="$NSE_TAG" \
    --set pullPolicy="$PULL_POLICY" \
    --set nsm.serviceName="$SERVICE_NAME" $opts | kubectl --context "$cluster" $OPERATION -f -

  CONDITION="condition=Ready"
  if [[ ${OPERATION} = delete ]]; then
    CONDITION="delete"
  fi

  sleep 10
  if [[ ${OPERATION} != delete ]]; then
    echo "Waiting for kiknos job responder-cfg-job to be complete:"
    kubectl wait --context "$cluster" -n default --timeout=150s --for condition=complete job/responder-cfg-job || {
      ec=$?
      echo "kubectl wait for responder-cfg-job to complete  failed, returned code ${ec}."
    }
  fi

  echo "Waiting for kiknos pods condition to be '$CONDITION':"
  echo "Pod labels: k8s-app=kiknos-etcd and networkservicemesh.io/app=${SERVICE_NAME}"
  kubectl wait --context "$cluster" -n default --timeout=30s --for ${CONDITION} --all pods -l k8s-app=kiknos-etcd || {
    ec=$?
    if [[ ${OPERATION} != delete ]]; then
        echo "kubectl wait for ${CONDITION} failed, returned code ${ec}.  Gathering data"
        kubectl cluster-info dump --all-namespaces --context "${cluster}" --output-directory=/tmp/error_logs_kiknos_etcd/
        kubectl get pods -A --context "${cluster}"
        kubectl describe pod --context "${cluster}" \
            -l k8s-app=kiknos-etcd -n=default
        exit ${ec}
    fi
  }
  kubectl --context "$cluster" wait -n default --timeout=150s --for ${CONDITION} --all pods -l networkservicemesh.io/app=${SERVICE_NAME} || {
    ec=$?
    if [[ ${OPERATION} != delete ]]; then
      echo "kubectl wait failed, returned code ${ec}.  Gathering data"
      kubectl cluster-info dump --all-namespaces --output-directory=/tmp/error_logs_nsm_io/
      kubectl get pods -A --context "${cluster}"
      kubectl describe pod --context "${cluster}" \
          -l networkservicemesh.io/app=${SERVICE_NAME} -n=default
      exit ${ec}
    fi
  }

  if [[ ${OPERATION} = delete ]]; then
    echo "Delete '${SERVICE_NAME}' network service if exists"
    kubectl --context "$cluster" delete networkservices.networkservicemesh.io ${SERVICE_NAME}
  fi
}

if [[ -n "$CLUSTER_REF" ]]; then
    POD_NAME=$(kubectl --context "$CLUSTER_REF" get pods -o name | grep endpoint | cut -d / -f 2)
    IP_ADDR=$(kubectl --context "$CLUSTER_REF" exec -it "$POD_NAME" -- ip addr | grep -E "global (dynamic )?eth0" | grep inet | awk '{print $2}' | cut -d / -f 1)

    SSWAN_REMOTE_SUBNETS="{172.31.22.0/24}"
    if [[ -n "${NSE_NAT_IP}" ]]; then
      SSWAN_REMOTE_SUBNETS="{${NSE_NAT_IP}/32}"
    fi
    performNSE "$CLUSTER" --set strongswan.network.remoteAddr="$IP_ADDR" \
      --set nse.localSubnet=172.31.23.0/24 \
      --set strongswan.network.localSubnet=172.31.23.0/24 \
      --set strongswan.network.remoteSubnets="${SSWAN_REMOTE_SUBNETS}"
    exit 0
fi

SSWAN_LOCAL_SUBNET="172.31.22.0/24"
if [[ -n "${NSE_NAT_IP}" ]]; then
  SSWAN_LOCAL_SUBNET="${NSE_NAT_IP}/32"
fi
performNSE "$CLUSTER" \
  --set nse.localSubnet=172.31.22.0/24 \
  --set strongswan.network.localSubnet="${SSWAN_LOCAL_SUBNET}" \
  --set strongswan.network.remoteSubnets="{172.31.23.0/24,$SUBNET_IP/24}" \
  --set nse.natIP="${NSE_NAT_IP}"
