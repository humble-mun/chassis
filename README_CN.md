# chassis

> [English](./README.md)

`chassis` 是 humble-mun 多个生产环境项目共享的基础设施工具集。它提供各服务在引导 HTTP/gRPC 服务器、指标、日志、健康探针以及 controller-runtime 管理器时所需的通用构建块。

Go 模块：`github.com/humble-mun/chassis`

## 项目定位

`chassis` **不是**脚手架工具，也无意取代 kubebuilder、operator-sdk 等项目生成器。它是 humble-mun 内部各组件共同依赖的共通逻辑库 —— 引导、服务器、指标、日志、探针等构建块，否则每个组件都要各自重复实现一遍。

由于这些组件正在开源，它们共同依赖的这个库也随之开源，以保证它们在我们内部环境之外仍可独立构建、自成一体。换句话说，本仓库的存在主要是为那些组件服务，而非要做成一个通用框架。

尽管如此，我们非常欢迎需求与建议。本工具集源自内部需求，仍带有一些当时的假设；来自外部的具体使用场景能帮助我们把它重构得更通用、更可复用。如果你在使用中遇到别扭之处，或需要某个尚未提供的构建块，欢迎提 issue。

## 包结构

| 包 | 用途 |
|---------|---------|
| `pkg/app` | 应用引导：`PrepareFlags` + `BaseContext`，以及用于监听器和 gRPC 的函数式选项 |
| `pkg/server` | 基于 Gin 的多监听器 HTTP/gRPC 服务器，支持 TLS、mTLS、H2C、CORS 与优雅关停 |
| `pkg/metrics` | Prometheus 注册表封装、`/metrics` 端点以及可选的抓取钩子 |
| `pkg/manager` | controller-runtime 管理器引导 |
| `pkg/logging` | klog 初始化与 `logr.Logger` 工厂 |
| `pkg/utils` | 叶子辅助包：切片中间件、镜像/k8s 名称规范化、SSH、viper flag、探针、infra-token 认证 |
| `pkg/constants` | 通用 flag 名称与默认值 |
| `pkg/version` | 由 ldflags 注入的构建/版本模板 |

## 快速开始

