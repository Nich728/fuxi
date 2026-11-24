# Fuxi design system

"Ink and signal": a dark, quiet page, very large type, and one bright jade signal. Decided on 25 November 2026, replacing the first "scholar's desk" direction (rounded silk cards, seals), which read as generic. Treat these as decided; change them here first, then in code.

## Intent

- **Who:** an adult learning Mandarin in a structured course. Short sessions on a phone, longer ones on a laptop at home, often late in the evening.
- **What they do:** clear today's reviews, look a word up, add words to decks.
- **Feel:** modern editorial. Confident, calm and sharp. One loud thing per screen, everything else steps back. Not a game, not a chatbot, not a dashboard.

## Colour

Tokens are named for materials. Ink is the default theme; Paper is the light theme (Settings → Theme: Ink, Paper or Match device, kept per device).

| Token | Ink (default) | Paper | Use |
|---|---|---|---|
| `--paper` | `#151615` | `#EEEAE1` | Page background |
| `--silk` | `#1D1F1C` | `#F7F4EE` | Raised surfaces: sheets, secondary buttons |
| `--stack` | `#262824` | `#DEDAD0` | Empty dots, wells, pressed rows |
| `--rule` | `#2C2E2B` | `#D6D1C4` | Hairlines, only where a list needs them |
| `--ink` | `#ECE8DF` | `#161715` | Primary text |
| `--ink-mid` | `#B9B5AB` | `#4A4842` | Serif reading text (meanings, summaries) |
| `--ink-soft` | `#8C8A83` | `#6B6860` | Labels and metadata (4.5:1 on paper) |
| `--jade` | `#3FD98A` | `#187A4A` | The signal: the one action, due counts, done days |
| `--on-jade` | `#0B2A1A` | `#F7F4EE` | Text on jade |
| `--gold` | `#C9A24A` | `#80631F` | HSK level only |
| `--cinnabar` | `#E4583E` | `#B23A26` | The Again rating only |
| `--tone-1` … `--tone-4` | `#F08A70` `#D9B45C` `#5FD99A` `#8DB4F0` | `#B23A26` `#8A6A1F` `#187A4A` `#3D5A8A` | Tone colours (Settings, off by default) |

Rules:
- **Jade is a signal, not a theme colour.** One filled jade button per screen; elsewhere jade marks only a count that needs you or a day that's done. If two things on a screen are jade, one of them is wrong.
- Cinnabar means Again and nothing else. Destructive confirms (delete) are ink-filled.
- No gradients, no glass, no glow, no shadows.

## Type

- **Display:** Plus Jakarta Sans 800, tight tracking (−0.04 to −0.05em). The due count on home is 96px; page titles 34px; deck names in lists 20px at 700.
- **Hanzi:** Noto Serif SC 600. Review card 88–104px, word page 72px, list rows 26px. Example sentences 18px at 400.
- **Reading text:** Noto Serif 400 for meanings, summaries and example translations ("the voice").
- **Pinyin:** Noto Serif italic, in `--ink-mid` (tone colours when the setting is on).
- **Labels:** 11px, 600, uppercase through CSS, 0.14em tracking, `--ink-soft`. The source text stays in sentence case.
- **Interface:** Plus Jakarta Sans 400–700. Body 15px, small 13px, minimum 11px.
- Numbers that change use tabular figures.

## Shape and depth

- Radius: 6px for buttons and small surfaces, 14px for the top of sheets. Dots are circles. Nothing else is rounded.
- No borders around content. Hierarchy comes from size, weight and space; hairlines (`--rule`) only separate rows in long lists.
- Inputs are underlined fields, not boxes.
- Touch targets at least 44px; primary buttons 52px tall.

## Signature elements

