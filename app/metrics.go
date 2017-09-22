package app

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	// HTTPResponseCtr allows the counting of Http Responses and their status codes
	HTTPResponseCtr = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "undergang_http_response_total",
			Help: "Number of http responses",
		},
		[]string{"code"},
	)
	// BackendActive allows the counting of active (registered) backends
	BackendActive = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "undergang_backend_active_total",
			Help: "Number of active backends",
		},
	)
	// BackendsRegistered allows the counting of backends that have been registered
	BackendsRegistered = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "undergang_backend_registered_total",
			Help: "Number of backends that have been registered",
		},
	)
	// BackendsStarted allows the counting of backends that have been started
	BackendsStarted = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "undergang_backend_started_total",
			Help: "Number of backends that have been started",
		},
	)
	// BackendsUnregistered allows the counting of backends that have been unregistered
	BackendsUnregistered = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "undergang_backend_unregistered_total",
			Help: "Number of backends that have been unregistered",
		},
	)
	// BackendFailure allows the counting of backends and the corresponding failure reason
	BackendFailure = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "undergang_backend_failure_total",
			Help: "Number of backends that have failed",
		},
		[]string{"reason"},
	)
	// BackendReconnectSSH allows the counting of backends that have reconnected to the SSH server
	BackendReconnectSSH = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "undergang_backend_reconnect_ssh_total",
			Help: "Number of backends that have reconnected to SSH",
		},
	)
	// BackendProvisioningDuration allows the histogram of provisioning durations
	BackendProvisioningDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "undergang_backend_provisioning_seconds",
			Help:    "Provisioning duration for backends",
			Buckets: []float64{1, 5, 10, 30, 1 * 60, 2 * 60, 3 * 60, 4 * 60, 5 * 60, 10 * 60, 20 * 60},
		},
	)
	// BackendConnectSSHDuration allows the histogram of provisioning durations
	BackendConnectSSHDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "undergang_backend_connect_ssh_seconds",
			Help:    "SSH connection duration for backends",
			Buckets: []float64{1, 5, 10, 30, 1 * 60, 2 * 60, 3 * 60, 4 * 60, 5 * 60},
		},
	)
	// BackendBootstrapDuration allows the histogram of provisioning durations
	BackendBootstrapDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "undergang_backend_bootstrap_seconds",
			Help:    "SSH bootstrap duration for backends",
			Buckets: []float64{1, 5, 10, 30, 1 * 60, 2 * 60, 3 * 60, 4 * 60, 5 * 60, 10 * 60, 20 * 60},
		},
	)
)

func init() {
	prometheus.MustRegister(HTTPResponseCtr)
	prometheus.MustRegister(BackendActive)
	prometheus.MustRegister(BackendsRegistered)
	prometheus.MustRegister(BackendsStarted)
	prometheus.MustRegister(BackendsUnregistered)
	prometheus.MustRegister(BackendFailure)
	prometheus.MustRegister(BackendReconnectSSH)
	prometheus.MustRegister(BackendProvisioningDuration)
	prometheus.MustRegister(BackendConnectSSHDuration)
	prometheus.MustRegister(BackendBootstrapDuration)
}
