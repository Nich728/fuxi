import { describe, expect, it } from 'vitest'
import { SPLASH_AFTER, splashDue } from './motion'

describe('splashDue', () => {
  const now = Date.parse('2026-11-28T20:00:00Z')

  it.each([
    ['the first visit', null, true],
    ['back after a day', now - 24 * 60 * 60_000, true],
    ['back after exactly 30 minutes', now - SPLASH_AFTER, true],
    ['a reload while studying', now - 5 * 60_000, false],
    ['back after 29 minutes', now - 29 * 60_000, false],
  ])('%s → %s', (_, lastSeen, want) => {
    expect(splashDue(lastSeen, now)).toBe(want)
  })
})
