import { createSignal, onCleanup } from 'solid-js'

function parse(): string[] {
  return location.hash.replace(/^#\/?/, '').split('/').filter(Boolean)
}

// 简易 hash 路由, 无需引入路由库。
// 页面切换走 View Transitions API (浏览器原生, 零依赖), 不支持时退化为直切。
export function createRoute() {
  const [parts, setParts] = createSignal<string[]>(parse())
  const onChange = () => {
    const apply = () => setParts(parse())
    if (document.startViewTransition) document.startViewTransition(apply)
    else apply()
  }
  window.addEventListener('hashchange', onChange)
  onCleanup(() => window.removeEventListener('hashchange', onChange))
  return parts
}

export function navigate(to: string) {
  if (location.hash === '#/' + to) return
  location.hash = '/' + to
}