# Play Framework 详细参考

## 目录

1. [完整 Processor 示例](#完整-processor-示例)
2. [Input 标签完整参考](#input-标签完整参考)
3. [Meta XML 字段类型表](#meta-xml-字段类型表)
4. [生成的查询 API 方法](#生成的查询-api-方法)
5. [main.go 完整模板](#maingo-完整模板)
6. [Hook 完整实现](#hook-完整实现)
7. [错误处理模式](#错误处理模式)
8. [Action DSL 完整语法](#action-dsl-完整语法)

---

## 完整 Processor 示例

### 查询列表（带分页和筛选）

```go
package order

import (
    "myproject/library/db"
    "github.com/leochen2038/play"
)

type ProcOrderList struct {
    Input struct {
        Uid    int    `key:"uid"    note:"用户ID"   required:"true"`
        Status int    `key:"status" note:"订单状态" default:"-1"`
        Page   int    `key:"page"   note:"页码"     default:"0"`
        Size   int    `key:"size"   note:"每页条数" default:"20"`
    }
    Output struct {
        List  interface{} `key:"list"  note:"订单列表"`
        Total int         `key:"total" note:"总数"`
    }
}

func (p *ProcOrderList) Run(ctx *play.Context) (string, error) {
    query := db.DmydbTOrder(ctx).WhereFuidEqual(p.Input.Uid)

    if p.Input.Status >= 0 {
        query = query.WhereFstatusEqual(p.Input.Status)
    }

    total, err := query.Count()
    if err != nil && err != play.ErrQueryEmptyResult {
        return "", play.WrapErr(err).WrapTip("查询总数失败")
    }

    list, err := query.OrderByFctimeDesc().Limit(p.Input.Page*p.Input.Size, p.Input.Size).GetList()
    if err != nil && err != play.ErrQueryEmptyResult {
        return "", play.WrapErr(err).WrapTip("查询列表失败")
    }

    p.Output.List = list
    p.Output.Total = total
    return "RC_NORMAL", nil
}
```

### 新增记录

```go
package order

import (
    "myproject/library/db"
    "myproject/library/metas"
    "github.com/leochen2038/play"
)

type ProcOrderCreate struct {
    Input struct {
        Uid       int     `key:"uid"        note:"用户ID"  required:"true"`
        ProductId int     `key:"product_id" note:"商品ID"  required:"true"`
        Amount    float64 `key:"amount"     note:"金额"    required:"true"`
    }
    Output struct {
        OrderId int `key:"order_id" note:"订单ID"`
    }
}

func (p *ProcOrderCreate) Run(ctx *play.Context) (string, error) {
    order := metas.NewDmydbTOrder().
        SetFuid(p.Input.Uid).
        SetFproductId(p.Input.ProductId).
        SetFamount(p.Input.Amount).
        SetFstatus(1)

    if err := db.DmydbTOrder(ctx).Save(order); err != nil {
        return "", play.WrapErr(err).WrapCode(2001).WrapTip("创建订单失败")
    }

    p.Output.OrderId = order.Fid
    return "RC_NORMAL", nil
}
```

### 更新记录

```go
func (p *ProcOrderUpdate) Run(ctx *play.Context) (string, error) {
    order := metas.NewDmydbTOrder().SetFstatus(p.Input.Status)

    err := db.DmydbTOrder(ctx).WhereFidEqual(p.Input.OrderId).WhereFuidEqual(p.Input.Uid).Update(order)
    if err != nil {
        return "", play.WrapErr(err).WrapTip("更新失败")
    }
    return "RC_NORMAL", nil
}
```

---

## Input 标签完整参考

| 标签 | 说明 | 示例 | 适用范围 |
|------|------|------|---------|
| `key` | 参数名（逗号分隔多别名） | `key:"uid,user_id"` | Input |
| `note` | 参数/字段描述（用于文档生成） | `note:"用户ID"` | Input & Output |
| `required` | 是否必填 | `required:"true"` | Input |
| `default` | 默认值 | `default:"0"` | Input |
| `regex` | 正则校验 | `regex:"^[0-9]+$"` | Input |
| `layout` | 时间格式（用于 time.Time） | `layout:"2006-01-02"` | Input |
| `label` | 等同于 note | `label:"用户ID"` | Input & Output |

**支持的 Input 类型：** string, int, int8, int32, int64, uint, uint8, uint32, uint64, float32, float64, bool, time.Time, []string, []int, []int64, []float64, 嵌套 struct, []struct

---

## Meta XML 字段类型表

| XML type | Go 类型 | 说明 |
|----------|---------|------|
| `string` | `string` | 字符串 |
| `int` | `int` | 整数 |
| `int64` | `int64` | 长整数 |
| `float` | `float64` | 浮点数 |
| `auto` | `int` | 自增主键（MySQL） |
| `ctime` | `int64` | 创建时间戳（Save 时自动填充） |
| `mtime` | `int64` | 修改时间戳（Save/Update 时自动填充） |
| `dtime` | `int64` | 删除时间戳 |
| `array` | `[]interface{}` | 通用数组 |
| `array:int` | `[]int` | 整数数组 |
| `array:string` | `[]string` | 字符串数组 |
| `array:float` | `[]float64` | 浮点数组 |
| `map` | `map[string]interface{}` | 通用 Map |
| `map:int` | `map[string]int` | 整数 Map |
| `map:string` | `map[string]string` | 字符串 Map |

**存储策略：**
```xml
<!-- MySQL -->
<storage type="mysql" drive="mysql" database="d_mydb" table="t_user" router="mysql"/>
<!-- MongoDB -->
<storage type="mongodb" drive="mongodb" database="d_mydb" table="t_user" router="mongodb"/>
```

`router` 对应 `database.SetDest("mysql", dsn)` 中注册的连接名。

---

## 生成的查询 API 方法

假设 Meta 定义了字段 `Fname`，生成的方法包括：

**条件查询（Where）：**
- `WhereFnameEqual(val)` / `OrWhereFnameEqual(val)`
- `WhereFnameLess(val)` / `WhereFnameGreater(val)`
- `WhereFnameLessOrEqual(val)` / `WhereFnameGreaterOrEqual(val)`
- `WhereFnameLike(val)` / `WhereFnameNotLike(val)`
- `WhereFnameBetween(min, max)`
- `WhereFnameIn(vals...)` / `WhereFnameNotIn(vals...)`

**排序：**
- `OrderByFnameAsc()` / `OrderByFnameDesc()`

**终端操作：**
- `GetOne()` — 查询单条
- `GetList()` — 查询列表
- `Count()` — 计数
- `Save(meta)` — 插入
- `Update(meta)` — 更新
- `Delete()` — 删除
- `Limit(offset, size)` — 分页
- `GroupBy(fields...)` — 分组

---

## main.go 完整模板

```go
package main

import (
    "fmt"
    "time"
    "myproject/hook"

    "github.com/leochen2038/play/config"
    "github.com/leochen2038/play/database"
    "github.com/leochen2038/play/servers"
)

func main() {
    // 配置（每 30 秒检测文件变更自动热更新）
    parser, _ := config.NewFileJsonParser("config.json", 30*time.Second)
    config.InitConfig(parser)

    // 数据库连接
    mysqlDest, _ := config.String("mysql")
    database.SetDest("mysql", mysqlDest)

    // HTTP 服务
    httpInst := servers.NewHttpInstance("api", ":8090", hook.NewServerHook(), nil, 5*time.Second)
    httpInst.BindActionSpace("", "default")

    // 可选：WebSocket
    // wsInst := servers.NewWsInstance("ws", "", hook.NewServerHook(), nil, 5*time.Second)
    // wsInst.BindActionSpace("", "ws_actions")
    // httpInst.SetWSInstance(wsInst)

    // 可选：SSE
    // sseInst := servers.NewSSEInstance("sse", "", hook.NewServerHook(), nil, 5*time.Second)
    // sseInst.BindActionSpace("", "sse_actions")
    // httpInst.SetSSEInstance(sseInst)

    // 启动
    fmt.Println("Server running at :8090")
    if err := servers.Boot(httpInst); err != nil {
        fmt.Println(err)
    }
}
```

**config.json：**
```json
{
    "mysql": "root:password@tcp(127.0.0.1:3306)/d_mydb?charset=utf8mb4",
    "mongodb": "mongodb://root:password@127.0.0.1:27017/d_mydb"
}
```

---

## Hook 完整实现

```go
package hook

import (
    "time"
    "github.com/leochen2038/play"
    "github.com/leochen2038/play/logger"
)

type ServerHook struct{}

func NewServerHook() play.IServerHook { return &ServerHook{} }

func (h ServerHook) OnBoot(server play.IServer) {
    // 自定义超时
    server.UpdateActionTimeout("", "upload.file", 30*time.Second)
}

func (h ServerHook) OnShutdown(server play.IServer) {}

func (h ServerHook) OnConnect(sess *play.Session, err error) {}

func (h ServerHook) OnClose(sess *play.Session, err error) {}

func (h ServerHook) OnRequest(ctx *play.Context) error {
    // 鉴权示例：跳过白名单 Action
    // if ctx.ActionRequest.Name != "user.login" {
    //     token := ctx.Session.Conn.Http.Request.Header.Get("Authorization")
    //     if token == "" {
    //         return play.WrapErr(nil).WrapCode(401).WrapTip("未登录")
    //     }
    // }
    return nil
}

func (h ServerHook) OnResponse(ctx *play.Context) {
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
    logger.Access(ctx)
}
```

---

### 扩展 Resource / Prompt

```go
mcpInst.MCPServer().AddResource(...)
mcpInst.MCPServer().AddPrompt(...)
```

---

## 错误处理模式

```go
// 基础错误
return "", err

// 带错误码和提示
return "", play.WrapErr(err).WrapCode(1001).WrapTip("操作失败")

// 带上下文信息（用于日志）
return "", play.WrapErr(err, "uid", uid, "order_id", orderId).WrapCode(1001).WrapTip("操作失败")

// 查询空结果不算错误
list, err := db.MyTable(ctx).GetList()
if err != nil && err != play.ErrQueryEmptyResult {
    return "", play.WrapErr(err).WrapTip("查询失败")
}
```

---

## Action DSL 完整语法

```
[package_name]                     # 包声明

# @desc: 接口描述                  # 元数据注解
action.name {                      # Action 定义
    pkg.ProcA()                    # 单个处理器
}

action.chain {                     # 处理器链
    pkg.ProcA(
        RC_NORMAL => pkg.ProcB()   # A 返回 RC_NORMAL 时执行 B
    )
}

action.branch {                    # 条件分支
    pkg.ProcA(
        RC_NORMAL => pkg.ProcB()
        RC_ERROR => pkg.ProcC()    # A 返回 RC_ERROR 时执行 C
    )
}

action.deep {                      # 多级嵌套
    pkg.ProcA(
        RC_NORMAL => pkg.ProcB(
            RC_NORMAL => pkg.ProcC()
            RC_RETRY => pkg.ProcD()
        )
    )
}
```

**URL 映射规则：** HTTP `GET /hello/world` → Action `hello.world`（路径 `/` 转 `.`）
