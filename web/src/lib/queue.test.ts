import { describe, expect, it } from 'vitest'
import { isReady, laterToday } from './queue'

const now = Date.parse('2026-11-28T09:00:00Z')
const at = (mins: number) => new Date(now + mins * 60_000).toISOString()

describe('isReady', () => {
  it.each([
    ['a new card', 'new', at(600), true],
    ['a review card due this evening', 'review', at(540), true],
    ['a learning card whose step is up', 'learning', at(-5), true],
    ['a learning card due within a minute', 'learning', at(0.5), true],
    ['a learning card due in 10 minutes', 'learning', at(10), false],
    ['a relearning card due in an hour', 'relearning', at(60), false],
  ] as const)('%s → %s', (_, state, due_at, want) => {
    expect(isReady({ state, due_at }, now)).toBe(want)
  })
})

describe('laterToday', () => {
  it('counts only learning-step cards not ready yet, per deck', () => {
    const later = laterToday(
      [
        { deck_id: 'a', state: 'learning', due_at: at(10) },
        { deck_id: 'a', state: 'relearning', due_at: at(90) },
        { deck_id: 'a', state: 'review', due_at: at(300) },
        { deck_id: 'b', state: 'learning', due_at: at(-1) },
      ],
      now,
    )
    expect(Object.fromEntries(later)).toEqual({ a: 2 })
  })
})
