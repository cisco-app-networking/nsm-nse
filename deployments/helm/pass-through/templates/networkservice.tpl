---
apiVersion: networkservicemesh.io/v1alpha1
kind: NetworkService
metadata:
  name: {{ .Values.nsm.serviceName }}
spec:
  payload: IP
  matches:
    - match:
      sourceSelector:
        app: pass-through-nse-{{ .Values.nsm.serviceName }}
      route:
        - destination:
          destinationSelector:
            app: vl3-nse-vl3-service
    - match:
      route:
        - destination:
          destinationSelector:
            app: pass-through-nse-{{ .Values.nsm.serviceName }}
