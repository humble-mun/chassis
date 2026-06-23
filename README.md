# chassis

> [中文文档](./README_CN.md)

`chassis` is the shared infrastructure toolkit used across multiple humble-mun projects running in production. It provides the common building blocks that services use to bootstrap HTTP/gRPC servers, metrics, logging, health probes, and controller-runtime managers.

Go module: `github.com/humble-mun/chassis`

## Project positioning

`chassis` is **not** a scaffolding tool and does not aim to replace kubebuilder, operator-sdk, or any similar project generator. It is the shared logic library that humble-mun' internal components depend on in common — the bootstrap, server, metrics, logging, and probe building blocks they would otherwise each reimplement.

Because those components are being open-sourced, the dependency they all share is open-sourced too, so that they remain buildable and self-contained outside our internal environment. In other words, this repository exists primarily to serve those components rather than to be a general-purpose framework.

That said, feature requests and suggestions are very welcome. The toolkit grew out of internal needs and still carries some of those assumptions; concrete use cases from outside help us refactor it toward something more general and reusable. If something is awkward to consume or you need a building block that isn't here yet, please open an issue.

## Package layout

| Package | Purpose |
|---------|---------|
| `pkg/app` | Application bootstrap: `PrepareFlags` + `BaseContext`, functional options for listeners and gRPC |
| `pkg/server` | Multi-listener HTTP/gRPC server with Gin, TLS, mTLS, H2C, CORS and graceful shutdown; TLS certs and client-CA bundles hot-reload on rotation (no restart) |
| `pkg/metrics` | Prometheus registry wrapper, `/metrics` endpoint and optional scrape hooks |
| `pkg/manager` | controller-runtime manager bootstrap |
| `pkg/logging` | klog initialization and `logr.Logger` factory |
| `pkg/utils` | Leaf helpers: slice middleware, image/k8s name normalization, SSH, viper flags, probes, infra-token auth |
| `pkg/constants` | Common flag names and defaults |
| `pkg/version` | Build/version template populated by ldflags |
| `pkg/tls` | Hot-reloading TLS building blocks: `CertReloader` (server cert) and `CAReloader` (client-CA bundle), reloaded from disk on rotation for use as `crypto/tls.Config` callbacks (unix and Windows only) |

## Quick start

A typical consumer bootstraps in `main.go` like this:

```go
package main

import (
    "os"

    "github.com/spf13/cobra"

    "github.com/humble-mun/chassis/pkg/app"
    "github.com/humble-mun/chassis/pkg/version"
)

func main() {
    version.Name = "my-service"

    var init func() error
    cmd := &cobra.Command{
        Use: version.Name,
        RunE: func(cmd *cobra.Command, args []string) (err error) {
            // WithInit(init) runs the viper initialization produced by
            // PrepareFlags. Without it, flags and config are never parsed.
            var base app.Base
            if base, err = app.BaseContext(app.WithInit(init)); err != nil {
                return
            }
            _ = base.RootLogger
            _ = base.Logger
            _ = base.NodeName

            // Register business routes on the shared server, start controllers, etc.
            return base.HTTPGin.Start(base.Ctx)
        },
    }

    // PrepareFlags returns the viper init func; pass it to BaseContext via WithInit.
    init = app.PrepareFlags(version.Name, cmd)

    if err := cmd.Execute(); err != nil {
        os.Exit(1)
    }
}
```

