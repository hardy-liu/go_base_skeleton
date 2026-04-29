# Go Base Skeleton

这是一个 Go Web 项目骨架，提供 API 服务、Admin 服务、CLI、配置加载、日志、数据库、Redis、事件、缓存、分布式锁、告警、中间件、统一响应和测试辅助等基础能力。

当前仓库只保留骨架与示例用户模块，不包含具体业务域代码。

## 技术栈

- Go 1.25.6
- Gin：HTTP 路由与中间件
- GORM：MySQL 访问
- go-redis：Redis 客户端
- Cobra：CLI 命令
- Viper：配置加载与环境变量覆盖
- Zap + Lumberjack：结构化日志与文件轮转
- Resty：HTTP Client
- Testify、sqlmock、miniredis：单元测试与依赖模拟

## 目录结构

```text
.
├── cmd
│   ├── api                 # API 服务入口
│   ├── admin               # Admin 服务入口
│   └── cli                 # CLI 入口
├── config
│   ├── config.yaml         # 本地配置
│   └── config.yaml.example # 配置示例
├── internal
│   ├── app                 # 应用装配、启动与资源释放
│   ├── command             # Cobra 命令与 CLI 子命令
│   ├── config              # 配置结构、加载、校验
│   ├── constant            # 通用常量
│   ├── event               # Redis Stream 发布、消费与事件处理示例
│   ├── handler             # API/Admin HTTP Handler
│   ├── middleware          # Trace、Recovery、AccessLog、CORS、JWT、限流、超时等中间件
│   ├── model               # 示例数据模型
│   ├── pkg                 # 通用基础包
│   │   ├── alert           # 多渠道告警
│   │   ├── cache           # Redis JSON 缓存
│   │   ├── database        # MySQL 初始化
│   │   ├── errcode         # 统一错误码
│   │   ├── httpclient      # HTTP Client 封装
│   │   ├── lock            # Redis 分布式锁
│   │   ├── logger          # 日志初始化与写入器
│   │   ├── redis           # Redis 初始化
│   │   ├── response        # 统一 JSON 响应
│   │   ├── trace           # Trace ID 工具
│   │   ├── util            # 通用工具函数
│   │   └── validate        # 参数校验初始化
│   ├── repository          # 示例 Repository
│   ├── router              # API/Admin 路由注册
│   └── service             # 示例 Service
├── test
│   ├── integration         # 集成测试
│   └── testhelper          # 测试辅助工具
├── Makefile                # 常用构建、运行、测试命令
├── go.mod
└── go.sum
```

## 功能列表

### 服务入口

- `cmd/api`：启动业务 API 服务，默认读取 `config/config.yaml`。
- `cmd/admin`：启动 Admin 服务，默认读取 `config/config.yaml`。
- `cmd/cli`：启动 Cobra CLI，支持示例命令和事件消费者命令。

### HTTP 能力

- API 路由：
  - `GET /health`：健康检查。
  - `GET|POST /debug`：非生产环境调试请求头。
  - `GET /users/:uid`：JWT 保护的示例用户查询接口。
- Admin 路由：
  - `GET /users/:uid`：JWT 保护的示例用户查询接口。
- 内置中间件：
  - Trace ID
  - Request Context
  - Panic Recovery
  - Access Log
  - CORS
  - Rate Limit
  - JWT
  - Timeout

### 应用基础设施

- 配置加载：支持 YAML、`.env` 和环境变量覆盖。
- 日志：按服务类型写入 `log/api`、`log/admin`、`log/cli`、`log/event`。
- 数据库：MySQL + GORM 初始化。
- Redis：业务 Redis 与事件 Redis 可分开配置。
- 事件：Redis Stream Publisher / Consumer 封装。
- 缓存：Redis JSON 缓存、TTL、singleflight 回源。
- 分布式锁：基于 Redis 的锁封装。
- HTTP Client：基于 Resty 的统一客户端。
- 告警：支持 Telegram、Webhook、去重与多路 fanout。
- 统一响应：封装成功、失败 JSON 返回。
- 统一错误码：系统级错误和示例业务错误。

### 示例模块

- `internal/model/user.go`：示例用户模型。
- `internal/repository/user.go`：示例用户仓储。
- `internal/service/user.go`：示例用户服务。
- `internal/handler/api/user.go`：API 用户示例接口。
- `internal/handler/admin/user.go`：Admin 用户示例接口。

## 配置说明

复制示例配置后按本地环境调整：

```bash
cp config/config.yaml.example config/config.yaml
```

主要配置块：

- `app`：应用名称和运行环境。
- `server`：API/Admin 监听端口与超时。
- `database`：MySQL 连接池配置。
- `redis`：业务 Redis 配置。
- `jwt`：JWT 密钥、过期时间、签发方。
- `log`：日志目录、级别与轮转配置。
- `ratelimit`：全局限流配置。
- `event`：Redis Stream 事件配置。
- `alert`：告警开关和渠道配置。

生产环境必须设置 `jwt.secret`。

## 常用命令

构建所有入口：

```bash
make build
```

启动 API 服务：

```bash
make run-api
```

启动 Admin 服务：

```bash
make run-admin
```

运行 CLI：

```bash
make cli ARGS="example"
```

运行全部测试：

```bash
make test
```

运行简洁测试输出：

```bash
make test-simple
```

运行集成测试：

```bash
make test-integration
```

## CLI 示例

发布 debug 事件：

```bash
go run ./cmd/cli debug publish_event
```

消费 debug 事件：

```bash
go run ./cmd/cli consume debug
```

运行示例命令：

```bash
go run ./cmd/cli example
```

## 测试

项目已有覆盖以下基础能力的测试：

- 配置加载和校验
- CLI 命令
- Redis Stream 发布与消费
- 中间件
- 告警
- Redis 缓存
- 错误码
- HTTP Client
- 分布式锁
- 通用工具函数
- API 用户接口集成测试

全量验证命令：

```bash
go test ./...
```

## 扩展建议

新增业务模块时建议按以下顺序组织代码：

1. 在 `internal/model` 添加模型。
2. 在 `internal/repository` 添加数据访问。
3. 在 `internal/service` 添加业务逻辑。
4. 在 `internal/handler` 添加 HTTP Handler。
5. 在 `internal/router` 注册路由。
6. 为新增逻辑补充单元测试或集成测试。

保持业务代码和 `internal/pkg` 通用基础能力分离，避免把具体业务规则写入基础包。
