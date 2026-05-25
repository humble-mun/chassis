package utils

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
)

// ProbeCheck runs a liveness/readiness check using the incoming request context.
//
// Implicit contract: all checks must be registered before mgr.Start(ctx).
// After the manager starts, callers must not mutate the global check registry.
//
// Return semantics:
//   - ok=true, message="", err=nil: probe succeeded.
//   - ok=false, message!="", err=nil: probe failed for an expected reason.
//   - err!=nil: probe execution hit an unexpected internal error.
type ProbeCheck func(context.Context) (ok bool, message string, err error)

var (
	livenessChecks  = make(map[string]ProbeCheck)
	readinessChecks = make(map[string]ProbeCheck)
)

type namedProbeCheck struct {
	name  string
	check ProbeCheck
}

func rawOk(ctx *gin.Context) {
	ctx.String(http.StatusOK, "OK")
}

// RegisterLivenessCheck registers a named /healthz check.
//
// Re-registering the same name replaces the previous check.
func RegisterLivenessCheck(name string, check ProbeCheck) {
	registerProbeCheck(livenessChecks, name, check)
}

// RegisterReadinessCheck registers a named /readyz check.
//
// Re-registering the same name replaces the previous check.
func RegisterReadinessCheck(name string, check ProbeCheck) {
	registerProbeCheck(readinessChecks, name, check)
}

func registerProbeCheck(target map[string]ProbeCheck, name string, check ProbeCheck) {
	if name == "" {
		panic("utils: probe check name must not be empty")
	}
	if check == nil {
		panic(fmt.Sprintf("utils: probe check %q must not be nil", name))
	}
	target[name] = check
}

// RegisterProbeRoute registers health and readiness probe endpoints.
func RegisterProbeRoute(mux *gin.Engine) {
	mux.GET("/healthz", NoLog, probeHandler("healthz", livenessChecks))
	mux.GET("/readyz", NoLog, probeHandler("readyz", readinessChecks))
}

func probeHandler(kind string, checks map[string]ProbeCheck) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		namedChecks := listProbeChecks(checks)
		if len(namedChecks) == 0 {
			rawOk(ctx)
			return
		}

		failures := make([]string, 0)
		requestCtx := ctx.Request.Context()
		for _, item := range namedChecks {
			ok, message, err := item.check(requestCtx)
			switch {
			case err != nil:
				failures = append(failures, fmt.Sprintf("- %s: internal error: %v", item.name, err))
			case ok:
				continue
			case message != "":
				failures = append(failures, fmt.Sprintf("- %s: %s", item.name, message))
			default:
				failures = append(failures, fmt.Sprintf("- %s: check failed", item.name))
			}
		}

		if len(failures) == 0 {
			rawOk(ctx)
			return
		}

		ctx.String(http.StatusServiceUnavailable, "%s check failed\n%s", kind, strings.Join(failures, "\n"))
	}
}

func listProbeChecks(checks map[string]ProbeCheck) []namedProbeCheck {
	namedChecks := make([]namedProbeCheck, 0, len(checks))
	for name, check := range checks {
		namedChecks = append(namedChecks, namedProbeCheck{name: name, check: check})
	}
	sort.Slice(namedChecks, func(i, j int) bool {
		return namedChecks[i].name < namedChecks[j].name
	})
	return namedChecks
}
