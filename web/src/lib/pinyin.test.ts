import { describe, expect, it } from 'vitest'
import { syllables } from './pinyin'

const tones = (p: string) => syllables(p).filter((s) => s.text.trim()).map((s) => `${s.text}${s.tone}`)

describe('syllables', () => {
  it.each([
    ['nǐ hǎo', ['nǐ3', 'hǎo3']],
    ['cídiǎn', ['cí2', 'diǎn3']],
    ['Zhōngguó', ['Zhōng1', 'guó2']],
    ['shēngrì', ['shēng1', 'rì4']],
    ['fānyì', ['fān1', 'yì4']],
    ['xièxie', ['xiè4', 'xie0']],
    ['lǜsè', ['lǜ4', 'sè4']],
    ['nǚ’ér', ['nǚ3', '’0', 'ér2']],
  ])('%s', (input, want) => {
    expect(tones(input)).toEqual(want)
  })

  it('keeps every character, so the text reads the same', () => {
    for (const p of ['nǐ hǎo', 'cídiǎn', 'nǚ’ér, mā!']) {
      expect(syllables(p).map((s) => s.text).join('')).toBe(p)
    }
  })
})
