import { useState } from 'react'
import { Link, useLocation } from 'wouter'
import { api } from '../api/client'
import { uuidv7 } from '../api/ids'
import { ChevronIcon, PlusIcon } from '../components/Icons'
import { Empty, ErrorState, Loading, Screen, Sheet } from '../components/ui'
import { plural } from '../lib/format'
import { useResource } from '../lib/useResource'

/** Decks (PRD L3): divided rows with today's counts. */
export function DecksScreen() {
  const res = useResource(() =>
    Promise.all([api.decks(), api.summary()]).then(([decks, summary]) => ({ decks, summary })),
  )
  const [creating, setCreating] = useState(false)

  return (
    <Screen>
      <header className="page-head">
        <h1>Decks</h1>
      </header>
      {res.error && !res.data ? (
        <ErrorState error={res.error} retry={res.reload} />
      ) : !res.data ? (
        <Loading />
      ) : res.data.decks.length === 0 ? (
        <Empty glyph="册" title="No decks yet">
          <p>A deck holds the words you're learning, for a class, a book or an HSK level.</p>
        </Empty>
      ) : (
        <ul className="rows">
          {res.data.decks.map((d) => {
            const c = res.data!.summary.decks.find((x) => x.deck_id === d.id)
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
      )}
      <div className="page-foot">
        <button className="btn btn-primary btn-block" onClick={() => setCreating(true)}>
          <PlusIcon size={20} /> New deck
        </button>
      </div>
      <NewDeckSheet open={creating} onOpenChange={setCreating} />
    </Screen>
  )
}

function NewDeckSheet({ open, onOpenChange }: { open: boolean; onOpenChange: (o: boolean) => void }) {
  const [, navigate] = useLocation()
  const [name, setName] = useState('')
  const [perDay, setPerDay] = useState(20)
  const [error, setError] = useState<string>()
  const [busy, setBusy] = useState(false)

  async function create(e: React.FormEvent) {
    e.preventDefault()
    if (!name.trim()) {
      setError('Give the deck a name.')
      return
    }
    setBusy(true)
    setError(undefined)
    try {
      const deck = await api.createDeck(uuidv7(), name.trim(), { new_per_day: perDay })
      onOpenChange(false)
      setName('')
      navigate(`/decks/${deck.id}`)
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setBusy(false)
    }
  }

  return (
    <Sheet open={open} onOpenChange={onOpenChange} title="New deck">
      <form className="form" onSubmit={create}>
        <label className="field">
          <span className="field-label">Name</span>
          <input
            className="input"
            value={name}
            maxLength={100}
            placeholder="e.g. Class week 6"
            onChange={(e) => setName(e.target.value)}
            autoFocus
          />
        </label>
        <label className="field">
          <span className="field-label">New cards a day</span>
          <input
            className="input num"
            type="number"
            inputMode="numeric"
            min={0}
            max={500}
            value={perDay}
            onChange={(e) => setPerDay(Math.max(0, Math.min(500, Number(e.target.value) || 0)))}
          />
        </label>
        {error && (
          <p className="form-error" role="alert">
            {error}
          </p>
        )}
        <button className="btn btn-primary btn-block" disabled={busy}>
          {busy ? 'Creating…' : 'Create deck'}
        </button>
      </form>
    </Sheet>
  )
}
