#!/bin/bash

COMMAND_ROOT=$(dirname "${BASH_SOURCE}")
print_usage() {
  echo "$(basename "$0")
Usage: $(basename "$0") [options...]
Options:
  --nsm-hub=STRING          Hub for NSM images
                            (default=\"tiswanso\", environment variable: NSM_HUB)
  --nsm-tag=STRING          Tag for NSM images
                            (default=\"vl3_api_rebase\", environment variable: NSM_TAG)
  --spire-disabled          Disable spire
  --secure                  Make NSM components run in secure mode.
" >&2
}

NSM_HUB="${NSM_HUB:-"ciscoappnetworking"}"
NSM_TAG="${NSM_TAG:-"vl3_latest"}"
INSTALL_OP=${INSTALL_OP:-apply}
INSECURE=${INSECURE:-true}

for i in "$@"
do
case $i in
    --nsm-hub=*)
    NSM_HUB="${i#*=}"
    ;;
    --nsm-tag=*)
    NSM_TAG="${i#*=}"
    ;;
    --spire-disabled)
    SPIRE_DISABLED="true"
    ;;
    --secure)
    INSECURE=false
    ;;
    -h|--help)
      print_usage
      exit 0
    ;;
    *)
      print_usage
      exit 1
    ;;
esac
done

function print_header() {
    local resource=$1; shift
    if [[ "${INSTALL_OP}" == "delete" ]]; then
      echo "------------ Uninstalling ${resource} -----------"
    else
      echo "------------ Installing ${resource} -------------"
    fi
}

sdir=$(dirname ${0})

NSMDIR=${NSMDIR:-${GOPATH}/src/github.com/cisco-app-networking/networkservicemesh}
VL3DIR=${VL3DIR:-${GOPATH}/src/github.com/cisco-app-networking/nsm-nse}

if [[ "${INSTALL_OP}" != "delete" ]]; then
  echo "------------- Create nsm-system namespace ----------"
  kubectl create ns nsm-system ${KCONF:+--kubeconfig $KCONF}
fi

# echo "------------Installing NSM monitoring-----------"
# helm template ${NSMDIR}/deployments/helm/nsm-monitoring --namespace nsm-system --set monSvcType=NodePort --set org=${NSM_HUB},tag=${NSM_TAG} | kubectl ${INSTALL_OP} ${KCONF:+--kubeconfig $KCONF} -f -

print_header "Crossconnect Monitor"
helm template ${NSMDIR}/deployments/helm/crossconnect-monitor \
  --namespace nsm-system \
  --set insecure=${INSECURE} \
  --set global.JaegerTracing="true" | kubectl ${INSTALL_OP} ${KCONF:+--kubeconfig $KCONF} -f -

print_header "Jaeger"
helm template ${NSMDIR}/deployments/helm/jaeger \
  --namespace nsm-system \
  --set insecure=${INSECURE} \
  --set global.JaegerTracing="true" \
  --set monSvcType=NodePort | kubectl ${INSTALL_OP} ${KCONF:+--kubeconfig $KCONF} -f -

print_header "Skydive"
helm template ${NSMDIR}/deployments/helm/skydive \
  --namespace nsm-system \
  --set insecure=${INSECURE} \
  --set global.JaegerTracing="true" | kubectl ${INSTALL_OP} ${KCONF:+--kubeconfig $KCONF} -f -


# kubednsip=$(kubectl get svc -n kube-system ${KCONF:+--kubeconfig $KCONF} | grep kube-dns | awk '{ print $3 }')
# kinddnsip=$(kubectl get svc ${KCONF:+--kubeconfig $KCONF} | grep kind-dns | awk '{ print $3 }')

print_header "NSM"
helm template ${NSMDIR}/deployments/helm/nsm \
  --namespace nsm-system \
  --set org=${NSM_HUB},tag=${NSM_TAG} \
  --set admission-webhook.org=${NSM_HUB},admission-webhook.tag=${NSM_TAG} \
  --set pullPolicy=Always \
  --set spire.selfSignedCA="false" \
  --set insecure=${INSECURE} \
  --set global.JaegerTracing="true" \
  ${SPIRE_DISABLED:+--set spire.enabled=false} | kubectl ${INSTALL_OP} ${KCONF:+--kubeconfig $KCONF} -f -

print_header "NSM-addons"
helm template ${VL3DIR}/deployments/helm/nsm-addons \
  --namespace nsm-system \
  --set global.NSRegistrySvc=true | kubectl ${INSTALL_OP} ${KCONF:+--kubeconfig $KCONF} -f -

#helm template ${NSMDIR}/deployments/helm/nsm --namespace nsm-system --set global.JaegerTracing=true --set org=${NSM_HUB},tag=${NSM_TAG} --set pullPolicy=Always --set admission-webhook.org=tiswanso --set admission-webhook.tag=vl3-inter-domain2 --set admission-webhook.pullPolicy=Always --set admission-webhook.dnsServer=${kubednsip} ${kinddnsip:+--set "admission-webhook.dnsAltZones[0].zone=example.org" --set "admission-webhook.dnsAltZones[0].server=${kinddnsip}"} --set global.NSRegistrySvc=true --set global.NSMApiSvc=true --set global.NSMApiSvcPort=30501 --set global.NSMApiSvcAddr="0.0.0.0:30501" --set global.NSMApiSvcType=NodePort --set global.ExtraDnsServers="${kubednsip} ${kinddnsip}" --set global.OverrideNsmCoreDns="true" | kubectl ${INSTALL_OP} ${KCONF:+--kubeconfig $KCONF} -f -
#helm template ${NSMDIR}/deployments/helm/nsm --namespace nsm-system --set global.JaegerTracing=true --set org=${NSM_HUB},tag=${NSM_TAG} --set pullPolicy=Always --set admission-webhook.org=tiswanso --set admission-webhook.tag=vl3-inter-domain2 --set admission-webhook.pullPolicy=Always --set global.NSRegistrySvc=true --set global.NSMApiSvc=true --set global.NSMApiSvcPort=30501 --set global.NSMApiSvcAddr="0.0.0.0:30501" --set global.NSMApiSvcType=NodePort --set global.ExtraDnsServers="${kubednsip} ${kinddnsip}" --set global.OverrideDnsServers="${kubednsip} ${kinddnsip}" | kubectl ${INSTALL_OP} ${KCONF:+--kubeconfig $KCONF} -f -

print_header "proxy NSM"
helm template ${NSMDIR}/deployments/helm/proxy-nsmgr \
  --namespace nsm-system \
  --set org=${NSM_HUB},tag=${NSM_TAG} \
  --set pullPolicy=Always \
  --set insecure=${INSECURE} \
  --set global.JaegerTracing="true" \
  ${REMOTE_NSR_PORT:+ --set remoteNsrPort=${REMOTE_NSR_PORT}} | kubectl ${INSTALL_OP} ${KCONF:+--kubeconfig $KCONF} -f -

#helm template ${NSMDIR}/deployments/helm/proxy-nsmgr --namespace nsm-system --set global.JaegerTracing=true --set org=${NSM_HUB},tag=${NSM_TAG} --set pullPolicy=Always --set global.NSMApiSvc=true --set global.NSMApiSvcPort=30501 --set global.NSMApiSvcAddr="0.0.0.0:30501" | kubectl ${INSTALL_OP} ${KCONF:+--kubeconfig $KCONF} -f -

if [[ "${INSTALL_OP}" == "delete" ]]; then
  echo "------------- Delete nsm-system ns ----------------"
  kubectl delete ns nsm-system ${KCONF:+--kubeconfig $KCONF}
fi
