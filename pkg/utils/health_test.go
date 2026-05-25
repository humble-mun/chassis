package utils

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRegisterProbeRouteNoChecks(t *testing.T) {
	resetProbeRegistries()

	router := gin.New()
	RegisterProbeRoute(router)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("readyz status = %d, want %d", response.Code, http.StatusOK)
	}
	if body := strings.TrimSpace(response.Body.String()); body != "OK" {
		t.Fatalf("readyz body = %q, want %q", body, "OK")
	}
}

func TestRegisterProbeRouteReadinessAggregatesFailures(t *testing.T) {
	resetProbeRegistries()
	t.Cleanup(resetProbeRegistries)

	var canceled bool
	RegisterReadinessCheck("alpha", func(ctx context.Context) (bool, string, error) {
		select {
		case <-ctx.Done():
			canceled = true
		default:
		}
		return true, "", nil
	})
	RegisterReadinessCheck("beta", func(context.Context) (bool, string, error) {
		return false, "sshd not running", nil
	})
	RegisterReadinessCheck("gamma", func(context.Context) (bool, string, error) {
		return false, "", errors.New("boom")
	})

	router := gin.New()
	RegisterProbeRoute(router)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	router.ServeHTTP(response, request)

	if canceled {
		t.Fatal("request context unexpectedly canceled")
	}
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("readyz status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}

	body := response.Body.String()
	if !strings.Contains(body, "readyz check failed") {
		t.Fatalf("readyz body = %q, want summary header", body)
	}
	if !strings.Contains(body, "- beta: sshd not running") {
		t.Fatalf("readyz body = %q, want expected failure message", body)
	}
	if !strings.Contains(body, "- gamma: internal error: boom") {
		t.Fatalf("readyz body = %q, want internal error message", body)
	}
}

func TestRegisterProbeRouteLivenessUsesOwnRegistry(t *testing.T) {
	resetProbeRegistries()
	t.Cleanup(resetProbeRegistries)

	RegisterLivenessCheck("live", func(context.Context) (bool, string, error) {
		return true, "", nil
	})
	RegisterReadinessCheck("ready", func(context.Context) (bool, string, error) {
		return false, "not ready", nil
	})

	router := gin.New()
	RegisterProbeRoute(router)

	liveResponse := httptest.NewRecorder()
	liveRequest := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	router.ServeHTTP(liveResponse, liveRequest)
	if liveResponse.Code != http.StatusOK {
		t.Fatalf("healthz status = %d, want %d", liveResponse.Code, http.StatusOK)
	}

	readyResponse := httptest.NewRecorder()
	readyRequest := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	router.ServeHTTP(readyResponse, readyRequest)
	if readyResponse.Code != http.StatusServiceUnavailable {
		t.Fatalf("readyz status = %d, want %d", readyResponse.Code, http.StatusServiceUnavailable)
	}
}

func TestRegisterProbeCheckPanicsOnInvalidInput(t *testing.T) {
	resetProbeRegistries()
	t.Cleanup(resetProbeRegistries)

	t.Run("empty name", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("expected panic for empty name")
			}
		}()
		RegisterReadinessCheck("", func(context.Context) (bool, string, error) {
			return true, "", nil
		})
	})

	t.Run("nil check", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("expected panic for nil check")
			}
		}()
		RegisterLivenessCheck("nil", nil)
	})
}

func resetProbeRegistries() {
	livenessChecks = make(map[string]ProbeCheck)
	readinessChecks = make(map[string]ProbeCheck)
}
