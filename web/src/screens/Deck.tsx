import { useEffect, useRef, useState } from 'react'
import { Link, useLocation } from 'wouter'
import { api } from '../api/client'
import { uuidv7 } from '../api/ids'
import type { Deck, ImportResult, Note } from '../api/types'
import { useApp } from '../app/state'
import { ArrowIcon } from '../components/Icons'
import { Empty, ErrorState, Loading, Pinyin, Screen, Sheet, TopBar } from '../components/ui'
import { plural } from '../lib/format'
import { useResource } from '../lib/useResource'
import './deck.css'

const RETENTIONS = [0.8, 0.85, 0.9, 0.93, 0.95, 0.97]

export function DeckScreen({ id }: { id: string }) {
  const res = useResource(
    () =>
      Promise.all([api.deck(id), api.notes(id), api.summary()]).then(([deck, page, summary]) => ({
        deck,
        notes: page.notes,
        cursor: page.next_cursor,
        counts: summary.decks.find((d) => d.deck_id === id),
      })),
    [id],
  )

  return (
    <Screen>
      <TopBar back="/decks" />
      {res.error && !res.data ? (
        <ErrorState error={res.error} retry={res.reload} />
      ) : !res.data ? (
        <Loading />
      ) : (
        <DeckBody key={id} {...res.data} reload={res.reload} />
      )}
    </Screen>
  )
}

function DeckBody({
  deck: initial,
  notes: firstPage,
  cursor: firstCursor,
  counts,
  reload,
}: {
  deck: Deck
  notes: Note[]
  cursor?: string
  counts?: { due: number; new: number }
  reload: () => void
}) {
  const { toast } = useApp()
  const [deck, setDeck] = useState(initial)
  const [notes, setNotes] = useState(firstPage)
  const [cursor, setCursor] = useState(firstCursor)
  const [open, setOpen] = useState<Note>()
  const [renaming, setRenaming] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const due = counts?.due ?? 0
  const fresh = counts?.new ?? 0

  async function more() {
    if (!cursor) return
    try {
      const page = await api.notes(deck.id, cursor)
      setNotes((n) => [...n, ...page.notes])
      setCursor(page.next_cursor)
    } catch (e) {
      toast((e as Error).message)
    }
  }

  async function saveSettings(patch: Partial<Deck['settings']>) {
    try {
      setDeck(await api.updateDeck(deck.id, { settings: patch }))
    } catch (e) {
      toast((e as Error).message)
    }
  }

  return (
    <>
      <header className="deck-head">
        <h1 className="display">{deck.name}</h1>
        <p className="serif num">
          {plural(deck.note_count, 'word')} · {due} due · {fresh} new today
        </p>
      </header>

      {due + fresh > 0 ? (
        <Link href={`/review?deck=${deck.id}`} className="btn btn-primary btn-block">
          Study this deck <ArrowIcon />
        </Link>
      ) : (
        <p className="serif deck-idle">Nothing to study in this deck today.</p>
      )}

      <h2 className="label section-label">Words</h2>
      {notes.length === 0 ? (
        <Empty glyph="词" title="No words yet">
          <p>
            Add words from <Link href="/search">Search</Link>, or import your class list below.
          </p>
        </Empty>
      ) : (
        <>
          <ul className="rows">
            {notes.map((n) => (
              <li key={n.id}>
                <button className="row note-row" onClick={() => setOpen(n)}>
                  <span className="hanzi note-hanzi">{n.fields.hanzi}</span>
                  <span className="row-main">
                    <Pinyin text={n.fields.pinyin} className="note-pinyin" />
                    <span className="row-sub">{n.fields.meaning}</span>
                  </span>
                </button>
              </li>
            ))}
          </ul>
          {cursor && (
            <button className="btn btn-quiet btn-block" onClick={more}>
              Show more
            </button>
          )}
        </>
      )}

      <CsvImport deck={deck} onDone={reload} />

      <h2 className="label section-label">Settings</h2>
      <div className="settings-list">
        <label className="setting">
          <span>New cards a day</span>
          <input
            className="input setting-input num"
            type="number"
            inputMode="numeric"
            min={0}
            max={1000}
            defaultValue={deck.settings.new_per_day}
            onBlur={(e) => {
              const v = Math.max(0, Math.min(1000, Math.round(Number(e.target.value) || 0)))
              e.target.value = String(v)
              if (v !== deck.settings.new_per_day) saveSettings({ new_per_day: v })
            }}
          />
        </label>
        <label className="setting">
          <span>
            Target retention
            <span className="meta setting-help">Higher means more reviews and fewer lapses.</span>
          </span>
          <select
            className="input setting-input num"
            value={deck.settings.desired_retention}
            onChange={(e) => saveSettings({ desired_retention: Number(e.target.value) })}
          >
            {RETENTIONS.map((r) => (
              <option key={r} value={r}>
                {Math.round(r * 100)}%
              </option>
            ))}
          </select>
        </label>
      </div>
      <div className="deck-manage">
        <button className="btn" onClick={() => setRenaming(true)}>
          Rename
        </button>
        <button className="btn btn-quiet" onClick={() => setDeleting(true)}>
          Delete deck
        </button>
      </div>

      {open && (
        <NoteSheet
          note={open}
          onClose={() => setOpen(undefined)}
          onDeleted={() => {
            setNotes((ns) => ns.filter((n) => n.id !== open.id))
            setDeck((d) => ({ ...d, note_count: d.note_count - 1 }))
            setOpen(undefined)
          }}
        />
      )}
      <RenameSheet open={renaming} onOpenChange={setRenaming} deck={deck} onRenamed={setDeck} />
      <DeleteSheet open={deleting} onOpenChange={setDeleting} deck={deck} />
    </>
  )
}

