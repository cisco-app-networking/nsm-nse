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
			Namespace: "nse",
			Subsystem: "vl3",
			Name:      "received_conn_requests_total",
			Help:      "Total number of received connection requests from vL3 NSE",
		})
	PerormedConnRequests = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "nse",
			Subsystem: "vl3",
			Name:      "performed_conn_requests_total",
			Help:      "Total number of performed connection requests to vL3 NSE",
		})
	FailedFindNetworkService = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "nse",
			Subsystem: "vl3",
			Name:      "failed_network_service_find_total",
			Help:      "Total number of failed network service finds",
		})
	ActiveWorkloadCount = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "nse",
			Subsystem: "vl3",
			Name:      "active_workload",
			Help:      "Number of currently active workloads",
		})
	// TODO: define more metrics
)

func ServeMetrics(addr string, path string) {
	prometheus.MustRegister(ReceivedConnRequests)
	prometheus.MustRegister(PerormedConnRequests)
	prometheus.MustRegister(FailedFindNetworkService)
	prometheus.MustRegister(ActiveWorkloadCount)

	http.Handle(path, promhttp.Handler())

	logrus.Infof("Serving vl3 metrics on: %v", addr)

	go func() {
		if err := http.ListenAndServe(addr, nil); err != nil {
			logrus.Errorf("Cannot span Prometheus server for nsm-nse app: %v", err)
		}
	}()
}
