# CLAUDE.md

本文件为 Claude Code (claude.ai/code) 在此代码库中工作时提供指导。

## 项目概述

Kratos 是一个专注于治理和可靠性的 Go 微服务框架。它提供了传输层、服务发现、配置、日志和中间件等抽象。这是修改版 Fork (`github.com/yola1107/kratos/v2`)，包含额外的功能扩展。

## 构建命令

```bash
# 构建所有 CLI 工具 (kratos, protoc-gen-go-http, protoc-gen-go-errors 等)
make all

# 安装所有工具到 $GOBIN 或 $GOPATH/bin
make install

# 运行所有模块的测试
make test

# 运行测试并生成覆盖率报告
make test-coverage

# 运行代码检查
make lint

# 自动修复代码问题
make fix

# 清理/整理所有 go.mod 文件
make clean

# 生成内部元数据的 protobuf 代码
make proto
```

## 单包测试

```bash
# 运行指定包的测试
go test -race ./transport/http/...

# 运行指定测试函数
go test -race -run TestServerStart ./transport/grpc/...

# 测试并生成覆盖率报告
go test -race -coverprofile=coverage.out ./config/...
```

## 多模块仓库

这是一个多模块 Go 仓库，每个模块有自己的 `go.mod`：
- 根模块: `github.com/yola1107/kratos/v2`
- CLI 工具: `cmd/kratos/`, `cmd/protoc-gen-go-http/` 等
- Contrib 插件: `contrib/config/*`, `contrib/registry/*`, `contrib/log/*` 等

`hack/tools.sh` 中的测试/代码检查脚本会遍历 `util::find_modules` 找到的所有模块。

## 架构设计

### 核心抽象层

1. **App** (`app.go`): 应用生命周期管理器。管理服务启动/关闭、服务注册、信号处理。

2. **Transport** (`transport/transport.go`): 传输层接口，定义 `Start/Stop` 方法。支持五种传输：
   - HTTP (`transport/http/`)
   - gRPC (`transport/grpc/`)
   - TCP (`transport/tcp/`)
   - WebSocket (`transport/websocket/`)
   - GNet (`transport/gnet/`) - 基于 panjf2000/gnet 的高性能网络

3. **Middleware** (`middleware/middleware.go`): `type Middleware func(Handler) Handler`。使用 `middleware.Chain()` 链式组合。内置中间件包括: auth/jwt, circuitbreaker, logging, metrics, ratelimit, recovery, tracing, validate。

4. **Registry** (`registry/registry.go`): 服务发现接口，包含 `Registrar` 和 `Discovery`。实现在 `contrib/registry/`: consul, etcd, nacos, kubernetes, eureka, polaris, servicecomb, zookeeper。

5. **Config** (`config/config.go`): 配置系统，支持通过 `Watch()` 热重载。配置源在 `config/file/`, `config/env/`。远程配置在 `contrib/config/`: apollo, consul, etcd, kubernetes, nacos, polaris。

6. **Selector** (`selector/selector.go`): 负载均衡器接口，核心方法 `Select()`。实现: random, wrr (加权轮询), p2c, ewma。

7. **Errors** (`errors/errors.go`): 统一错误定义，包含 code/reason/message。与 gRPC status 集成。

### 目录结构

```
├── app.go              # 应用生命周期
├── options.go          # 应用选项（函数式选项模式）
├── cmd/                # CLI 工具
│   ├── kratos/         # 主 CLI: kratos new, proto, run, upgrade
│   └── protoc-gen-*/   # Protobuf 代码生成器
├── transport/          # 传输层实现
├── middleware/         # 内置中间件
├── registry/           # 服务发现接口
├── selector/           # 负载均衡
├── config/             # 配置系统
├── encoding/           # 编解码实现 (json, proto, yaml, xml, form)
├── errors/             # 错误处理
├── log/                # 日志接口
├── metadata/           # 元数据传播
├── library/            # 扩展工具库（此 Fork 特有）
│   ├── db/redis/       # Redis 客户端（集群+单机，函数式选项）
│   ├── db/xorm/        # XORM 数据库客户端封装
│   ├── mq/rabbitmq/    # RabbitMQ 发布/消费
│   ├── log/zap/        # Zap 日志，Wire DI，文件轮转
│   ├── work/           # 时间轮调度器，循环工作者
│   ├── xgo/            # 工具函数: retry, safecall, json, copy, diff, rand, time
│   └── event/          # 事件处理工具
├── contrib/            # 第三方集成
│   ├── config/         # 配置源 (apollo, nacos, consul 等)
│   ├── registry/       # 服务注册中心
│   ├── log/            # 日志实现 (zap, zerolog 等)
│   └── middleware/     # 额外中间件
├── third_party/        # Protobuf 定义
└── ztest/              # 测试应用（api-server, games）
```

