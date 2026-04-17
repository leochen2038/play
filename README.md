# Play Framework

Play 是一个 Go 语言多协议微服务框架，提供了从项目初始化、路由定义、业务处理、数据库访问到 API 文档生成的完整开发工具链。

## 特性

- **多协议支持** — HTTP、H2C (HTTP/2 Cleartext)、TCP、WebSocket、SSE (Server-Sent Events)、QUIC/HTTP3、MCP
- **Action 路由系统** — 基于 DSL 文件定义路由，支持处理器链式调用与条件分支
- **自动代码生成** — `goplay` CLI 工具自动生成 init 注册代码、Processor 模板、数据库查询代码
- **Meta 数据建模** — 通过 XML 定义数据模型，自动生成 ORM 风格的链式查询 API
- **生命周期钩子** — 提供 OnBoot / OnShutdown / OnConnect / OnClose / OnRequest / OnResponse / OnFinish 等完整生命周期回调
- **优雅重启** — 基于 SIGUSR2 信号的 Graceful Restart，实现零停机更新
- **内置日志** — 按天自动切割，自动清理过期日志，支持 trace 链路追踪
- **定时任务** — 集成 cron 定时任务，支持文件/JSON 动态更新调度
- **配置管理** — 支持 JSON 文件配置，支持热更新
- **SDK 与文档生成** — 自动生成调用方 SDK 代码和 Markdown 格式 API 文档
- **服务间调用** — 提供 Agent 接口，内置 H2C/HTTP/TCP/QUIC 多种调用方式
- **连接池** — 内置带权重的 TCP 连接池，支持服务分组

## 安装

```bash
go get github.com/leochen2038/play
```

安装 CLI 工具：

```bash
go install github.com/leochen2038/play/goplay@latest
```

## Hello World 快速上手

通过一个完整的例子，5 分钟跑通框架的核心流程：创建 HTTP 接口、接收参数、操作数据库、返回 JSON。

### Step 0: 安装CLI工具

```bash
go install github.com/leochen2038/play/goplay@latest
# 将bin路径添加到环境变量
export PATH="$PATH:$(go env GOPATH)/bin"
```

### Step 1: 初始化项目

```bash
mkdir hello && cd hello
goplay init .
go mod tidy
```

生成的项目结构：

```
hello/
├── main.go              # 入口文件
├── go.mod
├── hook/
│   └── server_hook.go   # 生命周期钩子
├── assets/
│   ├── action/          # 路由定义 (DSL 文件)
│   └── meta/            # 数据模型定义 (XML)
├── processor/           # 业务处理器
├── library/             # 自动生成的代码 (db 查询 / metas 结构体)
├── database/
├── template/
└── utils/
```

### Step 2: 建表 & 定义数据模型

先在 MySQL 中建一张表：

```sql
CREATE DATABASE IF NOT EXISTS d_hello;
CREATE TABLE d_hello.t_message (
    Fid        INT AUTO_INCREMENT PRIMARY KEY,
    Fname      VARCHAR(64)  NOT NULL DEFAULT '',
    Fcontent   VARCHAR(256) NOT NULL DEFAULT '',
    Fctime     BIGINT       NOT NULL DEFAULT 0,
    Fmtime     BIGINT       NOT NULL DEFAULT 0
);
```

在 `assets/meta/` 下创建 `hello_message.xml`：

```xml
<?xml version="1.0" encoding="UTF-8"?>
<meta module="d_hello" name="t_message">
    <key name="Fid" alias="id" type="auto" note="自增ID" />
    <fields>
        <field name="Fname"    alias="name"    type="string" default=""  note="姓名" />
        <field name="Fcontent" alias="content" type="string" default=""  note="内容" />
        <field name="Fctime"   alias="ctime"   type="ctime"  default="0" note="创建时间" />
        <field name="Fmtime"   alias="mtime"   type="mtime"  default="0" note="修改时间" />
    </fields>
    <strategy>
        <storage type="mysql" drive="mysql" database="d_hello" table="t_message" router="mysql"/>
    </strategy>
</meta>
```

### Step 3: 定义路由

