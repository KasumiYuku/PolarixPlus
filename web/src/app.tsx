import { createResource, createSignal, onCleanup, Switch, Match, For, Show } from 'solid-js'
import { api } from './api'
import { createRoute, navigate } from './router'
import { Icon, Toasts, fmtDur } from './ui'
import { currentTheme, applyTheme, LABEL } from './theme'
import { connectLive, disconnectLive, useLiveOverview, LiveOverview } from './live'
import Login from './pages/login'
import OverviewPage from './pages/overview'
import LogsPage from './pages/logs'
import PluginsPage from './pages/plugins'
import AssetsPage from './pages/assets'
import JobsPage from './pages/jobs'
import SettingsPage from './pages/settings'

const NAV = [
  { path: 'overview', label: '概览', icon: 'grid' as const },
  { path: 'logs', label: '日志', icon: 'list' as const },
  { path: 'plugins', label: '插件', icon: 'package' as const },
  { path: 'assets', label: '图床', icon: 'image' as const },
  { path: 'jobs', label: '定时任务', icon: 'clock' as const },
  { path: 'settings', label: '设置', icon: 'sliders' as const },
]

export default function App() {
  const [me] = createResource(() => api<{ ok: boolean }>('/api/me'))
  return (
    <>
      <Switch fallback={<Splash />}>
        <Match when={me.loading}> <Splash /> </Match>
        <Match when={me.error}> <Login /> </Match>
        <Match when={me()}> <Shell /> </Match>
      </Switch>
      <Toasts />
    </>
  )
}

function Splash() {
  return (
    <div class="flex h-full items-center justify-center">
      <div class="px-fade grid h-11 w-11 place-items-center rounded-xl bg-primary-600 text-[22px] font-bold text-white shadow-md shadow-primary-600/25">
        P
      </div>
    </div>
  )
}

