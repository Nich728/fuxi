import { useCallback, useEffect, useMemo, useRef, useState, type CSSProperties } from 'react'
import { Link, useLocation, useSearchParams } from 'wouter'
import { api, ApiError } from '../api/client'
import { deviceId, uuidv7 } from '../api/ids'
import type { DictEntryDetail, QueueCard, Rating } from '../api/types'
import { useApp } from '../app/state'
import { ArrowIcon, CloseIcon } from '../components/Icons'
import { DotSeal } from '../components/DotSeal'
import { Empty, ErrorState, Loading, Pinyin, Roll } from '../components/ui'
import { cardMeaning, formatInterval, hskLabel, plural, simplifiedExamples, spreadSiblings } from '../lib/format'
import { celebrateToday } from '../lib/motion'
import { isReady } from '../lib/queue'
import './review.css'

const RATINGS: { rating: Rating; key: keyof QueueCard['preview']; label: string }[] = [
  { rating: 1, key: 'again', label: 'Again' },
  { rating: 2, key: 'hard', label: 'Hard' },
  { rating: 3, key: 'good', label: 'Good' },
  { rating: 4, key: 'easy', label: 'Easy' },
]

/** A learning card due again within this long comes back in the same session. */
const SAME_SESSION = 30 * 60_000

interface Pending {
  card: QueueCard
  dueAt: number
}

interface Stats {
  reviewed: number
  again: number
  startedAt: number
}

/**
 * The review session (PRD flow 7.5): show a card, reveal, rate, next.
 * Reviews are sent in the background; the next card never waits for the
 * network. A failed send is retried with the same id, so it counts once.
 */
