/**
 * The interval shown under a rating button: how long from `from` until `due`.
 * Short and unambiguous: "<1m", "10m", "3h", "4d", "3mo", "1.2y".
 */
export function formatInterval(due: Date, from: Date): string {
  const mins = (due.getTime() - from.getTime()) / 60_000
  // Rounded, not truncated: a 1-minute step shown a moment later is still "1m".
  if (mins < 0.5) return '<1m'
  if (mins < 59.5) return `${Math.round(mins)}m`
  const hours = mins / 60
  if (hours < 23.5) return `${Math.round(hours)}h`
  const days = hours / 24
  if (days < 30) return `${Math.round(days)}d`
  const months = days / 30.44
  if (months < 12) return `${Math.round(months)}mo`
  const years = days / 365.25
  return `${years < 10 ? years.toFixed(1).replace(/\.0$/, '') : Math.round(years)}y`
}

/**
 * A meaning for a card face: CC-CEDICT's classifier notes ("CL:本[ben3]")
 * and "variant of" pointers are dictionary apparatus, not the meaning.
 */
export function cardMeaning(meaning: string | undefined): string {
  if (!meaning) return ''
  const parts = meaning.split(/;\s*/).filter((p) => p && !/^CL:/.test(p))
  return (parts.length ? parts : [meaning]).join('; ')
}

/**
 * Orders a session so two cards of the same note (recognition and recall of
 * one word) are never back to back: the second would give the first away.
 * Later siblings move after the cards of other notes, keeping relative order.
 */
export function spreadSiblings<T extends { note_id: string }>(cards: T[]): T[] {
  const seen = new Set<string>()
  const first: T[] = []
  const later: T[] = []
  for (const c of cards) {
    ;(seen.has(c.note_id) ? later : first).push(c)
    seen.add(c.note_id)
  }
  return [...first, ...later]
}

/**
 * Example sentences written in the same script as the word. Tatoeba mixes
 * simplified and traditional; a learner of 词典 shouldn't be shown 查詞典.
 */
export function simplifiedExamples<T extends { zh: string }>(examples: T[], simplified: string): T[] {
  return examples.filter((e) => e.zh.includes(simplified))
}

/**
 * The example shown on a review card: the first simplified-script sentence
 * that has an English translation, so the learner can check the meaning;
 * an untranslated one only if none has a translation.
 */
export function reviewExample<T extends { zh: string; en?: string | null }>(examples: T[], simplified: string): T | undefined {
  const usable = simplifiedExamples(examples, simplified)
  return usable.find((e) => e.en) ?? usable[0]
}

/** "HSK 3", or "HSK 7–9" for the combined band. */
export function hskLabel(level: number | null | undefined): string | null {
  if (level == null) return null
  return level >= 7 ? 'HSK 7–9' : `HSK ${level}`
}

/** Chinese weekday names for the seal week, Monday 一 to Sunday 日. */
export function weekdayGlyph(isoDate: string): string {
  const [y, m, d] = isoDate.split('-').map(Number)
  const dow = new Date(Date.UTC(y, m - 1, d)).getUTCDay() // 0 = Sunday
  return '日一二三四五六'[dow]
}

const DIGITS = '一二三四五六七八九'

/** 1–31 in Chinese numerals: 5 → 五, 11 → 十一, 25 → 二十五. */
function chineseNumber(n: number): string {
  if (n < 10) return DIGITS[n - 1]
  const tens = Math.floor(n / 10)
  const ones = n % 10
  return (tens > 1 ? DIGITS[tens - 1] : '') + '十' + (ones ? DIGITS[ones - 1] : '')
}

/** The home date line: "十一月二十五日 · 星期三" for 2026-11-25. */
export function chineseDate(isoDate: string): string {
  const [, m, d] = isoDate.split('-').map(Number)
  return `${chineseNumber(m)}月${chineseNumber(d)}日 · 星期${weekdayGlyph(isoDate)}`
}

/**
 * A rough session length for the home summary: about 8 seconds a review and
 * 20 a new word, rounded up to whole minutes.
 */
export function sessionMinutes(due: number, fresh: number): number {
  return Math.max(1, Math.ceil((due * 8 + fresh * 20) / 60))
}

/** Plural helper for plain English counts: plural(3, 'card') → "3 cards". */
export function plural(n: number, word: string, many = word + 's'): string {
  return `${n} ${n === 1 ? word : many}`
}
