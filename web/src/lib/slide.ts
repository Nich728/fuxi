import { flushSync } from 'react-dom'
import { reducedMotion } from './motion'

/**
 * Navigates with the page sliding in from the right (or back out, with
 * `back`) using View Transitions where the browser has them (system.md
 * motion). `before` can delay the switch briefly, e.g. until a word has
 * loaded, so the page arrives whole.
 */
export async function slideTo(
  navigate: (to: string) => void,
  href: string,
  back = false,
  before?: () => Promise<unknown>,
) {
  if (before) {
    // Wait a moment for the data, never long: a slow network shows the loading state instead.
    await Promise.race([before().catch(() => {}), new Promise((r) => setTimeout(r, 300))])
  }
  const doc = document as Document & { startViewTransition?: (cb: () => void) => { finished: Promise<void> } }
  if (!doc.startViewTransition || reducedMotion()) {
    navigate(href)
    return
  }
  const root = document.documentElement
  root.classList.toggle('vt-back', back)
  const t = doc.startViewTransition(() => flushSync(() => navigate(href)))
  t.finished.finally(() => root.classList.remove('vt-back'))
}