/** A word in the deck: its cards, which can be paused (PRD R7), and delete. */
function NoteSheet({ note, onClose, onDeleted }: { note: Note; onClose: () => void; onDeleted: () => void }) {
  const { toast } = useApp()
  const res = useResource(() => api.note(note.id), [note.id])
  const [confirm, setConfirm] = useState(false)

  async function toggle(cardId: string, suspended: boolean) {
    try {
      await api.setSuspended(cardId, suspended)
      res.reload()
    } catch (e) {
      toast((e as Error).message)
    }
  }

  async function remove() {
    try {
      await api.deleteNote(note.id)
      toast(`Deleted ${note.fields.hanzi}.`)
      onDeleted()
    } catch (e) {
      toast((e as Error).message)
    }
  }

  const full = res.data ?? note
  return (
    <Sheet open onOpenChange={(o) => !o && onClose()} title={note.fields.hanzi ?? 'Word'}>
      <div className="note-detail">
        <Pinyin text={full.fields.pinyin} className="note-detail-pinyin" />
        <p>{full.fields.meaning}</p>
        {full.fields.notes && <p className="meta">{full.fields.notes}</p>}
      </div>
      <h3 className="label section-label">Cards</h3>
      {!res.data ? (
        <Loading />
      ) : (
        <ul className="rows">
          {res.data.cards?.map((c) => (
            <li key={c.id} className="row">
              <span className="row-main">
                <span className="row-title">{c.type === 'recall' ? 'Recall (meaning → word)' : 'Recognition (word → meaning)'}</span>
                <span className="row-sub">
                  {c.suspended
                    ? 'Paused'
                    : c.state === 'new'
                      ? 'Not studied yet'
                      : `Next ${new Date(c.due_at).toLocaleDateString(undefined, { day: 'numeric', month: 'short' })} · ${plural(c.reps, 'review')}`}
                </span>
              </span>
              <button className="btn" onClick={() => toggle(c.id, !c.suspended)}>
                {c.suspended ? 'Resume' : 'Pause'}
              </button>
            </li>
          ))}
        </ul>
      )}
      <div className="note-delete">
        {confirm ? (
          <>
            <p className="meta">Delete this word and its cards? Your review history is kept.</p>
            <div className="confirm-row">
              <button className="btn" onClick={() => setConfirm(false)}>
                Keep it
              </button>
              <button className="btn btn-danger" onClick={remove}>
                Delete
              </button>
            </div>
          </>
        ) : (
          <button className="btn btn-quiet btn-block" onClick={() => setConfirm(true)}>
            Delete word
          </button>
        )}
      </div>
    </Sheet>
  )
}

