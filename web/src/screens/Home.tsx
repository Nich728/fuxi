import { useEffect, useState, type CSSProperties } from 'react'
import { Link } from 'wouter'
import { api } from '../api/client'
import type { Deck, Queue, Summary } from '../api/types'
import { ArrowIcon, SearchIcon, SettingsIcon } from '../components/Icons'
import { Empty, ErrorState, Loading, Screen, SlideLink, WeekDots } from '../components/ui'
import { useCountUp } from '../lib/useCountUp'
import { chineseDate, plural, sessionMinutes } from '../lib/format'
import { celebrated, celebrationPending, homeIntroPending, homeIntroPlayed } from '../lib/motion'
import { useResource } from '../lib/useResource'
import './home.css'

export function HomeScreen() {
  const res = useResource(() =>
    Promise.all([api.summary(), api.decks(), api.queue()]).then(([summary, decks, queue]) => ({ summary, decks, queue })),
  )

  return (
    <Screen>
      {res.error && !res.data ? (
        <>
          <HomeHead />
          <ErrorState error={res.error} retry={res.reload} />
        </>
      ) : !res.data ? (
        <>
          <HomeHead />
          <Loading />
        </>
      ) : (
        <HomeBody {...res.data} />
      )}
    </Screen>
  )
}

function HomeHead({ date, intro }: { date?: string; intro?: boolean }) {
  return (
    <header className="home-head">
      <p className={`label ${intro ? 'fade-up' : ''}`}>{date}</p>
      <SlideLink href="/settings" className="icon-btn" aria-label="Settings">
        <SettingsIcon />
      </SlideLink>
    </header>
  )
}

/** The summary line under the count, e.g. "18 reviews and 6 new words, about 5 minutes." */
function todayLine(due: number, fresh: number): string {
  const parts = [due > 0 && plural(due, 'review'), fresh > 0 && plural(fresh, 'new word')].filter(Boolean)
  return `${parts.join(' and ')}, about ${plural(sessionMinutes(due, fresh), 'minute')}.`
}

function HomeBody({ summary, decks, queue }: { summary: Summary; decks: Deck[]; queue: Queue }) {
  // One-shot moments (system.md motion): read while rendering, marked done after.
  const [intro] = useState(homeIntroPending)
  const [celebrate] = useState(celebrationPending)
  useEffect(() => {
    homeIntroPlayed()
    celebrated()
  }, [])

  const total = summary.due_count + summary.new_count
  const count = useCountUp(total, intro)
  const hasCards = queue.due.length + queue.new.length > 0
  const counts = new Map(summary.decks.map((d) => [d.deck_id, d]))
  // Entrance: a line rises from its mask after `ms`.
  const rise = (ms: number) => (intro ? { className: 'rise', style: { '--d': `${ms}ms` } as CSSProperties } : {})
  const fade = (ms: number) => (intro ? { style: { '--d': `${ms}ms` } as CSSProperties } : {})

  if (decks.length === 0) {
    return (
      <>
        <HomeHead date={chineseDate(summary.today)} />
        <Empty glyph="学" title="No decks yet">
          <p className="serif">Look up a word and add it to a deck, or make a deck and import your class list.</p>
          <div className="empty-actions">
            <Link href="/search" className="btn btn-primary">
              Look up a word <ArrowIcon />
            </Link>
            <Link href="/decks" className="btn">
              Make a deck
            </Link>
          </div>
        </Empty>
      </>
    )
  }

  return (
    <>
      <HomeHead date={chineseDate(summary.today)} intro={intro} />

      <section className="today" aria-labelledby="today-title">
        <p id="today-title" className={`label ${intro ? 'fade-up' : ''}`} {...fade(60)}>
          Due today
        </p>
        <p className={`today-count display num ${total === 0 ? 'is-zero' : ''}`}>
          <span className="mask">
            <span {...rise(100)}>{count}</span>
          </span>
        </p>
        <p className="serif today-line">
          <span className="mask">
            <span {...rise(180)}>
              {total > 0 ? todayLine(summary.due_count, summary.new_count) : 'Nothing else is due today.'}
            </span>
          </span>
        </p>
        {hasCards ? (
          <Link href="/review" className={`btn btn-primary btn-block today-start ${intro ? 'fade-up' : ''}`} {...fade(260)}>
            Start review <ArrowIcon />
          </Link>
        ) : (
          total > 0 && <p className="meta today-later">Cards you're learning come back later today.</p>
        )}
      </section>

      {/* The tutor's prompt bar takes this place in M4; until then it looks words up. */}
      <Link href="/search" className={`lookup ${intro ? 'fade-up' : ''}`} {...fade(320)}>
        <SearchIcon size={18} />
        <span>Look up a word</span>
      </Link>

      <h2 className={`label section-label ${intro ? 'fade-up' : ''}`} {...fade(360)}>
        This week
      </h2>
      <WeekDots days={summary.days} today={summary.today} intro={intro} celebrate={celebrate} />

      <h2 className={`label section-label ${intro ? 'fade-up' : ''}`} {...fade(420)}>
        Decks
      </h2>
      <ul className="deck-rows">
        {decks.map((d, i) => {
          const c = counts.get(d.id)
          const due = c?.due ?? 0
          const fresh = c?.new ?? 0
          return (
            <li key={d.id} className="mask">
              <span {...rise(460 + i * 50)}>
                <Link href={`/decks/${d.id}`} className={`deck-row ${due + fresh === 0 ? 'is-idle' : ''}`}>
                  <span className="deck-row-name">{d.name}</span>
                  <span className="deck-row-count num">
                    {due > 0 && <span className="due">{due} due</span>}
                    {due > 0 && fresh > 0 && ' · '}
                    {fresh > 0 && `${fresh} new`}
                  </span>
                </Link>
              </span>
            </li>
          )
        })}
      </ul>
    </>
  )
}
