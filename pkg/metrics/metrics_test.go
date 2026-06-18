package metrics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func TestRegisterFlags(t *testing.T) {
	pfs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	RegisterFlags(pfs)

	if pfs.Lookup(flagScrapeHookTimeout) == nil {
		t.Fatalf("expected flag %q to be registered", flagScrapeHookTimeout)
	}
}

func TestRegister(t *testing.T) {
	counter := Register(func(factory promauto.Factory) prometheus.Counter {
		return factory.NewCounter(prometheus.CounterOpts{
			Name: "test_register_total",
			Help: "test counter",
		})
	})
	counter.Inc()

	response := scrapeMetrics(t)
	if !strings.Contains(response, "test_register_total") {
		t.Fatalf("expected scrape to expose test_register_total, got:\n%s", response)
	}
}

func TestRegisterRouteRunsScrapeHooks(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set(flagScrapeHookTimeout, defaultScrapeHookTimeout)

	scrapeHooks = make([]ScrapeHook, 0)
	t.Cleanup(func() { scrapeHooks = make([]ScrapeHook, 0) })

	var fired atomic.Int32
	RegisterScrapeHook(func(ctx context.Context) {
		if ctx == nil {
			t.Error("expected non-nil context passed to scrape hook")
		}
		fired.Add(1)
	})

	scrapeMetrics(t)

	if got := fired.Load(); got != 1 {
		t.Fatalf("expected scrape hook to fire once, fired %d times", got)
	}
}

func scrapeMetrics(t *testing.T) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterRoute(router)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("metrics status = %d, want %d", response.Code, http.StatusOK)
	}
	return response.Body.String()
}
