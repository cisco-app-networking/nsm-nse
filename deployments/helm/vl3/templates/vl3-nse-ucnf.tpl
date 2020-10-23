---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: vl3-nse-{{ .Values.nsm.serviceName }}
  namespace: {{ .Release.Namespace }}
spec:
  replicas: {{ .Values.replicaCount }}
  selector:
    matchLabels:
      networkservicemesh.io/app: "vl3-nse-{{ .Values.nsm.serviceName }}"
      networkservicemesh.io/impl: {{ .Values.nsm.serviceName | quote }}
  template:
    metadata:
      labels:
        networkservicemesh.io/app: "vl3-nse-{{ .Values.nsm.serviceName }}"
        networkservicemesh.io/impl: {{ .Values.nsm.serviceName | quote }}
        wcm/nse.servicename: {{ .Values.nsm.serviceName | quote }}
      annotations:
        sidecar.istio.io/inject: "false"
{{- if .Values.nseControl.nsr.addr }}
        wcm/nsr.address: {{ .Values.nseControl.nsr.addr | quote }}
        wcm/nsr.port: {{ .Values.nseControl.nsr.port | quote }}
{{- end }}
    spec:
      serviceAccount: {{ .Values.nsm.serviceName }}-service-account
      containers:
        - name: vl3-nse
          image: {{ .Values.registry }}/{{ .Values.org }}/vl3_ucnf-nse:{{ .Values.tag }}
          imagePullPolicy: {{ .Values.pullPolicy }}
          ports:
          - name: monitoring-vpp
            containerPort: {{ .Values.vppMetricsPort }}
          - name: monitoring
            containerPort: {{ .Values.metricsPort  }}
          env:
            - name: ENDPOINT_NETWORK_SERVICE
              value: {{ .Values.nsm.serviceName | quote }}
            - name: ENDPOINT_LABELS
              value: "app=vl3-nse-{{ .Values.nsm.serviceName }}"
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
{{- if .Values.ipamUniqueOctet }}
            - name: NSE_IPAM_UNIQUE_OCTET
              value: {{ .Values.ipamUniqueOctet | quote }}
{{- end }}
            - name: NSE_POD_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
            - name: NSM_REMOTE_NS_IP_LIST
              valueFrom:
                configMapKeyRef:
                  name: nsm-vl3-{{ .Values.nsm.serviceName }}
                  key: remote.ip_list
            - name: METRICS_PORT
              value: {{ .Values.metricsPort | quote }}
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
            name: ucnf-vl3-{{ .Values.nsm.serviceName }}
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: ucnf-vl3-{{ .Values.nsm.serviceName }}
  namespace: {{ .Release.Namespace }}
data:
  config.yaml: |
    endpoints:
    - name: {{ .Values.nsm.serviceName | quote }}
      labels:
        app: "vl3-nse-{{ .Values.nsm.serviceName }}"
{{- if .Values.nseControl.nsr.addr }}
        wcm/nsr.addr: {{ .Values.nseControl.nsr.addr | quote }}
        wcm/nsr.port: {{ .Values.nseControl.nsr.port | quote }}
{{- end }}
      nseControl:
        name: {{ .Values.nsm.serviceName | quote }}
        address: "{{ .Values.nseControl.nsr.addr }}"
        connectivityDomain: "{{ .Values.nsm.serviceName }}-connectivity-domain"
      vl3:
       ipam:
          defaultPrefixPool: {{ .Values.nseControl.ipam.defaultPrefixPool | quote }}
{{- if .Values.nseControl.nsr.addr }}
          serverAddress: "ipam-{{ .Values.nseControl.nsr.addr }}:50051"
{{- end }}
          prefixLength: {{ .Values.nseControl.ipam.prefixLength }}
          routes: []
       ifName: "endpoint0"
{{- if .Values.nseControl.nameserver }}
       nameServers:
          - {{ .Values.nseControl.nameserver }}
       dnsZones:
          - {{ .Values.nseControl.dnszone }}
{{- end }}
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
  selector:
      wcm/nse.servicename: {{ .Values.nsm.serviceName | quote }}
  ports:
    - name: monitoring
      port: {{ .Values.vppMetricsPort }}
      targetPort: monitoring-vpp
      protocol: TCP
---
apiVersion: v1
kind: Service
metadata:
  name: "nse-pod-service-{{ .Values.nsm.serviceName }}"
  namespace: {{ .Release.Namespace }}
  labels:
    wcm/monitoring: vl3
spec:
  type: ClusterIP
  selector:
    wcm/nse.servicename: {{ .Values.nsm.serviceName | quote }}
  ports:
    - name: monitoring
      port: {{ .Values.metricsPort }}
      targetPort: monitoring
      protocol: TCP
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ .Values.nsm.serviceName }}-service-account
  namespace: {{ .Release.Namespace }}

