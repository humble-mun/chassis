# chassis

English | [简体中文](README_CN.md)

Shared infrastructure toolkit for humble-mun microservices.

## Overview

chassis provides common building blocks extracted from the humble-mun platform, enabling consistent application bootstrapping, HTTP serving, metrics collection, logging, and utility functions across multiple services.

## Packages

| Package | Description |
|---------|-------------|
| `pkg/app` | Application bootstrap pattern (flag registration, config loading, optional HTTP server initialization, signal handling) |
| `pkg/logging` | klog-based logger initialization and factory |
| `pkg/manager` | controller-runtime manager bootstrap (flag registration, client QPS/burst, leader election, scheme registration) |
| `pkg/metrics` | Prometheus metric registry and `/metrics` endpoint |
| `pkg/server` | HTTP/gRPC server with Gin: multi-listener (TCP + Unix socket), TLS per listener, CORS, H2C via standard library (`http.Server.Protocols`), graceful shutdown, and request logging |
| `pkg/service` | Common infrastructure constants (flag names, default bind addresses, TLS paths) |
| `pkg/utils` | Utility functions (slice operations, image name normalization, Kubernetes name normalization, SSH key generation, health probes, API route groups) |
| `pkg/version` | Build version info template with ldflags support |

## Usage

```go
import (
    "github.com/humble-mun/chassis/pkg/app"
    "github.com/humble-mun/chassis/pkg/server"
    "github.com/humble-mun/chassis/pkg/utils"
    "github.com/humble-mun/chassis/pkg/version"
)

// Set application name (typically via ldflags)
version.Name = "my-service"

// Bootstrap application
init := app.PrepareFlags("my-service", rootCmd)
rootLogger, logger, httpServer, ctx, nodeName, err := app.BaseContext(app.WithInit(init))

// Register routes
httpServer.RegisterRoute(utils.APIVersion("/api/v1")(
    myService.RegisterRoute,
))
```

`BaseContext` also returns the resolved `nodeName` value from the `node-name` flag/config so callers can reuse it directly.

`BaseContext` accepts functional options:

- `app.WithInit(fn)` - sets the viper initialization function returned by `PrepareFlags`
- `app.WithoutHTTPServer()` - skips HTTP server construction and route registration; `BaseContext` returns `nil` for `httpServer`, and server-related options are ignored
- `app.WithGRPCServer(s)` - attaches a gRPC server; requests with `Content-Type: application/grpc` are routed to it
- `app.WithTCPListener(...server.ListenerOption)` - adds a TCP listener; use `server.WithAddr(fn)` to supply the bind address and `server.WithTLSCert(cert, key)` to enable TLS
- `app.WithUnixListener(...server.ListenerOption)` - adds a Unix domain socket listener; use `server.WithAddr(fn)` to supply the socket path

When HTTP server construction is enabled, `BaseContext` prepends `server.WithDefaultListener()` and `server.WithDefaultCORSConfig()` automatically, so flag-driven defaults are always active unless overridden.

## Build

```bash
go build -v -mod=vendor -o /dev/null ./...
```

## Test

```bash
go test -mod=vendor ./...
```

## Health probes

`pkg/utils.RegisterProbeRoute` serves `/healthz` and `/readyz`.

Projects can extend probe behavior by calling:

- `utils.RegisterLivenessCheck(name, check)`
- `utils.RegisterReadinessCheck(name, check)`

`ProbeCheck` uses the signature:

```go
func(context.Context) (ok bool, message string, err error)
```

Conventions:

- `ok=true` means the check passed.
- `ok=false, message!="", err==nil` means the check failed for an expected reason.
- `err!=nil` means the check itself hit an unexpected internal error.

Implicit contract: register all checks before `mgr.Start(ctx)`. Runtime mutation of the global probe registry is not supported.

## License

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) and [NOTICE](NOTICE) for details.
