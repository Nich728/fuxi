# Fuxi — Product Requirements Document

| | |
|---|---|
| **Product** | Fuxi (复习, *fùxí*, "to review") |
| **Owner** | Nicholas Setiono |
| **Status** | v1.8, all open questions decided |
| **Last updated** | 25 November 2026 |
| **Companion doc** | [`SYSTEM_DESIGN.md`](./SYSTEM_DESIGN.md) |

---

## 1. Summary

Fuxi is a Mandarin study app built around two things:

1. **An AI tutor in a prompt bar.** You ask about any word, phrase or grammar point and get a clear explanation with example sentences at your level. Any word it mentions can become a flashcard in one tap.
2. **A spaced-repetition review system.** It schedules each flashcard just before you'd forget it, using the FSRS algorithm, so 15–20 minutes a day keeps thousands of words in memory. Reviews don't only ask "do you remember this?": as a word matures, Fuxi checks you by having you type it, pick its tones, fill it into a sentence, put a sentence in order, and finally use it in a sentence of your own.

A short **vocabulary check** when you first sign up estimates your level, so Fuxi never wastes your time on words you already know and pitches its suggestions just above where you are. It works offline for reviews and syncs across phone and laptop.

Fuxi is also a backend portfolio project. Its value as a portfolio piece comes from the engineering behind the product: a correct scheduling engine, an offline-first sync protocol, and an AI integration that is grounded, safe and cost-controlled.

## 2. Problem

- **Forgetting.** New vocabulary from class fades within days unless it's reviewed at the right moments.
- **Friction when capturing words.** Existing flashcard apps make you type every card by hand. Most learners stop adding words after a week.
- **Questions with no one to ask.** "What's the difference between 知道 and 认识?" comes up at 11pm when no teacher is around. General AI chatbots answer, but the answer is disconnected from what you're studying and sometimes gets tones wrong.
- **Generic examples.** Dictionary example sentences often use words far above the learner's level.
- **Starting from zero every time.** Flashcard apps don't know what you already know. A learner with a year of study either wastes weeks reviewing 你好, or has to delete hundreds of cards by hand.

## 3. Target user

**Primary (v1):** the owner. An adult learning Mandarin in a structured course, with weekly vocabulary, grammar patterns (把, 从…到…, resultative complements, location sentences) and homework. Studies on a phone in short sessions and on a laptop at home. Comfortable with English explanations.

**Secondary (v1, invite-only):** a few friends the owner invites to try it. Their usage must be cheap, private to each of them, and capped so it can't run up the AI bill.

**Later:** other self-directed Mandarin learners at roughly HSK 1–4.

## 4. Goals and non-goals

### Goals
- **G1** Make capturing a new word effortless: under 5 seconds from "what does this mean?" to "it's in my reviews".
- **G2** Keep long-term retention around 90% while minimising daily review time.
- **G3** Answer Mandarin questions accurately, grounded in a real dictionary, with examples pitched at the user's level.
- **G4** Work fully offline for reviewing, and never lose a review.
- **G5** Be a portfolio piece with documented design decisions and measured performance.
- **G6** Know the learner's level from day one (a check of about a minute) and keep that estimate current as they study.

### Non-goals (v1)
- No scraping of any website. Content comes from open datasets, the user, or the AI.
- No native iOS or Android apps. The client is an installable web app (PWA).
- No speech recognition or pronunciation scoring.
- No social features, leaderboards or shared decks.
- No general-purpose chatbot. The tutor stays on Mandarin learning.
- No payments or multi-tenant business features.

## 5. Core concepts (glossary for this doc)

| Term | Meaning |
|---|---|
| **Note** | One thing you're learning: a word, a phrase or a sentence. |
| **Card** | One way of testing a note (meaning, writing, tones, fill-in-the-blank, sentence order, translation, using it in a sentence). One note produces several cards. |
| **Exercise** | A card that checks your answer instead of asking you to judge yourself: typing the word, picking tones, filling a blank, ordering a sentence (checked by the app), or translating and writing your own sentence (checked by the AI). |
| **Deck** | A named collection of notes, such as "Class — Week 6" or "HSK 3". |
| **Inbox** | A holding area for words suggested by the AI or found in pasted text. Nothing reaches your reviews until you accept it. |
| **Due** | A card the scheduler says you should review today. |
| **Review rating** | Your self-assessment after each card: Again, Hard, Good, Easy. After an exercise the app suggests the rating from your answer, and you confirm or change it. |
| **FSRS** | The scheduling algorithm that picks each card's next review date. |
| **Vocabulary check** | A quick quiz at sign-up, about a minute long: a few screens of 5 words where you tap the ones you recognise. It estimates how many words you know at each HSK level. |
| **Known word** | A word the app believes you already know, from the check, an "I already know this" tap, or long-term review success. Known words aren't suggested and don't become cards unless you ask. |
| **Learner level** | The app's running estimate of your level, such as "HSK 2.6, about 650 words". The tutor uses it to pitch explanations and examples. |

