# 定时任务 API (`lib/schedule`)

插件可在 `init()` 中注册定时回调, 由框架在进程内调度执行。

包路径: `Plrx/lib/schedule`  
上下文: `*context.ScheduleContext` (`Plrx/lib/context`)

---

## 快速示例

```go
package myplugin

import (
	"Plrx/lib/constant"
	"Plrx/lib/context"
	"Plrx/lib/plugin"
	"Plrx/lib/schedule"
	"time"
)

func init() {
	plugin.Register(&plugin.Plugin{
		Id: "myplugin",
		// Commands: ...
	})

	// 固定间隔: 每小时一次, 启动后立即执行一次
	schedule.Register(&schedule.Job{
		Id:        "myplugin-heartbeat",
		PluginId:  "myplugin",
		Interval:  time.Hour,
		Immediate: true,
		GroupId:   "群OpenID",
		Handle:    onHeartbeat,
	})

	// Cron: 每天 09:00
	schedule.Register(&schedule.Job{
		Id:       "myplugin-daily",
		PluginId: "myplugin",
		Cron:     "0 9 * * *",
		UserId:   "用户OpenID",
		Handle:   onDaily,
	})
}

func onHeartbeat(ctx *context.ScheduleContext) error {
	// 已按 Job 预设写入 GroupId / Target
	// 定时任务无用户消息, 主动推送请标记 Initiative
	msg := ctx.Text("心跳")
	msg.SetInitiativeMessage()
	return msg.Send()
}

func onDaily(ctx *context.ScheduleContext) error {
	msg := ctx.Markdown("## 每日提醒")
	msg.SetInitiativeMessage()
	return msg.Send()
}
```

运行时控制:

```go
schedule.Pause("myplugin-daily")
schedule.Resume("myplugin-daily")
schedule.Cancel("myplugin-heartbeat") // 移除, 不可恢复
```

---

## 类型

### `schedule.HandleFunc`

```go
type HandleFunc func(ctx *context.ScheduleContext) error
```

处理函数返回的 `error` 仅写入日志, 不会发给 QQ。  
panic 会被调度器捕获并记录。

### `schedule.Job`

| 字段 | 类型 | 说明 |
|------|------|------|
| `Id` | `string` | **必填**, 全局唯一。重复注册会覆盖旧任务 |
| `PluginId` | `string` | 所属插件 ID, 用于日志与 storage 命名空间 |
| `Cron` | `string` | 5 段 cron: `分 时 日 月 周`。与 `Interval` 二选一 |
| `Interval` | `time.Duration` | 固定间隔。与 `Cron` 同时设置时 **优先 Interval** |
| `Immediate` | `bool` | 仅对 Interval 有效: 注册/启动后是否立刻执行一次 |
| `GroupId` | `string` | 预设群 OpenID (可选) |
| `UserId` | `string` | 预设用户 OpenID (可选) |
| `Target` | `constant.MessageOrigin` | 预设发送目标; 见下方推断规则 |
| `Handle` | `HandleFunc` | **必填**, 到点回调 |

#### 目标推断规则

触发时写入 `ScheduleContext`:

1. 若设置了 `GroupId` / `UserId`, 写入上下文对应字段
2. 若 `Target == constant.PrivateMessage`, 强制私聊
3. 否则有 `GroupId` → 群聊; 仅有 `UserId` → 私聊
4. 回调内仍可用 `ctx.SetGroupId` / `SetUserId` / `SetMessageOrigin` 覆盖

### `context.ScheduleContext`

| 字段/能力 | 说明 |
|-----------|------|
| `JobId` | 当前任务 Id |
| `PluginId` | 插件 Id |
| `FiredAt` | 本次触发时间 |
| `Text` / `Markdown` / `MarkdownTemplate` | 构造消息 (继承 `MessageManager`) |
| `GlobalStorage` / `PluginStorage` / `CommandStorage` | KV 存储 |
| `Request` | HTTP 客户端 |
| `SetGroupId` / `SetUserId` / `SetMessageOrigin` | 设置推送目标 |

`CommandStorage` 命名空间为: `schedule:{JobId}` (在 `PluginId` 下绑定)。

> 定时任务没有用户 `msg_id`。主动发消息请调用  
> `msg.SetInitiativeMessage()` 再 `Send()`。  
> 目标会话需已允许主动推送 (见 push 插件的 `/enablepush` 等)。

---

## 注册与查询

### `func Register(job *Job)`

注册任务。通常在插件 `init()` 中调用。

- `Id` / `Handle` 为空, 或未设置 `Cron`/`Interval`, 或 cron 非法 → 忽略并打日志
- 相同 `Id` 再次注册 → 取消旧任务并替换
- 调度器已 `Start` 后注册的 **Interval** 任务会立即拉起循环; **Cron** 任务由统一 cron 循环接管

### `func GetJobCount() int`

当前仍注册的任务数量 (含已 Pause 的)。

### `func Exists(id string) bool`

任务是否仍在注册表中 (未被 `Cancel`)。

### `func IsPaused(id string) (paused bool, exists bool)`

- `exists == false`: 无此任务
- `exists == true`: `paused` 表示是否处于暂停

---

## 暂停 / 恢复 / 取消

### `func Pause(id string) bool`

暂停任务: 保留注册, 到点不触发。  
返回是否找到该任务。

### `func Resume(id string) bool`

恢复已暂停任务。返回是否找到该任务。

### `func Cancel(id string) bool`

永久取消并移除任务, **不可** 通过 `Resume` 恢复, 需重新 `Register`。  
Interval 后台循环会退出。返回是否找到该任务。

| | Pause | Cancel |
|--|-------|--------|
| 保留注册 | 是 | 否 |
| 可 Resume | 是 | 否 |
| 到点触发 | 否 | 否 (已移除) |

---

## 调度器生命周期 (框架)

### `func Start(client *qqapi.Client)`

在 `main` 中, `qqapi` 初始化后调用。重复调用无效。

### `func Stop()`

停止调度器所有循环; 注册表中的 Job 仍保留, 再次 `Start` 不会自动恢复循环 (需进程重启或重新设计)。一般进程退出即可。

---

## Cron 表达式

格式: **分 时 日 月 周** (5 段, 空格分隔)

| 段 | 范围 |
|----|------|
| 分 | 0–59 |
| 时 | 0–23 |
| 日 | 1–31 |
| 月 | 1–12 |
| 周 | 0–6 (`0` = 周日) |

支持:

| 写法 | 含义 |
|------|------|
| `*` | 任意 |
| `n` | 固定值 |
| `n-m` | 范围 |
| `*/n` | 步进 |
| `n-m/s` | 范围步进 |
| `a,b,c` | 列表 |

示例:

| 表达式 | 含义 |
|--------|------|
| `0 9 * * *` | 每天 09:00 |
| `*/5 * * * *` | 每 5 分钟 |
| `0 9 * * 1` | 每周一 09:00 |
| `0 0 1 * *` | 每月 1 日 00:00 |
| `30 8-18 * * 1-5` | 工作日 8:30–18:30 每小时整点后 30 分 |

同一分钟内 cron 任务最多触发一次。

---

## 注意事项

1. **Id 全局唯一**, 建议带插件前缀: `"echo-daily"`  
2. 定时回调在 **独立 goroutine** 中执行, 注意并发安全  
3. 主动推送受 QQ 平台限制, 且目标需先授权  
4. 任务状态 (暂停/取消) **仅内存**, 进程重启后以再次 `Register` 为准  
5. `Pause` ≠ `Cancel`; 临时关闭用 Pause, 彻底不要用 Cancel  
