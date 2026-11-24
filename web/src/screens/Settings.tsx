import { useMemo, useState } from 'react'
import { api } from '../api/client'
import type { Me } from '../api/types'
import { useApp } from '../app/state'
import { Pinyin, Screen, TopBar } from '../components/ui'
import { getTheme, setTheme, type Theme } from '../lib/theme'
import './deck.css'

const HOURS = Array.from({ length: 24 }, (_, h) => h)

function hourLabel(h: number): string {
  return new Date(2000, 0, 1, h).toLocaleTimeString(undefined, { hour: 'numeric' })
}

/** Settings (PRD C1, C2, R5) and sign out (ADR-0018). */
export function SettingsScreen() {
  const { me, setMe, toast } = useApp()
  const [saving, setSaving] = useState(false)
  const [theme, setThemeState] = useState<Theme>(getTheme)
  const zones = useMemo(() => {
    try {
      return Intl.supportedValuesOf('timeZone')
    } catch {
      return [me.timezone]
    }
  }, [me.timezone])

  async function save(patch: Parameters<typeof api.updateSettings>[0]) {
    setSaving(true)
    try {
      setMe(await api.updateSettings(patch))
    } catch (e) {
      toast((e as Error).message)
    } finally {
      setSaving(false)
    }
  }

  const deviceZone = Intl.DateTimeFormat().resolvedOptions().timeZone
  return (
    <Screen>
      <TopBar back="/" />
      <header className="page-head">
        <h1>Settings</h1>
      </header>

      <h2 className="label section-label">Study day</h2>
      <div className="settings-list">
        <label className="setting">
          <span>
            Day starts at
            <span className="meta setting-help">Late-night reviews count for the evening before.</span>
          </span>
          <select
            className="input setting-input"
            value={me.day_starts_at}
            disabled={saving}
            onChange={(e) => save({ day_starts_at: Number(e.target.value) })}
          >
            {HOURS.map((h) => (
              <option key={h} value={h}>
                {hourLabel(h)}
              </option>
            ))}
          </select>
        </label>
        <label className="setting">
          <span>
            Time zone
            {deviceZone && deviceZone !== me.timezone && (
              <button type="button" className="link-btn setting-help" onClick={() => save({ timezone: deviceZone })}>
                Use this device's: {deviceZone}
              </button>
            )}
          </span>
          <select
            className="input setting-input"
            value={me.timezone}
            disabled={saving}
            onChange={(e) => save({ timezone: e.target.value })}
          >
            {zones.map((z) => (
              <option key={z} value={z}>
                {z.replaceAll('_', ' ')}
              </option>
            ))}
          </select>
        </label>
      </div>

      <h2 className="label section-label">Display</h2>
      <div className="settings-list">
        <label className="setting">
          <span>
            Theme
            <span className="meta setting-help">Kept on this device.</span>
          </span>
          <select
            className="input setting-input"
            value={theme}
            onChange={(e) => {
              const t = e.target.value as Theme
              setTheme(t)
              setThemeState(t)
            }}
          >
            <option value="ink">Ink</option>
            <option value="paper">Paper</option>
            <option value="auto">Match device</option>
          </select>
        </label>
        <label className="setting">
          <span>
            Tone colours
            <span className="meta setting-help">
              <Pinyin text="mā má mǎ mà" /> Tone marks always stay.
            </span>
          </span>
          <input
            type="checkbox"
            role="switch"
            aria-label="Tone colours"
            className="switch"
            checked={me.settings.tone_colours}
            disabled={saving}
            onChange={(e) => save({ tone_colours: e.target.checked })}
          />
        </label>
        <label className="setting">
          <span>
            Tutor explains in
            <span className="meta setting-help">For the AI tutor, coming soon. Chinese and pinyin stay as they are.</span>
          </span>
          <select
            className="input setting-input"
            value={me.settings.explanation_language}
            disabled={saving}
            onChange={(e) => save({ explanation_language: e.target.value as Me['settings']['explanation_language'] })}
          >
            <option value="en">English</option>
            <option value="id">Indonesian</option>
          </select>
        </label>
      </div>

      <h2 className="label section-label">Account</h2>
      <div className="settings-list">
        <div className="setting">
          <span>
            Signed in as
            <span className="meta setting-help">{me.email}</span>
          </span>
          {/* Cloudflare Access ends the session; the next visit asks to sign in again. */}
          <a className="btn" href="/cdn-cgi/access/logout">
            Sign out
          </a>
        </div>
      </div>

      <h2 className="label section-label">Credits</h2>
      <p className="meta credits">
        Dictionary: <a href="https://cc-cedict.org/wiki/">CC-CEDICT</a> (CC BY-SA 4.0). Example sentences:{' '}
        <a href="https://tatoeba.org">Tatoeba</a> (CC BY 2.0 FR). HSK 3.0 word lists, levels 7–9 shown as one band.
      </p>
    </Screen>
  )
}