## 6. User stories

Priority: **P0** = MVP must-have, **P1** = v1, **P2** = later.

### Account
| ID | Story | Priority |
|---|---|---|
| U1 | As a new user, I can sign in with Google or a code sent to my email; my account is created on first sign-in (ADR-0018). | P0 |
| U2 | I stay signed in on each device without signing in again every day. | P0 |
| U3 | I can install the web app on my phone's home screen, and it opens full-screen like an app. | P0 |
| U4 | When I log in on a new device, all my decks, cards and progress are already there. | P0 |
| U5 | I can see which devices are logged in and log one out remotely. | P1 |
| U6 | ~~I can reset a forgotten password by email.~~ No passwords since ADR-0018. | — |
| U7 | I can export all my data, or delete my account and all its data. | P2 |
| U8 | I can sign in with Google. (Met by U1, ADR-0018.) | P0 |
| U9 | Only people the owner has invited can sign in: the owner adds their email to the Cloudflare Access policy (ADR-0018). | P1 |
| U10 | As the owner, I can invite a friend by adding their email to the Access policy, and see roughly how much AI usage each account has. | P1 |

### Settings
| ID | Story | Priority |
|---|---|---|
| C1 | In Settings, I can choose the language the AI tutor explains in: English (default) or Indonesian. Chinese text and pinyin are always shown as they are. | P1 |
| C2 | Pinyin always shows tone marks (cídiǎn). In Settings, I can also turn on tone colours, one colour per tone. | P1 |

### Vocabulary check and learner level
| ID | Story | Priority |
|---|---|---|
| V1 | After signing up, I can take a vocabulary check of about a minute that estimates my level at each HSK band. | P1 |
| V2 | Each screen shows 5 words and I just tap the ones I recognise, so I can skim through it quickly. The next screen gets harder or easier based on my taps. | P1 |
| V3 | I see my result in plain terms ("HSK 1: ~95%, HSK 2: ~80%, HSK 3: ~35%, about 650 words") with a suggested next step. | P1 |
| V4 | Words I know from the check are marked as known: they're never suggested to me and don't become cards. | P1 |
| V5 | I can choose **Verify later**, which adds known words as cards that first appear weeks from now, just to confirm. | P1 |
| V6 | I can skip the check and set my level by hand. | P1 |
| V7 | I can tap **I already know this** on a word chip, an Inbox item or a new card, and it's marked known without creating or keeping a card. | P1 |
| V8 | My level keeps updating from my reviews and "I already know this" taps, and I can see it on my profile. | P1 |
| V9 | I can retake the check at any time. | P1 |
| V10 | On the result screen, I can tap **Sharpen my estimate** to continue for up to 30 more words if I want a more precise result. | P1 |

### Ask and capture (AI tutor)
| ID | Story | Priority |
|---|---|---|
| A1 | As a learner, I can type a question in a prompt bar on the home screen ("what's the difference between 知道 and 认识?") and get a streamed answer. | P1 |
| A2 | Each Chinese word in an answer is shown as a chip with pinyin and an **Add** button, so I can add it to a deck in one tap. | P1 |
| A3 | Explanations use example sentences built mostly from words I already know, and suggested words sit at my level or one level above (see V8). | P1 |
| A4 | I can paste a paragraph (for example, my homework) and get a list of words I don't have yet, then add the ones I want. | P1 |
| A5 | I can ask "Is this sentence correct?" and get a correction with an explanation. | P1 |
| A6 | While reviewing a card, I can tap **Ask about this** to open the tutor with that word already in context. | P1 |
| A7 | I can scroll back through past conversations. | P1 |
| A8 | I can turn on **auto-capture**, so every word I ask about goes to the Inbox without a tap. | P2 |
| A9 | When the AI writes a word with pinyin that disagrees with the dictionary, the app flags it. | P2 |
| A10 | A "use it" card asks me to write my own sentence with a word I know well. The tutor says whether it's correct and natural, shows a corrected version and explains the difference. | P1 |
| A11 | A translation card gives me an English sentence at my level to write in Chinese, and the tutor accepts any correct translation, not only one fixed answer. | P1 |
| A12 | AI-checked exercises are capped per day (5 by default). Offline or over the cap, they wait for another day and my other reviews carry on. | P1 |

