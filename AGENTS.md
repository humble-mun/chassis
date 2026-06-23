# AGENTS.md

This file provides guidance to AI agents when working with code in this repository.

## Project Overview

chassis is a shared infrastructure toolkit for humble-mun microservices. It provides common building blocks that are consumed as a dependency by multiple humble-mun services running in production.

**Go module**: `github.com/humble-mun/chassis`

## Package Structure

| Package | Purpose | Dependencies |
|---------|---------|--------------|
| `pkg/utils` | Leaf package: slice helpers, gin middleware, image/k8s name normalization, SSH, viper config, health probes, infra-token auth | gin, viper, pflag, crypto/ssh |
| `pkg/logging` | Leaf package: klog initialization and logger factory | klog, logr, pflag |
| `pkg/metrics` | Prometheus registry wrapper, scrape hooks and /metrics endpoint | prometheus, gin, utils |
| `pkg/version` | Build version info template | stdlib only |
| `pkg/constants` | Common infrastructure constants (flag names, defaults) | stdlib only |
| `pkg/tls` | Leaf-ish package: hot-reloading TLS helpers — `CertReloader` (server cert via `GetCertificate`) and `CAReloader` (client-CA bundle via `CurrentPool`/`ConfigForClient`), with a generic `reloader[T]` base that detects file rotation by inode (size+modTime fallback) and falls back to the last good material on a bad reload. Supported on unix and Windows only; constructing a reloader on any other GOOS (plan9, js/wasm, wasip1) panics | crypto/tls, crypto/x509, logr |
| `pkg/server` | HTTP/gRPC server with Gin: multi-listener (TCP + Unix socket), TLS/mTLS per listener, lazy listener resolution, CORS, H2C via standard library, graceful shutdown, request logging; server certs and client-CA bundles hot-reload on rotation without restart (via `pkg/tls`) | gin, cors, grpc, errgroup, metrics, constants, utils, version, tls |
| `pkg/manager` | controller-runtime manager bootstrap (flags, client QPS/burst, leader election, scheme registration) | controller-runtime, viper, pflag, logr, constants |
| `pkg/app` | Application bootstrap (PrepareFlags + BaseContext); BaseContext returns a `Base` struct (RootLogger, Logger, HTTPGin `*server.HTTPServer`, Ctx, NodeName) and accepts functional options | all above packages, cobra, controller-runtime |

### Internal Dependency Graph

```
app --> logging, metrics, server, constants, utils, version
server --> metrics, constants, utils, version, tls
manager --> constants
metrics --> utils
logging --> (none)
version --> (none)
constants --> (none)
tls --> (none)
utils --> (none)
```

## Consumer Configuration

Consumers should set this variable before calling `app.BaseContext()`:

- `version.Name` - application name (typically set via ldflags); also used as the `component` label on the `http_failure` request-failure metric

`BaseContext` accepts functional options:

- `app.WithInit(fn)` - sets the viper initialization function returned by `PrepareFlags`
- `app.WithGRPCServer(s)` - attaches a gRPC server; requests with `Content-Type: application/grpc` are routed to it
- `app.WithTCPListener(...ListenerOption)` - adds a TCP listener; use `server.WithAddr(fn)` to supply the bind address, `server.WithTLSCert(certFn, keyFn)` to enable TLS, `server.WithMTLS(clientCAFn)` to enable mTLS, and `server.WithTLSMinVersion(version)` to set the minimum TLS version
- `app.WithUnixListener(...ListenerOption)` - adds a Unix domain socket listener; use `server.WithAddr(fn)` to supply the socket path
- `app.WithoutHTTPServer()` - skips creation of the HTTP server; useful for services that only need a controller-runtime manager

`BaseContext` prepends `server.WithDefaultListener()` and `server.WithDefaultCORSConfig()` automatically, so flag-driven defaults are always active unless overridden.

## Conventions

The following conventions are intentional and should be preserved or extended through options rather than duplicated in consumers.

### klog replace

`chassis` replaces `k8s.io/klog/v2` with `github.com/tedli/klog/v2` so that klog exposes a runtime HTTP API for changing log levels. This `replace` must be repeated in every downstream `go.mod` because Go ignores `replace` directives from dependencies. Do not try to remove the replace or rename the fork module path: indirect dependencies (`client-go`, `controller-runtime`) import `k8s.io/klog/v2` and their log output can only be tuned at runtime if the whole process resolves to the fork.

### BaseContext is the single composition root

Avoid creating a second viper instance, a second logger, or a second HTTP server in consumers. Extend `BaseContext` through options instead.

### Configuration is a single viper global singleton

