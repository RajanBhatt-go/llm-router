package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	RequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "llm_requests_total",
			Help: "Total number of LLM requests.",
		},
		[]string{"provider", "status"},
	)

	RequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "llm_request_duration_seconds",
			Help:    "Request duration in seconds.",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60},
		},
		[]string{"provider"},
	)

	FallbackTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "llm_fallback_total",
			Help: "Number of fallback events.",
		},
		[]string{"from", "to"},
	)

	CircuitBreakerState = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "llm_circuit_breaker_state",
			Help: "Circuit breaker state: 0=closed, 1=half-open, 2=open.",
		},
		[]string{"provider"},
	)

	ContextTruncatedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "llm_context_truncated_total",
			Help: "Number of context truncation events.",
		},
		[]string{"strategy"},
	)

	TokenCount = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "llm_token_count",
			Help:    "Token count per request.",
			Buckets: prometheus.ExponentialBuckets(100, 2, 12),
		},
		[]string{"direction"},
	)
)

func init() {
	prometheus.MustRegister(
		RequestsTotal,
		RequestDuration,
		FallbackTotal,
		CircuitBreakerState,
		ContextTruncatedTotal,
		TokenCount,
	)
}

func Handler() http.Handler {
	return promhttp.Handler()
}