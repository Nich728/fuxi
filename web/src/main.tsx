import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
// Tokens and base styles first, so screen styles can refine them.
import './index.css'
import App from './App.tsx'
import { finishSplash, shouldPlaySplash } from './lib/motion'

// The launch splash plays on a fresh start only (system.md motion).
const splash = shouldPlaySplash()
if (!splash) finishSplash()

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App splash={splash} />
  </StrictMode>,
)

// The service worker caches the app shell so the installed app opens
// instantly; a new deploy is picked up on the next launch (registerType
// autoUpdate). Not in development, where Vite serves fresh files.
if (import.meta.env.PROD && 'serviceWorker' in navigator) {
  import('virtual:pwa-register').then(({ registerSW }) => registerSW({ immediate: true }))
}
