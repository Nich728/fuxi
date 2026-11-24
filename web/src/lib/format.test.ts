import { describe, expect, it } from 'vitest'
import { cardMeaning, formatInterval, hskLabel, plural, simplifiedExamples, spreadSiblings, weekdayGlyph } from './format'

describe('formatInterval', () => {
  const from = new Date('2026-11-24T09:00:00Z')
  const at = (ms: number) => new Date(from.getTime() + ms)
  const min = 60_000
  const day = 24 * 60 * min
  it.each([
    [20_000, '<1m'],
    [59_000, '1m'],
    [10 * min, '10m'],
    [90 * min, '2h'],
    [day, '1d'],
    [25 * day, '25d'],
    [45 * day, '1mo'],
    [300 * day, '10mo'],
    [400 * day, '1.1y'],
    [730.5 * day, '2y'],
  ])('%d ms → %s', (ms, want) => {
    expect(formatInterval(at(ms), from)).toBe(want)
  })
})

describe('labels', () => {
  it('names HSK bands, with 7–9 as one', () => {
    expect(hskLabel(3)).toBe('HSK 3')
    expect(hskLabel(7)).toBe('HSK 7–9')
    expect(hskLabel(null)).toBeNull()
  })
  it('labels weekdays from Monday 一 to Sunday 日', () => {
    expect(weekdayGlyph('2026-11-23')).toBe('一') // a Monday
    expect(weekdayGlyph('2026-11-29')).toBe('日')
  })
  it('pluralises', () => {
    expect(plural(1, 'card')).toBe('1 card')
    expect(plural(0, 'card')).toBe('0 cards')
  })
})

describe('simplifiedExamples', () => {
  it('keeps sentences written with the simplified word', () => {
    const ex = [{ zh: '查詞典。' }, { zh: '我有词典。' }]
    expect(simplifiedExamples(ex, '词典')).toEqual([{ zh: '我有词典。' }])
  })
})

describe('cardMeaning', () => {
  it('drops classifier notes', () => {
    expect(cardMeaning('dictionary; CL:本[ben3],部[bu4]')).toBe('dictionary')
    expect(cardMeaning('CL:个[ge4]')).toBe('CL:个[ge4]') // nothing else to show
    expect(cardMeaning(undefined)).toBe('')
  })
})

describe('spreadSiblings', () => {
  it('never puts two cards of one note next to each other', () => {
    const c = (id: string, note: string) => ({ id, note_id: note })
    const out = spreadSiblings([c('a1', 'a'), c('a2', 'a'), c('b1', 'b'), c('b2', 'b'), c('c1', 'c')])
    expect(out.map((x) => x.id)).toEqual(['a1', 'b1', 'c1', 'a2', 'b2'])
  })
})
