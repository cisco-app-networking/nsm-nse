---
apiVersion: networkservicemesh.io/v1alpha1
kind: NetworkService
metadata:
  name: ucnf-nse-{{ .Values.nsm.serviceName }}
spec:
  payload: IP
  matches:
    - match:
      sourceSelector:
        app: {{ .Values.nsm.serviceName }}
      route:
        - destination:
          destinationSelector:
            app: pass-through-icmp
    - match:
      route:
        - destination:
          destinationSelector:
            app: {{ .Values.nsm.serviceName }} {{/* this needs to match the config.yaml.labels.app in the endpoint yaml*/}}