- **One loud thing per screen:** the due count on home, the hanzi on a card, the hanzi on a word page, the total on progress.
- **The dot grid:** the brand motif. The review screen sits on a faint dot grid; the launch and the end of a session draw a character out of dots.
- **Week dots:** Monday to Sunday as seven dots with 一 to 日 under them. Done = jade, today = jade ring, missed = `--stack`, still to come = faint `--stack`.
- **Chinese tab bar:** 今日 · 词库 · 查 · 进度, with small English labels under them.

## Motion

Motion follows how often you see it. Everything respects `prefers-reduced-motion`.

- **Launch (cold start only, once per browser session):** dots flow in top to bottom, settle into a solid jade 复, "Fuxi · 复习" fades in, then the splash dissolves (about 3.3s). A tap skips it. Skipped entirely with reduced motion.
- **Home (first view per launch):** lines rise from a mask with a short stagger (600ms, `cubic-bezier(.22,1,.36,1)`), the due count rolls up (700ms), week dots pop in one after another.
- **Review (hundreds of times a day):** kept fast. The answer fades up in 200ms; a new card slides in from the right in 220ms. No exit animation, so keyboard rating never waits.
- **Rating:** the pressed button dips, the progress line grows (300ms) and the counter rolls to the next number.
- **Session end:** the dot grid gathers into 毕 (about 1.5s).
- **Today done:** back on home after a session, today's ring fills with jade and a ripple spreads once.
- **Tab bar:** a 2px jade line slides under the active tab (280ms).
- **Opening a word:** the word page slides in from the right over the page it came from (280ms); Back reverses it (View Transitions where the browser has them, nothing otherwise).
- **Add to a deck:** a tick draws on the button and the deck's word count rolls up.
- **I already know this:** a line strikes through the link, then "Known" rises in its place.
- **Search results:** the first eight rows fade up with a 30ms stagger, once per new query.
- **Progress:** the total rolls up and each day's bar grows from the left in turn (50ms stagger).
- **Undo toast:** slides up; a thin line under it shrinks for as long as Undo stays.
- **Presses:** `scale(0.97)`, 120ms. Sheets slide up in 260ms.
- **Later:** stroke-order drawing on the word page (needs stroke data and its licence).
- No scroll-triggered reveals: daily-use content is shown at once.

## Layout patterns

- Phone first, 390px wide with 20px side padding. Content is left-aligned.
- **Home:** date in Chinese (十一月二十五日 · 星期三) and settings → "Due today" label → the count → a serif line (reviews, new words, about how long) → Start review → look-up field (the tutor's prompt bar in M4) → "This week" week dots → "Decks" as big type rows with due counts in jade → tab bar.
- **Review:** close, deck label and count, 2px progress line → the hanzi, left-aligned on the dot grid → pinyin and meaning once revealed, then an example → Show answer, then four ratings with their next intervals; only Good is filled, Again's label is cinnabar.
- **Word:** back → hanzi → pinyin and HSK → numbered meanings in serif → Add to a deck (jade) and "I already know this" (a text link) → examples.

## Component patterns

- **Buttons:** primary 52px, jade fill, 700; secondary 44px, `--silk` fill, no border; quiet buttons are text. Press: `scale(0.97)`, 120ms.
- **Rows:** in long lists, hairline-divided, 60px minimum, hanzi first at 26px when the row is a word. Deck rows are big type (20px, 700) with no chevrons.
- **Sheets:** Radix dialog, slide up from the bottom on phones, centred 440px panel from 640px wide. `--silk` surface.
- **Rating buttons:** four columns, 60px tall, label over the interval (11px, tabular); only Good is filled.
- **Example sentences** are shown only when written in the word's simplified form.

## Open items

- Focus states: 2px jade ring, 2px offset; never removed.
- Offline state, the vocabulary check, the tutor conversation.
- App icons still show the earlier cinnabar seal until they're redrawn for the ink theme.
- An installed iPhone app uses a translucent status bar with light text (right for Ink); with Paper the clock and battery are harder to read. Check on a real iPhone in the deploy PR.
