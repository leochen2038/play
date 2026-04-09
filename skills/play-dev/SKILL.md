---
name: play-dev
description: >
  Play 框架 (github.com/leochen2038/play) Go 微服务开发助手。当用户在使用 Play 框架的项目中工作时使用此技能——包括创建 Action 路由、编写 Processor、定义 Meta XML 数据模型、配置 Hook、设置 MCP 服务、操作数据库等。
  只要项目的 go.mod 中包含 github.com/leochen2038/play 依赖，就应该使用这个技能。即使用户没有明确提到"Play 框架"，只要他们在做路由定义、Processor 编写、Meta XML、goplay 命令等相关工作，都应该触发。
---

# Play Framework 开发助手

## 框架识别

检查项目 `go.mod` 是否包含 `github.com/leochen2038/play`。如果包含，当前项目就是 Play 框架项目，后续所有开发操作都应遵循本技能描述的约定。

## 核心开发工作流

Play 框架使用代码生成，开发流程是：

```
1. 编辑 assets/action/* (路由 DSL) 或 assets/meta/*.xml (数据模型)
2. 运行 goplay reconst ./
3. 编写/修改 processor/**/*.go (业务逻辑)
4. go run .
```

**关键规则：每次修改了 `assets/action/` 或 `assets/meta/` 下的文件后，必须运行 `goplay reconst ./` 重新生成代码。** 这会生成 `init.go`（Action 注册）、Processor 模板文件、数据库查询 API 和数据结构体。如果用户忘了这步，要主动提醒。

## Action 路由 DSL

路由定义在 `assets/action/` 目录下的无后缀文件中：

```
[package_name]

# @desc: 接口描述
action.name {
    package.ProcName()
}

# 带条件分支的处理器链
order.create {
    session.ProcCheckLogin(
        RC_NORMAL => order.ProcCheckStock(
            RC_NORMAL => order.ProcCreateOrder()
            RC_OUT_OF_STOCK => order.ProcNotifyRestock()
        )
    )
}
```

- `[package_name]` — 包声明，分组用，对应 `goplay reconst` 时的 actionPackage
- `# @desc:` — 接口描述，同时用于文档生成和 MCP Tool 描述
- `action.name` — Action 名称，HTTP 路径 `/action/name` 自动映射（`/` 转 `.`）
- `package.ProcName()` — 指向 `processor/package/ProcName.go`
- `RC_NORMAL =>` — 当 Processor 返回 `"RC_NORMAL"` 时走下一个处理器

## Processor 编写

每个 Processor 是一个 Go 结构体，放在 `processor/<package>/` 目录下：

```go
package user

import "github.com/leochen2038/play"

type ProcGetUserInfo struct {
    Input struct {
        Uid int `key:"uid" note:"用户ID" required:"true"`
    }
    Output struct {
        Name string `key:"name" note:"用户名"`
    }
}

func (p *ProcGetUserInfo) Run(ctx *play.Context) (string, error) {
    // Input 字段已自动绑定（URL Query / Form / JSON Body）
    // 编写业务逻辑，填充 Output 字段
    p.Output.Name = "张三"
    return "RC_NORMAL", nil  // 返回路由码 + error
}
```

**Input 标签：**
- `key:"name,alias"` — 参数名（逗号分隔多别名）
- `note:"描述"` — 参数描述
- `required:"true"` — 必填校验
- `default:"值"` — 默认值
- `regex:"^[0-9]+$"` — 正则校验

**返回值：**
- 第一个返回值是路由码字符串，对应 DSL 中的 `RC_xxx =>`
- 第二个返回值是 error，非 nil 时中断处理器链

## Meta XML 数据模型

在 `assets/meta/` 下定义 XML，`goplay reconst` 会生成：
- `library/metas/` — 数据结构体（带 Setter）
- `library/db/` — 链式查询 API

```xml
<?xml version="1.0" encoding="UTF-8"?>
<meta module="d_mydb" name="t_user">
    <key name="Fid" alias="id" type="auto" note="自增ID" />
    <fields>
        <field name="Fname"  alias="name"  type="string" default="" note="姓名" />
        <field name="Fage"   alias="age"   type="int"    default="0" note="年龄" />
        <field name="Fctime" alias="ctime" type="ctime"  default="0" note="创建时间" />
        <field name="Fmtime" alias="mtime" type="mtime"  default="0" note="修改时间" />
    </fields>
    <strategy>
        <storage type="mysql" drive="mysql" database="d_mydb" table="t_user" router="mysql"/>
    </strategy>
</meta>
```

生成后的查询 API 用法：
```go
// 查询
user, err := db.DmydbTUser(ctx).WhereFidEqual(1).GetOne()
list, err := db.DmydbTUser(ctx).WhereFnameEqual("张三").OrderByFidDesc().Limit(0, 20).GetList()
count, err := db.DmydbTUser(ctx).WhereFageGreater(18).Count()

// 写入
msg := metas.NewDmydbTUser().SetFname("张三").SetFage(25)
err := db.DmydbTUser(ctx).Save(msg)

// 更新
err := db.DmydbTUser(ctx).WhereFidEqual(1).Update(msg)

// 删除
err := db.DmydbTUser(ctx).WhereFidEqual(1).Delete()
```

## Hook 生命周期

实现 `play.IServerHook` 接口的七个方法：

| 方法 | 时机 | 典型用途 |
|------|------|---------|
| `OnBoot` | 服务启动后 | 自定义超时、初始化资源 |
| `OnShutdown` | 服务关闭前 | 释放资源 |
| `OnConnect` | 连接建立 | — |
| `OnClose` | 连接关闭 | — |
| `OnRequest` | 请求进入，Action 执行前 | 鉴权、参数预处理 |
| `OnResponse` | Action 执行后，发送前 | 统一响应格式包装（rc/msg/tm） |
| `OnFinish` | 请求完全结束（异步） | 日志记录 |

## 多协议服务创建

```go
servers.NewHttpInstance(name, addr, hook, packer, timeout)       // HTTP
servers.NewTcpInstance(name, addr, hook, packer, timeout)        // TCP
servers.NewH2cInstance(name, addr, hook, packer, timeout)        // H2C
servers.NewMCPInstance(name, addr, hook, transport, timeout)     // MCP
// WS/SSE 挂载到 HTTP：httpInst.SetWSInstance(wsInst) / httpInst.SetSSEInstance(sseInst)
```

## 错误处理

```go
return "", play.WrapErr(err, "uid", uid).WrapCode(1001).WrapTip("操作失败")
```

## 常见错误提醒

1. **修改 DSL/XML 后忘记 reconst** — 新 Action 不生效、找不到 Processor
2. **Processor 命名不匹配** — DSL 中的 `package.ProcName` 必须和 `processor/package/ProcName.go` 中的类型名完全一致
3. **缺少 `play.ErrQueryEmptyResult` 判断** — 查询为空时 `GetOne`/`GetList` 返回此 error，不是真正的错误
4. **BindActionSpace 的 package 名** — 对应 DSL 文件中 `[package_name]` 声明

## 详细参考

需要完整代码示例、Meta XML 字段类型表、查询 API 方法列表、main.go 模板等详细信息时，读取 `references/patterns.md`。