### Look up and organise
| ID | Story | Priority |
|---|---|---|
| L1 | I can search the dictionary by characters, by pinyin with or without tones ("ma" finds 妈, 马, 吗), or by English. | P0 |
| L2 | A word's page shows pinyin, meanings, HSK level and real example sentences. | P0 |
| L3 | I can create, rename and delete decks. | P0 |
| L4 | I can add a word from search to a deck in one tap. | P0 |
| L5 | I can upload a CSV of class vocabulary (characters, pinyin, meaning) and get a note per row. | P0 |
| L6 | I can start a deck from an HSK level list. | P1 |
| L7 | I can review the Inbox and accept or dismiss each suggestion, or accept all. | P1 |
| L8 | I can create fill-in-the-blank (cloze) cards for grammar patterns from example sentences. | P1 |

### Review
| ID | Story | Priority |
|---|---|---|
| R1 | The home screen shows how many cards are due today. | P0 |
| R2 | I review cards one at a time and rate each with Again / Hard / Good / Easy. | P0 |
| R3 | The next review date is chosen automatically by FSRS. | P0 |
| R4 | I can set a daily limit on new cards per deck. | P0 |
| R5 | My study day rolls over at 4am, not midnight, so late-night reviews count for that evening. | P0 |
| R6 | I can choose which card types a deck uses (recognition, recall, tones, cloze, sentence order, translation, use in a sentence). | P1 |
| R7 | I can suspend a card I don't want to see for now. | P1 |
| R8 | On a recall card, I can type the answer (hanzi, or pinyin with tone marks or numbers) and the app tells me whether it's right. | P1 |
| R9 | I can rebuild a real sentence from its shuffled words, to practise word order and grammar patterns. | P1 |
| R10 | After an exercise, the app suggests a rating from my answer (Again if wrong), and I confirm it or pick another. | P1 |
| R11 | Harder exercises unlock as I learn a word: tones, cloze and sentence order once I recognise it, translation and my own sentence once I've known it for a few weeks. My daily sessions don't get much longer. | P1 |

### Offline and sync
| ID | Story | Priority |
|---|---|---|
| S1 | I can review with no internet connection. | P1 |
| S2 | When I reconnect, my phone and laptop end up identical, with no lost or duplicated reviews. | P1 |
| S3 | I can add and edit notes offline. | P1 |
| S4 | The prompt bar shows a clear "available when online" state when offline. | P1 |

### Progress
| ID | Story | Priority |
|---|---|---|
| T1 | I can see my retention rate, reviews per day and streak. | P1 |
| T2 | I can see a forecast of how many reviews are due over the next 7 and 30 days. | P1 |
| T3 | The scheduler adapts to my own memory after enough reviews (personalised FSRS). | P1 |

## 7. Key user flows

### 7.1 First launch
1. Sign in with Google or an emailed code. On a phone, the app offers **Add to Home Screen**.
2. "How long have you been studying Mandarin?" (Never · under 6 months · 6–12 months · 1–2 years · over 2 years). This only sets the starting point of the check.
3. **Vocabulary check**, about a minute:
   ```
   Tap the words you recognise, even if you're not 100% sure
   ┌──────┐ ┌──────┐ ┌──────┐
   │ 认识  │ │ 已经  │ │ 结果  │
   └──────┘ └──────┘ └──────┘
   ┌──────┐ ┌──────┐
   │ 打算  │ │ 环境  │          [ Next → ]
   └──────┘ └──────┘
   ```
   - Each screen shows 5 words. Untapped means "don't know".
   - The next screen is harder if you knew most of the words, and easier if you knew few.
   - After some screens, one tapped word gets a quick follow-up: "Which meaning?" with four choices.
   - Occasionally a made-up word built from real characters is mixed in. Tapping it lowers the estimate.
   - Up to 6 screens (30 words). A progress bar shows how much is left, and **Finish now** is always available.
4. **Result:** per-band percentages, an estimated vocabulary size, and a suggested starter deck, such as "HSK 3 words you don't know yet, most common first". A **Sharpen my estimate** button continues for up to 30 more words.
5. Choose what to do with the words marked known: **Mark as known** (default) or **Verify later**.
6. Land on the home screen.

