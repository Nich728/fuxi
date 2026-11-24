import * as Dialog from '@radix-ui/react-dialog'
import type { ReactNode } from 'react'
import { Link, useLocation } from 'wouter'
import { useApp } from '../app/state'
import { weekdayGlyph } from '../lib/format'
import { syllables } from '../lib/pinyin'
import { BackIcon, CloseIcon, DecksIcon, HomeIcon, ProgressIcon, SearchIcon } from './Icons'
import './ui.css'

/** Bottom tab bar: Home, Decks, Search, Progress (system.md layout). */
export function TabBar() {
  const [path] = useLocation()
  const tabs = [
    { to: '/', label: 'Home', icon: <HomeIcon />, active: path === '/' },
    { to: '/decks', label: 'Decks', icon: <DecksIcon />, active: path.startsWith('/decks') },
    { to: '/search', label: 'Search', icon: <SearchIcon />, active: path.startsWith('/search') || path.startsWith('/word') },
    { to: '/progress', label: 'Progress', icon: <ProgressIcon />, active: path.startsWith('/progress') },
  ]
  return (
    <nav className="tabbar" aria-label="Main">
      {tabs.map((t) => (
        <Link key={t.to} href={t.to} className="tab" aria-current={t.active ? 'page' : undefined}>
          {t.icon}
          <span>{t.label}</span>
        </Link>
      ))}
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

/** A title bar with an optional back link and trailing control. */
export function TopBar({ title, back, trailing }: { title?: ReactNode; back?: string; trailing?: ReactNode }) {
  return (
    <header className="topbar">
      {back ? (
        <Link href={back} className="icon-btn" aria-label="Back">
          <BackIcon />
        </Link>
      ) : (
        <span className="icon-btn-spacer" />
      )}
      {title && <h1 className="topbar-title">{title}</h1>}
      {trailing ?? <span className="icon-btn-spacer" />}
    </header>
  )
}

/** Pinyin in jade italics, coloured by tone when the setting is on (PRD C2). */
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
 * The seal week: Monday to Sunday of this week, labelled 一 to 日. A day with
 * reviews is a filled cinnabar seal; today, until it's earned, is dashed;
 * days still to come are plain outlines (system.md signature elements).
 */
export function SealWeek({ days, today }: { days: { date: string; reviews: number }[]; today: string }) {
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
    <ol className="seals" aria-label={`This week: studied on ${done} of 7 days`}>
      {week.map((w) => (
        <li key={w.iso} className={`seal seal-${w.kind}`} title={w.n ? `${w.n} reviews` : undefined}>
          <span className="hanzi" aria-hidden="true">
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
