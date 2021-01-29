---
apiVersion: v1
kind: ConfigMap
metadata:
  name: ucnf-nse-pass-through-icmp
  namespace: default
data:
  config.yaml: |
    endpoints:
    - name: "ucnf-nse-pass-through-nse"
      labels:
        app: "pass-through-icmp"
      nseControl:
        name: "pass-through-icmp"
        address: "vl3-service.wcm-cisco.com"
        connectivityDomain: "pass-through-nse-connectivity-domain"
      vl3:
        ipam:
          defaultPrefixPool: "192.168.0.0/16"
          prefixLength: 24
          routes: []
        ifName: "endpoint0"
---
# Source: pass-through/templates/pass-through-nse-ucnf.tpl
apiVersion: apps/v1
kind: Deployment
metadata:
  name: pass-through-icmp
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      networkservicemesh.io/app: "pass-through-icmp"
      networkservicemesh.io/impl: {{ .Values.nsm.serviceName | quote }}
  template:
    metadata:
      labels:
        networkservicemesh.io/app: "pass-through-icmp"
        networkservicemesh.io/impl: {{ .Values.nsm.serviceName | quote }}
    spec:
      serviceAccount: pass-through-nse-service-account
      containers:
        - name: ucnf-nse
{{/*          image: docker.io/ciscoappnetworking/vl3_ucnf-nse:master*/}}
          image: docker.io/darrenlau1227/vl3_ucnf-nse:test
          imagePullPolicy: Always
          ports:
          - name: monitoring-vpp
            containerPort: 9191
          - name: monitoring
            containerPort: 2112
          env:
            - name: ENDPOINT_NETWORK_SERVICE
              value: "pass-through-nse"
            - name: ENDPOINT_LABELS
              value: "app=pass-through-icmp"
            - name: TRACER_ENABLED
              value: "true"
            - name: JAEGER_SERVICE_HOST
              value: jaeger.nsm-system
            - name: JAEGER_SERVICE_PORT_JAEGER
              value: "6831"
            - name: JAEGER_AGENT_HOST
              value: jaeger.nsm-system
            - name: JAEGER_AGENT_PORT
              value: "6831"
            - name: NSREGISTRY_ADDR
              value: "nsmgr.nsm-system"
            - name: NSREGISTRY_PORT
              value: "5000"
            - name: INSECURE
              value: "true"
            - name: METRICS_PORT
              value: "2112"
            - name: STRICT_DECODING
              value: "true"
          securityContext:
            capabilities:
              add:
                - NET_ADMIN
            privileged: true
          resources:
            limits:
              networkservicemesh.io/socket: 1
          volumeMounts:
            - mountPath: /etc/universal-cnf/config.yaml
              subPath: config.yaml
              name: universal-cnf-config-volume
      volumes:
        - name: universal-cnf-config-volume
          configMap:
            name: ucnf-nse-pass-through-icmp