Build-time version information is injected via ldflags (see [Build recommendations](#build-recommendations) below).

## Conventions

This section records design decisions that are intentional and should be preserved across consumers.

### klog fork and the `replace` directive

`chassis` depends on a fork of `k8s.io/klog/v2`:

```go
replace k8s.io/klog/v2 => github.com/tedli/klog/v2 v2.0.0-20260407032038-5a4969b5a1c1
```

The fork adds an HTTP API (`klog.Configure`) that lets operators change the log level at runtime. This is extremely useful in production: when an incident occurs, you can temporarily raise verbosity without restarting the process, gather the extra logs, and then lower it again.

`replace` directives in Go only take effect in the **main module** being built. When `chassis` is consumed as a dependency, its own `replace` is ignored by the Go toolchain. Therefore every downstream service must repeat the same `replace` line in its `go.mod`. This is a Go hard constraint, not a choice. Keeping the fork under the original `k8s.io/klog/v2` module path is required so that indirect dependencies (`client-go`, `controller-runtime`, …) share the same global klog state and can also be tuned at runtime.

### BaseContext is the single composition root

`app.BaseContext` is the intended single place where the logger, signal-handled context, HTTP server, and node name are created. Consumers should prefer extending it through options (`app.WithInit`, `app.WithGRPCServer`, `app.WithTCPListener`, `app.WithUnixListener`, `app.WithoutHTTPServer`) rather than creating a second viper instance, a second logger, or a second server.

### Configuration is a single viper global singleton

All configuration in a `chassis` service flows through a single process-global viper instance composed by `BaseContext`. This is intentional: do **not** introduce additional config files or a second viper instance. If a component accumulates so many options that it appears to need its own config file, that is a signal the component should be split into smaller pieces, not that another config source should be added. For a complex nested configuration item, bind it to a single flag and unmarshal the sub-tree with `viper.UnmarshalKey`, for example `viper.UnmarshalKey("some-complex-config", &cfg)`, rather than reading a separate file.

### Version information is process-global and for troubleshooting

`pkg/version` exposes build metadata (`Name`, `CommitID`, `BuiltAt`, `Architecture`, `Variant`, `RecentCommits`) as package-level variables injected at compile time via ldflags. Like the global viper instance, this is process-global by design: if a service needs to track more than one version, that is a signal it has grown too complex and should be split, not that `version` should hold per-component instances. The problem `version` solves is production troubleshooting — in production you often cannot tell whether a running binary already contains a given change, so injecting the build time and commit id at compile time lets an operator confirm and locate issues. It is **not** meant to surface a version to end users as a UI feature.

### Controller-runtime metric server and webhook are intentionally unused

All services built on `chassis` disable the controller-runtime metrics server (`BindAddress: "0"`) and do **not** use controller-runtime's webhook server. Instead:

* Metrics are served through the shared Gin server on `/metrics` (`pkg/metrics`). In production we found controller-runtime's default metrics to be low-signal and prometheus-expensive; exposing only the business-relevant metrics keeps the prometheus footprint small.
* Webhooks reuse the same Gin server. Controller-runtime's webhook helpers mainly handle serialization; the actual validating/mutating logic must be written anyway, so using the shared server keeps the architecture uniform.

### Registries are lock-free and registered once

`pkg/utils/health.go` stores probe checks in a plain `map` without locks, and `pkg/metrics/metrics.go` stores scrape hooks in a plain slice without locks. This is intentional. Registration functions (`utils.RegisterLivenessCheck`, `utils.RegisterReadinessCheck`, `metrics.RegisterScrapeHook`) should be called once during startup (`main` or the composition root) before `mgr.Start(ctx)` runs. After startup these registries are treated as immutable. Kubernetes relies on `/healthz` and `/readyz` staying unauthenticated; never add authentication to the probe routes.

### Infrastructure endpoints are opt-in authenticated

`/logging` (klog runtime configuration) and `/debug/pprof` can change process behavior and may be exposed through ingress. They are guarded by an optional infra token:

```bash
my-service --infra-api-token=<secret>
```

When the token is set, requests must include the header:

```text
X-API-Token: <secret>
```

When the token is empty, the endpoints remain open (backward-compatible behavior). `/metrics`, `/healthz`, and `/readyz` stay unauthenticated.

## Build recommendations

`chassis` requires **Go 1.26 or newer** (see the `go` directive in `go.mod`).

The repository uses vendored dependencies. Build and test from the repo root:

```bash
# compile all packages
go build -mod=vendor -v ./...

# run tests
go test -mod=vendor ./...

# run go vet
go vet -mod=vendor ./...
```

For linting (when a Linux `golangci-lint-v2` binary is available):

```bash
golangci-lint-v2 run -c .golangci.yaml ./...
```

### Injecting version information

Populate `pkg/version` variables at link time. A `Makefile` and `Dockerfile` similar to the following are recommended:

**Makefile**

```makefile
MODULE       ?= github.com/humble-mun/my-service
IMAGE        ?= humble-mun/my-service
VERSION_PKG  ?= github.com/humble-mun/chassis/pkg/version
GIT_COMMIT   := $(shell git rev-parse --short HEAD)
BUILD_TIME   := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GO_VERSION   := $(shell go version | awk '{print $$3}')
GOOS         ?= linux
GOARCH       ?= amd64
CGO_ENABLED  ?= 0

LDFLAGS := -s -w \
    -X '$(VERSION_PKG).Name=$(MODULE)' \
    -X '$(VERSION_PKG).CommitID=$(GIT_COMMIT)' \
    -X '$(VERSION_PKG).BuiltAt=$(BUILD_TIME)' \
    -X '$(VERSION_PKG).Architecture=$(GOOS)/$(GOARCH)'

.PHONY: build
build:
	GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=$(CGO_ENABLED) \
	    go build -mod=vendor -ldflags "$(LDFLAGS)" -o bin/$(MODULE) ./cmd/$(MODULE)

.PHONY: image
image:
	docker build -t $(IMAGE):$(GIT_COMMIT) .
```

**Dockerfile**

```dockerfile
FROM golang:1.26.0-trixie AS builder
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -mod=vendor -ldflags "-s -w -X 'github.com/humble-mun/chassis/pkg/version.Name=my-service'" -o /bin/my-service ./cmd/my-service

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /bin/my-service /bin/my-service
USER nonroot:nonroot
ENTRYPOINT ["/bin/my-service"]
```

Replace `my-service` with the actual service name and import path.

## License

`chassis` is licensed under the Apache License 2.0. See [LICENSE](LICENSE) and [NOTICE](NOTICE).
