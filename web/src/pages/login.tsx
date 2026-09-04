import { createSignal, Show } from 'solid-js'
import { post } from '../api'
import { Button, Icon } from '../ui'

export default function Login() {
  const [password, setPassword] = createSignal('')
  const [remember, setRemember] = createSignal(false)
  const [busy, setBusy] = createSignal(false)
  const [error, setError] = createSignal('')

  const submit = async (e: SubmitEvent) => {
    e.preventDefault()
    setBusy(true)
    setError('')
    try {
      await post('/api/login', { password: password(), remember: remember() })
      location.hash = '#/overview'
      location.reload()
    } catch (err) {
      setError((err as Error).message || '登录失败')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div class="flex h-full items-center justify-center bg-background p-5">
      <form
        class="px-pop w-full max-w-[380px] rounded-2xl border border-line-3 bg-card p-8 shadow-xl shadow-black/5"
        onSubmit={submit}
      >
        <div class="mb-7 flex flex-col items-center gap-3 text-center">
          <span class="grid h-12 w-12 place-items-center rounded-2xl bg-primary-600 text-2xl font-bold text-white shadow-md shadow-primary-600/25">
            P
          </span>
          <div>
            <h1 class="text-lg font-semibold tracking-[-0.01em] text-foreground">Polarix WebUI</h1>
            <p class="mt-1 text-[12.5px] text-muted-foreground-2">请输入 config.json 中配置的管理密码</p>
          </div>
        </div>

        <label class="mb-3 block">
          <span class="mb-1.5 block text-[12.5px] font-medium text-muted-foreground">管理密码</span>
          <input
            type="password"
            value={password()}
            autocomplete="current-password"
            placeholder="admin_password"
            onInput={(e) => setPassword(e.currentTarget.value)}
            autofocus
            class="h-10 w-full rounded-lg border border-line-3 bg-background px-3.5 text-[13.5px] text-foreground outline-none transition-colors duration-150 placeholder:text-muted-foreground-2 focus:border-primary-600 focus:ring-2 focus:ring-primary-500/30"
          />
        </label>

        <label class="mb-5 flex cursor-pointer select-none items-center gap-2">
          <input
            type="checkbox"
            class="h-3.5 w-3.5 accent-[var(--primary-600)]"
            checked={remember()}
            onChange={(e) => setRemember(e.currentTarget.checked)}
          />
          <span class="text-[12.5px] text-muted-foreground">记住密码（30 天）</span>
        </label>

        <Show when={error()}>
          <div class="mb-4 flex items-center gap-2 rounded-lg border border-red-200 bg-red-50 px-3.5 py-2.5 text-[12.5px] text-red-600 dark:bg-red-400/10 dark:text-red-300">
            <Icon name="alert" size={15} class="shrink-0" />
            <span>{error()}</span>
          </div>
        </Show>

        <Button variant="primary" type="submit" disabled={busy() || !password()} class="w-full">
          {busy() ? '登录中…' : '登录'}
        </Button>
      </form>
    </div>
  )
}
