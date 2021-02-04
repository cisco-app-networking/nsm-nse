---
apiVersion: networkservicemesh.io/v1alpha1
kind: NetworkService
metadata:
  name: vl3-service
spec:
  payload: IP
  matches:
    - match:
      sourceSelector:
        app: {{ .Values.nsm.serviceName }}
      route:
        - destination:
          destinationSelector:
            app: vl3-nse-vl3-service {{/* vl3-nse label */}}
    - match:
      route:
        - destination:
          destinationSelector:
            app: {{ .Values.nsm.serviceName }} {{/* this needs to match the config.yaml.labels.app in the endpoint yaml*/}}