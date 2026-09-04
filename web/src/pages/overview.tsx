import { createEffect, createSignal, For, onCleanup, Show } from 'solid-js'
import { api, LogsView, LogEntry } from '../api'
import { Card, Badge, Icon, fmtClock, fmtDur, fmtBytes, fmtNum, fmtTime } from '../ui'
import { useLiveOverview, onLog, LiveOverview } from '../live'
import { Trend } from '../components/trend'

const MAX_POINTS = 60

export default function OverviewPage(props: { uptime: () => string }) {
  const overview = useLiveOverview()
  const [recent, setRecent] = createSignal<LogEntry[]>([])
  const [pts, setPts] = createSignal<number[][]>([[], []])

  // 首次进入拉一份日志历史, 之后由实时订阅增量补齐
  api<LogsView>('/api/logs?limit=12')
    .then((v) => setRecent(v.entries))
    .catch(() => {})
  const unlog = onLog((e) => {
    setRecent((cur) => [e, ...cur.filter((x) => x.id !== e.id)].slice(0, 12))
  })
  onCleanup(unlog)

  let primed = false
  let last = { recv: 0, sent: 0 }
  createEffect(() => {
    const ov = overview()
    if (!ov) return
    const c = ov.runtime.counters
    if (!primed) {
      primed = true
      last = { recv: c.recv, sent: c.sent }
      return
    }
    const dr = Math.max(0, c.recv - last.recv)
    const ds = Math.max(0, c.sent - last.sent)
    last = { recv: c.recv, sent: c.sent }
    const [ra, sa] = pts()
    setPts([[...ra, dr].slice(-MAX_POINTS), [...sa, ds].slice(-MAX_POINTS)])
  })

  const rates = () => {
    const p = pts()
    return { recv: p[0][p[0].length - 1] ?? 0, sent: p[1][p[1].length - 1] ?? 0 }
  }

  const ov = () => overview() as LiveOverview

  return (
    <Show when={overview()} fallback={<OverviewSkeleton />}>
      <div class="px-rise flex items-center justify-between gap-4">
        <div>
          <h2 class="text-xl font-semibold tracking-[-0.015em] text-foreground">概览</h2>
          <p class="mt-1 text-[13px] text-muted-foreground-2">进程实时状态，数据每秒推送</p>
        </div>
        <span class="inline-flex items-center gap-2 rounded-full border border-primary-500/30 bg-primary-50 px-3 py-1 text-xs font-medium text-primary-700 dark:bg-primary-400/10 dark:text-primary-300">
          <span class="relative flex h-1.5 w-1.5">
            <span class="px-breathe absolute inset-0 rounded-full bg-primary-500/60" />
            <span class="relative inline-flex h-1.5 w-1.5 rounded-full bg-primary-500" style={{ '--dot-c': 'var(--primary-500)', '--ring-color': 'color-mix(in srgb, var(--primary-500) 20%, transparent)' }} />
          </span>
          实时
        </span>
      </div>

      <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <Kpi icon="clock" label="运行时长" value={props.uptime()} foot="从进程启动起计" delay={0} />
        <Kpi icon="activity" label="收到消息" value={fmtNum(ov().runtime.counters.recv)} foot={`发送 ${fmtNum(ov().runtime.counters.sent)} · 按钮 ${fmtNum(ov().runtime.counters.button)}`} delay={1} />
        <Kpi icon="cpu" label="堆内存" value={fmtBytes(ov().runtime.mem.heap_alloc)} foot={`系统占用 ${fmtBytes(ov().runtime.mem.heap_sys)}`} delay={2} />
        <Kpi icon="layers" label="注册统计" value={`${fmtNum(ov().counts.plugins)} 插件`} foot={`${fmtNum(ov().counts.commands)} 指令 · ${fmtNum(ov().counts.templates)} 模板`} delay={3} />
      </div>

      <div class="grid grid-cols-1 gap-4 xl:grid-cols-2">
        <Card
          title="实时流量"
          actions={
            <div class="flex flex-wrap gap-3.5">
              <span class="inline-flex items-center gap-1.5 text-[12.5px] text-muted-foreground tnum">
                <i class="h-2 w-2 rounded-full" style={{ background: 'var(--chart-primary)' }} />
                收 {rates().recv}/s
              </span>
              <span class="inline-flex items-center gap-1.5 text-[12.5px] text-muted-foreground tnum">
                <i class="h-2 w-2 rounded-full" style={{ background: 'var(--chart-2)' }} />
                发 {rates().sent}/s
              </span>
            </div>
          }
        >
          <div class="px-4 pb-3 pt-2">
            <Trend series={pts} colors={['--chart-primary', '--chart-2']} height={150} />
          </div>
        </Card>

        <Card title="资源占用">
          <div class="px-5 py-2.5">
            <Stat label="Goroutines" value={fmtNum(ov().runtime.goroutines)} />
            <Stat label="堆内存分配" value={fmtBytes(ov().runtime.mem.heap_alloc)} />
            <Stat label="系统占用" value={fmtBytes(ov().runtime.mem.heap_sys)} />
            <Stat label="GC 次数" value={String(ov().runtime.mem.num_gc)} />
            <Stat label="最近 GC" value={ov().runtime.mem.last_gc ? fmtTime(ov().runtime.mem.last_gc) : '尚未发生'} />
            <Stat label="运行环境" value={`${ov().runtime.go_version} · :${ov().runtime.port}`} />
          </div>
        </Card>
      </div>

      <div class="grid grid-cols-1 gap-4 xl:grid-cols-2">
        <Card title="注册统计">
          <div class="px-5 py-2.5">
            <Stat label="插件" value={fmtNum(ov().counts.plugins)} />
            <Stat label="指令" value={fmtNum(ov().counts.commands)} />
            <Stat label="定时任务" value={fmtNum(ov().counts.jobs)} />
            <Stat label="Markdown 模板" value={fmtNum(ov().counts.templates)} />
            <Show when={ov().assets}>
              <Stat label="图床 Provider" value={`${ov().assets!.enabled} / ${ov().assets!.providers} 启用`} />
            </Show>
            <Stat label="累计日志 / 错误" value={`${fmtNum(ov().logs.total)} / ${fmtNum(ov().logs.errors)}`} />
          </div>
        </Card>

        <Card title="网关状态">
          <GatewayCard ov={ov()} />
        </Card>
      </div>

      <Card
        title="最近日志"
        actions={
          <a class="text-[13px] text-primary-600 hover:underline dark:text-primary-400" href="#/logs" onClick={(e) => { e.preventDefault(); location.hash = '#/logs' }}>
            查看全部 →
          </a>
        }
      >
        <Show when={recent().length} fallback={<div class="px-4 py-9 text-center text-[13px] text-muted-foreground-2">暂无日志</div>}>
          <div class="max-h-[320px] overflow-y-auto">
            <For each={recent()}>
              {(entry) => (
                <div class="px-rise grid grid-cols-[auto_auto_minmax(0,1fr)] items-baseline gap-x-3 border-b border-border px-5 py-[7px] font-mono text-[12.3px] tnum last:border-b-0">
                  <span class="whitespace-nowrap text-muted-foreground-2">{fmtClock(entry.time)}</span>
                  <LevelPill level={entry.level} />
                  <span class="truncate text-primary-600 dark:text-primary-400">{entry.scope}</span>
                  <span class="col-span-3 truncate text-foreground" style={{ 'font-family': 'var(--font-sans)' }}>{entry.msg}</span>
                </div>
              )}
            </For>
          </div>
        </Show>
      </Card>
    </Show>
  )
}

