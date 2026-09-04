import { createResource, createSignal, For, Show } from 'solid-js'
import { api, ManagedPlugin, AccessConfig, AccessRule, PluginField } from '../api'
import { Button, Badge, Card, Empty, Field, SkeletonRows, Switch, toast } from '../ui'
import { navigate } from '../router'

export default function PluginsPage(props: { route: () => string[] }) {
  // 路由参数需在 JSX 内响应式读取, 顶层三元只在挂载时执行一次
  return (
    <Show when={props.route()[1]} keyed fallback={<PluginList />}>
      {(id) => <PluginDetail id={id} />}
    </Show>
  )
}

function PluginList() {
  const [plugins] = createResource(() => api<ManagedPlugin[]>('/api/plugins'))
  return (
    <div class="px-rise">
      <h2 class="mb-6 font-display text-[26px] font-semibold leading-tight tracking-tight text-foreground">插件</h2>
      <Show when={plugins()} fallback={<SkeletonRows rows={6} />}>
        <div class="grid grid-cols-1 gap-5 md:grid-cols-2 xl:grid-cols-3">
          <For each={plugins()}>
            {(p, i) => (
              <button
                class="px-rise px-blob group flex flex-col gap-3 border border-line-2 bg-card p-6 text-left shadow-sm transition-all duration-500 outline-none hover:translate-y-0.5 hover:border-primary-500/40 hover:bg-muted-hover hover:shadow-md focus-visible:ring-2 focus-visible:ring-primary-500/40"
                style={{ 'animation-delay': `${i() * 40}ms` }}
                onClick={() => navigate('plugins/' + encodeURIComponent(p.id))}
              >
                <div class="flex items-start justify-between gap-3">
                  <div class="min-w-0">
                    <h3 class="font-display truncate text-[16px] font-semibold leading-snug text-foreground transition-colors duration-300 group-hover:text-primary-700 dark:group-hover:text-primary-300">
                      {p.name}
                    </h3>
                    <span class="font-mono text-xs text-primary-600 dark:text-primary-400">{p.id}</span>
                  </div>
                  <span class="font-mono text-xs text-muted-foreground-2">{String(i() + 1).padStart(2, '0')}</span>
                </div>
                <p class="flex-1 text-[13px] leading-relaxed text-muted-foreground">{p.description || '暂无插件说明'}</p>
                <div class="flex flex-wrap gap-2">
                  <Badge tone="accent">{p.fields.length} 项设置</Badge>
                  <Badge>{p.commands.length} 条指令</Badge>
                  <Badge tone={p.access.default.mode === 'off' ? 'dim' : 'ok'}>
                    {modeLabel(p.access.default.mode)}
                  </Badge>
                </div>
              </button>
            )}
          </For>
        </div>
      </Show>
    </div>
  )
}

const MODE_LABEL: Record<string, string> = { off: '默认放行', whitelist: '白名单', blacklist: '黑名单' }
export function modeLabel(mode: string): string {
  return MODE_LABEL[mode] ?? mode
}

function PluginDetail(props: { id: string }) {
  const id = decodeURIComponent(props.id)
  const [data, setData] = createSignal<ManagedPlugin | null>(null)
  const [values, setValues] = createSignal<Record<string, unknown>>({})
  const [access, setAccess] = createSignal<AccessConfig>({ default: { mode: 'off', users: [], groups: [] }, commands: {} })
  const [busy, setBusy] = createSignal(false)

  const load = async () => {
    try {
      const p = await api<ManagedPlugin>('/api/plugins/' + encodeURIComponent(id))
      setData(p)
      setValues({ ...p.values })
      setAccess(JSON.parse(JSON.stringify(p.access)) as AccessConfig)
    } catch (err) {
      toast((err as Error).message, 'err')
    }
  }
  load()

  const setValue = (key: string, v: unknown) => setValues((cur) => ({ ...cur, [key]: v }))
  const setRule = (path: string, patch: Partial<AccessRule>) =>
    setAccess((cur) => {
      if (path === 'default') return { ...cur, default: { ...cur.default, ...patch } }
      return { ...cur, commands: { ...cur.commands, [path]: { ...cur.commands[path], ...patch } } }
    })

  const save = async () => {
    const p = data()!
    setBusy(true)
    try {
      if (p.fields.length) {
        await api('/api/plugins/' + encodeURIComponent(id), { method: 'PUT', body: JSON.stringify(values()) })
      }
      await api('/api/plugins/' + encodeURIComponent(id) + '/access', { method: 'PUT', body: JSON.stringify(access()) })
      toast('已保存并即时生效')
      load()
    } catch (err) {
      toast((err as Error).message, 'err')
    } finally {
      setBusy(false)
    }
  }

  return (
    <Show when={data()} keyed fallback={<SkeletonRows rows={8} />}>
      {(p) => (
        <div class="px-rise">
          <div class="mb-6 flex items-start justify-between gap-4">
            <div>
              <h2 class="font-display text-[26px] font-semibold leading-tight tracking-tight text-foreground">{p.name}</h2>
              <p class="mt-1 font-mono text-xs text-muted-foreground-2">{p.id}</p>
            </div>
            <button class="text-[13px] text-primary-600 hover:underline dark:text-primary-400" onClick={() => navigate('plugins')}>
              ← 返回插件目录
            </button>
          </div>

          <Card title="插件设置" actions={<span class="text-xs text-muted-foreground-2">保存后即时生效</span>}>
            <Show when={p.fields.length} fallback={<Empty text="此插件没有自定义设置" />}>
              <For each={p.fields}>
                {(f) => (
                  <Field label={f.label} desc={f.description}>
                    <PluginInput field={f} value={values()[f.key]} onChange={(v) => setValue(f.key, v)} />
                  </Field>
                )}
              </For>
            </Show>
          </Card>

          <Card title="访问控制" actions={<span class="text-xs text-muted-foreground-2">未覆盖的指令使用默认规则</span>}>
            <RuleEditor path="default" title="插件默认规则" hint="未覆盖的指令将使用此规则" rule={access().default} onChange={(patch) => setRule('default', patch)} />
            <For each={p.commands}>
              {(cmd) => (
                <RuleEditor
                  path={cmd}
                  title={cmd}
                  hint="单指令覆盖"
                  rule={access().commands[cmd] ?? { mode: 'off', users: [], groups: [] }}
                  onChange={(patch) => setRule(cmd, patch)}
                />
              )}
            </For>
          </Card>

          <div class="sticky bottom-0 mt-4 flex justify-end bg-background-2/80 pb-1 pt-2 backdrop-blur-sm">
            <Button variant="primary" onClick={save} disabled={busy()}>
              {busy() ? '保存中…' : '保存全部配置'}
            </Button>
          </div>
        </div>
      )}
    </Show>
  )
}

