package metrics

import (
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/humble-mun/chassis/pkg/utils"
)

var (
	registry = prometheus.NewRegistry()
	factory  = promauto.With(registry)
	handler  = promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
)

// Register registers a Prometheus metric with the global registry
func Register[Metric prometheus.Collector](register func(promauto.Factory) Metric) Metric {
	return register(factory)
}

// RegisterRoute registers the Prometheus metrics endpoint
func RegisterRoute(mux *gin.Engine) {
	mux.GET("/metrics", utils.NoLog, gin.WrapH(handler))
}
