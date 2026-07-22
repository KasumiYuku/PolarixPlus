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
	paused   bool
	cancel   chan struct{}
	stopOnce sync.Once
}

func (rj *registeredJob) stop() {
	rj.stopOnce.Do(func() {
		close(rj.cancel)
	})
}

var (
	jobs       []*registeredJob
	jobsLock   sync.RWMutex
	client     *qqapi.Client
	stopCh     chan struct{}
	started    bool
	cronLoopOn bool
	startMu    sync.Mutex
)

// Register 注册定时任务 (插件 init 中调用, 风格同 buttons.RegisterCallbackFunc)
// 若 Id 已存在则覆盖旧任务 (旧任务会被取消)
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

	rj := &registeredJob{
		job:    job,
		cancel: make(chan struct{}),
	}
	if job.Interval <= 0 {
		expr, err := parseCron(job.Cron)
		if err != nil {
			log.Printf("[schedule] Job %v cron 解析失败: %v", job.Id, err)
			return
		}
		rj.cron = expr
	}

	jobsLock.Lock()
	replaced := false
	for i, existing := range jobs {
		if existing.job.Id == job.Id {
			log.Printf("[schedule] 警告: Job Id=%v 已存在, 覆盖旧任务", job.Id)
			existing.stop()
			jobs[i] = rj
			replaced = true
			break
		}
	}
	if !replaced {
		jobs = append(jobs, rj)
	}
	jobsLock.Unlock()

	// 调度器已启动时, 为新任务拉起对应循环
	startMu.Lock()
	running := started
	needCron := running && job.Interval <= 0 && !cronLoopOn
	if needCron {
		cronLoopOn = true
	}
	startMu.Unlock()
	if !running {
		return
	}
	if job.Interval > 0 {
		go runInterval(rj)
	} else if needCron {
		go runCronLoop()
	}
}

// Cancel 取消并移除任务, 不可恢复. 返回是否找到该任务
func Cancel(id string) bool {
	jobsLock.Lock()
	defer jobsLock.Unlock()
	for i, rj := range jobs {
		if rj.job.Id == id {
			rj.stop()
			jobs = append(jobs[:i], jobs[i+1:]...)
			log.Printf("[schedule] 已取消 Job %v", id)
			return true
		}
	}
	return false
}

// Pause 暂停任务 (保留注册, 到点不触发). 返回是否找到该任务
func Pause(id string) bool {
	jobsLock.Lock()
	defer jobsLock.Unlock()
	for _, rj := range jobs {
		if rj.job.Id == id {
			rj.paused = true
			log.Printf("[schedule] 已暂停 Job %v", id)
			return true
		}
	}
	return false
}

// Resume 恢复已暂停的任务. 返回是否找到该任务
func Resume(id string) bool {
	jobsLock.Lock()
	defer jobsLock.Unlock()
	for _, rj := range jobs {
		if rj.job.Id == id {
			rj.paused = false
			log.Printf("[schedule] 已恢复 Job %v", id)
			return true
		}
	}
	return false
}

// IsPaused 查询任务是否暂停. exists 表示任务是否仍注册
func IsPaused(id string) (paused bool, exists bool) {
	jobsLock.RLock()
	defer jobsLock.RUnlock()
	for _, rj := range jobs {
		if rj.job.Id == id {
			return rj.paused, true
		}
	}
	return false, false
}

// Exists 任务是否仍注册 (未 Cancel)
func Exists(id string) bool {
	jobsLock.RLock()
	defer jobsLock.RUnlock()
	for _, rj := range jobs {
		if rj.job.Id == id {
			return true
		}
	}
	return false
}

// GetJobCount 已注册任务数 (含暂停中的)
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
		cronLoopOn = true
		go runCronLoop()
	}
	log.Printf("[schedule] 调度器已启动 (interval=%d, cron=%d)", intervalN, cronN)
}

// Stop 停止整个调度器 (所有任务循环退出, 注册表保留)
func Stop() {
	startMu.Lock()
	defer startMu.Unlock()
	if !started {
		return
	}
	close(stopCh)
	started = false
	cronLoopOn = false
	log.Printf("[schedule] 调度器已停止")
}

func runInterval(rj *registeredJob) {
	job := rj.job
	if job.Immediate {
		tryFire(rj)
	}
	ticker := time.NewTicker(job.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-stopCh:
			return
		case <-rj.cancel:
			return
		case <-ticker.C:
			tryFire(rj)
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
			var toFire []*registeredJob
			jobsLock.Lock()
			for _, rj := range jobs {
				if rj.cron == nil || rj.paused {
					continue
				}
				if !rj.cron.match(t) {
					continue
				}
				if !rj.lastFire.IsZero() && rj.lastFire.Equal(minuteKey) {
					continue
				}
				rj.lastFire = minuteKey
				toFire = append(toFire, rj)
			}
			jobsLock.Unlock()
			for _, rj := range toFire {
				go fire(rj)
			}
		}
	}
}

func tryFire(rj *registeredJob) {
	jobsLock.RLock()
	if rj.paused {
		jobsLock.RUnlock()
		return
	}
	// 已被 Cancel 时 cancel channel 已关闭
	select {
	case <-rj.cancel:
		jobsLock.RUnlock()
		return
	default:
	}
	jobsLock.RUnlock()
	fire(rj)
}

func fire(rj *registeredJob) {
	job := rj.job
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
