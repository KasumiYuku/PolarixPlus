# PolarixPlus

> QQ 官方机器人轻量开发框架 · Go
> 面向 QQ 官方开放平台的机器人框架：插件化命令系统、WebSocket / Webhook 双接入、内置管理面板与图床聚合，运行时零外部依赖。

> [!NOTE]
> 本项目是 [YearnstudioYangyi/Polarix](https://github.com/YearnstudioYangyi/Polarix/tree/dev) 的个人 fork 改版，在保留上游核心框架的基础上定制与扩展

## 目录

- [快速开始](#快速开始)
- [配置](#配置)
- [接入方式](#接入方式)
- [管理台](#管理台)
- [编写插件](#编写插件)
- [前端开发](#前端开发)
- [许可证](#许可证)

## 快速开始

```bash
cp config.example.json config.json   # 填入 AppID / AppSecret / admin_password
go build -o polarix .
./polarix
```

启动后访问 `http://127.0.0.1:端口/admin` 打开管理台。

> 修改 `web/` 前端后需重新构建：`cd web && pnpm build`（产物输出到 `lib/admin/dist`）。只改 Go 代码无需前端。

## 配置

<details>
<summary>config.json 完整字段与可订阅事件（点击展开）</summary>

```json
{
  "port": 8080,
  "appid": "你的机器人AppID",
  "secret": "你的机器人AppSecret",
  "proxy": "https://api.sgroup.qq.com",
  "protocol": "webhook",
  "intents": ["GROUP_AT_MESSAGE_CREATE", "INTERACTION_CREATE"],
  "database": "bot.db",
  "admin_password": "设置一个管理面板密码",
  "global_markdown": false,
  "markdown_verify_image": false,
  "retry_when": [11253, 630006],
  "upload_threshold": 3145728,
  "log_level": "info",
  "plugin_settings": {},
  "plugin_access": {}
}
```

| 字段 | 说明 |
|---|---|
| `port` | 服务端口 |
| `appid` / `secret` | 开放平台机器人凭证 |
| `proxy` | 反代地址。服务器 IP 动态时，在固定 IP 设备上反代 QQ API，填写其地址绕过 IP 白名单 |
| `protocol` | `webhook`（平台推送回调）或 `websocket`（长连接网关） |
| `intents` | websocket 模式下订阅的事件名列表 |
| `database` | SQLite 数据库路径 |
| `admin_password` | 管理台密码；留空则仅本机可访问 |
| `global_markdown` | 全局 Markdown：所有文字按 Markdown 渲染，图片/按钮内联 |
| `markdown_verify_image` | Markdown 图片转存失败时中断发送 |
| `retry_when` | 遇到这些 QQ 业务错误码时自动重试 |
| `upload_threshold` | 超过该字节数的文件走分片上传（默认 3MB） |
| `log_level` | 控制台最低日志级别：`debug` / `info` / `warn` / `error`，可在线热更 |
| `plugin_settings` | 插件配置，面板修改后即时生效 |
| `plugin_access` | 插件访问控制 |

**可订阅事件（intents）**

| # | 事件 | 说明 |
|---|---|---|
| 1 | `GROUP_AT_MESSAGE_CREATE` | 群里 @ 机器人消息 |
| 2 | `GROUP_MESSAGE_CREATE` | 群内全部消息 |
| 3 | `C2C_MESSAGE_CREATE` | 私聊消息 |
| 4 | `INTERACTION_CREATE` | 互动事件（按钮回调等） |
| 5 | `GROUP_MEMBER_ADD` | 新成员入群 |
| 6 | `GROUP_MEMBER_REMOVE` | 成员退群 |
| 7 | `MESSAGE_AUDIT_PASS` | 消息审核通过 |
| 8 | `MESSAGE_AUDIT_REJECT` | 消息审核驳回 |

</details>

## 接入方式

- **Webhook（默认）** — 平台配置回调地址为 `你的地址:端口/webhook`，按需勾选事件
- **WebSocket** — `"protocol": "websocket"`，框架自动连接网关长连接，无需公网回调地址，管理台与主动推送仍可用

## 管理台

单页应用，支持浅色 / 深色 / 跟随系统。

| 页面 | 能力 |
|---|---|
| 概览 | 运行时长 / 内存 / goroutine / 消息计数 / 网关状态 / 最近日志 |
| 日志 | 级别与来源筛选、关键字搜索、SSE 实时推送、导出 |
| 插件 | 目录卡片、配置编辑（保存即热更）、访问控制 |
| 图床 | Provider 启停 / 优先级 / 配置，白名单直通 |
| 定时任务 | 任务列表、暂停 / 恢复 |
| 设置 | 核心参数分组，即时生效项与需重启项分离 |

登录使用 `admin_password`，会话为 HttpOnly Cookie（可保留30 天）。重启 = 关闭后以相同参数拉起新进程；由 systemd 等守护管理时设置 `POLARIX_SUPERVISED=1`，面板重启会直接退出交由守护拉起。

<details>
<summary>图床（assets）与自定义 Provider（点击展开）</summary>

图床配置独立存放于本地 `assets.json`（不入 git），管理台可视化编辑并热更新。上传按 `priority` 从高到低尝试，失败自动切换；`whitelist` 命中的 URL 原样透传。密钥字段不回显，留空保存表示不修改。

```json
{
  "whitelist": ["https://q.qlogo.cn"],
  "providers": [
    { "name": "announce", "enabled": true, "priority": 90, "config": { "token": "xxx" } }
  ]
}
```

新 Provider 只需实现 `assets.ImageProvider` 并在 `init()` 中注册，面板自动出现配置表单：

```go
func init() {
	assets.Register("mine", newMine, []assets.ConfigField{
		{Key: "token", Label: "访问令牌", Type: "password", Required: true},
	})
}

type mine struct{ cl *assets.Client; token string }

func newMine(cl *assets.Client, cfg map[string]any) (assets.ImageProvider, error) { /* ... */ }
func (p *mine) Name() string                       { return "mine" }
func (p *mine) Upload(ctx context.Context, in assets.ProviderInput) (string, error) { /* ... */ }
```

</details>

## 编写插件

插件放在 `plugins/` 目录（不是 `lib/plugin`），在 `plugins/register.go` 匿名导入。框架在 `init()` 阶段收集插件，`main` 启动时统一加载配置与访问控制。

<details>
<summary>插件结构：模板与 Plugin / Config 字段（点击展开）</summary>

```go
package myplugin

import (
	"Plrx/lib/constant"
	"Plrx/lib/context"
	"Plrx/lib/plugin"
)

func init() {
	plugin.Register(&plugin.Plugin{
		Id:          "myplugin",
		Name:        "示例插件",
		Description: "一个示例",
		Commands: []*plugin.Command{
			{
				Prefix:         "/hello",
				Role:           constant.RoleMember,
				DisablePrivate: false,
				Describe:       "打招呼",
				Handle:         hello,
			},
		},
		Config: []plugin.ConfigField{
			{Key: "greeting", Label: "问候语", Type: "text", Placeholder: "你好"},
			{Key: "admin_only", Label: "仅管理员", Type: "boolean"},
			{Key: "api_key", Label: "API Key", Type: "password", Required: true},
		},
		ValidateConfig: validate,
		ApplyConfig:    apply,
	})
}

func hello(ctx *context.MessageContext) error { return ctx.Text("你好").Send() }
func validate(values map[string]any) error    { /* 返回 error 拒绝保存 */ }
func apply(values map[string]any) error       { /* 配置生效时调用，可初始化客户端 */ }
```

| Plugin 字段 | 说明 |
|---|---|
| `Id` | 插件唯一 ID，日志与配置定位 |
| `Name` / `Description` | 展示名称与描述，管理台卡片显示 |
| `Commands` | 指令列表 |
| `Config` | 配置项声明，面板据此渲染表单（保存即热更） |
| `ValidateConfig` | 保存前校验，返回 error 则拒绝写入 |
| `ApplyConfig` | 启动加载与每次保存后调用，适合初始化客户端 |

配置类型：`text` / `password` / `boolean` / `number`。`password` 不回显，留空保存表示保留原值；`Required` 标记必填。

</details>

<details>
<summary>指令系统：Command 字段 / 子指令 / 参数解析（点击展开）</summary>

| Command 字段 | 说明 |
|---|---|
| `Prefix` | 指令前缀，只有匹配的消息进入插件；后注册的同前缀覆盖先注册的 |
| `Role` | 最低身份要求：`RoleMember` / `RoleAdmin` / `RoleOwner`，不满足静默失败 |
| `DisablePrivate` | 禁止私聊使用 |
| `Describe` | 指令描述 |
| `Handle` | 处理函数 `func(*context.MessageContext) error`，错误写日志不发给用户 |
| `PermissionDenied` | 权限不足时调用，可自定义提示 |
| `HandleError` | `Handle` 返回错误或 panic 后调用 |
| `SubCommand` / `SubCommandFallback` | 子指令列表；未命中任何子指令时回退（默认父指令 `Handle`） |
| `Parser` / `ParserTarget` | 参数解析器与解析目标类型 |

子指令就是嵌套的普通指令：命中 `Prefix` 后按消息下一个词继续匹配，可多层嵌套。访问控制会为每个子指令路径生成独立规则（如 `/parent child`）。

```go
plugin.Register(&plugin.Plugin{
	Id: "console",
	Commands: []*plugin.Command{
		{
			Prefix: "/db",
			Handle: dbHelp,
			SubCommand: []*plugin.Command{
				{Prefix: "list", Handle: dbList},
				{Prefix: "clean", Role: constant.RoleOwner, Handle: dbClean},
			},
		},
	},
})
```

**参数解析** — 默认 `DefaultParser` 把原始消息整体作为字符串放入 `ctx.Parsed`。需要结构化参数时自定义 `parser.Parser`（`Parse(msg string, result any) error`），`ParserTarget` 指向目标结构体类型：

```go
type Args struct{ Keyword string; Count int }

type myParser struct{}
func (p *myParser) Parse(msg string, result any) error {
	fields := strings.Fields(msg)
	target := result.(*Args)
	target.Keyword = fields[1]
	target.Count, _ = strconv.Atoi(fields[2])
	return nil
}

&plugin.Command{
	Prefix:       "/search",
	Parser:       &myParser{},
	ParserTarget: reflect.TypeOf(Args{}),
	Handle: func(ctx *context.MessageContext) error {
		args := ctx.Parsed.(Args) // 类型断言
		return ctx.Text(fmt.Sprintf("搜索 %s x%d", args.Keyword, args.Count)).Send()
	},
}
```

</details>

**访问控制** — 管理台自动为所有指令（含子指令路径）提供规则，保存在 `plugin_access`：

- `off` — 关闭限制，所有人可用（默认）
- `whitelist` — 仅名单内用户 / 群可用
- `blacklist` — 名单内用户 / 群禁用

每个插件可设 `default` 规则再按指令路径覆盖；群聊优先使用 `member_openid`，缺失回退 `union_openid`，私聊使用 `user_openid`。

<details>
<summary>常用 API：发送消息 / 按钮键盘（点击展开）</summary>

**发送消息** — 链式 API 按消息来源自动路由：

```go
ctx.Text("你好").Send()
ctx.Markdown("## 标题\n正文").Send()
ctx.UnsafeMarkdownTemplate("UserIdCard", &templates.Args{"id": ctx.UserId}).Send()

// 图片：显式尺寸可选，缺省自动探测（WebP/PNG/JPEG/GIF）
ctx.Image(url, "图", 800, 600).Send()
ctx.Image(localFile, "本地图").Send()

// 构造器聚合：文本 + 图片 + Markdown + 按钮一次发送
ctx.Msg().At(ctx.UserId).Text(" 看这个").
	Add(ctx.Image(re.Url, "随机图")).
	Keyboard(k).Send()
```

主动推送（非回复场景）：`ctx.Client.SendGroupMessage(data, groupId)` / `SendPrivateMessage(data, userId)`，或 HTTP 接口 `/push/:scope/:openid`。

**按钮键盘** — 最多 5 行 × 每行 5 个，`AppendButton` 返回 `*Button` 指针链式设置：

```go
k := &buttons.Keyboard{}
btn, _ := k.AppendButton("id", "点击前", "点击后", buttons.Blue, 0)
btn.SetAutoCommand("/hello", false, false)   // 点击发送指令
btn.SetHref("https://example.com")           // 跳转链接（带协议头）
btn.SetCallback("data", cbHandle)            // 回调 + 处理函数
btn.SetCallbackWithoutHandle("data")         // 仅回调数据，由别处注册
btn.SetPermission(buttons.AllUser)           // SomeUser / Admin / AllUser
btn.SetUserWhiteList([]string{"openid1"})    // 仅指定用户可点
btn.SetUnsupportedTip("此按钮不可用")
```

按钮样式枚举仅 `buttons.Gray` / `buttons.Blue`。回调函数用 `buttons.RegisterCallbackFunc("id", handle)` 注册，签名 `func(*context.CallbackContext) error`，数据在 `ctx.Data`；`SetCallback` 的 ID 会自动绑定处理函数，无需重复注册。

</details>

<details>
<summary>进阶能力：定时任务 / 存储 / Markdown 模板 / 请求（点击展开）</summary>

**定时任务** — 支持 Cron 与 Interval（同时设置优先 Interval）：

```go
schedule.Register(&schedule.Job{
	Id:        "daily-report",
	PluginId:  "myplugin",
	Cron:      "0 9 * * *", // 5 段 cron：分 时 日 月 周
	Interval:  time.Hour,
	Immediate: true,        // Interval 任务启动后立即执行一次
	GroupId:   "",          // 预设推送目标（可选）
	Handle:    func(ctx *context.ScheduleContext) error { /* ... */ },
})
```

任务可 `Cancel` / `Pause` / `Resume`，管理台实时展示。`ScheduleContext` 无关联用户消息，发送需主动推送。

**存储** — SQLite 键值，五级命名空间自动绑定上下文：

```go
ctx.GlobalStorage.Set("k", 42)     // 全局
ctx.PluginStorage.Set("k", "v")    // 当前插件
ctx.CommandStorage.Set("k", true)  // 当前指令
ctx.UserStorage.Set("k", "维度")    // 当前用户
ctx.GroupStorage.Set("k", "维度")   // 当前群

var n int
ctx.UserStorage.Get("k", &n)
```

每个 `*storage.Store` 提供 `Set / Get / Has / Delete / Clear`，value 为任意可 JSON 序列化的值。

**Markdown 模板** — `templates/markdown/*.md` 即模板，文件名（不含后缀）为模板 ID：

```markdown
## {{ title }}
用户：{{ user.name }}
第一条：{{ items.#0 }}
```

参数支持 `string / int / int64 / float64 / bool` 及嵌套 `map[string]any` / `[]any`。模板缺失、参数不足或类型不合法时返回错误（`UnsafeMarkdownTemplate` 则 panic）。

**请求** — 统一用 `ctx.Request`，自带超时：

```go
ctx.Request.Get(url, &result, headers)
ctx.Request.Post(url, body, &result, headers)
ctx.Request.PostForm(url, form, &result, headers)
ctx.Request.PostMultipart(url, mp, &result, headers)
```

`body` 可为 `[]byte` 或可 JSON 序列化对象；`result` 为带 json 标签的结构体指针（nil 不解析）。

**入站消息** — `ctx.Raw` / `ctx.Content` / `ctx.Parsed`（需类型断言）/ `ctx.MessageId` / `ctx.UserId` / `ctx.GroupId` / `ctx.Target`（`constant.PrivateMessage` / `GroupMessage`）；解析增强：`ctx.Mentions` @列表、`ctx.Quote` 引用、`ctx.Emojis`、`ctx.AttachmentTypes`、`ctx.AvatarURL`。

</details>

## 前端开发

<details>
<summary>构建与开发（点击展开）</summary>

```bash
cd web && pnpm install
pnpm dev        # 开发服务器，/admin/api 代理到本地 4514 端口
pnpm build      # 构建产物输出到 ../lib/admin/dist，随后 go build 嵌入
```

</details>

## 目录结构

<details>
<summary>仓库布局（点击展开）</summary>

```
Polarix/
├── main.go                 # 入口
├── lib/
│   ├── admin/              # 管理台后端（go:embed dist）
│   ├── assets/             # 图床聚合与 Provider 注册表
│   ├── buttons/            # 按钮键盘
│   ├── config/             # 配置加载
│   ├── context/            # 插件上下文与消息构造器
│   ├── gateway/            # WebSocket 网关
│   ├── images/             # 图片尺寸探测
│   ├── message/            # 消息部件（文本/图片/媒体/Markdown）
│   ├── plugin/             # 插件与指令注册
│   ├── qqapi/              # QQ 开放平台 API 客户端
│   ├── requests/           # HTTP 请求封装
│   ├── schedule/           # 定时任务
│   ├── storage/            # SQLite 存储
│   └── templates/          # Markdown 模板引擎
├── plugins/                # 插件
├── templates/markdown/     # Markdown 消息模板
└── web/                    # 管理台前端
```

</details>

## 许可证

MIT
