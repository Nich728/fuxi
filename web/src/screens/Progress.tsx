import { api } from '../api/client'
import type { Summary } from '../api/types'
import { ErrorState, Loading, Screen, WeekDots } from '../components/ui'
import { useCountUp } from '../lib/useCountUp'
import { plural, weekdayGlyph } from '../lib/format'
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
        <span className="label">进度</span>
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

function Week({ summary }: { summary: Summary }) {
  const total = summary.days.reduce((n, d) => n + d.reviews, 0)
  const shown = useCountUp(total, true)
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
        <p className="label">Reviews · last 7 days</p>
        <p className="progress-figure display num">{shown}</p>
        <p className="serif num">{streak > 0 ? `${plural(streak, 'day')} in a row.` : 'Review today to start a streak.'}</p>
      </section>

      <h2 className="label section-label">This week</h2>
      <WeekDots days={summary.days} today={summary.today} />

      <h2 className="label section-label">Reviews per day</h2>
      {/* Each day's bar grows from the left in turn (system.md motion). */}
      <ol className="day-bars">
        {summary.days.map((d, i) => (
          <li key={d.date}>
            <span className="day-label hanzi" aria-hidden="true">
              {weekdayGlyph(d.date)}
            </span>
            <span className="visually-hidden">
              {new Date(d.date + 'T12:00:00').toLocaleDateString(undefined, { weekday: 'long', day: 'numeric', month: 'long' })}
            </span>
            <span className="day-bar" aria-hidden="true">
              <span style={{ transform: `scaleX(${d.reviews / max})`, animationDelay: `${i * 50}ms` }} />
            </span>
            <span className="day-n num">{d.reviews}</span>
          </li>
        ))}
      </ol>
    </>
  )
}
