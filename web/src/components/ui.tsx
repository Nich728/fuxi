import * as Dialog from '@radix-ui/react-dialog'
import { useLayoutEffect, useState, type MouseEvent, type ReactNode } from 'react'
import { Link, useLocation } from 'wouter'
import { useApp } from '../app/state'
import { weekdayGlyph } from '../lib/format'
import { slideTo } from '../lib/slide'
import { syllables } from '../lib/pinyin'
import { BackIcon, CloseIcon } from './Icons'
import './ui.css'

const TABS = [
  { to: '/', zh: '今日', en: 'Today', match: (p: string) => p === '/' },
  { to: '/decks', zh: '词库', en: 'Decks', match: (p: string) => p.startsWith('/decks') },
  { to: '/search', zh: '查', en: 'Search', match: (p: string) => p.startsWith('/search') || p.startsWith('/word') },
  { to: '/progress', zh: '进度', en: 'Progress', match: (p: string) => p.startsWith('/progress') },
]

// Where the tab line was last drawn, so it slides from there to the new tab.
let lastTab = -1

/** The tab bar: Chinese names, small English labels, and a jade line that slides to the open tab (system.md). */
export function TabBar() {
  const [path] = useLocation()
  const active = TABS.findIndex((t) => t.match(path))
  const [from] = useState(() => (lastTab >= 0 ? lastTab : active))
  const [at, setAt] = useState(from)
  useLayoutEffect(() => {
    lastTab = active
    // Draw at the previous tab first, then slide on the next frame.
    const id = requestAnimationFrame(() => setAt(active))
    return () => cancelAnimationFrame(id)
  }, [active])

  return (
    <nav className="tabbar" aria-label="Main">
      {TABS.map((t, i) => (
        <Link key={t.to} href={t.to} className="tab" aria-current={i === active ? 'page' : undefined}>
          <span className="tab-zh hanzi" lang="zh-Hans">
            {t.zh}
          </span>
          <span className="tab-en">{t.en}</span>
        </Link>
      ))}
      {at >= 0 && <span className="tab-line" aria-hidden="true" style={{ transform: `translateX(${at * 100}%)` }} />}
    </nav>
  )
}

/** A screen with the tab bar under it. */
export function Screen({ children, tabs = true, className = '' }: { children: ReactNode; tabs?: boolean; className?: string }) {
  return (
    <>
      <main className={`screen ${tabs ? 'with-tabs' : ''} ${className}`}>{children}</main>
      {tabs && <TabBar />}
    </>
  )
}

/**
 * A link that slides the next page in from the right (or back out, with
 * `back`) using View Transitions where the browser has them (system.md
 * motion). `before` can delay the switch briefly, e.g. until a word has
 * loaded, so the page arrives whole.
 */
export function SlideLink({
  href,
  back = false,
  before,
  className,
  children,
  ...rest
}: {
  href: string
  back?: boolean
  before?: () => Promise<unknown>
  className?: string
  children: ReactNode
  'aria-label'?: string
}) {
  const [, navigate] = useLocation()
  function go(e: MouseEvent) {
    if (e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) return
    e.preventDefault()
    slideTo(navigate, href, back, before)
  }
  return (
    <Link href={href} className={className} onClick={go} {...rest}>
      {children}
    </Link>
  )
}

/** A title bar with an optional back link and trailing control. */
export function TopBar({ title, back, trailing }: { title?: ReactNode; back?: string; trailing?: ReactNode }) {
  return (
    <header className="topbar">
      {back ? (
        <SlideLink href={back} back className="icon-btn" aria-label="Back">
          <BackIcon />
        </SlideLink>
      ) : (
        <span className="icon-btn-spacer" />
      )}
      {title && <h1 className="topbar-title">{title}</h1>}
      {trailing ?? <span className="icon-btn-spacer" />}
    </header>
  )
}

/** Pinyin in serif italics, coloured by tone when the setting is on (PRD C2). */
export function Pinyin({ text, className = '' }: { text?: string; className?: string }) {
  const { me } = useApp()
  if (!text) return null
  if (!me.settings.tone_colours) return <span className={`pinyin ${className}`}>{text}</span>
  return (
    <span className={`pinyin ${className}`}>
      {syllables(text).map((s, i) => (
        <span key={i} className={s.tone ? `tone-${s.tone}` : undefined}>
          {s.text}
        </span>
      ))}
    </span>
  )
}

/**
 * Week dots: Monday to Sunday of this week, labelled 一 to 日. A day with
 * reviews is jade; today, until it's done, is a jade ring; days to come are
 * faint (system.md signature elements). `intro` pops them in one after
 * another; `celebrate` fills today's dot with a ripple, once, after a session.
 */