/** CSV import (PRD L5): characters, pinyin, meaning per row; runs as a job. */
function CsvImport({ deck, onDone }: { deck: Deck; onDone: () => void }) {
  const file = useRef<HTMLInputElement>(null)
  const [status, setStatus] = useState<'idle' | 'uploading' | 'running'>('idle')
  const [result, setResult] = useState<ImportResult>()
  const [error, setError] = useState<string>()
  const [jobId, setJobId] = useState<number>()

  useEffect(() => {
    if (jobId === undefined) return
    let live = true
    let delay = 500
    const poll = async () => {
      try {
        const job = await api.job(jobId)
        if (!live) return
        if (job.status === 'queued' || job.status === 'running') {
          delay = Math.min(delay * 1.5, 4000)
          setTimeout(poll, delay)
          return
        }
        setStatus('idle')
        setJobId(undefined)
        if (job.status === 'succeeded' && job.result) {
          setResult(job.result)
          onDone()
        } else {
          setError(job.error ?? 'The import failed.')
        }
      } catch (e) {
        if (live) {
          setStatus('idle')
          setError((e as Error).message)
        }
      }
    }
    poll()
    return () => {
      live = false
    }
  }, [jobId, onDone])

  async function upload(f: File) {
    setStatus('uploading')
    setResult(undefined)
    setError(undefined)
    try {
      const { job_id } = await api.importCSV(deck.id, uuidv7(), f)
      setStatus('running')
      setJobId(job_id)
    } catch (e) {
      setStatus('idle')
      setError((e as Error).message)
    } finally {
      if (file.current) file.current.value = ''
    }
  }

  return (
    <section aria-labelledby="import-title">
      <h2 id="import-title" className="label section-label">
        Import
      </h2>
      <p className="meta">
        A CSV with one word per line: characters, pinyin, meaning. Pinyin and meaning can be left empty; they're filled in from the
        dictionary. Words already in the deck are skipped.
      </p>
      <input
        ref={file}
        type="file"
        accept=".csv,.tsv,.txt,text/csv,text/plain"
        className="visually-hidden"
        id="csv-file"
        onChange={(e) => e.target.files?.[0] && upload(e.target.files[0])}
      />
      <label htmlFor="csv-file" className={`btn btn-block import-btn ${status !== 'idle' ? 'is-busy' : ''}`} aria-disabled={status !== 'idle'}>
        {status === 'uploading' ? 'Uploading…' : status === 'running' ? 'Importing…' : 'Choose a CSV file'}
      </label>
      {result && (
        <div className="import-result" role="status">
          <p>
            {plural(result.imported, 'word')} imported
            {result.skipped > 0 && `, ${result.skipped} already in the deck`}
            {result.failed > 0 && `, ${result.failed} couldn't be read`}.
          </p>
          {result.errors.length > 0 && (
            <ul className="meta import-errors">
              {result.errors.slice(0, 10).map((e) => (
                <li key={e.line}>
                  Line {e.line}: {e.message}
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
      {error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}
    </section>
  )
}

function RenameSheet({ open, onOpenChange, deck, onRenamed }: { open: boolean; onOpenChange: (o: boolean) => void; deck: Deck; onRenamed: (d: Deck) => void }) {
  const [name, setName] = useState(deck.name)
  const [error, setError] = useState<string>()
  async function save(e: React.FormEvent) {
    e.preventDefault()
    try {
      onRenamed(await api.updateDeck(deck.id, { name: name.trim() }))
      onOpenChange(false)
    } catch (err) {
      setError((err as Error).message)
    }
  }
  return (
    <Sheet open={open} onOpenChange={onOpenChange} title="Rename deck">
      <form className="form" onSubmit={save}>
        <input className="input" value={name} maxLength={100} onChange={(e) => setName(e.target.value)} aria-label="Deck name" autoFocus />
        {error && (
          <p className="form-error" role="alert">
            {error}
          </p>
        )}
        <button className="btn btn-primary btn-block">Save</button>
      </form>
    </Sheet>
  )
}

function DeleteSheet({ open, onOpenChange, deck }: { open: boolean; onOpenChange: (o: boolean) => void; deck: Deck }) {
  const [, navigate] = useLocation()
  const { toast } = useApp()
  async function remove() {
    try {
      await api.deleteDeck(deck.id)
      toast(`Deleted ${deck.name}.`)
      navigate('/decks')
    } catch (e) {
      toast((e as Error).message)
    }
  }
  return (
    <Sheet
      open={open}
      onOpenChange={onOpenChange}
      title={`Delete ${deck.name}?`}
      description={`Its ${plural(deck.note_count, 'word')} and their cards are removed. Your review history is kept.`}
    >
      <div className="confirm-row">
        <button className="btn" onClick={() => onOpenChange(false)}>
          Keep it
        </button>
        <button className="btn btn-danger" onClick={remove}>
          Delete deck
        </button>
      </div>
    </Sheet>
  )
}
