// JSON shapes of the Fuxi API (SYSTEM_DESIGN §10). Times are RFC 3339 strings.

export type Rating = 1 | 2 | 3 | 4 // again, hard, good, easy

export interface Me {
  id: string
  email: string
  role: 'member' | 'owner'
  timezone: string
  day_starts_at: number
  settings: { explanation_language: 'en' | 'id'; tone_colours: boolean }
  created_at: string
}

export interface DictEntry {
  id: number
  simplified: string
  traditional: string
  pinyin: string
  definitions: string[]
  hsk_level: number | null // 7 means the combined 7–9 band
}

export interface Example {
  id: number
  source: 'tatoeba' | 'user' | 'ai'
  zh: string
  en: string | null
  max_hsk: number | null
}

export interface DictEntryDetail extends DictEntry {
  pinyin_numbered: string
  examples: Example[]
}

export interface SearchResult {
  results: DictEntry[]
  segmented: boolean
}

export interface DeckSettings {
  new_per_day: number
  card_types: string[]
  desired_retention: number
}

export interface Deck {
  id: string
  name: string
  settings: DeckSettings
  note_count: number
  created_at: string
}

export interface Fields {
  hanzi?: string
  pinyin?: string
  meaning?: string
  sentence?: string
  cloze?: string
  notes?: string
}

export type CardState = 'new' | 'learning' | 'review' | 'relearning'

export interface Card {
  id: string
  type: string
  suspended: boolean
  state: CardState
  due_at: string
  reps: number
  lapses: number
  last_review_at: string | null
}

export interface Note {
  id: string
  deck_id: string
  entry_id: number | null
  kind: 'word' | 'sentence'
  fields: Fields
  tags: string[]
  origin: string
  created_at: string
  cards?: Card[]
}

export interface RatingPreview {
  again: string
  hard: string
  good: string
  easy: string
}

export interface QueueCard {
  card_id: string
  type: 'recognition' | 'recall' | 'tones' | 'cloze'
  state: CardState
  due_at: string
  reps: number
  lapses: number
  deck_id: string
  note_id: string
  kind: 'word' | 'sentence'
  entry_id: number | null
  fields: Fields
  preview: RatingPreview
}

export interface Queue {
  day_start: string
  day_end: string
  due: QueueCard[]
  new: QueueCard[]
  due_count: number
  new_count: number
}

export interface Summary {
  day_start: string
  day_end: string
  today: string
  due_count: number
  new_count: number
  decks: { deck_id: string; due: number; new: number }[]
  days: { date: string; reviews: number }[]
}

export interface ReviewResult {
  card: { card_id: string; state: CardState; due_at: string }
  duplicate: boolean
  replayed: boolean
}

export interface Knowledge {
  entry_id: number
  status: 'known' | 'learning' | 'unknown'
  source: 'placement' | 'manual' | 'review'
}

export interface ImportResult {
  imported: number
  skipped: number // the deck already had that hanzi
  failed: number
  errors: { line: number; message: string }[]
}

export interface Job {
  id: number
  kind: string
  status: 'queued' | 'running' | 'succeeded' | 'failed' | 'dead'
  attempts: number
  error?: string
  result?: ImportResult
}
