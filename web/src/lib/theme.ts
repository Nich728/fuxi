// The theme is a per-device choice (system.md colour): Ink by default, Paper,
// or whatever the device prefers. index.html applies it before first paint.

export type Theme = 'ink' | 'paper' | 'auto'

const KEY = 'fuxi.theme'

export function getTheme(): Theme {
  try {
    const t = localStorage.getItem(KEY)
    return t === 'paper' || t === 'auto' ? t : 'ink'
  } catch {
    return 'ink'
  }
}

export function setTheme(t: Theme) {
  try {
    localStorage.setItem(KEY, t)
  } catch {
    // still applied for this visit
  }
  applyTheme(t)
}

export function applyTheme(t: Theme) {
  const root = document.documentElement
  root.dataset.theme = t
  // The status bar and browser chrome follow the page colour.
  const paper = getComputedStyle(root).getPropertyValue('--paper').trim()
  document.querySelector('meta[name="theme-color"]')?.setAttribute('content', paper)
}
