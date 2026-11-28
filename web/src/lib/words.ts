import { api } from '../api/client'
import type { Deck, DictEntryDetail, Note } from '../api/types'

export interface WordData {
  entry: DictEntryDetail
  notes: Note[]
  known: boolean
  decks: Deck[]
}

function fetchWord(id: number): Promise<WordData> {
  return Promise.all([api.entry(id), api.notesForEntry(id), api.knowledge([id]), api.decks()]).then(
    ([entry, notes, knowledge, decks]) => ({ entry, notes, known: knowledge.some((k) => k.status === 'known'), decks }),
  )
}

// Words loaded ahead of opening, so the page slides in whole (system.md motion).
const prefetched = new Map<number, Promise<WordData>>()

/** Starts loading a word page; Search calls it as a result is opened. */
export function prefetchWord(id: number): Promise<WordData> {
  let p = prefetched.get(id)
  if (!p) {
    p = fetchWord(id)
    prefetched.set(id, p)
    p.catch(() => prefetched.delete(id))
  }
  return p
}

export function takeWord(id: number): Promise<WordData> {
  const p = prefetched.get(id)
  prefetched.delete(id)
  return p ?? fetchWord(id)
}
