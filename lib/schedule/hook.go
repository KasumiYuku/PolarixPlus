package schedule

import "sync/atomic"

// 任务注册/状态/触发变化时通知管理台实时总线, 触发即时推送。
// 用原子指针保存, 生命周期内可随时挂载/卸载, 无订阅时成本为零。
var changeHook atomic.Value // func()

// SetChangeHook 挂载变更通知回调; 传 nil 表示卸载。
func SetChangeHook(fn func()) {
	changeHook.Store(fn)
}

// NotifyChanged 触发一次变更通知, 无回调时静默。
func NotifyChanged() {
	if fn, ok := changeHook.Load().(func()); ok && fn != nil {
		fn()
	}
}