function Shell() {
  const route = createRoute()
  const overview = useLiveOverview()
  const [now, setNow] = createSignal(Date.now())

  connectLive()
  const ticker = setInterval(() => setNow(Date.now()), 1000)
  onCleanup(() => {
    disconnectLive()
    clearInterval(ticker)
  })

  // uptime 锚点取自快照内的服务器时钟: now - uptime_sec*1000, 每秒随快照重锚,
  // 进程重启或后台唤醒后自动归零重计, 不再依赖本地单次锚定。
  const anchor = () => {
    const ov = overview()
    return ov ? ov.now - ov.runtime.uptime_sec * 1000 : 0
  }
  const uptime = () => {
    const ov = overview()
    if (!ov) return '—'
    return fmtDur((now() - anchor()) / 1000)
  }

  const page = () => route()[0] ?? 'overview'
  const theme = () => currentTheme()

  const gatewayOnline = (): boolean | null => {
    const ov = overview()
    if (!ov) return null
    if (ov.runtime.protocol === 'websocket') return ov.gateway?.connected ? true : false
    return true
  }

  return (
    <div class="flex h-full overflow-hidden bg-background-2 text-foreground">
      <aside class="flex w-[236px] shrink-0 flex-col border-r border-sidebar-line bg-sidebar px-3 py-4">
        <div class="flex items-center gap-3 px-2 pb-5 pt-1">
          <span class="grid h-9 w-9 shrink-0 place-items-center rounded-[10px] bg-primary-600 text-[17px] font-bold text-white shadow-md shadow-primary-600/20">
            P
          </span>
          <div class="min-w-0">
            <b class="block text-[14.5px] font-semibold leading-tight tracking-[-0.01em] text-foreground">Polarix</b>
            <small class="text-[11.5px] text-muted-foreground-2">后台管理终端</small>
          </div>
        </div>

        <nav class="flex min-h-0 flex-1 flex-col gap-0.5 overflow-y-auto">
          <For each={NAV}>
            {(item) => (
              <a
                class={`group flex items-center gap-2.5 rounded-lg px-2.5 py-2 text-[13.5px] transition-colors duration-150 outline-none focus-visible:ring-2 focus-visible:ring-primary-500/40 ${
                  page() === item.path
                    ? 'bg-primary-600/10 font-medium text-primary-700 dark:bg-primary-400/15 dark:text-primary-300'
                    : 'text-muted-foreground hover:bg-sidebar-nav-hover hover:text-foreground'
                }`}
                href={'#/' + item.path}
                onClick={(e) => {
                  e.preventDefault()
                  navigate(item.path)
                }}
              >
                <Icon name={item.icon} size={17} class="shrink-0" />
                <span>{item.label}</span>
              </a>
            )}
          </For>
        </nav>

        <div class="mt-2 flex flex-col gap-0.5 border-t border-sidebar-divider pt-2.5">
          <div class={`flex items-center gap-2 rounded-lg px-2.5 py-2 text-[12.5px] ${gatewayOnline() === false ? 'text-destructive' : 'text-muted-foreground'}`}>
            <span class="relative flex h-2 w-2 shrink-0">
              <span
                class={`inline-flex h-2 w-2 rounded-full ${gatewayOnline() === false ? 'bg-destructive' : gatewayOnline() ? 'bg-primary-500' : 'bg-surface-4'}`}
                style={
                  gatewayOnline()
                    ? { '--ring-color': 'color-mix(in srgb, var(--primary-500) 20%, transparent)', '--dot-c': 'var(--primary-500)' }
                    : undefined
                }
              />
              {gatewayOnline() && <span class="px-breathe absolute inset-0 rounded-full bg-primary-500/60" />}
            </span>
            <span class="truncate">{statusText(overview(), gatewayOnline())}</span>
          </div>

          <Show when={overview()}>
            <div class="flex flex-col gap-0.5 rounded-lg px-2.5 py-1.5">
              <span class="text-[11px] text-muted-foreground-2">运行时长</span>
              <b class="tnum text-[12.5px] font-medium text-foreground">{uptime()}</b>
            </div>
          </Show>

          <button
            class="flex cursor-pointer items-center gap-2 rounded-lg px-2.5 py-2 text-left text-[12.5px] text-muted-foreground transition-colors duration-150 outline-none hover:bg-sidebar-nav-hover hover:text-foreground focus-visible:ring-2 focus-visible:ring-primary-500/40"
            title="切换主题"
            onClick={() => applyTheme(nextTheme())}
          >
            <Icon name={theme() === 'dark' ? 'moon' : 'sun'} size={14} class="shrink-0" />
            <span>{LABEL[theme()]}</span>
          </button>
        </div>
      </aside>

      <main class="min-w-0 flex-1 overflow-y-auto">
        <div class="mx-auto flex w-full max-w-[1240px] flex-col gap-4 px-7 pb-16 pt-7">
          <Switch>
            <Match when={page() === 'overview'}>
              <OverviewPage uptime={uptime} />
            </Match>
            <Match when={page() === 'logs'}>
              <LogsPage />
            </Match>
            <Match when={page() === 'plugins'}>
              <PluginsPage route={route} />
            </Match>
            <Match when={page() === 'assets'}>
              <AssetsPage />
            </Match>
            <Match when={page() === 'jobs'}>
              <JobsPage />
            </Match>
            <Match when={page() === 'settings'}>
              <SettingsPage />
            </Match>
            <Match when={true}>
              <div class="px-4 pt-24 text-center text-sm text-muted-foreground-2">页面不存在</div>
            </Match>
          </Switch>
        </div>
      </main>
    </div>
  )
}

function nextTheme(): 'auto' | 'light' | 'dark' {
  const order = ['auto', 'light', 'dark'] as const
  return order[(order.indexOf(currentTheme()) + 1) % order.length]
}

function statusText(ov: LiveOverview | null, online: boolean | null): string {
  if (!ov) return '同步中'
  const proto = ov.runtime.protocol === 'websocket' ? '网关' : 'Webhook'
  if (online === false) return `${proto} 重连中`
  if (online === true) return `${proto} 正常 :${ov.runtime.port}`
  return proto
}