export function ReviewScreen() {
  const [params] = useSearchParams()
  const deck = params.get('deck') ?? undefined
  const [, navigate] = useLocation()
  const { toast } = useApp()

  const [cards, setCards] = useState<QueueCard[]>()
  const [error, setError] = useState<Error>()
  const [index, setIndex] = useState(0)
  const [revealed, setRevealed] = useState(false)
  const [later, setLater] = useState<Pending[]>([])
  const [stats, setStats] = useState<Stats>(() => ({ reviewed: 0, again: 0, startedAt: Date.now() }))
  const [now, setNow] = useState(() => Date.now())
  const [deckNames, setDeckNames] = useState(new Map<string, string>())
  const shownAt = useRef(0)
  const outbox = useRef<Promise<unknown>>(Promise.resolve())

  // all: include learning cards that aren't due yet ("review them now anyway").
  const load = useCallback(async (all = false) => {
    try {
      const [q, decks] = await Promise.all([api.queue(deck), api.decks()])
      setError(undefined)
      setDeckNames(new Map(decks.map((d) => [d.id, d.name])))
      const t = Date.now()
      // Learning cards rated a moment ago are "due today" but not due yet (lib/queue).
      const ready = [...q.due, ...q.new].filter((c) => all || isReady(c, t))
      setLater(
        q.due
          .filter((c) => !ready.includes(c) && Date.parse(c.due_at) - t < SAME_SESSION)
          .map((c) => ({ card: c, dueAt: Date.parse(c.due_at) })),
      )
      setCards(spreadSiblings(ready))
      setIndex(0)
      setRevealed(false)
      shownAt.current = Date.now()
    } catch (e) {
      setError(e as Error)
    }
  }, [deck])

  useEffect(() => {
    // load sets state only after awaiting the network, never synchronously.
    // oxlint-disable-next-line react/set-state-in-effect
    load()
  }, [load])

  const card = cards?.[index]
  const finished = cards !== undefined && !card

  // Waiting on learning cards: tick so the countdown and "ready" state update.
  useEffect(() => {
    if (!finished || later.length === 0) return
    const t = setInterval(() => setNow(Date.now()), 15_000)
    return () => clearInterval(t)
  }, [finished, later.length])

  const reveal = useCallback(() => setRevealed(true), [])

  const rate = useCallback(
    (rating: Rating) => {
      if (!card || !revealed) return
      const reviewedAt = new Date()
      const review = {
        id: uuidv7(reviewedAt.getTime()),
        card_id: card.card_id,
        rating,
        reviewed_at: reviewedAt.toISOString(),
        duration_ms: Math.min(Date.now() - shownAt.current, 3_600_000),
        device_id: deviceId(),
      }
      // Send in order, in the background; retry a failed send a few times
      // with the same id (the server records it once, PRD FR-R-3).
      outbox.current = outbox.current.then(() => send(review, 4)).catch((e: Error) => {
        toast(`A review couldn't be saved: ${e.message}`)
      })

      const nextDue = Date.parse(card.preview[RATINGS[rating - 1].key])
      if (nextDue - reviewedAt.getTime() < SAME_SESSION) {
        setLater((l) => [...l, { card: { ...card, state: card.state === 'new' ? 'learning' : card.state }, dueAt: nextDue }])
      }
      setStats((s) => ({ ...s, reviewed: s.reviewed + 1, again: s.again + (rating === 1 ? 1 : 0) }))
      setIndex((i) => i + 1)
      setRevealed(false)
      shownAt.current = Date.now()
    },
    [card, revealed, toast],
  )

  // Keyboard: space or enter reveals; 1–4 rate (laptop sessions).
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.target instanceof HTMLInputElement || e.metaKey || e.ctrlKey || e.altKey) return
      if (!revealed && (e.key === ' ' || e.key === 'Enter')) {
        e.preventDefault()
        reveal()
      } else if (revealed && ['1', '2', '3', '4'].includes(e.key)) {
        rate(Number(e.key) as Rating)
      } else if (e.key === 'Escape') {
        navigate('/')
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [revealed, reveal, rate, navigate])

  const continueLater = useCallback(async () => {
    await outbox.current // the server must have every review before the queue is read again
    await load(true)
  }, [load])

  if (error) {
    return (
      <main className="review">
        <ReviewBar onClose="/" />
        <ErrorState
          error={error}
          retry={() => {
            setError(undefined)
            setCards(undefined)
            load()
          }}
        />
      </main>
    )
  }
  if (!cards) {
    return (
      <main className="review">
        <ReviewBar onClose="/" />
        <Loading label="Loading your cards" />
      </main>
    )
  }
  if (finished) {
    return (
      <main className="review">
        <ReviewBar onClose="/" />
        <SessionEnd stats={stats} later={later} now={now} onContinue={continueLater} />
      </main>
    )
  }

  const total = cards.length
  return (
    <main className="review">
      <ReviewBar onClose="/" done={index} total={total} label={deckNames.get(card!.deck_id)} />
      <CardView key={card!.card_id + index} card={card!} deckName={deckNames.get(card!.deck_id)} revealed={revealed} />
      <div className="review-actions">
        {!revealed ? (
          <button className="btn btn-primary btn-block" onClick={reveal}>
            Show answer
          </button>
        ) : (
          <div className="ratings" role="group" aria-label="How well did you remember it?">
            {RATINGS.map((r) => (
              <button
                key={r.rating}
                className={`rating rating-${r.key}`}
                onClick={() => rate(r.rating)}
              >
                <span className="rating-label">{r.label}</span>
                <span className="rating-interval num">
                  {formatInterval(new Date(card!.preview[r.key]), new Date())}
                </span>
              </button>
            ))}
          </div>
        )}
      </div>
    </main>
  )
}

async function send(review: Parameters<typeof api.review>[0], tries: number): Promise<void> {
  for (let i = 0; ; i++) {
    try {
      await api.review(review)
      return
    } catch (e) {
      // 4xx other than a timeout won't get better by retrying.
      const status = e instanceof ApiError ? e.status : 0
      if (i + 1 >= tries || (status >= 400 && status < 500 && status !== 408 && status !== 429)) throw e
      await new Promise((r) => setTimeout(r, 1000 * 2 ** i))
    }
  }
}

function ReviewBar({ onClose, done, total, label }: { onClose: string; done?: number; total?: number; label?: string }) {
  const pct = total ? (done! / total) * 100 : 0
  return (
    <header className="review-bar">
      <div className="review-bar-row">
        <Link href={onClose} className="icon-btn" aria-label="End review">
          <CloseIcon />
        </Link>
        <span className="label review-deck">{label}</span>
        {total !== undefined && (
          <span className="label review-count" aria-label={`${plural(total - done!, 'card')} left`}>
            <Roll value={Math.min(done! + 1, total)} /> / {total}
          </span>
        )}
      </div>
      {total !== undefined && (
        <div className="progress" role="progressbar" aria-valuemin={0} aria-valuemax={total} aria-valuenow={done} aria-label="Cards reviewed">
          <div className="progress-fill" style={{ transform: `scaleX(${pct / 100})` }} />
        </div>
      )}
    </header>
  )
}

