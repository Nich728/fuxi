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

const SPLASH_SEEN = 'fuxi.splashSeen'

/**
 * The splash plays on a cold start only: once per browser session (an
 * installed app's launch), never with reduced motion, and only where a
 * canvas can draw it.
 */
export function shouldPlaySplash(): boolean {
  try {
    if (reducedMotion() || sessionStorage.getItem(SPLASH_SEEN) || !canDraw()) return false
    sessionStorage.setItem(SPLASH_SEEN, '1')
    return true
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
