package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

var (
	ReceivedConnRequests = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "vl3_nse",
			Name:      "received_conn_requests",
			Help:      "Total number of received connection requests from vL3 NSE",
		})
	PerormedConnRequests = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "vl3_nse",
			Name:      "performed_conn_requests",
			Help:      "Total number of performed connection requests to vL3 NSE",
		})
	FailedFindNetworkService = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "vl3_nse",
			Name:      "failed_find_network_service",
			Help:      "Total number of failed network service finds",
		})
	// TODO: define more metrics
)

func ServeMetrics(addr string, path string) {
	prometheus.MustRegister(ReceivedConnRequests)
	prometheus.MustRegister(PerormedConnRequests)
	prometheus.MustRegister(FailedFindNetworkService)

	http.Handle(path, promhttp.Handler())

	logrus.Infof("Serving vl3_nse metrics on: %v", addr)

	go func() {
		if err := http.ListenAndServe(addr, nil); err != nil {
			logrus.Errorf("Cannot span Prometheus server for nsm-nse app: %v", err)
		}
	}()
}
