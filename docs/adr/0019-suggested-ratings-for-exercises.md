# ADR-0019: Let checked answers suggest a rating, and unlock exercises as a word matures

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-11-25 |
| **Related** | SYSTEM_DESIGN §4.3, §6.3, §6.7, §8.10, §8.13; PRD R6, R8–R11, A10–A12, FR-X-1 to FR-X-7; ADR-0003, ADR-0009 |

## Context

Every card so far is self-rated: the learner looks at it, reveals the answer and judges how well they remembered. That works for recognition, but it lets "I'd probably have got that" pass as remembering, and it never asks the learner to produce Chinese. The owner wants reviews that actually test: typing the word, picking its tones, filling it into a sentence, putting a sentence in order, translating, and writing a sentence of their own.

Some of these can be checked exactly against data Fuxi already has (the dictionary entry, a Tatoeba sentence). Translation and free sentences have many right answers and need the AI to judge them, and the AI can be wrong.

Two forces pull against each other. FSRS needs ratings that reflect memory, and the review log is the source of truth (ADR-0003), so whatever reaches it must be trustworthy. But a check is blunt: a typo, a variant character, a valid synonym or a misjudged sentence would turn a remembered word into Again and reset weeks of progress. Exercises also take longer than a self-rated card, and daily sessions must stay short (PRD G2).

## Decision

**A check suggests a rating; the learner confirms it.** A wrong answer suggests Again, a right one Good (Hard if it was slow). AI verdicts map to Again, Hard or Good, never Easy. The learner taps to accept or picks another rating, and only that rating goes to FSRS. The answer and the check's result are stored alongside it in `review_logs.exercise`, which `Replay` never reads.

**Each exercise is its own card type with its own FSRS state** (`tones`, `cloze`, `order`, `translate`, `use`), so producing a word is scheduled apart from recognising it. Typed answers are the exception: they are a way of answering the existing recall card, which already tests the same skill.

**Exercises unlock as a word matures.** A gated card type is introduced only after the note's recognition card reaches an interval of 7 days (tones, cloze, order) or 21 days (translate, use), configurable per deck, and new exercise cards count toward the deck's daily new-card limit.

**AI-checked exercises are optional extras.** They are capped per day (5 by default) on top of the AI budget, need a connection, and fall back to self-rating if grading fails. When unavailable, those cards wait; the rest of the session is unaffected. Their corrections are labelled AI-generated and never become card data (ADR-0009).

## Alternatives considered

- **The check sets the rating automatically.** Faster, and closer to a strict test. Rejected because false negatives (typos, accepted variants, AI misjudgements) would reset cards and corrupt the log the whole schedule is rebuilt from, with no way for the learner to correct it after the fact.
- **Exercises as a separate practice mode outside FSRS.** Simpler to build and keeps the review log untouched. Rejected because practice nobody schedules is practice nobody does, and production is exactly the skill that fades without spaced review.
- **One card per note whose exercise rotates.** Fewer cards. Rejected because recognition and production have very different difficulty; one FSRS state for both would schedule production too rarely and recognition too often.
- **Exercises available from a word's first day.** Rejected because cloze and free sentences about a word you met yesterday mostly produce failures and long sessions; gating keeps them for words you can already recognise.
- **AI grading for every exercise type.** Rejected for typed recall, tones, cloze and order, which have exact answers: the app checks them for free, instantly and offline.

## Consequences

- Ratings keep meaning what the learner says they mean, so FSRS and the replay invariant are unchanged. The price is one extra tap after an exercise, and some leniency: a learner can overrule a correct "wrong". `review_logs.exercise` makes that visible, and M6 can report override rates per card type.
- A note can now have up to seven cards. Gating and the daily new-card limit keep sessions short, but the queue and the summary must count gated cards correctly, and the client must explain why a card type hasn't appeared yet.
- Cloze and sentence-order quality depends on Tatoeba and on the segmenter; sentences that segment badly or sit far above the learner's level need filtering, and the PRD's hide button for bad sentences becomes more important.
- AI grading adds cost, latency and a new eval set (SYSTEM_DESIGN §8.10). Its acceptance bar puts false "wrong" verdicts first, since they push a learner to Again.
- In M5, app-checked exercises work offline because the queue ships their content and expected answers; AI-checked ones remain online-only, like the tutor (ADR-0010).
