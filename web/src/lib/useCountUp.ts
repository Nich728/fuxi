import { useEffect, useState } from 'react'
import { reducedMotion, splashDone } from './motion'

/** Counts up from 0 to `to` once the launch splash is out of the way (700ms, ease-out). */
export function useCountUp(to: number, enabled: boolean): number {
  const animate = enabled && !reducedMotion()
  const [n, setN] = useState(animate ? 0 : to)
  useEffect(() => {
    if (!animate) return
    let raf = 0
    let live = true
    splashDone.then(() => {
      if (!live) return
      const t0 = performance.now()
      const step = (t: number) => {
        const p = Math.min(1, (t - t0) / 700)
        setN(Math.round(to * (1 - (1 - p) ** 3)))
        if (p < 1) raf = requestAnimationFrame(step)
      }
      raf = requestAnimationFrame(step)
    })
    return () => {
      live = false
      cancelAnimationFrame(raf)
    }
  }, [to, animate])
  return animate ? n : to
}
