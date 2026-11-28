// Shared motion state (system.md motion): reduced motion, the launch splash,
// and one-shot moments such as the home entrance.

export function reducedMotion(): boolean {
  return typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches
}

let drawable: boolean | undefined

/** Whether a 2D canvas works here (not in jsdom, where tests run). */
export function canDraw(): boolean {
  if (drawable === undefined) {
    drawable =
      typeof navigator !== 'undefined' &&
      !/jsdom/i.test(navigator.userAgent) &&
      !!document.createElement('canvas').getContext?.('2d')
  }
  return drawable
}

let endSplash: () => void = () => {}

/** Resolves when the launch splash is gone (at once if it never plays). */
export const splashDone = new Promise<void>((r) => {
  endSplash = r
})

export function finishSplash() {
  document.documentElement.classList.remove('splashing')
  endSplash()
}

const LAST_SEEN = 'fuxi.lastSeen'

/** How long Fuxi must have been out of sight for opening it to count as a fresh start. */
export const SPLASH_AFTER = 30 * 60_000

/**
 * Whether opening the app now is a fresh start: the first visit, or the
 * first after being away for SPLASH_AFTER. (Not "once per browser session":
 * phone browsers restore a tab's session after being closed, so a session
 * can last for days.)
 */
export function splashDue(lastSeen: number | null, now: number): boolean {
  return lastSeen === null || !(now - lastSeen < SPLASH_AFTER)
}

function readLastSeen(): number | null {
  const v = Number(localStorage.getItem(LAST_SEEN))
  return Number.isFinite(v) && v > 0 ? v : null
}

function markSeen() {
  try {
    localStorage.setItem(LAST_SEEN, String(Date.now()))
  } catch {
    // not essential
  }
}

/**
 * The splash plays on a fresh start only (splashDue), never with reduced
 * motion, and only where a canvas can draw it. "Seen" is the last time the
 * app was on screen: recorded now, and whenever it's hidden or closed.
 */
export function shouldPlaySplash(): boolean {
  try {
    const play = splashDue(readLastSeen(), Date.now()) && !reducedMotion() && canDraw()
    markSeen()
    document.addEventListener('visibilitychange', () => {
      if (document.visibilityState === 'hidden') markSeen()
    })
    addEventListener('pagehide', markSeen)
    return play
  } catch {
    return false
  }
}

let homeIntro = true

/**
 * The home entrance plays on the first home view after a launch, not on
 * every visit. Read it while rendering; mark it played from an effect.
 */
export function homeIntroPending(): boolean {
  return homeIntro && !reducedMotion()
}

export function homeIntroPlayed() {
  homeIntro = false
}

const CELEBRATE = 'fuxi.celebrate'

/** A finished session asks home to fill today's dot with a ripple, once. */
export function celebrateToday() {
  try {
    sessionStorage.setItem(CELEBRATE, '1')
  } catch {
    // not essential
  }
}

export function celebrationPending(): boolean {
  try {
    return sessionStorage.getItem(CELEBRATE) === '1' && !reducedMotion()
  } catch {
    return false
  }
}

export function celebrated() {
  try {
    sessionStorage.removeItem(CELEBRATE)
  } catch {
    // not essential
  }
}