const LEVEL_PILL: Record<string, string> = {
  DEBUG: 'text-muted-foreground-2',
  INFO: 'text-muted-foreground',
  WARN: 'bg-amber-50 text-amber-700 dark:bg-amber-400/10 dark:text-amber-300',
  ERROR: 'bg-red-50 text-red-600 dark:bg-red-400/10 dark:text-red-300',
}

function LevelPill(props: { level: string }) {
  return (
    <span class={`inline-block justify-self-start rounded px-1.5 py-px text-[10px] font-semibold uppercase tracking-wider ${LEVEL_PILL[props.level] ?? LEVEL_PILL.INFO}`}>
      {props.level}
    </span>
  )
}

function RollNum(props: { value: () => string }) {
  let node: HTMLElement | undefined
  createEffect(() => {
    const el = node
    const text = props.value()
    if (!el || el.textContent === text) return
    el.textContent = text
    el.classList.remove('px-tick')
    void el.offsetWidth
    el.classList.add('px-tick')
  })
  return <b ref={node} class="px-tick tnum block truncate text-[22px] font-semibold leading-6 tracking-[-0.015em] text-foreground">{props.value()}</b>
}

function Kpi(props: { icon: 'clock' | 'activity' | 'cpu' | 'layers'; label: string; value: string; foot: string; delay: number }) {
  return (
    <div
      class="px-rise flex flex-col gap-1 rounded-xl border border-line-2 bg-card p-4 shadow-sm transition-shadow duration-200 hover:shadow-md"
      style={{ 'animation-delay': `${props.delay * 40}ms` }}
    >
      <div class="flex items-center justify-between gap-2.5">
        <span class="text-[12.5px] font-medium text-muted-foreground">{props.label}</span>
        <span class="grid h-7 w-7 shrink-0 place-items-center rounded-lg bg-primary-50 text-primary-600 dark:bg-primary-400/10 dark:text-primary-300">
          <Icon name={props.icon} size={15} />
        </span>
      </div>
      <RollNum value={() => props.value} />
      <span class="truncate text-xs text-muted-foreground-2">{props.foot}</span>
    </div>
  )
}