在 `assets/action/` 下创建文件 `hello`（无后缀）：

```
[default]

# @desc: Hello World
hello.say {
    hello.ProcHelloSay()
}

# @desc: 保存一条消息
hello.save {
    hello.ProcHelloSave()
}

# @desc: 查询消息列表
hello.list {
    hello.ProcHelloList()
}
```

> **URL 映射规则**：HTTP 请求路径中的 `/` 会被转为 `.` 作为 Action 名称。
> 例如 `GET /hello/say` → Action `hello.say`，`POST /hello/save` → Action `hello.save`。

### Step 4: 生成代码

```bash
goplay rebuild .
```

框架会自动：
- 在 `processor/hello/` 下为每个 Processor 生成模板文件
- 在 `library/metas/` 下生成 `DhelloTMessage` 数据结构体
- 在 `library/db/` 下生成 `DhelloTMessage(ctx)` 链式查询 API
- 生成 `init.go` 注册所有 Action

### Step 5: 编写 Processor

**processor/hello/ProcHelloSay.go** — 最简单的 Hello World：

```go
package hello

import (
    "github.com/leochen2038/play"
)

type ProcHelloSay struct {
    Input struct {
        Name string `key:"name" default:"World" note:"姓名"`
    }
    Output struct {
        Message string `key:"message" note:"问候语"`
    }
}

func (p *ProcHelloSay) Run(ctx *play.Context) (string, error) {
    p.Output.Message = "Hello, " + p.Input.Name + "!"
    return "RC_NORMAL", nil
}
```

**processor/hello/ProcHelloSave.go** — 写入数据库：

```go
package hello

import (
    "hello/library/db"
    "hello/library/metas"

    "github.com/leochen2038/play"
)

type ProcHelloSave struct {
    Input struct {
        Name    string `key:"name"     required:"true"  note:"姓名"`
        Content string `key:"content"  required:"true"  note:"内容" `
    }
    Output struct {
        Id int `key:"id" note:"新记录ID"`
    }
}

func (p *ProcHelloSave) Run(ctx *play.Context) (string, error) {
    msg := metas.NewDHelloTMessage().
        SetFname(p.Input.Name).
        SetFcontent(p.Input.Content)

    if err := db.DHelloTMessage(ctx).Save(msg); err != nil {
        return "", play.WrapErr(err).WrapTip("保存失败")
    }

    p.Output.Id = msg.Fid
    return "RC_NORMAL", nil
}
```

**processor/hello/ProcHelloList.go** — 查询数据库：

```go
package hello

import (
    "hello/library/db"

    "github.com/leochen2038/play"
)

type ProcHelloList struct {
    Input struct {
        Name string `key:"name" note:"按姓名筛选(可选)"`
    }
    Output struct {
        List []Message `key:"list" note:"消息列表"`
    }
}

type Message struct {
	Id      int    `key:"id" note:"ID"`
	Name    string `key:"name" note:"姓名"`
	Content string `key:"content" note:"内容"`
}

func (p *ProcHelloList) Run(ctx *play.Context) (string, error) {
    query := db.DHelloTMessage(ctx)

    // 如果传了 name 参数，按姓名筛选
    if p.Input.Name != "" {
        query = query.WhereFnameEqual(p.Input.Name)
    }

    list, err := query.OrderByFidDesc().Limit(0, 20).GetList()
    if err != nil && err != play.ErrQueryEmptyResult {
        return "", play.WrapErr(err).WrapTip("查询失败")
    }

    p.Output.List = make([]Message, len(list))
	for i, msg := range list {
        p.Output.List[i] = Message{
            Id:      msg.Fid,
            Name:    msg.Fname,
            Content: msg.Fcontent,
        }
    }

    return "RC_NORMAL", nil
}
```

### Step 6: 配置 & 启动

创建 `config.json`：

```json
{
    "mysql": "root:123456@tcp(127.0.0.1:3306)/d_hello?charset=utf8"
}
```

修改 `main.go`：

