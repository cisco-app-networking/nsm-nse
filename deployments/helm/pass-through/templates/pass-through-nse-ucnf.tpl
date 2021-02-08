---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: pass-through-nse-{{ .Values.nsm.serviceName }}
  namespace: {{ .Release.Namespace }}
spec:
  replicas: {{ .Values.replicaCount }}
  selector:
    matchLabels:
      networkservicemesh.io/app: "pass-through-nse-{{ .Values.nsm.serviceName }}"
      networkservicemesh.io/impl: {{ .Values.nsm.serviceName | quote }}
  template:
    metadata:
      labels:
        networkservicemesh.io/app: "pass-through-nse-{{ .Values.nsm.serviceName }}"
        networkservicemesh.io/impl: {{ .Values.nsm.serviceName | quote }}
    spec:
      containers:
        - name: pass-through-nse
          image: {{ .Values.registry }}/{{ .Values.org }}/pass-through-nse:{{ .Values.tag }}
          imagePullPolicy: {{ .Values.pullPolicy }}
          ports:
          - name: monitoring-vpp
            containerPort: {{ .Values.vppMetricsPort }}
          - name: monitoring
            containerPort: {{ .Values.metricsPort }}
          env:
            - name: ENDPOINT_NETWORK_SERVICE
              value: {{ .Values.nsm.serviceName | quote }}
            - name: ENDPOINT_LABELS
              value: "app=pass-through-nse-{{ .Values.nsm.serviceName }}"
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
              value: "{{ .Values.insecure }}"
            - name: METRICS_PORT
              value: {{ .Values.metricsPort | quote }}
            - name: STRICT_DECODING
              value: "true"
            - name: PASS_THROUGH
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
            name: pass-through-nse-{{ .Values.nsm.serviceName }}
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: pass-through-nse-{{ .Values.nsm.serviceName }}
  namespace: {{ .Release.Namespace }}
data:
  config.yaml: |
    endpoints:
    - name: {{ .Values.nsm.serviceName | quote }}
      labels:
        app: "pass-through-nse-{{ .Values.nsm.serviceName }}"
        test: "pass-through-nse-test"
      passThrough:
        ifName: "endpoint0"
