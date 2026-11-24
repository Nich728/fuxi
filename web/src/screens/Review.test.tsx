import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { QueueCard } from '../api/types'
import { fakeApi, renderAt } from '../test/fakeApi'
import { ReviewScreen } from './Review'

const deck = {
  id: 'd1',
  name: 'HSK 1',
  settings: { new_per_day: 20, card_types: ['recognition', 'recall'], desired_retention: 0.9 },
  note_count: 2,
  created_at: '',
}

function card(id: string, hanzi: string, type: QueueCard['type'] = 'recognition'): QueueCard {
  const now = Date.now()
  const at = (ms: number) => new Date(now + ms).toISOString()
  return {
    card_id: id,
    type,
    state: 'new',
    due_at: at(-1000),
    reps: 0,
    lapses: 0,
    deck_id: 'd1',
    note_id: 'n-' + id,
    kind: 'word',
    entry_id: null,
    fields: { hanzi, pinyin: 'cídiǎn', meaning: 'dictionary' },
    preview: { again: at(60_000), hard: at(6 * 60_000), good: at(10 * 60_000), easy: at(8 * 86_400_000) },
  }
}

const queueOf = (...cards: QueueCard[]) => ({ day_start: '', day_end: '', due: [], new: cards, due_count: 0, new_count: cards.length })
const reviewed = (cardId: string) => ({ card: { card_id: cardId, state: 'learning', due_at: '' }, duplicate: false, replayed: false })

afterEach(() => {
  vi.unstubAllGlobals()
  localStorage.clear()
})

describe('ReviewScreen', () => {
  it('reveals, rates, sends the review and moves on', async () => {
    const { calls } = fakeApi({
      'GET /study/queue': () => queueOf(card('c1', '词典'), card('c2', '你好', 'recall')),
      'GET /decks': () => ({ decks: [deck] }),
      'POST /reviews': (c) => reviewed((c.body as { card_id: string }).card_id),
    })
    const posts = () => calls.filter((c) => c.method === 'POST')
    const user = userEvent.setup()
    renderAt('/review', <ReviewScreen />)

    expect(await screen.findByText('词典')).toBeInTheDocument()
    expect(screen.getByText('HSK 1')).toBeInTheDocument()
    expect(screen.queryByText('dictionary')).not.toBeInTheDocument()
    expect(screen.getByText('2 left')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Show answer' }))
    expect(screen.getByText('dictionary')).toBeInTheDocument()
    // Each rating shows its interval from the server's preview.
    expect(screen.getByRole('button', { name: /Again\s*1m/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Good\s*10m/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Easy\s*8d/ })).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /Good/ }))

    // The recall card asks with the meaning and hides the hanzi until revealed.
    expect(await screen.findByText('1 left')).toBeInTheDocument()
    expect(screen.getByText('dictionary')).toBeInTheDocument()
    expect(screen.queryByText('你好')).not.toBeInTheDocument()

    await waitFor(() => expect(posts()).toHaveLength(1))
    const sent = posts()[0].body as Record<string, unknown>
    expect(sent).toMatchObject({ card_id: 'c1', rating: 3, device_id: localStorage.getItem('fuxi.device') })
    expect(sent.id).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-7/)
    expect(Date.parse(sent.reviewed_at as string)).not.toBeNaN()

    // Keyboard: space reveals, 1 is Again.
    await user.keyboard(' ')
    expect(screen.getByText('你好')).toBeInTheDocument()
    await user.keyboard('1')

    // Both ratings were learning steps due within the session, so they come back.
    expect(await screen.findByText(/2 cards are still being learned/)).toBeInTheDocument()
    expect(screen.getByText('50%')).toBeInTheDocument() // one Again out of two
    await waitFor(() => expect(posts()).toHaveLength(2))
  })

  it('retries a failed send with the same id', async () => {
    let attempts = 0
    const ids = new Set<string>()
    fakeApi({
      'GET /study/queue': () => queueOf(card('c1', '词典')),
      'GET /decks': () => ({ decks: [deck] }),
      'POST /reviews': (c) => {
        ids.add((c.body as { id: string }).id)
        attempts++
        if (attempts === 1) return new Response('{}', { status: 503 })
        return reviewed('c1')
      },
    })
    const user = userEvent.setup()
    renderAt('/review', <ReviewScreen />)
    await user.click(await screen.findByRole('button', { name: 'Show answer' }))
    await user.click(screen.getByRole('button', { name: /Easy/ }))
    expect(await screen.findByText('Done for now')).toBeInTheDocument()
    await waitFor(() => expect(attempts).toBe(2), { timeout: 3000 })
    expect(ids.size).toBe(1)
  })

  it('says when nothing is due', async () => {
    fakeApi({
      'GET /study/queue': () => queueOf(),
      'GET /decks': () => ({ decks: [deck] }),
    })
    renderAt('/review', <ReviewScreen />)
    expect(await screen.findByText('Nothing to review')).toBeInTheDocument()
  })
})