典型的消费方在 `main.go` 中这样引导：

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
            // WithInit(init) 会执行 PrepareFlags 产生的 viper 初始化函数；
            // 不传它，flag 与配置就不会被解析。
            var base app.Base
            if base, err = app.BaseContext(app.WithInit(init)); err != nil {
                return
            }
            _ = base.RootLogger
            _ = base.Logger
            _ = base.NodeName

            // 在共享服务器上注册业务路由、启动控制器等。
            return base.HTTPGin.Start(base.Ctx)
        },
    }

    // PrepareFlags 返回 viper 初始化函数；通过 WithInit 传给 BaseContext。
    init = app.PrepareFlags(version.Name, cmd)

    if err := cmd.Execute(); err != nil {
        os.Exit(1)
    }
}
```

构建期版本信息通过 ldflags 注入（参见下文 [构建建议](#构建建议)）。

## 约定

本节记录有意为之的设计决策，应当在各消费方之间保留延续。

### klog fork 与 `replace` 指令

`chassis` 依赖 `k8s.io/klog/v2` 的一个 fork：

```go
replace k8s.io/klog/v2 => github.com/tedli/klog/v2 v2.0.0-20260407032038-5a4969b5a1c1
```

该 fork 增加了一个 HTTP API（`klog.Configure`），允许运维人员在运行时改变日志级别。这在生产环境中极为有用：发生故障时，你可以临时调高 verbosity 而无需重启进程，收集额外日志后再调回去。

Go 中的 `replace` 指令只在被构建的**主模块**中生效。当 `chassis` 作为依赖被消费时，它自身的 `replace` 会被 Go 工具链忽略。因此每个下游服务都必须在其 `go.mod` 中重复同一行 `replace`。这是 Go 的硬性约束，而非选择。将 fork 保持在原始的 `k8s.io/klog/v2` 模块路径下是必需的，这样间接依赖（`client-go`、`controller-runtime` 等）才能共享同一份全局 klog 状态，并同样可在运行时调节。

### BaseContext 是唯一的组合根

`app.BaseContext` 是创建 logger、带信号处理的 context、HTTP 服务器和节点名的唯一预期位置。消费方应优先通过选项（`app.WithInit`、`app.WithGRPCServer`、`app.WithTCPListener`、`app.WithUnixListener`、`app.WithoutHTTPServer`）来扩展它，而不是创建第二个 viper 实例、第二个 logger 或第二个服务器。

### 配置是唯一的 viper 全局单例

所有配置都流经由 `BaseContext` 组合的同一个进程级全局 viper 实例。不要引入额外的配置文件，也不要创建第二个 viper 实例。这条规则是刻意的：如果某个组件积累了太多选项，以至于看起来需要自己的配置文件，那是该组件应当被拆分的信号，而不是应当再增加一个配置源的信号。对于复杂的嵌套配置项，应将其绑定到单个 flag，并用 `viper.UnmarshalKey` 解析其子树，例如 `viper.UnmarshalKey("some-complex-config", &cfg)`，而不是读取一个独立的文件。

### 版本信息是进程级的，用于排查问题

`pkg/version` 通过 ldflags 在编译时注入构建元数据（`Name`、`CommitID`、`BuiltAt`、`Architecture`、`Variant`、`RecentCommits`），以包级变量的形式暴露。与全局 viper 实例一样，这是刻意的进程级设计：如果一个服务需要跟踪多个 version，那是它已经足够复杂、应当被拆分的信号，而不是应当让 `version` 持有按组件区分的多个实例。`version` 想解决的问题是生产环境的排查定位 —— 在生产环境往往无法判断当前运行的二进制是否已经包含某次改动，因此在编译时注入构建时间、commit id 等，便于运维确认并定位问题。它**不是**用来作为 “UI” 把版本展示给最终用户的功能。

### Controller-runtime 的指标服务器与 webhook 有意未使用

所有基于 `chassis` 构建的服务都禁用了 controller-runtime 的指标服务器（`BindAddress: "0"`），并且**不**使用 controller-runtime 的 webhook 服务器。取而代之：

* 指标通过共享的 Gin 服务器在 `/metrics` 上提供（`pkg/metrics`）。在生产中我们发现 controller-runtime 的默认指标在排查问题时参考价值不大，却会带来较大的 prometheus 开销；只暴露与业务相关的指标可以让 prometheus 占用保持较小。
* Webhook 复用同一个 Gin 服务器。Controller-runtime 的 webhook 辅助工具主要处理序列化；实际的校验/变更逻辑无论如何都要自己编写，因此使用共享服务器可以让架构保持统一。

### 注册函数无锁：启动期一次性注册，之后不可变

`pkg/utils/health.go` 将探针检查存储在一个无锁的普通 `map` 中，`pkg/metrics/metrics.go` 将抓取钩子存储在一个无锁的普通切片中。这是有意为之的。`utils.RegisterLivenessCheck`、`utils.RegisterReadinessCheck` 和 `metrics.RegisterScrapeHook` 都应在启动期间（`main` 或组合根中）于 `mgr.Start(ctx)` 运行之前调用一次，之后这些注册表被视为不可变，运行时不再修改。因此它们无需加锁。Kubernetes 依赖 `/healthz` 和 `/readyz` 保持未认证；切勿为探针路由添加认证。

### 基础设施端点为可选认证

`/logging`（klog 运行时配置）和 `/debug/pprof` 可以改变进程行为，并且可能通过 ingress 暴露。它们由一个可选的 infra token 保护：

```bash
my-service --infra-api-token=<secret>
```

当设置了 token 时，请求必须包含以下请求头：

```text
X-API-Token: <secret>
```

当 token 为空时，这些端点保持开放（向后兼容行为）。`/metrics`、`/healthz` 和 `/readyz` 保持未认证。

## 构建建议

`chassis` 要求 **Go 1.26 或更高版本**（见 `go.mod` 中的 `go` 指令）。

本仓库使用 vendored 依赖。从仓库根目录构建和测试：

```bash
# 编译所有包
go build -mod=vendor -v ./...

# 运行测试
go test -mod=vendor ./...

# 运行 go vet
go vet -mod=vendor ./...
```

进行 lint 时（当有 Linux 版 `golangci-lint-v2` 二进制可用时）：

```bash
golangci-lint-v2 run -c .golangci.yaml ./...
```

### 注入版本信息

在链接期填充 `pkg/version` 变量。推荐使用类似如下的 `Makefile` 和 `Dockerfile`：

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

将 `my-service` 替换为实际的服务名称和导入路径。

## 许可证

`chassis` 基于 Apache License 2.0 许可。参见 [LICENSE](LICENSE) 与 [NOTICE](NOTICE)。
