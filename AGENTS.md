# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

chassis is a shared infrastructure toolkit for humble-mun microservices. It provides common building blocks that are reused across multiple services.

**Go module**: `github.com/humble-mun/chassis`

## Package Structure

| Package | Purpose | Dependencies |
|---------|---------|--------------|
| `pkg/utils` | Leaf package: slice helpers, gin middleware, image/k8s name normalization, SSH, viper config, health probes, route groups | gin, viper, pflag, crypto/ssh |
| `pkg/logging` | Leaf package: klog initialization and logger factory | klog, logr, pflag |
| `pkg/metrics` | Prometheus registry wrapper and /metrics endpoint | prometheus, gin, utils |
| `pkg/version` | Build version info template | stdlib only |
| `pkg/service` | Common infrastructure constants (flag names, defaults) | stdlib only |
| `pkg/server` | HTTP/gRPC server with Gin: multi-listener (TCP + Unix socket), TLS per listener, CORS, H2C via standard library, graceful shutdown, request logging | gin, cors, grpc, errgroup, metrics, service, utils |
| `pkg/manager` | controller-runtime manager bootstrap (flags, client QPS/burst, leader election, scheme registration) | controller-runtime, viper, pflag, logr, service |
| `pkg/app` | Application bootstrap (PrepareFlags + BaseContext); BaseContext returns optional `*server.HTTPServer` and accepts functional options | all above packages, cobra, controller-runtime |

### Internal Dependency Graph

```
app --> logging, metrics, server, service, utils, version
server --> metrics, service, utils
manager --> service
metrics --> utils
logging --> (none)
version --> (none)
service --> (none)
utils --> (none)
```

## Consumer Configuration

Consumers should set these variables before calling `app.BaseContext()`:

- `version.Name` - application name (typically set via ldflags)
- `service.ConfigName` - config file name for viper (set before PrepareFlags if needed)

`BaseContext` accepts functional options:

- `app.WithInit(fn)` - sets the viper initialization function returned by `PrepareFlags`
- `app.WithoutHTTPServer()` - skips HTTP server construction and route registration; `BaseContext` returns `nil` for `httpGin`, and server-related options are ignored
- `app.WithGRPCServer(s)` - attaches a gRPC server; requests with `Content-Type: application/grpc` are routed to it
- `app.WithTCPListener(...ListenerOption)` - adds a TCP listener; use `server.WithAddr(fn)` to supply the bind address and `server.WithTLSCert(cert, key)` to enable TLS
- `app.WithUnixListener(...ListenerOption)` - adds a Unix domain socket listener; use `server.WithAddr(fn)` to supply the socket path

When HTTP server construction is enabled, `BaseContext` prepends `server.WithDefaultListener()` and `server.WithDefaultCORSConfig()` automatically, so flag-driven defaults are always active unless overridden.

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

# Lint (when golangci-lint-v2 is available)
golangci-lint-v2 run -c .golangci.yaml ./...
```

## YAML Style

- 2-space indentation
- Array elements `-` not indented

## Dependencies

- **Vendor mode**: use `go mod vendor` to manage dependencies
- **klog replace**: `k8s.io/klog/v2 => github.com/tedli/klog/v2` (custom fork)
