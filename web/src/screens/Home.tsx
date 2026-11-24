import { Link } from 'wouter'
import { api } from '../api/client'
import { ChevronIcon, SearchIcon, SettingsIcon } from '../components/Icons'
import { Empty, ErrorState, Loading, Screen, SealWeek } from '../components/ui'
import { plural } from '../lib/format'
import { useResource } from '../lib/useResource'
import './home.css'

export function HomeScreen() {
  const res = useResource(() =>
    Promise.all([api.summary(), api.decks(), api.queue()]).then(([summary, decks, queue]) => ({ summary, decks, queue })),
  )

  return (
    <Screen>
      <header className="home-head">
        <div className="wordmark">
          <span className="hanzi">复习</span>
          <span className="wordmark-latin">Fuxi</span>
        </div>
        <Link href="/settings" className="icon-btn" aria-label="Settings">
          <SettingsIcon />
        </Link>
      </header>

      {/* The tutor's prompt bar takes this place in M4; until then it looks words up. */}
      <Link href="/search" className="lookup-bar">
        <SearchIcon size={20} />
        <span>Look up a word…</span>
      </Link>

      {res.error && !res.data ? (
        <ErrorState error={res.error} retry={res.reload} />
      ) : !res.data ? (
        <Loading />
      ) : (
        <HomeBody {...res.data} />
      )}
    </Screen>
  )
}

function HomeBody({ summary, decks, queue }: { summary: Awaited<ReturnType<typeof api.summary>>; decks: Awaited<ReturnType<typeof api.decks>>; queue: Awaited<ReturnType<typeof api.queue>> }) {
  const next = queue.due[0] ?? queue.new[0]
  const after = [...queue.due, ...queue.new].slice(1, 3)
  const total = summary.due_count + summary.new_count
  const counts = new Map(summary.decks.map((d) => [d.deck_id, d]))

  if (decks.length === 0) {
    return (
      <Empty glyph="学" title="Start with a few words">
        <p>Look up a word and add it to a deck, or make a deck and import your class list.</p>
        <div className="empty-actions">
          <Link href="/search" className="btn btn-primary">
            Look up a word
          </Link>
          <Link href="/decks" className="btn">
            Make a deck
          </Link>
        </div>
      </Empty>
    )
  }

  return (
    <>
      <section className="today" aria-labelledby="today-title">
        {next ? (
          <>
            <div className="stack" aria-hidden="true">
              {after.map((c, i) => (
                <div key={c.card_id} className={`stack-layer stack-layer-${i + 1}`} />
              ))}
              <div className="stack-top">
                <span className="hanzi stack-hanzi">
                  {next.type === 'recall' ? '?' : next.fields.hanzi}
                </span>
              </div>
            </div>
            <div className="today-text">
              <h2 id="today-title" className="today-count num">
                {plural(total, 'card')} today
              </h2>
              <p className="meta num">
                {summary.due_count} due · {summary.new_count} new
              </p>
            </div>
            <Link href="/review" className="btn btn-primary btn-block">
              Start review
            </Link>
          </>
        ) : (
          <div className="done-today">
            <span className="seal-big hanzi" aria-hidden="true">
              毕
            </span>
            <h2 id="today-title">Done for today</h2>
            <p className="meta">Nothing is due until tomorrow. Add words to learn more.</p>
          </div>
        )}
      </section>

      <section aria-label="This week" className="week">
        <SealWeek days={summary.days} today={summary.today} />
      </section>

      <h2 className="section-label">Decks</h2>
      <ul className="rows">
        {decks.map((d) => {
          const c = counts.get(d.id)
          return (
            <li key={d.id}>
              <Link href={`/decks/${d.id}`} className="row">
                <div className="row-main">
                  <div className="row-title">{d.name}</div>
                  <div className="row-sub num">{plural(d.note_count, 'word')}</div>
                </div>
                <div className="row-end num">
                  {c && c.due > 0 && <span className="count-due">{c.due} due</span>}
                  {c && c.new > 0 && <span>{c.new} new</span>}
                  <ChevronIcon />
                </div>
              </Link>
            </li>
          )
        })}
      </ul>
    </>
  )
}