## Transport 传输层详解

传输层是 Kratos 框架的核心抽象，所有传输类型都实现统一的 `transport.Server` 接口。

### 核心接口设计

```go
// transport/transport.go

// Server 传输服务接口
type Server interface {
    Start(context.Context) error
    Stop(context.Context) error
}

// Endpointer 注册端点接口
type Endpointer interface {
    Endpoint() (*url.URL, error)
}

// Transporter 传输上下文接口
type Transporter interface {
    Kind() Kind              // 传输类型: grpc, http, tcp, websocket, gnet
    Endpoint() string        // 端点地址
    Operation() string       // 操作名 (如 /helloworld.Greeter/SayHello)
    RequestHeader() Header   // 请求头
    ReplyHeader() Header     // 响应头
}

// Header 头信息接口
type Header interface {
    Get(key string) string
    Set(key, value string)
    Add(key, value string)
    Keys() []string
    Values(key string) []string
}
```

### 五种传输实现对比

| 特性 | gRPC | HTTP | TCP | WebSocket | GNet |
|------|------|------|-----|-----------|------|
| **底层库** | google.golang.org/grpc | net/http + gorilla/mux | net | gorilla/websocket | panjf2000/gnet |
| **协议格式** | Protobuf | JSON/Form/XML | 自定义 Protobuf | 自定义 Protobuf | 自定义 Protobuf |
| **连接模型** | 持久连接 | 短连接/Keep-Alive | 持久连接 | 持久双工 | 事件驱动 |
| **中间件** | ✅ 选择器匹配 | ✅ 选择器匹配 | ✅ 选择器匹配 | ✅ 选择器匹配 | ✅ 选择器匹配 |
| **服务发现** | ✅ discovery resolver | ✅ resolver | ❌ | ❌ | ❌ |
| **负载均衡** | ✅ WRR/P2C/EWMA | ✅ WRR/P2C/EWMA | ❌ | ❌ | ❌ |
| **健康检查** | ✅ grpc_health | ❌ | ❌ | ❌ | ❌ |
| **TLS支持** | ✅ | ✅ | ✅ | ✅ | ❌ |
| **典型场景** | 微服务间调用 | REST API/Web | 游戏服务器 | 实时通信 | 高性能游戏 |

### gRPC 传输 (`transport/grpc/`)

```
grpc/
├── server.go        # Server 封装，集成健康检查、反射、元数据服务
├── client.go        # Dial/DialInsecure，服务发现集成
├── interceptor.go   # Unary/Stream 拦截器，中间件注入
├── transport.go     # Transport 实现，gRPC metadata 封装
├── balancer.go      # 自定义负载均衡器
├── codec.go         # Protobuf 编解码
└── resolver/
    ├── direct/      # 直连解析器 (direct:///)
    └── discovery/   # 服务发现解析器 (discovery:///)
```

**关键特性：**
- 内置 gRPC 健康检查服务
- 支持 gRPC Reflection（可用 grpcurl 调试）
- 服务发现通过自定义 resolver 实现
- 支持 Unary 和 Stream 两种调用模式

**服务端创建示例：**
```go
srv := grpc.NewServer(
    grpc.Address(":9000"),
    grpc.Timeout(5*time.Second),
    grpc.Middleware(m1, m2),
)
```

### HTTP 传输 (`transport/http/`)

