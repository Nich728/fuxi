# Fuxi design system

Decisions so far, from the first mockup (home and review screens, light and dark). Treat these as decided; change them here first, then in code.

## Intent

- **Who:** an adult learning Mandarin in a structured course. Short sessions on a phone, longer ones on a laptop at home, often late in the evening.
- **What they do:** clear today's reviews, ask the tutor about a word, add words to decks.
- **Feel:** a scholar's desk. Rice paper and jade by day, lacquer and ink by night. Calm, unhurried, quietly luxurious. Not a game, not a chatbot.

## Colour

Tokens are named for materials. Light and dark are separate palettes, each checked for contrast on its own.

| Token | Light | Dark | Use |
|---|---|---|---|
| `--paper` | `#ECE3CE` | `#141B17` | Page background |
| `--silk` | `#F7F1E3` | `#1E2823` | Cards, inputs, raised surfaces |
| `--stack` | `#E2D6BA` | `#19221D` | Paper layers behind the next-card stack |
| `--rule` | `#D3C5A2` | `#34423A` | Borders and dividers |
| `--ink` | `#23211C` | `#E8DFCB` | Primary text |
| `--ink-soft` | `#5C574B` | `#A39C8A` | Secondary text (passes 4.5:1 on paper and silk) |
| `--jade` | `#2F6B4F` | `#8FC2A3` | Primary action, pinyin, due counts |
| `--on-jade` | `#F7F1E3` | `#141B17` | Text on jade |
| `--gold` | `#7E6231` | `#C9A86A` | Small metadata (HSK level, deck name on a card) |
| `--cinnabar` | `#A63A2B` | `#C8553F` | Seals and the Again rating only |

Rules:
- One filled jade button per screen.
- Cinnabar is never a general error or warning colour. It means a seal (a finished day) or Again.
- Tone colours (Settings, optional) must be designed against this palette in both modes, and tone marks always stay visible.
- No gradients, no glass, no glow.

## Type

- **Hanzi:** Noto Serif SC, 600 for display. Card face 104px; home preview 64px; example sentences 19px at 400.
- **Pinyin:** Noto Serif, italic, in jade. Chosen for full tone-mark coverage (ǚ, ǜ).
- **Interface:** Plus Jakarta Sans 400/500/600. Body 15px, small 13px, minimum 11px (tab labels only).
- Numbers that change (counts, progress) use tabular figures.
- Sentence case everywhere, no all-caps labels.

## Shape and depth

- Radius: 22px review card, 16px cards, 14px buttons and inputs, 10–12px small controls, 4px seals.
- Depth by borders (1px `--rule`), not shadows. The next-card stack uses slightly rotated paper layers instead.
- Touch targets at least 44px; primary buttons 52–60px tall.

## Signature elements

- **The hanzi is the hero** of every study screen. Nothing competes with it.
- **Seal streak:** a week of square seals labelled with Chinese weekday names (一 to 日). Done = filled cinnabar with the glyph in paper colour; today = dashed cinnabar outline; future = plain rule outline.
- **Next-card stack:** the next card's hanzi on a small stack of paper cards, instead of a big-number stat.

## Layout patterns

- Phone first, 390px wide with 20px side padding.
- Home: header (复习 wordmark, level pill) → tutor prompt bar → next-card stack and Start review → seal week → decks as divided rows (not cards) → bottom tab bar (Home, Decks, Search, Progress).
- Review: close, thin progress bar, count → card (deck and HSK level, hanzi, then pinyin and meaning once revealed, example sentence, Ask about this) → Show answer, then four ratings with their next intervals; only Good is filled.

## Open items

- Pressed and focus states for every control (focus ring must never be removed).
- Offline state for the prompt bar, empty states, the vocabulary check, the tutor conversation.
- Motion: card reveal, rating press, sheet for choosing a deck.