```go
package main

import (
    "fmt"
    "time"
    "hello/hook"

    "github.com/leochen2038/play/config"
    "github.com/leochen2038/play/database"
    "github.com/leochen2038/play/servers"
)

func main() {
    // 加载配置（每 30 秒检测文件变更自动热更新）
    parser, _ := config.NewFileJsonParser("config.json", 30*time.Second)
    config.InitConfig(parser)

    // 注册数据库连接
    mysqlDest, _ := config.String("mysql")
    database.SetDest("mysql", mysqlDest)

    // 创建 HTTP 服务并绑定 Action
    httpInstance := servers.NewHttpInstance("hello", ":8090", hook.NewServerHook(), nil, 5*time.Second)
    httpInstance.BindActionSpace("", "default")

    // 启动
    fmt.Println("Server running at :8090")
    if err := servers.Boot(httpInstance); err != nil {
        fmt.Println(err)
    }
}
```

运行项目：

```bash
go run .
```

### Step 7: 测试接口

```bash
# Hello World
curl "http://localhost:8090/hello/say?name=Play"
# => {"message":"Hello, Play!"}

# 保存消息
curl -X POST "http://localhost:8090/hello/save" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "name=Alice&content=Hello World"
# => {"id":1}

# JSON 方式提交
curl -X POST "http://localhost:8090/hello/save" \
  -H "Content-Type: application/json" \
  -d '{"name":"Bob","content":"Hi there"}'
# => {"id":2}

# 查询列表
curl "http://localhost:8090/hello/list"
# => {"list":[{"id":2,"name":"Bob",...},{"id":1,"name":"Alice",...}]}

# 按姓名筛选
curl "http://localhost:8090/hello/list?name=Alice"
# => {"list":[{"id":1,"name":"Alice",...}]}
```

### 工作流程总结

```
编写/修改代码 → goplay rebuild . → go run .

具体流程:
1. assets/meta/*.xml     定义数据模型 → 自动生成 library/db/ 和 library/metas/
2. assets/action/*       定义路由      → 自动生成 processor/ 模板和 init.go
3. processor/**/*.go     编写业务逻辑
4. hook/server_hook.go   自定义钩子 (鉴权、统一响应格式、日志等)
5. main.go               配置并启动服务
```

---

## 详细指南

### 项目结构

```
myproject/
├── main.go              # 入口文件
├── go.mod
├── init.go              # 自动生成，注册所有 Action 和 CronJob
├── config.json          # 配置文件
├── hook/
│   └── server_hook.go   # 生命周期钩子
├── assets/
│   ├── action/          # 路由定义文件 (DSL)
│   └── meta/            # 数据模型定义 (XML)
├── processor/           # 业务处理器
├── library/
│   ├── db/              # 自动生成: 链式查询 API
│   └── metas/           # 自动生成: 数据结构体
├── crontab/             # 定时任务
├── database/
├── template/
└── utils/
```

### Action 路由 DSL

在 `assets/action/` 目录下创建路由文件：

```
[default]

# @desc: 获取用户信息
user.info {
    session.ProcCheckLogin(
        RC_NORMAL => user.ProcGetUserInfo()
    )
}

# @desc: 用户登录
user.login {
    user.ProcUserLogin()
}

# 支持多级处理器链和条件分支
order.create {
    session.ProcCheckLogin(
        RC_NORMAL => order.ProcCheckStock(
            RC_NORMAL => order.ProcCreateOrder(),
            RC_OUT_OF_STOCK => order.ProcNotifyRestock()
        )
    )
}
```

**DSL 语法说明：**

| 语法 | 含义 |
|------|------|
| `[default]` | 声明 package 名，用于分组 |
| `user.info` | Action 名称，对应 HTTP 路径 `/user/info` |
| `{ ... }` | 定义处理器链 |
| `session.ProcCheckLogin(...)` | 指向 `processor/session/ProcCheckLogin.go` |
| `RC_NORMAL =>` | 当处理器返回 `"RC_NORMAL"` 时进入下一个处理器 |
| 多个 `RC_xxx =>` | 不同返回值走不同的处理器链路 |

**URL → Action 映射规则：**