```
http/
├── server.go        # Server 封装，gorilla/mux 路由
├── client.go        # HTTP Client，服务发现支持
├── transport.go     # Transport 实现，http.Header 封装
├── router.go        # 路由器封装
├── filter.go        # HTTP 中间件
├── context.go       # 请求上下文
├── codec.go         # 编解码器
├── binding/         # 请求绑定
├── status/          # HTTP 状态码转换
└── pprof/           # pprof 集成
```

**关键特性：**
- 使用 `gorilla/mux` 作为路由器
- 支持自定义请求解码器/响应编码器
- 支持 CORS 跨域
- 支持 pprof 性能分析端点

### TCP 传输 (`transport/tcp/`)

```
tcp/
├── server.go        # 主服务，Bucket 会话管理
├── server_tcp.go    # TCP 连接处理
├── client.go        # TCP 客户端
├── transport.go     # Transport 实现
├── interceptor.go   # 拦截器
├── proto/           # 协议定义
└── internal/
    ├── bucket/      # 会话分桶管理
    ├── round/       # 读写协程池
    ├── proxy/       # 代理协议支持
    └── websocket/   # 内置 WebSocket 支持
```

**协议格式：**
```protobuf
message Payload {
  int32 Place = 1;    // 来源：客户端/服务端
  int32 Type = 2;     // 类型：Ping/Pong/Request/Response/Push
  int32 Code = 3;     // 错误码
  bytes Body = 4;     // 消息体
}

message Body {
  int32 Ops = 1;      // 操作码
  bytes Data = 2;     // 业务数据
}
```

**关键特性：**
- Bucket 分桶管理海量连接
- Round 读写协程池提高性能
- 支持服务端主动推送

### WebSocket 传输 (`transport/websocket/`)

```
websocket/
├── server.go        # 服务端，HTTP 升级 WebSocket
├── client.go        # 客户端
├── session.go       # 会话管理
├── session_mgr.go   # 会话管理器
├── transport.go     # Transport 实现
├── interceptor.go   # 拦截器
└── proto/           # 协议定义
```

**关键特性：**
- 基于 HTTP 升级，支持 CORS
- Session 管理连接生命周期
- 支持连接/断开事件回调
- 自动 Ping/Pong 心跳

### GNet 传输 (`transport/gnet/`)

```
gnet/
├── server.go        # 高性能事件驱动服务
├── client.go        # 客户端
├── service.go       # 服务注册
└── transport.go     # Transport 实现
```

**关键特性：**
- 基于 `panjf2000/gnet` 高性能网络库
- 事件驱动模型
- 自定义帧协议：4字节长度头 + Protobuf 消息体

### 传输层选择建议

| 场景 | 推荐传输 | 理由 |
|------|----------|------|
| 微服务间同步调用 | gRPC | 强类型、服务发现、负载均衡、健康检查 |
| 对外 REST API | HTTP | 通用协议、易于调试、Swagger 支持 |
| 实时游戏/聊天 | WebSocket | 双向通信、浏览器兼容 |
| 高性能游戏服务端 | GNet | 事件驱动、零拷贝、极致性能 |
| 自定义协议服务 | TCP | 完全控制、灵活协议设计 |

### 统一设计模式

**1. 所有 Server 实现相同接口：**
```go
var _ transport.Server = (*grpc.Server)(nil)
var _ transport.Server = (*http.Server)(nil)
var _ transport.Server = (*tcp.Server)(nil)
var _ transport.Server = (*websocket.Server)(nil)
var _ transport.Server = (*gnet.Server)(nil)
```

**2. Context 传播：**
```go
// 服务端：将 Transport 信息注入 Context
ctx = transport.NewServerContext(ctx, tr)

// 从 Context 提取 Transport 信息
if tr, ok := transport.FromServerContext(ctx); ok {
    // 获取请求头、操作名等
}
```

**3. 中间件匹配器：**
- 支持 `/*` 全局匹配
- 支持 `/service/*` 服务级匹配
- 支持 `/service/method` 方法级匹配

## Protobuf 代码生成

框架使用自定义 protoc 插件：
- `protoc-gen-go-http`: 生成 HTTP 传输代码
- `protoc-gen-go-tcp`: 生成 TCP 传输代码
- `protoc-gen-go-websocket`: 生成 WebSocket 传输代码
- `protoc-gen-go-gnet`: 生成 GNet 传输代码
- `protoc-gen-go-errors`: 生成错误定义

