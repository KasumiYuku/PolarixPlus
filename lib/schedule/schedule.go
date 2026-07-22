package schedule

import (
	"Plrx/lib/constant"
	"Plrx/lib/context"
	"Plrx/lib/qqapi"
	"log"
	"sync"
	"time"
)

// HandleFunc 定时任务处理函数
type HandleFunc func(ctx *context.ScheduleContext) error

// Job 定时任务定义
// Cron 与 Interval 二选一; 同时设置时优先 Interval
// 可预设推送目标 (GroupId / UserId / Target), 触发时自动写入 ScheduleContext
type Job struct {
	Id        string                 // 任务唯一 ID
	PluginId  string                 // 所属插件 ID
	Cron      string                 // 5 段 cron: 分 时 日 月 周, 如 "0 9 * * *" 每天 9:00
	Interval  time.Duration          // 固定间隔, 如 time.Hour
	Immediate bool                   // Interval 任务是否立即执行一次
	GroupId   string                 // 预设群 OpenID (可选)
	UserId    string                 // 预设用户 OpenID (可选)
	Target    constant.MessageOrigin // 预设发送目标; 未设时按 GroupId/UserId 推断
	Handle    HandleFunc
}

type registeredJob struct {
	job      *Job
	cron     *cronExpr
	lastFire time.Time // cron: 上次触发的分钟刻度
}

var (
	jobs     []*registeredJob
	jobsLock sync.RWMutex
	client   *qqapi.Client
	stopCh   chan struct{}
	started  bool
	startMu  sync.Mutex
)

// Register 注册定时任务 (插件 init 中调用, 风格同 buttons.RegisterCallbackFunc)
func Register(job *Job) {
	if job == nil {
		log.Printf("[schedule] 忽略空 Job")
		return
	}
	if job.Id == "" {
		log.Printf("[schedule] 忽略无 Id 的 Job (plugin=%v)", job.PluginId)
		return
	}
	if job.Handle == nil {
		log.Printf("[schedule] 忽略无 Handle 的 Job: %v", job.Id)
		return
	}
	if job.Interval <= 0 && job.Cron == "" {
		log.Printf("[schedule] 忽略未设置 Cron/Interval 的 Job: %v", job.Id)
		return
	}

	rj := &registeredJob{job: job}
	if job.Interval <= 0 {
		expr, err := parseCron(job.Cron)
		if err != nil {
			log.Printf("[schedule] Job %v cron 解析失败: %v", job.Id, err)
			return
		}
		rj.cron = expr
	}

	jobsLock.Lock()
	defer jobsLock.Unlock()
	for i, existing := range jobs {
		if existing.job.Id == job.Id {
			log.Printf("[schedule] 警告: Job Id=%v 已存在, 覆盖旧任务", job.Id)
			jobs[i] = rj
			return
		}
	}
	jobs = append(jobs, rj)
}

// GetJobCount 已注册任务数
func GetJobCount() int {
	jobsLock.RLock()
	defer jobsLock.RUnlock()
	return len(jobs)
}

// Start 启动调度器 (main 中在 qqapi 初始化后调用)
func Start(c *qqapi.Client) {
	startMu.Lock()
	defer startMu.Unlock()
	if started {
		log.Printf("[schedule] 调度器已在运行")
		return
	}
	if c == nil {
		log.Printf("[schedule] 无法启动: qqapi.Client 为空")
		return
	}
	client = c
	stopCh = make(chan struct{})
	started = true

	jobsLock.RLock()
	snapshot := append([]*registeredJob(nil), jobs...)
	jobsLock.RUnlock()

	var intervalN, cronN int
	for _, rj := range snapshot {
		if rj.job.Interval > 0 {
			intervalN++
			go runInterval(rj)
		} else {
			cronN++
		}
	}
	if cronN > 0 {
		go runCronLoop()
	}
	log.Printf("[schedule] 调度器已启动 (interval=%d, cron=%d)", intervalN, cronN)
}

// Stop 停止调度器
func Stop() {
	startMu.Lock()
	defer startMu.Unlock()
	if !started {
		return
	}
	close(stopCh)
	started = false
	log.Printf("[schedule] 调度器已停止")
}

func runInterval(rj *registeredJob) {
	job := rj.job
	if job.Immediate {
		fire(job)
	}
	ticker := time.NewTicker(job.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			fire(job)
		}
	}
}

func runCronLoop() {
	// 对齐到下一整秒后每秒检查, 同一分钟只触发一次
	now := time.Now()
	time.Sleep(time.Until(now.Truncate(time.Second).Add(time.Second)))
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-stopCh:
			return
		case t := <-ticker.C:
			t = t.Local()
			minuteKey := t.Truncate(time.Minute)
			var toFire []*Job
			jobsLock.Lock()
			for _, rj := range jobs {
				if rj.cron == nil {
					continue
				}
				if !rj.cron.match(t) {
					continue
				}
				if !rj.lastFire.IsZero() && rj.lastFire.Equal(minuteKey) {
					continue
				}
				rj.lastFire = minuteKey
				toFire = append(toFire, rj.job)
			}
			jobsLock.Unlock()
			for _, job := range toFire {
				go fire(job)
			}
		}
	}
}

func fire(job *Job) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[schedule] Job %v (plugin=%v) panic: %v", job.Id, job.PluginId, r)
		}
	}()

	ctx := &context.ScheduleContext{}
	ctx.Init(client)
	ctx.JobId = job.Id
	ctx.PluginId = job.PluginId
	if job.PluginId != "" {
		ctx.BindStorage(job.PluginId, "schedule:"+job.Id)
	}
	applyJobTarget(ctx, job)

	log.Printf("[schedule] 触发 Job %v (plugin=%v)", job.Id, job.PluginId)
	if err := job.Handle(ctx); err != nil {
		log.Printf("[schedule] Job %v (plugin=%v) error: %v", job.Id, job.PluginId, err)
	}
}

// 将 Job 上预设的推送目标写入上下文
// Target 为 PrivateMessage 时强制私聊; 否则: 有 GroupId 走群, 仅有 UserId 走私聊
func applyJobTarget(ctx *context.ScheduleContext, job *Job) {
	if job.GroupId != "" {
		ctx.SetGroupId(job.GroupId)
	}
	if job.UserId != "" {
		ctx.SetUserId(job.UserId)
	}

	if job.Target == constant.PrivateMessage {
		ctx.SetMessageOrigin(constant.PrivateMessage)
		return
	}
	if job.GroupId != "" {
		ctx.SetMessageOrigin(constant.GroupMessage)
		return
	}
	if job.UserId != "" {
		ctx.SetMessageOrigin(constant.PrivateMessage)
	}
}