/** The card. The hanzi is the hero (system.md); nothing competes with it. */
function CardView({ card, deckName, revealed }: { card: QueueCard; deckName?: string; revealed: boolean }) {
  const [entry, setEntry] = useState<DictEntryDetail>()
  useEffect(() => {
    if (card.entry_id == null) return
    let live = true
    api.entry(card.entry_id).then((e) => live && setEntry(e), () => {})
    return () => {
      live = false
    }
  }, [card.entry_id])

  const f = card.fields
  const recall = card.type === 'recall'
  const example = entry && simplifiedExamples(entry.examples, entry.simplified)[0]
  const hsk = hskLabel(entry?.hsk_level)

  return (
    <article className="card" aria-live="polite" aria-label={deckName}>
      {hsk && <p className="label gold card-meta">{hsk}</p>}

      <div className="card-face">
        {recall ? (
          <>
            <p className="recall-prompt">{cardMeaning(f.meaning)}</p>
            {revealed && <span className="hanzi card-hanzi card-hanzi-answer">{f.hanzi}</span>}
          </>
        ) : (
          <span className="hanzi card-hanzi">{f.hanzi}</span>
        )}
      </div>

      <div className={`card-answer ${revealed ? 'is-shown' : ''}`} aria-hidden={!revealed}>
        {revealed && (
          <>
            <Pinyin text={f.pinyin} className="card-pinyin" />
            {!recall && <p className="serif card-meaning">{cardMeaning(f.meaning)}</p>}
            {f.notes && <p className="meta card-notes">{f.notes}</p>}
            {example && (
              <figure className="example">
                <blockquote className="example-zh hanzi">{example.zh}</blockquote>
                {example.en && <figcaption className="serif example-en">{example.en}</figcaption>}
              </figure>
            )}
          </>
        )}
      </div>
      {recall && !revealed && <p className="meta card-hint">Say or write it, then check.</p>}
    </article>
  )
}

function SessionEnd({ stats, later, now, onContinue }: { stats: Stats; later: Pending[]; now: number; onContinue: () => void }) {
  const minutes = Math.max(1, Math.round((now - stats.startedAt) / 60_000))
  const accuracy = stats.reviewed ? Math.round(((stats.reviewed - stats.again) / stats.reviewed) * 100) : 0
  const soonest = useMemo(() => Math.min(...later.map((p) => p.dueAt)), [later])
  const ready = later.length > 0 && soonest <= now + 60_000
  // Home fills today's dot when you get back (system.md motion).
  useEffect(() => {
    if (stats.reviewed > 0) celebrateToday()
  }, [stats.reviewed])

  if (stats.reviewed === 0 && later.length === 0) {
    return (
      <Empty glyph="毕" title="Nothing to review">
        <p>No cards are due right now.</p>
        <Link href="/" className="btn">
          Back to home
        </Link>
      </Empty>
    )
  }

  return (
    <section className="session-end">
      <DotSeal glyph="毕" />
      <p className="label">{stats.reviewed === 0 ? 'Still learning' : 'Session done'}</p>
      {stats.reviewed > 0 && (
        <>
          <h1 className="display session-cards num">
            <span className="mask">
              <span className="rise" style={{ '--d': '900ms' } as CSSProperties}>
                {plural(stats.reviewed, 'card')}
              </span>
            </span>
          </h1>
          <p className="serif session-line num">
            <span className="mask">
              <span className="rise" style={{ '--d': '980ms' } as CSSProperties}>
                {accuracy}% remembered · {minutes} min
              </span>
            </span>
          </p>
        </>
      )}
      {later.length > 0 ? (
        <div className="session-later fade-up" style={{ '--d': '1100ms' } as CSSProperties}>
          <p className="meta">
            {plural(later.length, 'card')} {later.length === 1 ? 'is' : 'are'} still being learned
            {ready ? ' and ready now.' : `; the next is ready in ${formatInterval(new Date(soonest), new Date(now))}.`}
          </p>
          <button className={`btn btn-block ${ready ? 'btn-primary' : ''}`} onClick={onContinue}>
            {ready ? 'Continue' : 'Review them now anyway'}
          </button>
          <Link href="/" className="btn btn-quiet btn-block">
            Back to home
          </Link>
        </div>
      ) : (
        <Link href="/" className="btn btn-primary btn-block session-home fade-up" style={{ '--d': '1100ms' } as CSSProperties}>
          Back to home <ArrowIcon />
        </Link>
      )}
    </section>
  )
}
