import { afterEach, describe, expect, it, vi } from 'vitest'
import { api, ApiError, signInUrl } from './client'

afterEach(() => vi.unstubAllGlobals())

describe('api client', () => {
  it('turns problem+json into an ApiError with its detail', async () => {
    const problem = { title: 'Unprocessable', detail: 'name must be 1–100 characters' }
    vi.stubGlobal('fetch', vi.fn(async () => new Response(JSON.stringify(problem), { status: 422 })))
    const err = await api.createDeck('x', '').catch((e: unknown) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect((err as ApiError).status).toBe(422)
    expect((err as ApiError).message).toBe('name must be 1–100 characters')
  })

  it('reports a network failure plainly', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => {
        throw new TypeError('Failed to fetch')
      }),
    )
    await expect(api.me()).rejects.toThrow('reach Fuxi')
  })

  it('sends the browser through /sign-in when the Access session has expired', async () => {
    const assign = vi.fn()
    vi.stubGlobal('location', { pathname: '/decks/d1', search: '?x=1', assign })
    const redirect = { type: 'opaqueredirect', status: 0, ok: false } as Response
    vi.stubGlobal('fetch', vi.fn(async () => redirect))
    await expect(api.decks()).rejects.toThrow()
    expect(assign).toHaveBeenCalledWith('/sign-in?next=%2Fdecks%2Fd1%3Fx%3D1')
  })

  it('builds sign-in URLs that return to the page', () => {
    expect(signInUrl('/word/12')).toBe('/sign-in?next=%2Fword%2F12')
  })
})
