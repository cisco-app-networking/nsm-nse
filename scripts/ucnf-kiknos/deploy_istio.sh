#!/usr/bin/env bash

function print_usage() {
    echo "$(basename "$0") - Deploy an Istio service mesh. All properties can also be provided through env variables

NOTE: The defaults will change to the env values for the ones set.

Usage: $(basename "$0") [options...]
Options:
  --cluster     Cluster name            env var: CLUSTER    - (Default: $CLUSTER)
  --help -h     Help
" >&2

}

for i in "$@"; do
  case $i in
  --cluster=*)
    CLUSTER="${i#*=}"
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

pushd "$(dirname "${BASH_SOURCE[0]}")/../.." || exit 1

echo "Installing Istio control plane"
kubectl --context "$CLUSTER" apply -f ./deployments/k8s/ucnf-kiknos/istio_cfg.yaml

sleep 2

kubectl --context "$CLUSTER" wait -n istio-system --timeout=150s --for condition=Ready --all pods || {
    ec=$?
    echo "kubectl wait failed, returned code ${ec}.  Gathering data"
    kubectl cluster-info dump --all-namespaces --context "${CLUSTER}" --output-directory=/tmp/error_logs_istio_cfg/
    kubectl get pods -A --context "${CLUSTER}"
    kubectl describe pod --context "${CLUSTER}" -n=istio-system
    exit ${ec}
  }