export function WeekDots({
  days,
  today,
  intro = false,
  celebrate = false,
}: {
  days: { date: string; reviews: number }[]
  today: string
  intro?: boolean
  celebrate?: boolean
}) {
  const reviews = new Map(days.map((d) => [d.date, d.reviews]))
  const [y, m, d] = today.split('-').map(Number)
  const t = new Date(Date.UTC(y, m - 1, d))
  const monday = new Date(t)
  monday.setUTCDate(t.getUTCDate() - ((t.getUTCDay() + 6) % 7))
  const week = Array.from({ length: 7 }, (_, i) => {
    const day = new Date(monday)
    day.setUTCDate(monday.getUTCDate() + i)
    const iso = day.toISOString().slice(0, 10)
    const n = reviews.get(iso) ?? 0
    const kind = n > 0 ? 'done' : iso === today ? 'today' : iso > today ? 'future' : 'missed'
    return { iso, n, kind }
  })
  const done = week.filter((w) => w.kind === 'done').length
  return (
    <ol className="week-dots" aria-label={`This week: studied on ${done} of 7 days`}>
      {week.map((w, i) => (
        <li
          key={w.iso}
          className={`day day-${w.kind} ${celebrate && w.iso === today && w.kind === 'done' ? 'day-celebrate' : ''}`}
          title={w.n ? `${w.n} reviews` : undefined}
        >
          <span className={`dot ${intro ? 'pop' : ''}`} style={intro ? { animationDelay: `${300 + i * 45}ms` } : undefined} />
          <span className="day-glyph hanzi" aria-hidden="true">
            {weekdayGlyph(w.iso)}
          </span>
          <span className="visually-hidden">
            {w.iso}: {w.kind === 'done' ? `${w.n} reviews` : w.kind === 'future' ? 'still to come' : 'no reviews'}
          </span>
        </li>
      ))}
    </ol>
  )
}

/**
 * A number that rolls up to its next value when it changes: the review
 * counter, a deck's word count (system.md motion).
 */
export function Roll({ value, from, className = '' }: { value: number | string; from?: number | string; className?: string }) {
  // With `from`, it rolls from there on first show (a count that just went up).
  const [shown, setShown] = useState({ prev: from ?? value, cur: value, key: from === undefined ? 0 : 1 })
  if (shown.cur !== value) setShown((s) => ({ prev: s.cur, cur: value, key: s.key + 1 }))
  const rolling = shown.key > 0 && shown.prev !== shown.cur
  return (
    <span className={`roll num ${rolling ? 'is-rolling' : ''} ${className}`}>
      {rolling ? (
        <span key={shown.key} aria-hidden="true">
          {shown.prev}
          <br />
          {shown.cur}
        </span>
      ) : (
        <span aria-hidden="true">{shown.cur}</span>
      )}
      <span className="visually-hidden">{shown.cur}</span>
    </span>
  )
}

/** A bottom sheet on phones, a centred panel on wider screens (Radix Dialog). */
export function Sheet({
  open,
  onOpenChange,
  title,
  description,
  children,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  description?: string
  children: ReactNode
}) {
  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className="sheet-overlay" />
        <Dialog.Content className="sheet">
          <div className="sheet-head">
            <Dialog.Title className="sheet-title">{title}</Dialog.Title>
            <Dialog.Close className="icon-btn" aria-label="Close">
              <CloseIcon />
            </Dialog.Close>
          </div>
          {description ? (
            <Dialog.Description className="meta sheet-desc">{description}</Dialog.Description>
          ) : (
            <Dialog.Description className="visually-hidden">{title}</Dialog.Description>
          )}
          {children}
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  )
}

/** Toasts slide up; with an action (Undo) a thin line shrinks for as long as it stays. */
export function Toasts() {
  const { toasts, dismiss } = useApp()
  return (
    <div className="toasts" role="status" aria-live="polite">
      {toasts.map((t) => (
        <div key={t.id} className="toast">
          <span>{t.message}</span>
          {t.action && (
            <button
              className="toast-action"
              onClick={() => {
                t.action!.run()
                dismiss(t.id)
              }}
            >
              {t.action.label}
            </button>
          )}
          {t.action && <span className="toast-timer" aria-hidden="true" style={{ animationDuration: `${t.ms}ms` }} />}
        </div>
      ))}
    </div>
  )
}

/** Loading, error and empty states share one quiet layout. */
export function Loading({ label = 'Loading' }: { label?: string }) {
  return (
    <div className="state" aria-busy="true">
      <span className="state-dots" aria-hidden="true">
        <span />
        <span />
        <span />
      </span>
      <span className="visually-hidden">{label}</span>
    </div>
  )
}

export function ErrorState({ error, retry }: { error: Error; retry?: () => void }) {
  return (
    <div className="state" role="alert">
      <p>{error.message}</p>
      {retry && (
        <button className="btn" onClick={retry}>
          Try again
        </button>
      )}
    </div>
  )
}

export function Empty({ glyph, title, children }: { glyph: string; title: string; children?: ReactNode }) {
  return (
    <div className="state empty">
      <span className="hanzi empty-glyph" aria-hidden="true">
        {glyph}
      </span>
      <h2>{title}</h2>
      {children}
    </div>
  )
}
