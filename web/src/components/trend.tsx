import { createEffect, onCleanup, onMount } from 'solid-js'

function resolveToken(name: string, depth = 0): string {
  if (depth > 8) return '#888'
  const raw = getComputedStyle(document.documentElement).getPropertyValue(name).trim()
  if (!raw) return '#888'
  const m = raw.match(/^var\((--[\w-]+)\)$/)
  if (m) return resolveToken(m[1], depth + 1)
  return raw
}

export function Trend(props: {
  series: () => number[][]
  colors: string[] // 设计令牌变量名
  height?: number
}) {
  let canvas: HTMLCanvasElement | undefined
  let box: HTMLDivElement | undefined

  const draw = () => {
    const c = canvas
    const el = box
    if (!c || !el) return
    const dpr = window.devicePixelRatio || 1
    const w = el.clientWidth
    const h = props.height ?? 140
    if (c.width !== w * dpr || c.height !== h * dpr) {
      c.width = w * dpr
      c.height = h * dpr
    }
    const ctx = c.getContext('2d')
    if (!ctx) return
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0)

    const grid = resolveToken('--chart-colors-grid-border')
    const colors = props.colors.map(resolveToken)
    const data = props.series()
    const n = Math.max(...data.map((s) => s.length), 0)
    const padX = 2
    const padY = 6

    // 底色网格: 三条水平参考线
    ctx.clearRect(0, 0, w, h)
    ctx.strokeStyle = grid
    ctx.lineWidth = 1
    for (let i = 1; i <= 3; i++) {
      const y = padY + ((h - padY * 2) * i) / 4
      ctx.beginPath()
      ctx.moveTo(padX, y)
      ctx.lineTo(w - padX, y)
      ctx.stroke()
    }

    if (n < 2) return
    const lo = 0
    let hi = 1
    for (const s of data) for (const v of s) if (v > hi) hi = v
    const span = hi - lo || 1

    const xAt = (i: number) => padX + ((w - padX * 2) * i) / (n - 1)
    const yAt = (v: number) => padY + (h - padY * 2) * (1 - (v - lo) / span)

    data.forEach((s, si) => {
      const offset = n - s.length // 左侧留空
      ctx.strokeStyle = colors[si % colors.length]
      ctx.lineWidth = 1.6
      ctx.lineJoin = 'round'
      ctx.beginPath()
      for (let i = 0; i < s.length; i++) {
        const x = xAt(i + offset)
        const y = yAt(s[i])
        if (i === 0) ctx.moveTo(x, y)
        else ctx.lineTo(x, y)
      }
      ctx.stroke()
    })
  }

  onMount(() => {
    const ro = new ResizeObserver(draw)
    if (box) ro.observe(box)
    onCleanup(() => ro.disconnect())
  })
  createEffect(draw)

  return (
    <div class="h-full w-full" ref={box} style={{ height: `${props.height ?? 140}px` }}>
      <canvas ref={canvas} style={{ width: '100%', height: '100%' }} aria-hidden="true" />
    </div>
  )
}
