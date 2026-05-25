# chassis

[English](README.md) | 简体中文

humble-mun 微服务的共享基础设施工具集。

## 概述

chassis 提供从 humble-mun 平台中提炼出的通用构建模块,让多个服务在应用启动引导、HTTP 服务、指标采集、日志以及工具函数等方面保持一致。

## 包

| 包 | 说明 |
|---------|-------------|
| `pkg/app` | 应用启动引导模式(flag 注册、配置加载、HTTP server 初始化、信号处理) |
| `pkg/logging` | 基于 klog 的 logger 初始化与工厂 |
| `pkg/manager` | controller-runtime manager 启动引导(flag 注册、client QPS/burst、leader 选举、scheme 注册) |
| `pkg/metrics` | Prometheus 指标注册表与 `/metrics` 端点 |
| `pkg/server` | 基于 Gin 的 HTTP/gRPC server:多监听器(TCP + Unix socket)、按监听器配置 TLS、CORS、通过标准库实现 H2C(`http.Server.Protocols`)、优雅关闭、请求日志 |
| `pkg/service` | 通用基础设施常量(flag 名称、默认绑定地址、TLS 路径) |
| `pkg/utils` | 工具函数(切片操作、镜像名归一化、Kubernetes 名称归一化、SSH 密钥生成、健康探针、API 路由组) |
| `pkg/version` | 构建版本信息模板,支持 ldflags |

## 用法

```go
import (
    "github.com/humble-mun/chassis/pkg/app"
    "github.com/humble-mun/chassis/pkg/server"
    "github.com/humble-mun/chassis/pkg/utils"
    "github.com/humble-mun/chassis/pkg/version"
)

// 设置应用名称(通常通过 ldflags 注入)
version.Name = "my-service"

// 启动引导应用
init := app.PrepareFlags("my-service", rootCmd)
rootLogger, logger, httpServer, ctx, nodeName, err := app.BaseContext(app.WithInit(init))

// 注册路由
httpServer.RegisterRoute(utils.APIVersion("/api/v1")(
    myService.RegisterRoute,
))
```

`BaseContext` 还会返回从 `node-name` flag/配置解析得到的 `nodeName` 值,调用方可直接复用。

`BaseContext` 接受函数式选项(functional options):

- `app.WithInit(fn)` - 设置由 `PrepareFlags` 返回的 viper 初始化函数
- `app.WithGRPCServer(s)` - 挂载一个 gRPC server;`Content-Type: application/grpc` 的请求会被路由到该 server
- `app.WithTCPListener(...server.ListenerOption)` - 添加一个 TCP 监听器;用 `server.WithAddr(fn)` 提供绑定地址,用 `server.WithTLSCert(cert, key)` 启用 TLS
- `app.WithUnixListener(...server.ListenerOption)` - 添加一个 Unix domain socket 监听器;用 `server.WithAddr(fn)` 提供 socket 路径

`BaseContext` 会自动在最前面加入 `server.WithDefaultListener()` 和 `server.WithDefaultCORSConfig()`,因此除非显式覆盖,基于 flag 的默认配置始终生效。

## 构建

```bash
go build -v -mod=vendor -o /dev/null ./...
```

## 测试

```bash
go test -mod=vendor ./...
```

## 健康探针

`pkg/utils.RegisterProbeRoute` 提供 `/healthz` 和 `/readyz`。

项目可以通过调用以下函数扩展探针行为:

- `utils.RegisterLivenessCheck(name, check)`
- `utils.RegisterReadinessCheck(name, check)`

`ProbeCheck` 使用如下签名:

```go
func(context.Context) (ok bool, message string, err error)
```

约定:

- `ok=true` 表示检查通过。
- `ok=false, message!="", err==nil` 表示检查因预期原因而失败。
- `err!=nil` 表示检查本身遇到了非预期的内部错误。

隐式契约:必须在 `mgr.Start(ctx)` 之前注册所有检查。不支持在运行时修改全局探针注册表。

## 许可证

基于 Apache License, Version 2.0 授权。详见 [LICENSE](LICENSE) 与 [NOTICE](NOTICE)。