HTTP 路径中的 `/` 转换为 `.`，路径末尾的 `.json` / `.html` 等后缀决定响应格式（默认 JSON）：

| HTTP 请求 | Action 名称 | 响应格式 |
|-----------|-------------|---------|
| `GET /hello/say` | `hello.say` | JSON |
| `POST /user/info` | `user.info` | JSON |
| `GET /page/index.html` | `page.index` | HTML |
| `GET /` | `index` | JSON |

### Processor 编写

每个 Processor 是一个包含 `Input`/`Output` 结构体的类型，实现 `Run` 方法：

```go
package user

import "github.com/leochen2038/play"

type ProcGetUserInfo struct {
    Input struct {
        Uid int `key:"uid" note:"用户ID" required:"true"`
    }
    Output struct {
        Name   string `key:"name" note:"用户名"`
        Avatar string `key:"avatar" note:"头像"`
    }
}

func (p *ProcGetUserInfo) Run(ctx *play.Context) (string, error) {
    // Input 已自动从请求参数绑定 (支持 URL Query / Form / JSON Body)
    uid := p.Input.Uid

    // 业务逻辑...
    p.Output.Name = "张三"
    p.Output.Avatar = "https://example.com/avatar.png"

    // Output 字段会自动序列化到响应 JSON 中
    return "RC_NORMAL", nil
}
```

**Input 结构体标签：**

| 标签 | 说明 | 示例 |
|------|------|------|
| `key` | 请求参数名 (支持逗号分隔多别名) | `key:"uid,user_id"` |
| `note` | 参数描述 (用于文档生成) | `note:"用户ID"` |
| `required` | 是否必填 | `required:"true"` |
| `default` | 默认值 | `default:"1"` |
| `regex` | 正则校验 | `regex:"^[0-9]+$"` |
| `layout` | 时间格式 (用于 `time.Time` 类型) | `layout:"2006-01-02"` |

### 生命周期钩子

```go
package hook

import (
    "time"
    "github.com/leochen2038/play"
    "github.com/leochen2038/play/logger"
)

type ServerHook struct{}

func NewServerHook() play.IServerHook {
    return &ServerHook{}
}

func (h ServerHook) OnBoot(server play.IServer) {
    // 服务启动后回调，可在此自定义 Action 超时
    server.UpdateActionTimeout("", "order.create", 3*time.Second)
}

func (h ServerHook) OnShutdown(server play.IServer) {
    // 服务关闭前回调
}

func (h ServerHook) OnConnect(sess *play.Session, err error) {
    // 连接建立时回调
}

func (h ServerHook) OnClose(sess *play.Session, err error) {
    // 连接关闭时回调
}

func (h ServerHook) OnRequest(ctx *play.Context) error {
    // 请求进入时回调（在 Action 执行前）
    // 可用于鉴权、参数预处理等
    return nil
}

func (h ServerHook) OnResponse(ctx *play.Context) {
    // Action 执行后、响应发送前的回调
    // 通常在此统一包装响应格式
    if ctx.Err() != nil {
        rc, msg := 0x100, ctx.Err().Error()
        if errCode, ok := ctx.Err().(play.Err); ok {
            if errCode.Code() > 0 { rc = errCode.Code() }
            if errCode.Tip() != "" { msg = errCode.Tip() }
        }
        ctx.Response.Output.Set("rc", rc)
        ctx.Response.Output.Set("msg", msg)
    } else {
        ctx.Response.Output.Set("rc", 0)
    }
    ctx.Response.Output.Set("tm", time.Now().Unix())
}

func (h ServerHook) OnFinish(ctx *play.Context) {
    // 请求完全结束后的回调（异步执行）
    logger.Access(ctx)
}
```

### 代码生成

每次修改了 Action 文件、Meta XML 或 Processor 后，执行：

```bash
goplay rebuild .
```

框架会自动：
- 扫描 `assets/action/` 下的路由定义
- 为不存在的 Processor 生成模板文件
- 扫描 `assets/meta/` 下的 XML 定义，生成数据结构体和查询 API
- 生成 `init.go` 注册所有 Action 和 CronJob

## 数据建模 (Meta)

