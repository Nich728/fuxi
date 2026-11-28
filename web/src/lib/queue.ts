import type { QueueCard } from '../api/types'

/** A card due within this long counts as ready now. */
export const READY_SLACK = 60_000

/**
 * Whether a card from the queue can be reviewed now (SYSTEM_DESIGN §6.3).
 * New cards and review cards are ready all study day: FSRS intervals of a
 * day or more mean "some time today", not a minute of the day. Cards in
 * learning or relearning are on short steps (minutes), so they wait until
 * their step is up.
 */
export function isReady(card: Pick<QueueCard, 'state' | 'due_at'>, now: number): boolean {
  if (card.state === 'new' || card.state === 'review') return true
  return Date.parse(card.due_at) <= now + READY_SLACK
}

/** Learning-step cards from today's queue that aren't ready yet, per deck. */
export function laterToday(due: Pick<QueueCard, 'state' | 'due_at' | 'deck_id'>[], now: number): Map<string, number> {
  const later = new Map<string, number>()
  for (const c of due) {
    if (!isReady(c, now)) later.set(c.deck_id, (later.get(c.deck_id) ?? 0) + 1)
  }
  return later
}
