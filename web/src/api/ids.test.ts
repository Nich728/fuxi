import { describe, expect, it } from 'vitest'
import { deviceName, uuidv7 } from './ids'

describe('uuidv7', () => {
  it('is a version 7, variant 10 UUID that starts with the time', () => {
    const t = Date.UTC(2026, 10, 24, 9, 0, 0)
    const id = uuidv7(t)
    expect(id).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/)
    expect(parseInt(id.replace(/-/g, '').slice(0, 12), 16)).toBe(t)
  })
  it('sorts by creation time', () => {
    const a = uuidv7(1_000)
    const b = uuidv7(2_000)
    expect(a < b).toBe(true)
  })
  it('is unique', () => {
    const ids = new Set(Array.from({ length: 1000 }, () => uuidv7(5)))
    expect(ids.size).toBe(1000)
  })
})

describe('deviceName', () => {
  it.each([
    ['Mozilla/5.0 (iPhone; CPU iPhone OS 19_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/19.0 Mobile/15E148 Safari/604.1', 'iPhone · Safari'],
    ['Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/142.0.0.0 Safari/537.36 Edg/142.0.0.0', 'Windows · Edge'],
    ['Mozilla/5.0 (Linux; Android 16; Pixel 9) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/142.0.0.0 Mobile Safari/537.36', 'Android · Chrome'],
  ])('%s', (ua, want) => expect(deviceName(ua)).toBe(want))
})
