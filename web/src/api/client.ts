import type {
  Deck,
  DeckSettings,
  DictEntryDetail,
  Fields,
  Job,
  Knowledge,
  Me,
  Note,
  Queue,
  Rating,
  ReviewResult,
  SearchResult,
  Summary,
} from './types'
import { splashDone } from '../lib/motion'

/** An RFC 9457 problem returned by the API. */
export class ApiError extends Error {
  readonly status: number
  constructor(status: number, detail: string) {
    super(detail)
    this.status = status
  }
}

/**
 * Where to send the browser when Cloudflare Access needs the user to sign in
 * again. The service worker never answers /sign-in from its cache, so the
 * request reaches Cloudflare, which shows its login and then returns here;
 * the app then goes back to `next` (see App.tsx).
 */
export function signInUrl(next = location.pathname + location.search): string {
  return '/sign-in?next=' + encodeURIComponent(next)
}

let redirecting = false

function needSignIn(): never {
  if (!redirecting) {
    redirecting = true
    // The installed app opens from its cache and can start the launch
    // animation before it learns the session has expired: let the launch
    // finish, then go to sign in (at once if no launch is playing).
    const to = signInUrl()
    splashDone.then(() => location.assign(to))
  }
  throw new ApiError(401, 'Signing you in again…')
}

async function request<T>(method: string, path: string, body?: unknown, raw?: BodyInit): Promise<T> {
  let res: Response
  try {
    res = await fetch('/v1' + path, {
      method,
      headers: body !== undefined ? { 'Content-Type': 'application/json' } : undefined,
      body: raw ?? (body !== undefined ? JSON.stringify(body) : undefined),
      credentials: 'same-origin',
      // An expired Access session answers with a redirect to Cloudflare's
      // login, which fetch can't follow across origins. Seeing it here lets
      // the app reload through /sign-in instead of failing with a CORS error.
      redirect: 'manual',
    })
  } catch {
    throw new ApiError(0, "Can't reach Fuxi. Check your connection and try again.")
  }
  if (res.type === 'opaqueredirect' || res.status === 401) {
    needSignIn()
  }
  if (!res.ok) {
    let detail = ''
    try {
      const p = (await res.json()) as { detail?: string; title?: string }
      detail = p.detail || p.title || ''
    } catch {
      // not JSON
    }
    throw new ApiError(res.status, detail || `Something went wrong (${res.status}).`)
  }
  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}

const get = <T>(path: string) => request<T>('GET', path)

export const api = {
  me: () => get<Me>('/me'),
  updateSettings: (patch: Partial<Me['settings']> & { timezone?: string; day_starts_at?: number }) =>
    request<Me>('PATCH', '/me/settings', patch),
  registerDevice: (id: string, name: string) => request<unknown>('PUT', `/devices/${id}`, { name }),

  search: (q: string) => get<SearchResult>('/dict/search?q=' + encodeURIComponent(q)),
  entry: (id: number) => get<DictEntryDetail>(`/dict/entries/${id}`),

  decks: () => get<{ decks: Deck[] }>('/decks').then((r) => r.decks),
  deck: (id: string) => get<Deck>(`/decks/${id}`),
  createDeck: (id: string, name: string, settings?: Partial<DeckSettings>) =>
    request<Deck>('POST', '/decks', { id, name, settings }),
  updateDeck: (id: string, patch: { name?: string; settings?: Partial<DeckSettings> }) =>
    request<Deck>('PATCH', `/decks/${id}`, patch),
  deleteDeck: (id: string) => request<void>('DELETE', `/decks/${id}`),

  notes: (deckId: string, after?: string) =>
    get<{ notes: Note[]; next_cursor?: string }>(`/decks/${deckId}/notes?limit=100${after ? '&after=' + after : ''}`),
  notesForEntry: (entryId: number) => get<{ notes: Note[] }>(`/notes?entry_id=${entryId}`).then((r) => r.notes),
  note: (id: string) => get<Note>(`/notes/${id}`),
  createNote: (deckId: string, note: { id: string; entry_id?: number; fields?: Fields; origin?: string }) =>
    request<Note>('POST', `/decks/${deckId}/notes`, note),
  updateNote: (id: string, fields: Partial<Record<keyof Fields, string>>) =>
    request<Note>('PATCH', `/notes/${id}`, { fields }),
  deleteNote: (id: string) => request<void>('DELETE', `/notes/${id}`),
  setSuspended: (cardId: string, suspended: boolean) => request<unknown>('PATCH', `/cards/${cardId}`, { suspended }),

  importCSV: (deckId: string, importId: string, file: Blob) =>
    request<{ job_id: number }>('POST', `/decks/${deckId}/import?import_id=${importId}`, undefined, file),
  job: (id: number) => get<Job>(`/jobs/${id}`),

  queue: (deckId?: string) => get<Queue>('/study/queue' + (deckId ? '?deck=' + deckId : '')),
  summary: () => get<Summary>('/study/summary'),
  review: (r: { id: string; card_id: string; rating: Rating; reviewed_at: string; duration_ms: number; device_id: string }) =>
    request<ReviewResult>('POST', '/reviews', r),

  knowledge: (entryIds: number[]) =>
    get<{ knowledge: Knowledge[] }>('/knowledge?entry_ids=' + entryIds.join(',')).then((r) => r.knowledge),
  markKnown: (entryId: number) =>
    request<{ knowledge: Knowledge; cards_suspended: number }>('PUT', `/knowledge/${entryId}`, { status: 'known' }),
  unmarkKnown: (entryId: number) => request<{ cards_restored: number }>('DELETE', `/knowledge/${entryId}`),
}
