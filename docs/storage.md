# 数据存储 API (`lib/storage`)

插件可通过 Context 上的存储对象, 将数据持久化到 SQLite。

包路径: `Plrx/lib/storage`  
上下文字段: `*context.Context` (`Plrx/lib/context`)

底层使用纯 Go 驱动 `modernc.org/sqlite`, 无需 CGO。默认数据库文件为 `bot.db`, 可在 `config.json` 中通过 `database` 字段自定义路径。

---

## 快速示例

```go
package myplugin

import (
	"Plrx/lib/context"
	"Plrx/lib/plugin"
	"fmt"
)

func init() {
	plugin.Register(&plugin.Plugin{
		Id: "myplugin",
		Commands: []*plugin.Command{
			{
				Prefix: "/score",
				Handle: scoreHandle,
			},
		},
	})
}

type UserScore struct {
	Points int    `json:"points"`
	Title  string `json:"title"`
}

func scoreHandle(ctx *context.MessageContext) error {
	// 读取当前用户积分
	var score UserScore
	found, err := ctx.UserStorage.Get("score", &score)
	if err != nil {
		return err
	}
	if !found {
		score = UserScore{Points: 0, Title: "新手"}
	}

	score.Points++
	if err := ctx.UserStorage.Set("score", score); err != nil {
		return err
	}

	// 群聊计数
	var groupCount int
	_, _ = ctx.GroupStorage.Get("cmd_count", &groupCount)
	groupCount++
	_ = ctx.GroupStorage.Set("cmd_count", groupCount)

	return ctx.Text(fmt.Sprintf("你的积分: %d\n本群调用次数: %d", score.Points, groupCount)).Send()
}
```

---

## 配置

`config.json` 可选字段:

```json
{
  "port": 8080,
  "appid": "10000",
  "secret": "11111",
  "proxy": "https://api.sgroup.qq.com",
  "database": "bot.db"
}
```

| 字段 | 说明 | 默认值 |
|------|------|--------|
| `database` | SQLite 文件路径 | `bot.db` |

应用启动时调用 `storage.Open`, 退出时调用 `storage.Close`。数据库文件及 WAL 临时文件 (`.db` / `.db-shm` / `.db-wal`) 已加入 `.gitignore`。

### 插件 `init()` 中使用

插件包的 `init()` 会在 `main` 的 `storage.Open` **之前**执行。存储层会在首次读写时自动打开数据库 (默认 `bot.db`), 因此可直接在 `init()` 里 `Get`/`Set`:

```go
func init() {
	var enabled bool
	found, err := storage.Global().Get("feature_x", &enabled)
	// ...
}
```

注意: 若 `config.json` 里 `database` 不是默认 `bot.db`, 请避免在 `init()` 中依赖自定义路径; 此时应把读库逻辑放到指令处理函数或 `main` 启动之后。

---

## Context 暴露的存储对象

指令处理函数 (`*context.MessageContext`) 与按钮回调 (`*context.CallbackContext`) 均可使用下列字段 (二者均嵌入 `*context.Context`):

| 字段 | 作用域 | 隔离维度 | 绑定时机 |
|------|--------|----------|----------|
| `ctx.GlobalStorage` | 全局 | 全机器人共享 | `Init` |
| `ctx.PluginStorage` | 插件 | 当前插件 ID | `BindStorage` |
| `ctx.CommandStorage` | 指令 | 插件 ID + 指令路径 | `BindStorage` |
| `ctx.UserStorage` | 用户 | 当前 `UserId` | `SetUserId` |
| `ctx.GroupStorage` | 群聊 | 当前 `GroupId` | `SetGroupId` |

### 命名空间规则

| 作用域 | scope | namespace 示例 |
|--------|-------|----------------|
| 全局 | `global` | `global` |
| 插件 | `plugin` | `echo` |
| 指令 | `command` | `bind:/bind` |
| 指令 (含子指令) | `command` | `bind:/bind confirm` |
| 用户 | `user` | `用户 OpenID` |
| 群聊 | `group` | `群 OpenID` |

子指令会按完整路径隔离。例如 `/bind` 与 `/bind confirm` 使用不同的 `CommandStorage` 命名空间。

### 注意事项

- 私聊场景通常没有 `GroupId`, 此时 `ctx.GroupStorage` 可能为 `nil`, 使用前请判断。
- `UserStorage` / `GroupStorage` 仅在对应 ID 非空时绑定。
- 回调上下文也会通过 `SetUserId` / `SetGroupId` 绑定用户层与群聊层存储。

---

## Store API

所有作用域共享同一套方法:

```go
type Store struct { /* 内部字段 */ }

func (store *Store) Set(key string, value any) error
func (store *Store) Get(key string, target any) (found bool, err error)
func (store *Store) Has(key string) (bool, error)
func (store *Store) Delete(key string) error
func (store *Store) Clear() error
```

