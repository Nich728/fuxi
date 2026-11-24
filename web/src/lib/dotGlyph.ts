/**
 * Draws a character out of dots (system.md signature elements): fine dots
 * flow in from around it, top to bottom (roughly stroke order), then melt
 * into the solid glyph so it reads cleanly. Used by the launch splash (复)
 * and the end of a session (毕).
 */
export interface DotGlyphOptions {
  glyph: string
  /** Glyph size in CSS pixels. */
  size: number
  /** Glyph centre in CSS pixels. */
  cx: number
  cy: number
  colour: string
  /** Delay spread from the top of the glyph to the bottom, ms. */
  stagger?: number
  /** Flight time of each dot, ms. */
  flight?: number
  /** When the solid glyph starts to show, and how long it takes, ms. */
  solidAt?: number
  solidFor?: number
  /** How far dots start from the centre, as a multiple of the glyph size. */
  spread?: number
}

interface Dot {
  tx: number
  ty: number
  sx: number
  sy: number
  delay: number
  curve: number
}

const font = (size: number) => `600 ${size}px "Noto Serif SC", "Songti SC", serif`
const clamp = (v: number) => Math.min(1, Math.max(0, v))
const easeOut = (t: number) => 1 - (1 - t) ** 4
const easeInOut = (t: number) => (t < 0.5 ? 4 * t * t * t : 1 - (-2 * t + 2) ** 3 / 2)

/** Loads the glyph's font (Google Fonts serves CJK in slices), but never waits long. */
export async function loadGlyphFont(glyph: string, size: number) {
  if (!document.fonts) return
  await Promise.race([document.fonts.load(font(size), glyph).catch(() => {}), new Promise((r) => setTimeout(r, 900))])
}

function sample(o: DotGlyphOptions): Dot[] {
  const pad = Math.ceil(o.size * 0.1)
  const box = o.size + pad * 2
  const c = document.createElement('canvas')
  c.width = c.height = box
  const g = c.getContext('2d')
  if (!g) return []
  g.font = font(o.size)
  g.textAlign = 'center'
  g.textBaseline = 'middle'
  g.fillText(o.glyph, box / 2, box / 2 + o.size * 0.04)
  const data = g.getImageData(0, 0, box, box).data
  const step = Math.max(3, o.size / 40)
  const dots: Dot[] = []
  for (let y = 0; y < box; y += step)
    for (let x = 0; x < box; x += step) {
      if (data[(Math.round(y) * box + Math.round(x)) * 4 + 3] > 140) {
        dots.push({ tx: o.cx - box / 2 + x, ty: o.cy - box / 2 + y, sx: 0, sy: 0, delay: 0, curve: 0 })
      }
    }
  const top = Math.min(...dots.map((d) => d.ty))
  const bottom = Math.max(...dots.map((d) => d.ty))
  const spread = (o.spread ?? 1.6) * o.size
  for (const d of dots) {
    const a = Math.random() * Math.PI * 2
    const r = o.size * 0.7 + Math.random() * spread
    d.sx = o.cx + Math.cos(a) * r
    d.sy = o.cy + Math.sin(a) * r * 1.3
    d.delay = ((d.ty - top) / Math.max(1, bottom - top)) * (o.stagger ?? 650) + Math.random() * 180
    d.curve = (Math.random() - 0.5) * o.size * 0.5
  }
  return dots
}

/** Runs the animation on `canvas` (sized to its CSS box); returns a stop function. */
export function playDotGlyph(canvas: HTMLCanvasElement, o: DotGlyphOptions): () => void {
  const ctx = canvas.getContext('2d')
  if (!ctx) return () => {}
  const dpr = Math.min(3, window.devicePixelRatio || 1)
  const w = canvas.clientWidth
  const h = canvas.clientHeight
  canvas.width = Math.round(w * dpr)
  canvas.height = Math.round(h * dpr)
  ctx.scale(dpr, dpr)

  const dots = sample(o)
  const flight = o.flight ?? 1100
  const solidAt = o.solidAt ?? 1700
  const solidFor = o.solidFor ?? 600
  const end = solidAt + solidFor
  const r0 = Math.max(0.9, o.size / 150)
  let raf = 0
  const t0 = performance.now()

  const frame = (t: number) => {
    const el = t - t0
    ctx.clearRect(0, 0, w, h)
    const solid = easeInOut(clamp((el - solidAt) / solidFor))
    ctx.fillStyle = o.colour
    for (const d of dots) {
      const q = easeOut(clamp((el - d.delay) / flight))
      if (q <= 0) continue
      const x = d.sx + (d.tx - d.sx) * q + d.curve * Math.sin(q * Math.PI) * (1 - q)
      const y = d.sy + (d.ty - d.sy) * q
      ctx.globalAlpha = (0.25 + 0.75 * q) * (1 - solid * 0.9)
      ctx.beginPath()
      ctx.arc(x, y, (r0 + r0 * 1.1 * q) * (1 - solid * 0.6), 0, Math.PI * 2)
      ctx.fill()
    }
    if (solid > 0) {
      ctx.globalAlpha = solid
      ctx.font = font(o.size)
      ctx.textAlign = 'center'
      ctx.textBaseline = 'middle'
      ctx.fillText(o.glyph, o.cx, o.cy + o.size * 0.04)
    }
    ctx.globalAlpha = 1
    if (el < end + 50) raf = requestAnimationFrame(frame)
  }
  raf = requestAnimationFrame(frame)
  return () => cancelAnimationFrame(raf)
}