示例（来自 `ztest/api-server/Makefile`）：
```bash
protoc --proto_path=./api \
       --proto_path=./third_party \
       --go_out=paths=source_relative:./api \
       --go-http_out=paths=source_relative:./api \
       --go-grpc_out=paths=source_relative:./api \
       $(API_PROTO_FILES)
```

## 依赖注入

使用 Google Wire (`github.com/google/wire`)。示例见 `library/log/zap/wire.go`。

## 关键设计模式

1. **函数式选项模式**: 贯穿整个框架 (如 `kratos.New(kratos.Name("service"), kratos.Version("1.0"))`)

2. **接口隔离原则**: 小而专注的接口 (Logger, Registrar, Discovery, Selector)

3. **Context 传播**: 通过 `NewServerContext`/`FromServerContext` 在请求链路传递传输信息

4. **中间件链**: `middleware.Chain(m1, m2, m3)` 按顺序应用

5. **错误包装**: 使用 `errors.New(code, reason, message)` 和 `FromError(err)` 处理错误

## Go 版本要求

需要 Go 1.24.2+（见 `go.mod`）。

## Library 扩展库（此 Fork 特有）

`library/` 目录包含此 Fork 特有的扩展工具：

### Redis 客户端 (`library/db/redis/`)
```go
// 支持单机和集群模式，使用 UniversalClient
client := redis.NewClient(
    redis.WithAddrs("127.0.0.1:6379"),
    redis.WithPassword("password"),
    redis.WithPoolSize(20),
)
```

### 时间轮调度器 (`library/work/`)
高精度定时调度器，用于周期性任务：
- `timer_wheel.go` - 时间轮实现，可配置精度
- `loop.go` - 循环工作者模式
- 内部使用 `github.com/RussellLuo/timingwheel`

### xgo 工具函数 (`library/xgo/`)
常用工具函数：
- `retry.go` - 带退避的重试
- `safecall.go` - 安全函数调用，带 panic 恢复
- `copy.go` - 深拷贝工具
- `diff.go` - 结构体比较
- `json.go` - JSON 辅助函数，使用 json-iterator

## 测试说明

- 测试使用 `testify` 断言库
- `ztest/` 中的测试应用展示真实使用模式（游戏服务器: whot, ludo）
- 忽略的模块列在 `hack/.test_ignored_files`
- Lint 例外列在 `hack/.lintcheck_failures`


---

## 开发规范

**重构后必须执行：**
1. 精简优化代码，移除冗余
2. 运行 code-review 检查代码质量
3. 确保编译通过

## 代码可读性与规范

### 命名规范
1. **意图明确** - 变量/函数名应准确表达其用途，避免缩写
2. **一致术语** - 同一概念在不同位置使用相同词汇
3. **布尔变量** - 使用 is/has/can 等前缀表示状态

### 函数设计
1. **单一职责** - 每个函数只做一件事
2. **参数限制** - 参数不超过3个，过多则封装为结构体
3. **长度控制** - 函数不超过30行，超出需拆分

### 代码组织
1. **就近原则** - 辅助函数紧随主函数
2. **逻辑分块** - 相关函数集中放置
3. **垂直间隔** - 逻辑单元用空行分隔

### 控制流优化
1. **提前返回** - 减少嵌套层级
2. **卫语句** - 异常情况优先处理
3. **避免else** - 用guard clauses替代条件嵌套

### 注释策略
1. **解释意图** - 说明为什么这么做，而非代码逻辑
2. **复杂算法** - 核心逻辑需简要说明
3. **避免废话** - 不重复显而易见的信息

### 避免过度优化
1. **KISS原则** - 保持简单，优先选择最容易理解和维护的实现
2. **渐进式优化** - 先实现功能，再根据实际性能数据进行针对性优化
3. **验证驱动** - 任何性能优化都应有明确的基准测试数据支持
4. **复杂度阈值** - 优化带来的性能提升应显著（≥20%）才考虑引入复杂度
5. **可读性优先** - 在性能差异不大的情况下，选择更易读易懂的实现方式

---
