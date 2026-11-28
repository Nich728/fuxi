import { useState } from 'react'
import { api } from '../api/client'
import { uuidv7 } from '../api/ids'
import type { Deck, DictEntryDetail, Note } from '../api/types'
import { takeWord, type WordData } from '../lib/words'
import { lastDeck, rememberDeck, searchHref, useApp } from '../app/state'
import { CheckIcon } from '../components/Icons'
import { ErrorState, Loading, Pinyin, Roll, Screen, Sheet, TopBar } from '../components/ui'
import { hskLabel, plural, simplifiedExamples } from '../lib/format'
import { useResource } from '../lib/useResource'
import './word.css'

const pause = (ms: number) => new Promise((r) => setTimeout(r, ms))

/** A word's page (PRD L2): pinyin, meanings, HSK level and real example sentences. */
export function WordScreen({ id }: { id: number }) {
  const res = useResource<WordData>(() => takeWord(id), [id])

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
  // "I already know this": the link is struck through before "Known" replaces it.
  const [striking, setStriking] = useState(false)
  const [paused, setPaused] = useState(0)
  // The deck a word was just added to: the button shows a tick for a moment.
  const [added, setAdded] = useState<Deck>()
  const hsk = hskLabel(entry.hsk_level)
  const examples = simplifiedExamples(entry.examples, entry.simplified)
  const inDecks = decks.filter((d) => notes.some((n) => n.deck_id === d.id))

  async function markKnown() {
    setBusy(true)
    setStriking(true)
    try {
      const [r] = await Promise.all([api.markKnown(entry.id), pause(320)])
      setPaused(r.cards_suspended)
      update({ ...data, known: true })
      toast(
        r.cards_suspended ? `Marked as known. ${plural(r.cards_suspended, 'card')} paused.` : 'Marked as known.',
        { label: 'Undo', run: () => unmark() },
      )
    } catch (e) {
      toast((e as Error).message)
    } finally {
      setBusy(false)
      setStriking(false)
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
        <p className="word-sub">
          <Pinyin text={entry.pinyin} className="word-pinyin" />
          {hsk && <span className="label gold">{hsk}</span>}
        </p>
        {entry.traditional !== entry.simplified && (
          <p className="meta" lang="zh-Hant">
            Traditional <span className="hanzi word-trad">{entry.traditional}</span>
          </p>
        )}
      </header>

      <ol className="definitions serif">
        {entry.definitions.map((d, i) => (
          <li key={i}>{d}</li>
        ))}
      </ol>

      <div className="word-actions">
        <button className={`btn btn-primary btn-block ${added ? 'is-added' : ''}`} onClick={() => setAdding(true)}>
          {added ? (
            <span className="added">
              <CheckIcon size={18} />
              Added to {added.name}
            </span>
          ) : (
            'Add to a deck'
          )}
        </button>
        {known ? (
          <p className="known">
            <span className="known-state">
              <CheckIcon size={16} /> Known{paused > 0 && ` · ${plural(paused, 'card')} paused`}
            </span>
            <button className="link-btn" onClick={unmark} disabled={busy}>
              Undo
            </button>
          </p>
        ) : (
          <button className={`link-btn know-link ${striking ? 'is-striking' : ''}`} onClick={markKnown} disabled={busy}>
            I already know this
          </button>
        )}
      </div>

      {inDecks.length > 0 && (
        <ul className="in-decks">
          {inDecks.map((d) => (
            <li key={d.id}>
              <span className="label">In</span> <span className="in-deck-name">{d.name}</span>{' '}
              <span className="meta">
                <Roll value={d.note_count} from={added?.id === d.id ? d.note_count - 1 : undefined} />{' '}
                {d.note_count === 1 ? 'word' : 'words'}
              </span>
            </li>
          ))}
        </ul>
      )}

      {examples.length > 0 && (
        <section aria-labelledby="examples">
          <h2 id="examples" className="label section-label">
            Examples
          </h2>
          <ul className="examples">
            {examples.slice(0, 6).map((ex) => (
              <li key={ex.id}>
                <p className="hanzi example-line" lang="zh-Hans">
                  {ex.zh}
                </p>
                {ex.en && <p className="serif example-en">{ex.en}</p>}
                {ex.source === 'ai' && <p className="label gold">Written by AI</p>}
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
          const counted = { ...deck, note_count: newDeck ? 1 : deck.note_count + 1 }
          update({
            ...data,
            notes: [...notes, note],
            decks: newDeck ? [...decks, counted] : decks.map((d) => (d.id === deck.id ? counted : d)),
          })
          setAdding(false)
          setAdded(counted)
          setTimeout(() => setAdded(undefined), 2200)
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