Skipping is allowed at step 2 or 3. You then pick a level by hand, and the app refines it from your reviews.

### 7.2 Home screen
```
┌───────────────────────────────────────────┐
│  Ask anything about Mandarin…         [→] │  ← prompt bar
├───────────────────────────────────────────┤
│  Today: 23 due · 5 new        [ Start ]   │
│  Inbox: 4 suggestions          [ Review ] │
├───────────────────────────────────────────┤
│  Decks                                    │
│   Class — Week 6        12 due            │
│   HSK 3                  9 due            │
│   Grammar patterns       2 due            │
└───────────────────────────────────────────┘
```

### 7.3 Ask, then capture
1. Type "difference between 知道 and 认识".
2. The answer streams in, with chips: `[知道 zhīdào +]` `[认识 rènshi +]`.
3. Tap **+** on 认识 and choose a deck (it defaults to the last one used).
4. The card fields come from the dictionary, not the AI's text. Cards appear in tomorrow's new-card queue.

### 7.4 Paste homework, then extract
1. Paste a paragraph and ask "which words should I learn?"
2. Fuxi lists words that aren't already in your decks and aren't known to you, each with pinyin, meaning and HSK level, pre-ticked by relevance. Each row has an **I already know this** button.
3. **Add selected** sends them to the chosen deck. **Later** sends them to the Inbox.

### 7.5 Daily review
1. Tap **Start**. A card shows 词典.
2. Think, tap to reveal *cídiǎn · dictionary*, then rate **Good**.
3. Repeat until done. A summary shows cards reviewed, accuracy and time spent.
4. Stuck on a card? **Ask about this** opens the tutor with the word preloaded.
5. Some cards are exercises. For 学习 once it's familiar: a sentence from Tatoeba with a gap (我在___中文), you type 学习, the app marks it right and suggests **Good**, you tap to confirm. Weeks later a "use it" card asks for your own sentence with 学习; the tutor corrects 我学习中文在家 to 我在家学习中文, explains the word order, and suggests **Hard**.

## 8. Functional requirements

### 8.1 Accounts
- **FR-U-1** Every deck, note, card, review, conversation and known-word record belongs to exactly one user. No request can read or change another user's data.
- **FR-U-2** Logging out on one device does not log out the others. Signing a device out remotely is done in the Cloudflare Zero Trust dashboard (ADR-0018).
- **FR-U-3** Reviewing offline works without contacting the server, even after the access token has expired. The device re-authenticates when it reconnects.
- **FR-U-4** Sign-in is closed by default: only emails on the owner's Cloudflare Access policy can sign in (ADR-0018, replacing invite codes).

### 8.2 Vocabulary check and learner level
- **FR-V-1** The check shows at most 30 words (6 screens of 5), about a minute. **Sharpen my estimate** extends it to at most 60 words. **Finish now** produces a result from the screens answered so far.
- **FR-V-2** It adapts: after each screen it picks the next screen's difficulty from the answers so far.
- **FR-V-3** Honesty checks: about 1 in 3 tapped words gets a four-choice meaning question (at most one per screen), and about one made-up word appears every other screen. The result is corrected for over-claiming.
- **FR-V-4** The result reports an estimated share of known words for each HSK band, a total vocabulary estimate, and an overall level.
- **FR-V-5** A word counts as **known** if (a) it was tapped in the check and not contradicted by a failed meaning question, (b) the user tapped "I already know this", (c) its recognition card has reached a long interval, or (d) it belongs to a band the user almost entirely knows (about 90% or more) and has no evidence against it.
- **FR-V-6** Known words are excluded from tutor suggestions, paste-to-extract results and starter decks.
- **FR-V-7** If a known word is later failed in a review, it stops counting as known.
- **FR-V-8** The learner level is recalculated at least daily from all of the above, and can move up as well as down.
- **FR-V-9** Interrupted checks can be resumed: answering the same question twice (for example on a retry) records it once.

