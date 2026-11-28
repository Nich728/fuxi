import { render } from '@testing-library/react'
import type { ReactNode } from 'react'
import { vi } from 'vitest'
import { Router } from 'wouter'
import { memoryLocation } from 'wouter/memory-location'
import type { Me } from '../api/types'
import { AppProvider } from '../app/context'
import { Toasts } from '../components/ui'

export const me: Me = {
  id: '0193a000-0000-7000-8000-000000000001',
  email: 'me@example.com',
  role: 'owner',
  timezone: 'Asia/Jakarta',
  day_starts_at: 4,
  settings: { explanation_language: 'en', tone_colours: false },
  created_at: '2026-11-01T00:00:00Z',
}

export interface Call {
  method: string
  path: string
  body: unknown
}

type Handler = (call: Call) => unknown

/**
 * Replaces fetch with routes keyed "METHOD /path" (path without /v1 or query;
 * :name matches one segment). A handler's return value is sent as JSON; a
 * Response is sent as is. Unknown routes are 404s.
 */
export function fakeApi(routes: Record<string, Handler>) {
  const calls: Call[] = []
  const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = new URL(String(input), 'http://fuxi.test')
    const method = init?.method ?? 'GET'
    const path = url.pathname.replace(/^\/v1/, '')
    const body = typeof init?.body === 'string' ? JSON.parse(init.body) : init?.body
    const call = { method, path: path + url.search, body }
    calls.push(call)
    const key = Object.keys(routes).find((k) => {
      const [m, p] = k.split(' ')
      return m === method && new RegExp('^' + p.replace(/:\w+/g, '[^/]+') + '$').test(path)
    })
    if (!key) return new Response(JSON.stringify({ title: 'not found' }), { status: 404 })
    const out = routes[key](call)
    if (out instanceof Response) return out
    return new Response(JSON.stringify(out ?? {}), { status: 200, headers: { 'Content-Type': 'application/json' } })
  })
  vi.stubGlobal('fetch', fetchMock)
  return { calls }
}

export function renderAt(path: string, ui: ReactNode) {
  const [p, search = ''] = path.split('?')
  const loc = memoryLocation({ path: p, searchPath: search, record: true })
  const result = render(
    <Router hook={loc.hook} searchHook={loc.searchHook}>
      <AppProvider me={me}>
        {ui}
        <Toasts />
      </AppProvider>
    </Router>,
  )
  return { ...result, history: loc.history }
}
