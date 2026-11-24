import { useEffect, useRef } from 'react'
import { loadGlyphFont, playDotGlyph } from '../lib/dotGlyph'
import { canDraw, reducedMotion } from '../lib/motion'

const SIZE = 132
/** The canvas reaches one gutter (index.css --gutter) past the text column on each side. */
const GUTTER = 20

/**
 * A character drawn out of dots, then settled solid: the end of a session
 * gathers into 毕 (system.md signature elements). Without motion, or
 * without a canvas, it is simply the character.
 */
export function DotSeal({ glyph }: { glyph: string }) {
  const canvas = useRef<HTMLCanvasElement>(null)
  const animate = canDraw() && !reducedMotion()

  useEffect(() => {
    if (!animate) return
    let stop = () => {}
    let live = true
    loadGlyphFont(glyph, SIZE).then(() => {
      const c = canvas.current
      if (!live || !c) return
      stop = playDotGlyph(c, {
        glyph,
        size: SIZE,
        // Left-aligned with the text below; the canvas spans the screen so flying dots aren't cut off.
        cx: GUTTER + SIZE / 2,
        cy: c.clientHeight / 2,
        colour: getComputedStyle(c).color,
        stagger: 450,
        flight: 850,
        solidAt: 1050,
        solidFor: 450,
        spread: 1.1,
      })
    })
    return () => {
      live = false
      stop()
    }
  }, [glyph, animate])

  return animate ? (
    <canvas ref={canvas} className="dot-seal" aria-hidden="true" />
  ) : (
    <span className="dot-seal hanzi" aria-hidden="true">
      {glyph}
    </span>
  )
}
