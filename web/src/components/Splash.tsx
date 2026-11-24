import { useEffect, useRef, useState, type CSSProperties } from 'react'
import { loadGlyphFont, playDotGlyph } from '../lib/dotGlyph'
import { finishSplash } from '../lib/motion'
import './splash.css'

/** How long the launch plays before it dissolves (system.md motion). */
const HOLD = 2900
const FADE = 500

/**
 * The launch (cold start only): dots gather into 复, "Fuxi · 复习" fades in
 * under it, then the splash dissolves into the app. A tap skips it.
 */
export function Splash() {
  const canvas = useRef<HTMLCanvasElement>(null)
  const [phase, setPhase] = useState<'play' | 'out' | 'gone'>('play')
  const [started, setStarted] = useState(false)
  const [size] = useState(() => Math.max(140, Math.min(220, Math.min(innerWidth, innerHeight) * 0.42)))

  useEffect(() => {
    document.documentElement.classList.add('splashing')
    let stop = () => {}
    let live = true
    let out: ReturnType<typeof setTimeout> | undefined
    loadGlyphFont('复', size).then(() => {
      if (!live || !canvas.current) return
      const jade = getComputedStyle(document.documentElement).getPropertyValue('--jade').trim() || '#3fd98a'
      stop = playDotGlyph(canvas.current, {
        glyph: '复',
        size,
        cx: canvas.current.clientWidth / 2,
        cy: canvas.current.clientHeight / 2 - size * 0.25,
        colour: jade,
      })
      setStarted(true)
      out = setTimeout(() => setPhase('out'), HOLD)
    })
    return () => {
      live = false
      stop()
      clearTimeout(out)
    }
  }, [size])

  useEffect(() => {
    if (phase !== 'out') return
    finishSplash()
    const t = setTimeout(() => setPhase('gone'), FADE)
    return () => clearTimeout(t)
  }, [phase])

  if (phase === 'gone') return null
  return (
    <div
      className={`splash ${phase === 'out' ? 'is-out' : ''}`}
      onPointerDown={() => setPhase('out')}
      style={{ '--g': `${size}px` } as CSSProperties}
      aria-hidden="true"
    >
      <canvas ref={canvas} className="splash-canvas" />
      <div className={`splash-name ${started ? 'is-on' : ''}`}>
        <span className="splash-latin">Fuxi</span>
        <span className="splash-zh hanzi">复习 · to review</span>
      </div>
    </div>
  )
}
