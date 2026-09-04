// Package state 运行态快照与热点计数, 供管理台概览读取。
package state

import (
	"runtime"
	"sync/atomic"
	"time"
)

// Counters 进程热点计数, 全部走原子变量, 不引入锁。
type Counters struct {
	Recv   atomic.Uint64 // 收到的消息事件
	Sent   atomic.Uint64 // 成功发出的消息
	Button atomic.Uint64 // 按钮交互事件
}

var (
	std      = &Runtime{}
	bootOnce atomic.Bool
)

// Runtime 全局运行态。
type Runtime struct {
	boot     time.Time
	protocol atomic.Value // string
	port     atomic.Uint32
	counters Counters
}

// Boot 启动时调用一次, 记录协议与监听端口。
func Boot(protocol string, port int) {
	if !bootOnce.CompareAndSwap(false, true) {
		return
	}
	std.boot = time.Now()
	std.protocol.Store(protocol)
	std.port.Store(uint32(port))
}

func IncRecv()   { std.counters.Recv.Add(1) }
func IncSent()   { std.counters.Sent.Add(1) }
func IncButton() { std.counters.Button.Add(1) }

// Mem 运行时内存快照。
type Mem struct {
	HeapAlloc uint64 `json:"heap_alloc"`
	HeapSys   uint64 `json:"heap_sys"`
	NumGC     uint32 `json:"num_gc"`
	LastGC    int64  `json:"last_gc"` // Unix 毫秒, 0 表示未发生过
}

// View 概览接口返回的运行态。
type View struct {
	UptimeSec  int64           `json:"uptime_sec"`
	Protocol   string          `json:"protocol"`
	Port       int             `json:"port"`
	GoVersion  string          `json:"go_version"`
	Goroutines int             `json:"goroutines"`
	Mem        Mem             `json:"mem"`
	Counters   CounterSnapshot `json:"counters"`
}

type CounterSnapshot struct {
	Recv   uint64 `json:"recv"`
	Sent   uint64 `json:"sent"`
	Button uint64 `json:"button"`
}

// Snapshot 组装当前快照。
func Snapshot() View {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	lastGC := int64(0)
	if mem.NumGC > 0 {
		lastGC = int64(mem.LastGC) / int64(time.Millisecond)
	}
	protocol, _ := std.protocol.Load().(string)
	return View{
		UptimeSec:  int64(time.Since(std.boot).Seconds()),
		Protocol:   protocol,
		Port:       int(std.port.Load()),
		GoVersion:  runtime.Version(),
		Goroutines: runtime.NumGoroutine(),
		Mem: Mem{
			HeapAlloc: mem.HeapAlloc,
			HeapSys:   mem.HeapSys,
			NumGC:     mem.NumGC,
			LastGC:    lastGC,
		},
		Counters: CounterSnapshot{
			Recv:   std.counters.Recv.Load(),
			Sent:   std.counters.Sent.Load(),
			Button: std.counters.Button.Load(),
		},
	}
}
