/**
 * Draws a character out of dots (system.md signature elements). Each dot
 * starts a short way out from its own place in the glyph, as if the
 * character had been gently exploded, and drifts straight back in, top to
 * bottom (roughly stroke order). The shape reads from the first frames and
 * tightens into the character; the finished dotted glyph holds for a beat,
 * then melts into the solid one. Used by the launch splash (复) and the end
 * of a session (毕).
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
  /** Travel time of each dot, ms. */
  flight?: number
  /** How long the finished dotted glyph holds before turning solid, ms. */
  hold?: number
  /** How long the dots take to melt into the solid glyph, ms. */
  solidFor?: number
  /** How far dots start from their place, as a multiple of the glyph size. */
  spread?: number
}

interface Dot {
  tx: number
  ty: number
  sx: number
  sy: number
  delay: number
}

const SERIF = '"Noto Serif SC"'
const FALLBACK = '"Songti SC", "SimSun", serif'
const clamp = (v: number) => Math.min(1, Math.max(0, v))
const easeOut = (t: number) => 1 - (1 - t) ** 3
const easeInOut = (t: number) => (t < 0.5 ? 4 * t * t * t : 1 - (-2 * t + 2) ** 3 / 2)

/** Loads the glyph's font (Google Fonts serves CJK in slices), but never waits long. */
export async function loadGlyphFont(glyph: string, size: number) {
  if (!document.fonts) return
  await Promise.race([
    document.fonts.load(`600 ${size}px ${SERIF}`, glyph).catch(() => {}),
    new Promise((r) => setTimeout(r, 900)),
  ])
}

/**
 * The font for the whole animation, fixed at the start: Noto Serif SC if it
 * has loaded, otherwise the system serif only. Naming Noto in the fallback
 * too would let the solid glyph switch font mid-animation if Noto arrived
 * late, so it no longer matched the dots.
 */
function pickFont(glyph: string, size: number): string {
  const noto = `600 ${size}px ${SERIF}`
  const ready = document.fonts?.check(noto, glyph) ?? false
  return ready ? noto : `600 ${size}px ${FALLBACK}`
}

function sample(o: DotGlyphOptions, font: string): Dot[] {
  const pad = Math.ceil(o.size * 0.1)
  const box = o.size + pad * 2
  const c = document.createElement('canvas')
  c.width = c.height = box
  const g = c.getContext('2d', { willReadFrequently: true })
  if (!g) return []
  g.font = font
  g.textAlign = 'center'
  g.textBaseline = 'middle'
  g.fillText(o.glyph, box / 2, box / 2 + o.size * 0.04)
  const data = g.getImageData(0, 0, box, box).data
  const step = Math.max(3, o.size / 42)
  const dots: Dot[] = []
  for (let y = step / 2; y < box; y += step)
    for (let x = step / 2; x < box; x += step) {
      if (data[(Math.round(y) * box + Math.round(x)) * 4 + 3] > 120) {
        dots.push({ tx: o.cx - box / 2 + x, ty: o.cy - box / 2 + y, sx: 0, sy: 0, delay: 0 })
      }
    }
  if (dots.length === 0) return dots
  const top = Math.min(...dots.map((d) => d.ty))
  const bottom = Math.max(...dots.map((d) => d.ty))
  const spread = (o.spread ?? 0.42) * o.size
  for (const d of dots) {
    // Out along the line from the glyph's centre through the dot, plus a little scatter.
    const dx = d.tx - o.cx
    const dy = d.ty - o.cy
    const len = Math.hypot(dx, dy) || 1
    const push = spread * (0.5 + Math.random() * 0.5)
    const jitter = o.size * 0.08
    d.sx = d.tx + (dx / len) * push + (Math.random() - 0.5) * jitter
    d.sy = d.ty + (dy / len) * push + (Math.random() - 0.5) * jitter
    d.delay = ((d.ty - top) / Math.max(1, bottom - top)) * (o.stagger ?? 450) + Math.random() * 120
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

  const font = pickFont(o.glyph, o.size)
  const dots = sample(o, font)
  const flight = o.flight ?? 1000
  // The solid glyph waits until every dot has landed, then a short hold.
  const landed = Math.max(0, ...dots.map((d) => d.delay)) + flight
  const solidAt = landed + (o.hold ?? 250)
  const solidFor = o.solidFor ?? 500
  const end = solidAt + solidFor
  // Dots end almost touching, so the dotted glyph reads as solid strokes.
  const step = Math.max(3, o.size / 42)
  const rEnd = step * 0.42
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
      ctx.globalAlpha = Math.min(1, q * 1.6) * (1 - solid)
      ctx.beginPath()
      ctx.arc(d.sx + (d.tx - d.sx) * q, d.sy + (d.ty - d.sy) * q, rEnd * (0.55 + 0.45 * q), 0, Math.PI * 2)
      ctx.fill()
    }
    if (solid > 0) {
      ctx.globalAlpha = solid
      ctx.font = font
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
