import { api } from '../api/client'
import { ErrorState, Loading, Screen, SealWeek } from '../components/ui'
import { plural } from '../lib/format'
import { useResource } from '../lib/useResource'
import './progress.css'

/**
 * Progress. In M3 it shows the week; retention, the review forecast and
 * personalised FSRS arrive with stats in M6 (PRD T1–T3).
 */
export function ProgressScreen() {
  const res = useResource(() => api.summary())

  return (
    <Screen>
      <header className="page-head">
        <h1>Progress</h1>
      </header>
      {res.error && !res.data ? (
        <ErrorState error={res.error} retry={res.reload} />
      ) : !res.data ? (
        <Loading />
      ) : (
        <Week summary={res.data} />
      )}
    </Screen>
  )
}

function Week({ summary }: { summary: Awaited<ReturnType<typeof api.summary>> }) {
  const total = summary.days.reduce((n, d) => n + d.reviews, 0)
  // Days in a row with reviews, ending today (or yesterday, if today isn't done yet).
  let streak = 0
  for (let i = summary.days.length - 1; i >= 0; i--) {
    if (summary.days[i].reviews > 0) streak++
    else if (i === summary.days.length - 1) continue
    else break
  }
  const max = Math.max(1, ...summary.days.map((d) => d.reviews))

  return (
    <>
      <section className="progress-hero">
        <p className="progress-figure num">{total}</p>
        <p className="meta">reviews in the last 7 days</p>
      </section>

      <SealWeek days={summary.days} today={summary.today} />
      <p className="meta progress-streak num">
        {streak > 0 ? `${plural(streak, 'day')} in a row` : 'Review today to start a streak.'}
      </p>

      <h2 className="section-label">Reviews per day</h2>
      <ol className="day-bars">
        {summary.days.map((d) => (
          <li key={d.date}>
            <span className="day-label">
              {new Date(d.date + 'T12:00:00').toLocaleDateString(undefined, { weekday: 'short', day: 'numeric' })}
            </span>
            <span className="day-bar" aria-hidden="true">
              <span style={{ transform: `scaleX(${d.reviews / max})` }} />
            </span>
            <span className="day-n num">{d.reviews}</span>
          </li>
        ))}
      </ol>
    </>
  )
}