### `Set`

将任意可 JSON 序列化的值写入指定 key。已存在则覆盖。

```go
err := ctx.PluginStorage.Set("enabled", true)
err = ctx.CommandStorage.Set("last_args", []string{"a", "b"})
err = ctx.UserStorage.Set("profile", map[string]any{
	"name": "Alice",
	"lv":   3,
})
```

### `Get`

读取并反序列化到 `target` (必须为非 nil 指针)。

- `found == false`: key 不存在 (不是错误)
- `err != nil`: 读库或反序列化失败

```go
var enabled bool
found, err := ctx.PluginStorage.Get("enabled", &enabled)
if err != nil {
	return err
}
if !found {
	enabled = false
}
```

### `Has`

判断 key 是否存在。

```go
ok, err := ctx.UserStorage.Has("score")
```

### `Delete`

删除单个 key。

```go
err := ctx.CommandStorage.Delete("tmp")
```

### `Clear`

清空当前命名空间下全部数据 (不影响其他作用域/命名空间)。

```go
err := ctx.PluginStorage.Clear()
```

---

## 直接使用 storage 包

一般推荐通过 Context 字段访问。若需在非请求路径 (如 `init`、定时任务) 中读写, 可直接构造:

```go
import "Plrx/lib/storage"

// 全局
store := storage.Global()

// 指定插件
store = storage.Plugin("myplugin")

// 指定指令路径
store = storage.Command("myplugin", "/score")
store = storage.Command("bind", "/bind confirm")

// 指定用户 / 群
store = storage.User("user-openid")
store = storage.Group("group-openid")

_ = store.Set("key", "value")
```

前提: 进程内已调用 `storage.Open(...)` (主程序启动时会完成)。

---

## 表结构

```sql
CREATE TABLE IF NOT EXISTS kv_data (
    scope TEXT NOT NULL,
    namespace TEXT NOT NULL,
    key TEXT NOT NULL,
    value BLOB NOT NULL,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (scope, namespace, key)
);
```

- `value` 以 JSON 二进制存储
- 主键 `(scope, namespace, key)` 保证同一命名空间下 key 唯一
- 不同作用域之间完全隔离

---

## 使用建议

1. **选对作用域**
   - 机器人级开关、共享缓存 → `GlobalStorage`
   - 插件配置、插件内部状态 → `PluginStorage`
   - 单条指令会话/临时状态 → `CommandStorage`
   - 用户个人数据 (积分、绑定信息) → `UserStorage`
   - 群设置、群统计 → `GroupStorage`

2. **key 命名**: 使用有意义的短字符串, 如 `score`、`bind_uid`、`settings`。

3. **结构体优先**: 复杂数据用结构体 + `json` tag, 便于演进字段。

4. **先查再写**: `Get` 返回 `found=false` 时自行初始化默认值, 再 `Set`。

5. **注意 nil**: 私聊中不要直接调用 `ctx.GroupStorage` 方法, 先判断:

```go
if ctx.GroupStorage != nil {
	_ = ctx.GroupStorage.Set("last_cmd", "/score")
}
```

---

## 完整示例: 用户绑定 UID

```go
package bind

import (
	"Plrx/lib/context"
	"Plrx/lib/plugin"
	"strings"
)

func init() {
	plugin.Register(&plugin.Plugin{
		Id: "bind",
		Commands: []*plugin.Command{
			{
				Prefix: "/bind",
				Handle: doBind,
			},
			{
				Prefix: "/myuid",
				Handle: showUID,
			},
		},
	})
}

func doBind(ctx *context.MessageContext) error {
	args := strings.Fields(ctx.Content)
	if len(args) < 2 {
		return ctx.Text("用法: /bind <UID>").Send()
	}
	uid := args[1]
	if err := ctx.UserStorage.Set("game_uid", uid); err != nil {
		return err
	}
	// 可选: 记入插件级索引
	_ = ctx.PluginStorage.Set("last_bind_user", ctx.UserId)
	return ctx.Text("绑定成功: " + uid).Send()
}

func showUID(ctx *context.MessageContext) error {
	var uid string
	found, err := ctx.UserStorage.Get("game_uid", &uid)
	if err != nil {
		return err
	}
	if !found {
		return ctx.Text("尚未绑定, 请使用 /bind <UID>").Send()
	}
	return ctx.Text("你的 UID: " + uid).Send()
}
```

---

## 相关文件

| 路径 | 说明 |
|------|------|
| `lib/storage/storage.go` | 存储实现 |
| `lib/storage/storage_test.go` | 单元测试 |
| `lib/context/base.go` | Context 绑定字段 |
| `lib/middleware/handle.go` | 消息/回调分发时注入存储 |
| `lib/plugin/register.go` | 子指令路径更新 `CommandStorage` |
| `lib/config/config.go` | `database` 配置 |
| `main.go` | 启动 Open / 退出 Close |