在 `assets/meta/` 下用 XML 定义数据模型：

```xml
<?xml version="1.0" encoding="UTF-8"?>
<meta module="mydb" name="user_info">
    <key name="_id" alias="id" type="auto" note="自增ID" />

    <fields>
        <field name="Fuid" type="int" default="0" note="用户ID" />
        <field name="Fname" type="string" default="" note="用户名" />
        <field name="Fstatus" type="int8" default="0" note="状态" />
        <field name="Fctime" type="ctime" default="0" note="创建时间" />
        <field name="Fmtime" type="mtime" default="0" note="修改时间" />
    </fields>

    <strategy>
        <storage type="mysql" drive="mysql" database="d_user" table="t_user_info" router="mysql"/>
    </strategy>
</meta>
```

执行 `goplay rebuild` 后自动生成：

- `library/metas/` — 数据结构体 (带 setter 方法)
- `library/db/` — 链式查询 API

生成的查询 API 使用示例：

```go
// 查询单条
user, err := db.MydbUserInfo(ctx).WhereFuidEqual(123).GetOne()

// 查询列表
list, err := db.MydbUserInfo(ctx).WhereFstatusEqual(1).GetList()

// 链式条件
list, err := db.MydbUserInfo(ctx).
    WhereFstatusEqual(1).
    WhereFctimeGreater(timestamp).
    GetList()
```

## 多协议服务

```go
// HTTP 服务
httpInstance := servers.NewHttpInstance("http", ":8080", hook, nil, timeout)

// H2C (HTTP/2 无 TLS) 服务
h2cInstance := servers.NewH2cInstance("h2c", ":8081", hook, nil, timeout)

// TCP 服务
tcpInstance := servers.NewTcpInstance("tcp", ":9000", hook, nil, timeout)

// WebSocket 服务 (挂载到 HTTP 实例)
wsInstance := servers.NewWsInstance("ws", "", hook, nil, timeout)
wsInstance.BindActionSpace("", "ws_actions")
httpInstance.SetWSInstance(wsInstance)

// SSE 服务 (挂载到 HTTP 实例)
sseInstance := servers.NewSSEInstance("sse", "", hook, nil, timeout)
sseInstance.BindActionSpace("", "sse_actions")
httpInstance.SetSSEInstance(sseInstance)

// TLS 支持
cert, _ := tls.LoadX509KeyPair("cert.pem", "key.pem")
httpInstance.WithCertificate(cert)

// 同时启动多个服务
servers.Boot(httpInstance, h2cInstance, tcpInstance)
```

## MCP 服务 (Model Context Protocol)

