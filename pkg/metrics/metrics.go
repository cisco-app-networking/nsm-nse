package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

const vl3Subsystem = "vl3"

var (
	ReceivedConnRequests = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "nse",
			Subsystem: vl3Subsystem,
			Name:      "received_conn_requests_total",
			Help:      "Total number of received connection requests from vL3 NSE peer",
		})
	PerormedConnRequests = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "nse",
			Subsystem: vl3Subsystem,
			Name:      "performed_conn_requests_total",
			Help:      "Total number of performed connection requests to vL3 NSE peer",
		})
	FailedFindNetworkService = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "nse",
			Subsystem: vl3Subsystem,
			Name:      "failed_network_service_find_total",
			Help:      "Total number of failed network service finds",
		})
	ActiveWorkloadCount = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "nse",
			Subsystem: vl3Subsystem,
			Name:      "active_workload",
			Help:      "Number of currently active workloads",
		})
)

func ServeMetrics(addr string, path string) {
	prometheus.MustRegister(ReceivedConnRequests)
	prometheus.MustRegister(PerormedConnRequests)
	prometheus.MustRegister(FailedFindNetworkService)
	prometheus.MustRegister(ActiveWorkloadCount)

	http.Handle(path, promhttp.Handler())

	go func() {
		if err := http.ListenAndServe(addr, nil); err != nil {
			logrus.Errorf("Failed to start metrics server: %v", err)
		}
	}()
}
