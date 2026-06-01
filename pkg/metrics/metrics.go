package metrics

import (
	"context"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/humble-mun/chassis/pkg/utils"
)

const (
	flagScrapeHookTimeout    = "metrics-scrape-hook-timeout"
	defaultScrapeHookTimeout = 1 * time.Second
)

// ScrapeHook is invoked when the metrics endpoint is scraped, giving passively
// sourced metrics a chance to refresh themselves just before they are exposed.
type ScrapeHook func(context.Context)

var (
	registry = prometheus.NewRegistry()
	factory  = promauto.With(registry)
	handler  = promhttp.HandlerFor(registry, promhttp.HandlerOpts{})

	scrapeHooks = make([]ScrapeHook, 0)
)

// RegisterScrapeHook adds a hook that runs when the metrics endpoint is scraped.
// Hooks fire concurrently on every scrape, bounded by the scrape-hook timeout.
func RegisterScrapeHook(hook ScrapeHook) {
	scrapeHooks = append(scrapeHooks, hook)
}

// RegisterFlags registers the metrics flags on the given flag set.
func RegisterFlags(pfs *pflag.FlagSet) {
	pfs.Duration(flagScrapeHookTimeout, defaultScrapeHookTimeout,
		"maximum time to wait for scrape hooks to refresh metrics before serving a scrape")
}

// Register registers a Prometheus metric with the global registry
func Register[Metric prometheus.Collector](register func(promauto.Factory) Metric) Metric {
	return register(factory)
}

// RegisterRoute registers the Prometheus metrics endpoint
func RegisterRoute(mux *gin.Engine) {
	cachedTimeout := time.Duration(-1)
	scrapeHookTimeout := func() time.Duration {
		if cachedTimeout < 0 {
			cachedTimeout = viper.GetDuration(flagScrapeHookTimeout)
		}
		return cachedTimeout
	}
	mux.GET("/metrics", utils.NoLog, func(ctx *gin.Context) {
		if count := len(scrapeHooks); count > 0 {
			scrapeCtx, cancel := context.WithTimeout(ctx.Request.Context(), scrapeHookTimeout())
			defer cancel()
			wg := new(sync.WaitGroup)
			wg.Add(count)
			for i := range scrapeHooks {
				go func() {
					defer wg.Done()
					scrapeHooks[i](scrapeCtx)
				}()
			}
			wg.Wait()
		}
		gin.WrapH(handler)(ctx)
	})
}
