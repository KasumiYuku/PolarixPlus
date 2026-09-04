export type Theme = 'auto' | 'light' | 'dark'

const KEY = 'px-theme'
const ORDER: Theme[] = ['auto', 'light', 'dark']
const LABEL: Record<Theme, string> = { auto: '跟随系统', light: '浅色', dark: '深色' }

// 偏好 auto/light/dark 存 localStorage; DOM 恒 data-theme="theme-polaris" (index.html 设置),
// 深色由 .dark class 表达, auto 由系统偏好实时驱动。
export function currentTheme(): Theme {
  const v = localStorage.getItem(KEY)
  return (v === 'light' || v === 'dark' || v === 'auto' ? v : 'auto') as Theme
}

function sync(theme: Theme) {
  const dark = theme === 'dark' || (theme === 'auto' && matchMedia('(prefers-color-scheme: dark)').matches)
  document.documentElement.classList.toggle('dark', dark)
}

export function applyTheme(theme: Theme) {
  sync(theme)
  localStorage.setItem(KEY, theme)
}

/** 启动时与 auto 态下的系统偏好变化监听 */
export function initTheme() {
  sync(currentTheme())
  matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
    if (currentTheme() === 'auto') sync('auto')
  })
}

export function nextTheme(): Theme {
  const next = ORDER[(ORDER.indexOf(currentTheme()) + 1) % ORDER.length]
  applyTheme(next)
  return next
}

export { LABEL }