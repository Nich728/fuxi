import { useEffect, useState } from 'react'
import { Route, Switch } from 'wouter'
import { api } from './api/client'
import { deviceId, deviceName } from './api/ids'
import type { Me } from './api/types'
import { AppProvider } from './app/context'
import { ErrorState, Loading, Toasts } from './components/ui'
import { DeckScreen } from './screens/Deck'
import { DecksScreen } from './screens/Decks'
import { HomeScreen } from './screens/Home'
import { ProgressScreen } from './screens/Progress'
import { ReviewScreen } from './screens/Review'
import { SearchScreen } from './screens/Search'
import { SettingsScreen } from './screens/Settings'
import { WordScreen } from './screens/Word'

/**
 * Cloudflare Access signs people in before a request ever reaches Fuxi
 * (ADR-0018). After a renewed sign-in Access returns to /sign-in?next=…;
 * by then the session is fresh, so go back to where the user was.
 */
function returnAfterSignIn(): boolean {
  if (location.pathname !== '/sign-in') return false
  const next = new URLSearchParams(location.search).get('next') ?? '/'
  // Only same-origin paths: never an open redirect.
  location.replace(next.startsWith('/') && !next.startsWith('//') ? next : '/')
  return true
}

export default function App() {
  const [me, setMe] = useState<Me>()
  const [error, setError] = useState<Error>()
  const [attempt, setAttempt] = useState(0)

  useEffect(() => {
    if (returnAfterSignIn()) return
    let live = true
    api.me().then(
      (m) => {
        if (!live) return
        setMe(m)
        // Register this browser as a device; reviews name it (SYSTEM_DESIGN §6.2).
        api.registerDevice(deviceId(), deviceName()).catch(() => {})
      },
      (e: Error) => live && setError(e),
    )
    return () => {
      live = false
    }
  }, [attempt])

  if (error) {
    return (
      <main className="screen">
        <ErrorState
          error={error}
          retry={() => {
            setError(undefined)
            setAttempt((a) => a + 1)
          }}
        />
      </main>
    )
  }
  if (!me) {
    return (
      <main className="screen">
        <Loading label="Opening Fuxi" />
      </main>
    )
  }

  return (
    <AppProvider me={me}>
      <Switch>
        <Route path="/" component={HomeScreen} />
        <Route path="/review" component={ReviewScreen} />
        <Route path="/search" component={SearchScreen} />
        <Route path="/word/:id">{(p) => <WordScreen id={Number(p.id)} />}</Route>
        <Route path="/decks" component={DecksScreen} />
        <Route path="/decks/:id">{(p) => <DeckScreen id={p.id} />}</Route>
        <Route path="/progress" component={ProgressScreen} />
        <Route path="/settings" component={SettingsScreen} />
        <Route>
          <HomeScreen />
        </Route>
      </Switch>
      <Toasts />
    </AppProvider>
  )
}
