/**
 * Splits marked pinyin ("nǐ hǎo", "cídiǎn") into syllables with their tone,
 * so Settings › Tone colours can colour each one (PRD C2). Tone marks always
 * stay visible; colour only adds to them.
 */
export interface Syllable {
  text: string
  tone: 0 | 1 | 2 | 3 | 4 // 0 = neutral or not a syllable
}

const TONES: Record<string, 1 | 2 | 3 | 4> = {}
for (const [tone, chars] of [
  [1, 'āēīōūǖĀĒĪŌŪǕ'],
  [2, 'áéíóúǘÁÉÍÓÚǗ'],
  [3, 'ǎěǐǒǔǚǍĚǏǑǓǙ'],
  [4, 'àèìòùǜÀÈÌÒÙǛ'],
] as const) {
  for (const c of chars) TONES[c] = tone
}

// A syllable: optional initial, a vowel group (which carries the mark), an
// optional final n/ng/r. Written words run syllables together ("cídiǎn"),
// so syllables are found by pattern, not by spaces.
const SYLLABLE =
  /(?:zh|ch|sh|[bpmfdtnlgkhjqxrzcsyw])?[aeiouüvāáǎàēéěèīíǐìōóǒòūúǔùǖǘǚǜĀÁǍÀĒÉĚÈĪÍǏÌŌÓǑÒŪÚǓÙǕǗǙǛ]+(?:ng|n(?![aeiouüāáǎàēéěèīíǐìōóǒòūúǔùǖǘǚǜ])|r(?![aeiouüāáǎàēéěèīíǐìōóǒòūúǔùǖǘǚǜ]))?/giu

export function syllables(pinyin: string): Syllable[] {
  const out: Syllable[] = []
  let last = 0
  for (const m of pinyin.matchAll(SYLLABLE)) {
    if (m.index > last) out.push({ text: pinyin.slice(last, m.index), tone: 0 })
    let tone: Syllable['tone'] = 0
    for (const c of m[0]) {
      if (TONES[c]) tone = TONES[c]
    }
    out.push({ text: m[0], tone })
    last = m.index + m[0].length
  }
  if (last < pinyin.length) out.push({ text: pinyin.slice(last), tone: 0 })
  return out
}
