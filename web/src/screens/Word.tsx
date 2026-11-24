import { useState } from 'react'
import { api } from '../api/client'
import { uuidv7 } from '../api/ids'
import type { Deck, DictEntryDetail, Note } from '../api/types'
import { lastDeck, rememberDeck, searchHref, useApp } from '../app/state'
import { CheckIcon, PlusIcon } from '../components/Icons'
import { ErrorState, Loading, Pinyin, Screen, Sheet, TopBar } from '../components/ui'
import { hskLabel, plural, simplifiedExamples } from '../lib/format'
import { useResource } from '../lib/useResource'
import './word.css'

interface WordData {
  entry: DictEntryDetail
  notes: Note[]
  known: boolean
  decks: Deck[]
}

/** A word's page (PRD L2): pinyin, meanings, HSK level and real example sentences. */
export function WordScreen({ id }: { id: number }) {
  const res = useResource<WordData>(
    () =>
      Promise.all([api.entry(id), api.notesForEntry(id), api.knowledge([id]), api.decks()]).then(
        ([entry, notes, knowledge, decks]) => ({ entry, notes, known: knowledge.some((k) => k.status === 'known'), decks }),
      ),
    [id],
  )

  return (
    <Screen>
      <TopBar back={searchHref()} />
      {res.error && !res.data ? (
        <ErrorState error={res.error} retry={res.reload} />
      ) : !res.data ? (
        <Loading />
      ) : (
        <WordBody data={res.data} update={res.set} />
      )}
    </Screen>
  )
}

function WordBody({ data, update }: { data: WordData; update: (d: WordData) => void }) {
  const { entry, notes, known, decks } = data
  const { toast } = useApp()
  const [adding, setAdding] = useState(false)
  const [busy, setBusy] = useState(false)
  const deckName = new Map(decks.map((d) => [d.id, d.name]))
  const hsk = hskLabel(entry.hsk_level)
  const examples = simplifiedExamples(entry.examples, entry.simplified)

  async function markKnown() {
    setBusy(true)
    try {
      const r = await api.markKnown(entry.id)
      update({ ...data, known: true })
      toast(
        r.cards_suspended ? `Marked as known. ${plural(r.cards_suspended, 'card')} paused.` : 'Marked as known.',
        { label: 'Undo', run: () => unmark() },
      )
    } catch (e) {
      toast((e as Error).message)
    } finally {
      setBusy(false)
    }
  }

  async function unmark() {
    setBusy(true)
    try {
      await api.unmarkKnown(entry.id)
      update({ ...data, known: false })
    } catch (e) {
      toast((e as Error).message)
    } finally {
      setBusy(false)
    }
  }

  return (
    <article className="word">
      <header className="word-head">
        <h1 className="hanzi word-hanzi" lang="zh-Hans">
          {entry.simplified}
        </h1>
        {entry.traditional !== entry.simplified && (
          <p className="meta" lang="zh-Hant">
            Traditional <span className="hanzi word-trad">{entry.traditional}</span>
          </p>
        )}
        <Pinyin text={entry.pinyin} className="word-pinyin" />
        {hsk && <p className="gold word-hsk">{hsk}</p>}
      </header>

      <ol className="definitions">
        {entry.definitions.map((d, i) => (
          <li key={i}>{d}</li>
        ))}
      </ol>

      <div className="word-actions">
        <button className="btn btn-primary btn-block" onClick={() => setAdding(true)}>
          <PlusIcon size={20} /> Add to a deck
        </button>
        {known ? (
          <div className="known-row">
            <span className="known-pill">
              <CheckIcon size={18} /> You know this word
            </span>
            <button className="btn btn-quiet" onClick={unmark} disabled={busy}>
              Undo
            </button>
          </div>
        ) : (
          <button className="btn btn-block" onClick={markKnown} disabled={busy}>
            I already know this
          </button>
        )}
      </div>

      {notes.length > 0 && (
        <p className="meta in-decks">
          In {notes.map((n) => deckName.get(n.deck_id) ?? 'a deck').join(', ')}
        </p>
      )}

      {examples.length > 0 && (
        <section aria-labelledby="examples">
          <h2 id="examples" className="section-label">
            Examples
          </h2>
          <ul className="examples">
            {examples.slice(0, 6).map((ex) => (
              <li key={ex.id}>
                <p className="hanzi example-line" lang="zh-Hans">
                  {ex.zh}
                </p>
                {ex.en && <p className="meta">{ex.en}</p>}
                {ex.source === 'ai' && <p className="meta gold">Written by AI</p>}
              </li>
            ))}
          </ul>
          <p className="meta credit">Examples from Tatoeba (CC BY 2.0 FR).</p>
        </section>
      )}

      <AddToDeck
        open={adding}
        onOpenChange={setAdding}
        entry={entry}
        decks={decks}
        inDecks={new Set(notes.map((n) => n.deck_id))}
        onAdded={(note, deck, newDeck) => {
          rememberDeck(deck.id)
          update({ ...data, notes: [...notes, note], decks: newDeck ? [...decks, deck] : decks })
          setAdding(false)
          toast(`Added to ${deck.name}.`)
        }}
      />
    </article>
  )
}

