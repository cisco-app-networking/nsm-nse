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
	// TODO: define more metrics
)

func ServeMetrics(addr string, path string) {
	prometheus.MustRegister(ReceivedConnRequests)
	prometheus.MustRegister(PerormedConnRequests)

	http.Handle(path, promhttp.Handler())

	logrus.Infof("Serving vl3_nse metrics on: %v", addr)

	go func() {
		if err := http.ListenAndServe(addr, nil); err != nil {
			logrus.Errorf("Cannot span Prometheus server for nsm-nse app: %v", err)
		}
	}()
}