function Stat(props: { label: string; value: string }) {
  return (
    <div class="flex items-baseline justify-between gap-3 border-b border-dashed border-border py-[7.5px] last:border-b-0">
      <span class="text-[13px] text-muted-foreground">{props.label}</span>
      <b class="tnum break-all text-right font-mono text-[12.8px] font-medium text-foreground">{props.value}</b>
    </div>
  )
}

function GatewayCard(props: { ov: LiveOverview }) {
  const gw = props.ov.gateway
  if (props.ov.runtime.protocol === 'webhook') {
    return (
      <div class="flex flex-col gap-2.5 px-5 py-4">
        <Badge tone="ok">监听中</Badge>
        <p class="text-[13px] text-muted-foreground">Webhook 模式：QQ 平台将事件推送到 :{props.ov.runtime.port}/webhook</p>
      </div>
    )
  }
  if (!gw) {
    return (
      <div class="flex flex-col gap-2.5 px-5 py-4">
        <Badge tone="dim">未知</Badge>
        <p class="text-[13px] text-muted-foreground">网关状态尚未就绪</p>
      </div>
    )
  }
  return (
    <div class="px-5 py-2.5">
      <Stat label="连接" value={gw.connected ? '已连接' : '断开'} />
      <Stat label="心跳 ACK" value={gw.heartbeat_ack ? '正常' : '等待中'} />
      <Stat label="Session" value={gw.session_id || '—'} />
      <Stat label="Seq" value={String(gw.seq ?? 0)} />
      <Stat label="连接时长" value={gw.since_ms ? fmtDur(gw.since_ms / 1000) : '—'} />
    </div>
  )
}

function OverviewSkeleton() {
  return (
    <>
      <div class="flex items-center justify-between gap-4">
        <div>
          <h2 class="text-xl font-semibold tracking-[-0.015em] text-foreground">概览</h2>
          <p class="mt-1 text-[13px] text-muted-foreground-2">正在建立实时通道…</p>
        </div>
      </div>
      <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <For each={[0, 1, 2, 3]}>
          {() => (
            <div class="flex flex-col gap-3 rounded-xl border border-line-2 bg-card p-4">
              <span class="px-skeleton h-3.5 w-1/3" />
              <span class="px-skeleton h-6 w-1/2" />
              <span class="px-skeleton h-3 w-2/3" />
            </div>
          )}
        </For>
      </div>
      <div class="grid grid-cols-1 gap-4 xl:grid-cols-2">
        <div class="h-[240px] rounded-xl border border-line-2 bg-card" />
        <div class="h-[240px] rounded-xl border border-line-2 bg-card" />
      </div>
    </>
  )
}