/** The deck sheet (PRD flow 7.3): defaults to the deck used last. */
function AddToDeck({
  open,
  onOpenChange,
  entry,
  decks,
  inDecks,
  onAdded,
}: {
  open: boolean
  onOpenChange: (o: boolean) => void
  entry: DictEntryDetail
  decks: Deck[]
  inDecks: Set<string>
  onAdded: (note: Note, deck: Deck, newDeck: boolean) => void
}) {
  const available = decks.filter((d) => !inDecks.has(d.id))
  const preferred = available.find((d) => d.id === lastDeck())?.id ?? available[0]?.id ?? 'new'
  const [choice, setChoice] = useState<string>(preferred)
  const [name, setName] = useState('')
  const [error, setError] = useState<string>()
  const [busy, setBusy] = useState(false)
  const selected = available.some((d) => d.id === choice) ? choice : preferred

  async function add() {
    setBusy(true)
    setError(undefined)
    try {
      let deck = decks.find((d) => d.id === selected)
      const isNew = !deck
      if (!deck) {
        if (!name.trim()) {
          setError('Give the new deck a name.')
          return
        }
        deck = await api.createDeck(uuidv7(), name.trim())
      }
      const note = await api.createNote(deck.id, { id: uuidv7(), entry_id: entry.id })
      onAdded(note, deck, isNew)
      setName('')
    } catch (e) {
      setError((e as Error).message)
    } finally {
      setBusy(false)
    }
  }

  return (
    <Sheet open={open} onOpenChange={onOpenChange} title={`Add ${entry.simplified} to a deck`}>
      <fieldset className="deck-choices">
        <legend className="visually-hidden">Deck</legend>
        {decks.map((d) => {
          const here = inDecks.has(d.id)
          return (
            <label key={d.id} className={`deck-choice ${here ? 'is-disabled' : ''}`}>
              <input
                type="radio"
                name="deck"
                value={d.id}
                checked={selected === d.id}
                disabled={here}
                onChange={() => setChoice(d.id)}
              />
              <span className="row-main">
                <span className="row-title">{d.name}</span>
                <span className="row-sub num">{here ? 'Already in this deck' : plural(d.note_count, 'word')}</span>
              </span>
            </label>
          )
        })}
        <label className="deck-choice">
          <input type="radio" name="deck" value="new" checked={selected === 'new'} onChange={() => setChoice('new')} />
          <span className="row-main">
            <span className="row-title">New deck</span>
          </span>
        </label>
        {selected === 'new' && (
          <input
            className="input new-deck-name"
            placeholder="Deck name, e.g. Class week 6"
            value={name}
            maxLength={100}
            onChange={(e) => setName(e.target.value)}
            aria-label="New deck name"
            autoFocus
          />
        )}
      </fieldset>
      {error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}
      <button className="btn btn-primary btn-block" onClick={add} disabled={busy}>
        {busy ? 'Adding…' : 'Add'}
      </button>
      <p className="meta sheet-foot">Pinyin and meaning come from the dictionary.</p>
    </Sheet>
  )
}