### 8.3 AI tutor
- **FR-AI-1** Answers stream token by token, and the first text appears within 2 seconds (p95).
- **FR-AI-2** Before stating pinyin or meanings, the tutor looks the word up in the built-in dictionary through a tool call.
- **FR-AI-3** Example sentences prefer the user's known words and approximate HSK level.
- **FR-AI-4** The tutor can only *suggest* words (by adding them to the Inbox, or as chips in the answer). It cannot create, edit or delete cards, notes or decks by itself.
- **FR-AI-5** Sentences the AI writes are labelled "AI-generated" wherever they appear in the app.
- **FR-AI-6** Each user has a daily usage budget. When it's exceeded, the prompt bar explains the limit; reviews are unaffected.
- **FR-AI-7** Repeated identical questions are served from cache.
- **FR-AI-8** Off-topic requests get a short redirect back to Mandarin learning.
- **FR-AI-9** Word extraction returns only words that exist in the dictionary. Unknown strings are reported separately and never turned into cards automatically.

### 8.4 Content
- **FR-C-1** Dictionary data comes from CC-CEDICT; example sentences from Tatoeba; level tags from the HSK 3.0 word lists (2026 revision), with levels 7–9 treated as one band.
- **FR-C-2** Every dataset is credited in the app and the README, according to its license.
- **FR-C-3** Re-importing a dataset never breaks or duplicates the user's cards.

### 8.5 Reviews and scheduling
- **FR-R-1** Scheduling uses FSRS with a default target retention of 90%, adjustable per deck from 80% to 97%.
- **FR-R-2** Every review is stored permanently. A card's schedule can always be recomputed from its review history.
- **FR-R-3** Submitting the same review twice (for example, on a network retry) records it once.

### 8.6 Offline and sync
- **FR-S-1** Reviews and note edits made offline are kept in a local queue and sent when the device is back online.
- **FR-S-2** When two devices edit the same note, each field resolves to the latest edit. A delete wins over a concurrent edit.
- **FR-S-3** Conversations with the tutor are online-only and are not part of offline sync.

### 8.7 Exercises
- **FR-X-1** Exercises checked by the app (typed recall, tones, cloze, sentence order) compare the answer with dictionary data or the source sentence. Pinyin answers accept tone marks or tone numbers, ignore spaces and case, and treat ü and v alike.
- **FR-X-2** Cloze and sentence-order exercises use Tatoeba sentences in simplified characters that contain the word, preferring the user's known words and level (as FR-AI-3). A different sentence is chosen on each review where possible, so the word is tested, not one sentence.
- **FR-X-3** The result of a check only *suggests* a rating. The user confirms it or picks another, and that choice is what the scheduler uses. Neither the app nor the AI ever rates a card by itself.
- **FR-X-4** AI-checked exercises (translation, use in a sentence) return a verdict, a corrected version and a short explanation. The corrected sentence is labelled AI-generated (FR-AI-5) and never becomes card data (ADR-0009).
- **FR-X-5** Typed answers are a way of answering the recall card (a deck setting, on by default), not a separate card, so a word's recall history stays in one place. Harder card types are introduced only once the note's recognition card is established: tones, cloze and sentence order after its interval reaches 7 days; translation and use in a sentence after 21 days. Thresholds are deck settings.
- **FR-X-6** Every review stores the answer given and the check's result with it, so ratings can later be compared with checked results (for example, to see how often a suggestion was overridden).
- **FR-X-7** AI-checked exercises count against the AI budget (FR-AI-6) and a per-user daily cap. When offline, over the cap or over budget, those cards are left due for another day; nothing else in the session is affected.

## 9. Non-functional requirements

| Area | Requirement |
|---|---|
| Performance | Review submit under 50 ms p95; due queue under 100 ms p95; dictionary search under 80 ms p95 (server-side). |
| Availability | Personal-project level: daily backups; offline reviews cover server downtime. |
| Security | No passwords stored: sign-in through Cloudflare Access, whose signed token the API verifies on every request; every query scoped to the user; API keys for AI providers kept server-side only. |
| Privacy | Only the question, the conversation and a list of known words are sent to the AI provider. Never email addresses or account data. |
| Cost | AI spend capped by a per-user daily budget and a global monthly cap. Caching first. v1 uses a hosted, token-billed model; the design keeps a self-hosted open model possible later without code changes outside the AI adapter. |
| Accessibility | Readable CJK font sizes; tone colours never the only signal. |
| Licensing | Dataset attribution shown in-app; derived dictionary data kept separate from code (CC BY-SA). |

## 10. Success metrics

Measured on the owner's own usage, since that's the real test.

| Metric | Target |
|---|---|
| Days used per week | 6 of 7 after the first month |
| True retention (share of mature cards recalled) | 85–92% |
| Median daily review time | under 20 minutes |
| Time from question to word captured | under 5 seconds |
| AI answers with wrong pinyin (from the eval set) | under 2% |
| Check accuracy: "known" words later recalled correctly in Verify-later reviews | over 85% |
| Time to complete the vocabulary check | median under 90 seconds |
| Words added via the AI tutor vs. typed manually | more than 50% via the tutor |

