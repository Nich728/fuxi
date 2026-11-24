import { useEffect, useRef, useState } from 'react'
import { Link, useSearchParams } from 'wouter'
import { api } from '../api/client'
import type { SearchResult } from '../api/types'
import { rememberSearch } from '../app/state'
import { SearchIcon } from '../components/Icons'
import { Empty, ErrorState, Loading, Pinyin, Screen } from '../components/ui'
import { hskLabel } from '../lib/format'
import './search.css'

/** Dictionary search (PRD L1): characters, pinyin with or without tones, or English. */
export function SearchScreen() {
  const [params, setParams] = useSearchParams()
  const [q, setQ] = useState(params.get('q') ?? '')
  // The latest answer, tagged with the query it answers.
  const [answer, setAnswer] = useState<{ query: string; result?: SearchResult; error?: Error }>()
  const input = useRef<HTMLInputElement>(null)
  const query = q.trim()
  const current = answer && answer.query === query ? answer : undefined
  const result = query ? (current?.result ?? answer?.result) : undefined // keep old results while typing
  const error = current?.error
  const loading = query !== '' && !current

  useEffect(() => {
    if (!query) return
    let live = true
    // Wait for a pause in typing; keep the query in the URL so Back returns to it.
    const t = setTimeout(() => {
      setParams({ q: query }, { replace: true })
      rememberSearch(query)
      api.search(query).then(
        (r) => live && setAnswer({ query, result: r }),
        (e: Error) => live && setAnswer({ query, error: e }),
      )
    }, 180)
    return () => {
      live = false
      clearTimeout(t)
    }
  }, [query, setParams])

  return (
    <Screen>
      <form
        className="search-form"
        role="search"
        onSubmit={(e) => {
          e.preventDefault()
          input.current?.blur() // closes the phone keyboard to show results
        }}
      >
        <label className="search-field">
          <SearchIcon size={20} />
          <span className="visually-hidden">Search the dictionary</span>
          <input
            ref={input}
            className="search-input"
            type="search"
            enterKeyHint="search"
            autoFocus={!q}
            autoCapitalize="off"
            autoCorrect="off"
            spellCheck={false}
            placeholder="词典, cidian or dictionary"
            value={q}
            onChange={(e) => setQ(e.target.value)}
          />
        </label>
      </form>

      {!query ? (
        <div className="search-hint meta">
          <p>Search by characters, by pinyin with or without tones (ma, ma3), or in English.</p>
        </div>
      ) : error ? (
        <ErrorState error={error} />
      ) : !result ? (
        loading && <Loading label="Searching" />
      ) : result.results.length === 0 ? (
        <Empty glyph="无" title="No matches">
          <p>Try fewer characters, pinyin without tones, or an English word.</p>
        </Empty>
      ) : (
        <>
          {result.segmented && <p className="meta search-note">No exact match, so here are the words in that phrase.</p>}
          <ul className="rows results" aria-busy={loading}>
            {result.results.map((e) => (
              <li key={e.id}>
                <Link href={`/word/${e.id}`} className="row result">
                  <span className="hanzi result-hanzi">{e.simplified}</span>
                  <div className="row-main">
                    <Pinyin text={e.pinyin} className="result-pinyin" />
                    <div className="row-sub">{e.definitions.join('; ')}</div>
                  </div>
                  {e.hsk_level != null && <span className="gold result-hsk">{hskLabel(e.hsk_level)}</span>}
                </Link>
              </li>
            ))}
          </ul>
        </>
      )}
    </Screen>
  )
}
