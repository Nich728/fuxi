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

/** Plural helper for plain English counts: plural(3, 'card') → "3 cards". */
export function plural(n: number, word: string, many = word + 's'): string {
  return `${n} ${n === 1 ? word : many}`
}