## 11. Release plan

| Release | Milestones | Contents |
|---|---|---|
| **MVP** | M0–M3 | Accounts, dictionary and search, decks, notes and cards, FSRS reviews, CSV import, "I already know this", minimal installable web client, deployed. Online-only. |
| **v0.9** | M3.5 | Exercises checked by the app: typed recall, tones, cloze, sentence order; suggested ratings; card types per deck; exercises unlock as a word matures. |
| **v1.0** | M4 | Vocabulary check and learner level; AI tutor: prompt bar, streaming answers, word chips, paste-and-extract, Inbox, "Ask about this"; AI-checked translation and "use it" exercises. |
| **v1.1** | M5 | Offline-first sync across devices. |
| **v1.2** | M6–M7 | Stats, forecast, personalised FSRS, observability, load tests, write-up. |
| **Later** | — | Auto-capture, pinyin checker, text-to-speech audio, stroke order, Discord reminder, simplified / traditional toggle. |

## 12. Risks

| Risk | Mitigation |
|---|---|
| AI gives wrong tones or meanings | Dictionary lookups through tools; card fields always from the dictionary; AI content labelled; eval set |
| AI costs grow, especially with friends using it | Caching, a cheaper model for extraction, per-user and global budgets, invite-only sign-up, and the option to switch to a self-hosted model |
| AI feature overshadows the backend story | AI goes through the same rigour: tool contracts, budgets, evals, metrics |
| Too many cards from enthusiastic capturing | Inbox as a buffer; daily new-card limits |
| Sync complexity delays the project | App is useful before sync; sync is additive |
| Low-quality Tatoeba sentences | Trust flag, level filter, hide button |
| Learners over-claim in the vocabulary check | Meaning follow-ups, made-up words, the Verify-later option, and automatic correction from later reviews |
| The check only measures recognition | Stated in the result; tones and writing are learned through review cards |
| Reviews only test recognition, so words feel known but can't be used | Exercises that check typed answers, tones, cloze, word order and the learner's own sentences (§8.7) |
| The AI misjudges a sentence (accepts a wrong one or rejects a right one) | It only suggests a rating and the learner confirms; corrections are labelled AI-generated; grading cases in the eval set (SYSTEM_DESIGN §8.10) |
| Exercises make daily sessions too long | They unlock only as words mature, AI exercises are capped per day, and exercise cards count toward the deck's daily limits |
| A bug lets one user see or change another user's data (real once friends use the app) | Every query is scoped to the logged-in user; an automated test for every endpoint tries to access another user's data and must fail; invite-only sign-up limits who has accounts |
| Cautious learners skip words they half-know, so their level is underestimated | Instruction to tap anything they recognise; underestimating is the safe direction (they see some too-easy words rather than missing words they need), and "I already know this" taps and reviews raise the level quickly |

## 13. Decisions

These started as open questions; all are now decided.

1. **Go or Spring Boot?** **Go.** The owner wants Go experience, and go-fsrs is available (ADR-0007).
2. **Which AI provider first?** **Anthropic**, the same provider as the owner's Discord bot, through the token-billed API. A self-hosted open model stays possible later (SYSTEM_DESIGN §8.12).
3. **Answer language:** English by default, configurable per user in Settings (C1).
4. **Tone display:** tone marks always, with optional tone colours in Settings (C2).
5. **Should the AI tutor be in the MVP?** **No.** It ships in v1.0 (M4), after the dictionary, decks and cards exist.
6. **Tapping a word chip: straight to a deck, or to the Inbox?** **Straight to a deck.** The tap is a deliberate choice, so it shouldn't need confirming again. The Inbox is only for words the user didn't pick one by one (paste-to-extract, auto-capture).
7. **HSK 2.0 or HSK 3.0?** **HSK 3.0**, the latest curriculum, using the examination syllabus word list published in November 2025 (in force for exams from 2026), from an MIT-licensed extraction (SYSTEM_DESIGN §5). Levels 1–6 are separate bands and levels 7–9 share one, so the app works with 7 bands.
8. **Should a checked answer set the rating by itself?** **No.** The check suggests a rating and the learner confirms it, because a typo or an accepted synonym shouldn't reset a card, and an AI verdict can be wrong (ADR-0019).
