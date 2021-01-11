---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ucnf-nse-{{ .Values.nsm.serviceName }}
  namespace: {{ .Release.Namespace }}
spec:
  replicas: {{ .Values.replicaCount }}
  selector:
    matchLabels:
      networkservicemesh.io/app: "ucnf-nse-{{ .Values.nsm.serviceName }}"
      networkservicemesh.io/impl: {{ .Values.nsm.serviceName | quote }}
  template:
    metadata:
      labels:
        networkservicemesh.io/app: "ucnf-nse-{{ .Values.nsm.serviceName }}"
        networkservicemesh.io/impl: {{ .Values.nsm.serviceName | quote }}
    spec:
      serviceAccount: {{ .Values.nsm.serviceName }}-service-account
      containers:
        - name: ucnf-nse
          image: {{ .Values.registry }}/{{ .Values.org }}/universal-cnf-vppagent:{{ .Values.tag }}
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
              value: "app=uconf-nse-{{ .Values.nsm.serviceName }}"
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
            name: ucnf-nse-{{ .Values.nsm.serviceName }}
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: ucnf-nse-{{ .Values.nsm.serviceName }}
  namespace: {{ .Release.Namespace }}
data:
  config.yaml: |
    endpoints:
    - name: "ucnf-nse-{{ .Values.nsm.serviceName }}"
      labels:
        app: {{ .Values.nsm.serviceName | quote }}
      nseControl:
        name: {{ .Values.nsm.serviceName | quote }}
        address: "{{ .Values.nseControl.nsr.addr }}"
        connectivityDomain: "{{ .Values.nsm.serviceName }}-connectivity-domain"
      vl3:
        ipam:
          defaultPrefixPool: {{ .Values.nseControl.ipam.defaultPrefixPool | quote }}
          prefixLength: {{ .Values.nseControl.ipam.prefixLength }}
          routes: []
        ifName: "endpoint0"
---
apiVersion: v1
kind: Service
metadata:
  name: "nse-pod-service-{{ .Values.nsm.serviceName }}-vpp"
  namespace: {{ .Release.Namespace }}
  labels:
    wcm/monitoring: vpp
spec:
  type: ClusterIP
  ports:
    - name: monitoring
      port: {{ .Values.vppMetricsPort }}
      targetPort: monitoring-vpp
      protocol: TCP
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ .Values.nsm.serviceName }}-service-account
  namespace: {{ .Release.Namespace }}
