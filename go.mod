module cisco-app-networking.github.io/nsm-nse

go 1.14

require (
	cisco-app-networking.github.io/networkservicemesh/controlplane/api v1.0.10
	cisco-app-networking.github.io/networkservicemesh/pkg v1.0.10
	cisco-app-networking.github.io/networkservicemesh/sdk v1.0.10
	github.com/davecgh/go-spew v1.1.1
	github.com/golang/protobuf v1.4.2
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0
	github.com/grpc-ecosystem/grpc-opentracing v0.0.0-20180507213350-8e809c8a8645
	github.com/onsi/gomega v1.10.3
	github.com/opentracing/opentracing-go v1.1.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.1.0
	github.com/sirupsen/logrus v1.6.0
	github.com/stretchr/testify v1.6.1
	go.ligato.io/vpp-agent/v3 v3.2.0
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e
	google.golang.org/grpc v1.29.1
	gopkg.in/yaml.v2 v2.3.0
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776
	gotest.tools v2.2.0+incompatible
)

replace (
	github.com/census-instrumentation/opencensus-proto v0.1.0-0.20181214143942-ba49f56771b8 => github.com/census-instrumentation/opencensus-proto v0.0.3-0.20181214143942-ba49f56771b8
	github.com/cisco-app-networking/nsm-nse => ./
)
