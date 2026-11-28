import { createContext, useContext } from 'react'
import type { Me } from '../api/types'

export interface Toast {
  id: number
  message: string
  action?: { label: string; run: () => void }
  /** How long it stays, ms. */
  ms: number
}

export interface AppState {
  me: Me
  setMe: (me: Me) => void
  toasts: Toast[]
  /** Shows a short message; with an action (e.g. Undo) it stays a little longer. */
  toast: (message: string, action?: Toast['action']) => void
  dismiss: (id: number) => void
}

export const AppContext = createContext<AppState | null>(null)

export function useApp(): AppState {
  const v = useContext(AppContext)
  if (!v) throw new Error('useApp outside AppProvider')
  return v
}

const LAST_SEARCH = 'fuxi.lastSearch'

/** Where a word page's Back goes: the search that led there. */
export function searchHref(): string {
  try {
    const q = sessionStorage.getItem(LAST_SEARCH)
    return q ? '/search?q=' + encodeURIComponent(q) : '/search'
  } catch {
    return '/search'
  }
}

export function rememberSearch(q: string) {
  try {
    sessionStorage.setItem(LAST_SEARCH, q)
  } catch {
    // not essential
  }
}

const LAST_DECK = 'fuxi.lastDeck'

/** The deck a word was last added to, offered first next time (PRD flow 7.3). */
export function lastDeck(): string | null {
  try {
    return localStorage.getItem(LAST_DECK)
  } catch {
    return null
  }
}

export function rememberDeck(id: string) {
  try {
    localStorage.setItem(LAST_DECK, id)
  } catch {
    // not essential
  }
}