All configuration flows through one process-global viper instance composed by `BaseContext`. Do not introduce additional config files or a second viper instance. The rule is deliberate: if a component accumulates so many options that it seems to need its own config file, that is a signal the component should be split, not that another config source should be added. For a complex nested config item, bind it to a single flag and unmarshal the sub-tree with `viper.UnmarshalKey`, e.g. `viper.UnmarshalKey("some-complex-config", &cfg)`, rather than reading a separate file.

### Version information is process-global and for troubleshooting

`pkg/version` exposes build metadata (`Name`, `CommitID`, `BuiltAt`, `Architecture`, `Variant`, `RecentCommits`) as package-level variables injected at compile time via ldflags. Like the viper singleton, these are process-global by design: if a service needs to track more than one version, that is a signal it is doing too much and should be split, not that `version` should grow per-component instances. The goal is production troubleshooting — letting an operator confirm whether a running binary contains a given change by inspecting the injected commit id and build time — not presenting a version to end users as a UI feature.

### Registries are lock-free and immutable after startup

`pkg/utils/health.go` stores liveness/readiness checks in a plain map without locks, and `pkg/metrics/metrics.go` stores scrape hooks in a plain slice without locks. Registration functions (`utils.RegisterLivenessCheck`, `utils.RegisterReadinessCheck`, `metrics.RegisterScrapeHook`) must be called before `mgr.Start(ctx)` and never mutated at runtime. Probe routes (`/healthz`, `/readyz`) must stay unauthenticated so Kubernetes probes do not fail.

### Controller-runtime metric server and webhook server are not used

Metrics are served through the shared Gin server (`/metrics`). Controller-runtime's metrics server is disabled (`BindAddress: "0"`). Webhooks reuse the same Gin server rather than controller-runtime's webhook server.

### Infrastructure endpoints are opt-in authenticated

`/logging` and `/debug/pprof` are protected by `--infra-api-token` when set. Empty token keeps them open for backward compatibility. `/metrics`, `/healthz`, and `/readyz` stay unauthenticated.

## Health Probes

`pkg/utils.RegisterProbeRoute` serves `/healthz` and `/readyz`.
Consumers can extend them with:

- `utils.RegisterLivenessCheck(name, check)`
- `utils.RegisterReadinessCheck(name, check)`

`ProbeCheck` signature:

```go
func(context.Context) (ok bool, message string, err error)
```

Contract:
- `ok=false` with `message` and `err==nil` means an expected probe failure.
- `err!=nil` means the check itself hit an unexpected internal error.
- Register all probe checks before `mgr.Start(ctx)`; runtime mutation of the global probe registry is not supported.

## Go Project Code Style

Key rules:

### Named Return Values
When a function returns more than 1 value, use named return values with bare `return`.

### Error Handling
- Always check errors with `if err != nil`
- Assign and check in the same expression when the current function has a named error return
- Log all errors explicitly
- Use `errors.Is()` / `errors.As()` for error comparison
- Define predictable errors as package-level `var` with `errors.New()`

### Variable Declaration
- Use `:=` only when ALL variables on the left are new
- Use `var` to declare new variables when mixing with existing ones
- Use `var name bool` for flag variables (leverage zero value)

### Comments
- Prefer self-documenting code over comments
- Use English for all comments
- Only add comments where logic is not self-evident

## Build Commands

```bash
# Build all packages
go build -v -mod=vendor -o /dev/null ./...

# Run tests
go test -mod=vendor ./...

# Run go vet
go vet -mod=vendor ./...

# Lint (when golangci-lint-v2 is available)
golangci-lint-v2 run -c .golangci.yaml ./...
```

## Version Information

Build with ldflags to populate `pkg/version`:

```bash
go build -mod=vendor \
  -ldflags "-X 'github.com/humble-mun/chassis/pkg/version.Name=my-service' -X 'github.com/humble-mun/chassis/pkg/version.CommitID=$(git rev-parse --short HEAD)' -X 'github.com/humble-mun/chassis/pkg/version.BuiltAt=$(date -u +%Y-%m-%dT%H:%M:%SZ)'" \
  -o ./bin/my-service ./cmd/my-service
```

## YAML Style

- 2-space indentation
- Array elements `-` not indented

## Dependencies

- **Vendor mode**: use `go mod vendor` to manage dependencies
- **klog replace**: `k8s.io/klog/v2 => github.com/tedli/klog/v2` (custom fork, required for runtime log-level tuning and must be repeated in downstream go.mod files)

## License

chassis is licensed under the Apache License 2.0. See `LICENSE` and `NOTICE`.