function PluginInput(props: { field: PluginField; value: unknown; onChange: (v: unknown) => void }) {
  if (props.field.type === 'boolean') {
    return <Switch checked={!!props.value} onChange={props.onChange} />
  }
  if (props.field.type === 'password') {
    return (
      <input
        class="h-10 w-full max-w-[480px] rounded-full border border-line-3 bg-field px-5 text-[13px] text-foreground outline-none transition-all duration-300 placeholder:text-muted-foreground-2 focus:border-primary-600 focus:ring-2 focus:ring-primary-500/30"
        type="password"
        placeholder={props.value ? '已配置，留空则保留' : (props.field.placeholder || '')}
        autocomplete="new-password"
        onInput={(e) => props.onChange(e.currentTarget.value)}
      />
    )
  }
  return (
    <input
      class="h-10 w-full max-w-[480px] rounded-full border border-line-3 bg-field px-5 text-[13px] text-foreground outline-none transition-all duration-300 placeholder:text-muted-foreground-2 focus:border-primary-600 focus:ring-2 focus:ring-primary-500/30"
      type={props.field.type === 'number' ? 'number' : 'text'}
      placeholder={props.field.placeholder || ''}
      value={String(props.value ?? '')}
      onInput={(e) => props.onChange(props.field.type === 'number' ? Number(e.currentTarget.value) : e.currentTarget.value)}
    />
  )
}

function RuleEditor(props: {
  path: string
  title: string
  hint: string
  rule: AccessRule
  onChange: (patch: Partial<AccessRule>) => void
}) {
  const lines = (arr: string[]) => (arr ?? []).join('\n')
  const parse = (v: string) => v.split(/\r?\n|,/).map((s) => s.trim()).filter(Boolean)
  const fieldCls =
    'min-h-[76px] w-full rounded-[1.4rem] border border-line-3 bg-field px-4 py-2.5 font-mono text-xs leading-relaxed text-foreground outline-none transition-all duration-300 placeholder:text-muted-foreground-2 focus:border-primary-600 focus:ring-2 focus:ring-primary-500/30'
  return (
    <div class="border-b border-card-divider px-6 py-5 last:border-b-0">
      <div class="mb-3 flex items-center justify-between gap-3.5">
        <div class="min-w-0">
          <b class="block text-[14px] font-semibold text-foreground">{props.title}</b>
          <span class="ml-1 text-xs text-muted-foreground-2">{props.hint}</span>
        </div>
        <select
          class="h-9 min-w-[130px] rounded-full border border-line-3 bg-field px-3.5 text-[12.5px] text-foreground outline-none transition-colors duration-300 focus:border-primary-600 focus:ring-2 focus:ring-primary-500/30"
          value={props.rule.mode}
          onChange={(e) => props.onChange({ mode: e.currentTarget.value as AccessRule['mode'] })}
        >
          <option value="off">关闭限制</option>
          <option value="whitelist">白名单</option>
          <option value="blacklist">黑名单</option>
        </select>
      </div>
      <div class="grid grid-cols-1 gap-3 md:grid-cols-2">
        <label class="flex flex-col gap-1.5 text-[12.5px] text-muted-foreground">
          QQ 用户
          <textarea
            class={fieldCls}
            placeholder="每行一个 QQ 号，也支持逗号分隔"
            value={lines(props.rule.users)}
            onInput={(e) => props.onChange({ users: parse(e.currentTarget.value) })}
          />
        </label>
        <label class="flex flex-col gap-1.5 text-[12.5px] text-muted-foreground">
          QQ 群
          <textarea
            class={fieldCls}
            placeholder="每行一个群号，也支持逗号分隔"
            value={lines(props.rule.groups)}
            onInput={(e) => props.onChange({ groups: parse(e.currentTarget.value) })}
          />
        </label>
      </div>
    </div>
  )
}