框架内置了 MCP 服务支持，已有的 Action 自动映射为 MCP Tool，可被 AI 客户端（如 Claude Desktop、Claude Code）直接调用。基于 [Go 官方 MCP SDK](https://github.com/modelcontextprotocol/go-sdk)。

### 核心概念映射

| Play 框架 | MCP 协议 | 说明 |
|---|---|---|
| Action | Tool | `BindActionSpace` 时自动注册 |
| Processor.Input | Tool InputSchema | Input 字段自动转为 JSON Schema |
| Processor.Output | Tool 返回内容 | Output 序列化为 JSON 返回 |
| Action metaData `@desc` | Tool.Description | 复用已有描述 |

### stdio 模式 (用于 Claude Desktop / Claude Code)

最简单的使用方式，Action 自动变成 MCP Tool：

```go
mcpInst := servers.NewMCPInstance("myMCP", "", hook, play.MCP_TRANSPORT_STDIO, 5*time.Second)
mcpInst.BindActionSpace("", "default")
servers.Boot(mcpInst)
```

编译后配置到 Claude Desktop 的 `claude_desktop_config.json`：

```json
{
    "mcpServers": {
        "myMCP": {
            "command": "/path/to/your/binary"
        }
    }
}
```

AI 客户端连接后，能看到所有 Action 对应的 Tool 并直接调用。例如 Action `hello.say` (Input: `name string`) 会自动生成一个名为 `hello.say` 的 Tool，AI 调用时传入 `{"name": "Play"}`，框架执行 Processor 链后返回结果。

### 独立 HTTP 服务 (Streamable HTTP)

MCP 实例可以独立启动 HTTP 服务：

```go
mcpInst := servers.NewMCPInstance("myMCP", ":8091", hook, play.MCP_TRANSPORT_STREAMABLE_HTTP, 5*time.Second)
mcpInst.BindActionSpace("", "default")
servers.Boot(mcpInst)
```

### 挂载到已有 HTTP 服务

也可以将 MCP 挂载到现有 HTTP 实例的 `/mcp` 路径：

```go
httpInst := servers.NewHttpInstance("api", ":8090", hook, nil, 5*time.Second)
httpInst.BindActionSpace("", "default")

mcpInst := servers.NewMCPInstance("mcp", "", hook, play.MCP_TRANSPORT_STREAMABLE_HTTP, 5*time.Second)
mcpInst.BindActionSpace("", "default")
httpInst.SetMCPInstance(mcpInst)

servers.Boot(httpInst)
```

### 使用 MCP 原生功能 (Resource / Prompt)

通过 `MCPServer()` 获取底层 MCP Server，可直接使用 MCP SDK 原生 API 注册 Resource 和 Prompt：

```go
mcpInst := servers.NewMCPInstance("myMCP", "", hook, play.MCP_TRANSPORT_STDIO, 5*time.Second)
mcpInst.BindActionSpace("", "default")

mcpSrv := mcpInst.MCPServer()
mcpSrv.AddResource(...)
mcpSrv.AddPrompt(...)

servers.Boot(mcpInst)
```

### 数据流

MCP 复用框架标准的 `Session.Write()` 流程：

```
MCP Tool Call → makeToolHandler
  → play.DoRequest (执行 Processor 链 + Hook)
    → sess.Write(&response)
      → mcpPacker.Pack()    → Output 序列化为 JSON
      → Transport()         → 数据暂存到 conn.Mcp.Data
  → 从 conn.Mcp.Data 读取结果
  → 包装为 MCP CallToolResult 返回给 AI 客户端
```

## 配置管理

```go
// 从 JSON 文件初始化（支持热更新）
parser, _ := config.NewFileJsonParser("config.json", 30*time.Second)
config.InitConfig(parser)

// 读取配置
port, _ := config.String("listen.http")    // 支持点号分隔的层级 key
debug, _ := config.Bool("debug")
maxConn, _ := config.Int("pool.maxConn")
rate, _ := config.Float64("rate.limit")
```

配置文件格式 (JSON)：

```json
{
    "listen": {
        "http": ":8090",
        "h2c": ":8091"
    },
    "mysql": "root:password@tcp(127.0.0.1:3306)/mydb?charset=utf8",
    "mongodb": "root:password@tcp(127.0.0.1:27017)/mydb",
    "debug": true
}
```

## 日志

```go
import "github.com/leochen2038/play/logger"

// 设置日志级别
logger.SetLevel(logger.LEVEL_INFO)  // DEBUG=3, INFO=2, WARN=1, ERROR=0

// 设置日志保留天数
logger.SetLogKeepDays(7)

// 在 Processor 中使用
func (p *ProcExample) Run(ctx *play.Context) (string, error) {
    logger.Info(ctx, "处理开始", "uid", uid)
    logger.Debug(ctx, "调试信息", "key", value)
    logger.Warn(ctx, "警告信息")
    logger.Error(ctx, err, "key", value)

    // Access 日志（通常在 OnFinish 钩子中调用）
    logger.Access(ctx)

    return "RC_NORMAL", nil
}
```

日志自动包含：时间、级别、Action 名称、耗时、TraceId、调用文件位置。

## 定时任务

1. 定义 CronJob 结构体（放在 `crontab/` 目录）：

```go
package crontab

type CleanExpiredData struct{}

func (c *CleanExpiredData) Run() {
    // 定时执行的逻辑
}
```

2. 配置调度（JSON 文件或代码）：

```go
// 从文件加载 cron 配置
play.CronStartWithFile("cron.json", 30)

// 或代码动态更新
play.CronUpdate([]play.CronConfig{
    {Name: "crontab.CleanExpiredData", Spec: "0 3 * * *"},
})
```

cron.json 格式：

```json
{
    "crontab.CleanExpiredData": "0 3 * * *"
}
```

## 错误处理

框架提供了 `play.Err` 类型，支持错误码、提示信息和调用栈追踪：

```go
func (p *ProcExample) Run(ctx *play.Context) (string, error) {
    data, err := queryData()
    if err != nil {
        // 包装错误，自动记录调用栈
        return "", play.WrapErr(err, "uid", uid).
            WrapCode(1001).           // 业务错误码
            WrapTip("查询数据失败")     // 用户可见的提示信息
    }
    return "RC_NORMAL", nil
}
```

## 服务间调用 (Agent)

```go
import "github.com/leochen2038/play/agents"

// 设置服务路由
agents.H2cWithForm.SetRouter("user-service", "http://10.0.0.1:8081")

// 发起调用
req := UserInfoReq{Uid: "123"}
sendData, _ := agents.H2cWithForm.Marshal(ctx, "user-service", "user.info", req)
recvData, _ := agents.H2cWithForm.Request(ctx, "user-service", "user.info", sendData)

var resp UserInfoResp
agents.H2cWithForm.Unmarshal(ctx, "user-service", "user.info", recvData, &resp)
```

## API 文档生成

```bash
goplay gendoc ./myproject
```

自动生成 Markdown 格式 API 文档，包含每个 Action 的：
- 请求参数表（名称、类型、是否必填、描述、默认值）
- 响应参数表（名称、类型、描述）
- 响应示例 JSON

## 优雅重启

向进程发送 `SIGUSR2` 信号触发优雅重启：

```bash
kill -USR2 <pid>
```

框架会：
1. 启动新进程并传递 listener fd
2. 新进程开始接受新连接
3. 旧进程停止接受新连接，等待已有请求处理完成后退出

## CLI 命令

```bash
goplay init <project-path>     # 初始化新项目
goplay rebuild <project-path>  # 重新生成注册代码
goplay gendoc <project-path>   # 生成 API 文档
```

## 架构概览

```
play/
├── action.go           # Action/Processor 核心 (注册、反射绑定、处理器链)
├── context.go          # 请求上下文 (Context, TraceId 生成)
├── server.go           # 服务器接口定义 (IServer, IPacker, IServerHook)
├── session.go          # 会话管理
├── input.go            # 请求输入参数绑定
├── output.go           # 响应输出
├── error.go            # 错误类型 (Err, 调用栈追踪)
├── query.go            # 数据库查询条件
├── crontab.go          # 定时任务
├── conv.go             # 类型转换工具
├── agent.go            # 服务间调用接口
├── socket_pool.go      # 带权重的 TCP 连接池
├── servers/            # 多协议服务器实现
│   ├── boot_server.go  # 服务启动、优雅重启、信号处理
│   ├── http_instance.go
│   ├── h2c_instance.go
│   ├── tcp_instance.go
│   ├── ws_instance.go
│   ├── sse_instance.go
│   ├── quic_server.go
│   └── mcp_instance.go # MCP 服务 (Action 自动映射为 Tool)
├── agents/             # 服务间调用实现
├── codec/
│   ├── binders/        # 请求参数绑定器 (JSON, URL, Map, Protobuf, Bytes)
│   ├── renders/        # 响应渲染器 (JSON, Protobuf)
│   └── protos/         # 自定义协议 (pproto, JSON 编解码)
├── packers/            # 协议打包/解包 (HTTP, JSON, Protobuf, Play, Telnet)
├── config/             # 配置管理
├── database/           # 数据库路由 (MySQL, MongoDB)
├── logger/             # 日志系统
├── page/               # 页面模板引擎 (XML 协议)
├── gentools/           # SDK/文档生成工具
├── goplay/             # CLI 工具
│   ├── initProject/    # 项目初始化
│   └── reconst/        # 代码重建 (Action 注册, Meta 生成)
└── client/             # 客户端连接池
```

## License

Copyright The Play Framework Authors.
