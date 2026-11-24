/// <reference types="vitest/config" />
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'
import { VitePWA } from 'vite-plugin-pwa'

// The app is served by the Go API from the same origin (SYSTEM_DESIGN §10),
// so in development Vite forwards /v1 to `task run:api` (dev sign-in).
export default defineConfig({
  plugins: [
    react(),
    VitePWA({
      registerType: 'autoUpdate',
      injectRegister: false, // registered in src/main.tsx
      manifest: {
        name: 'Fuxi',
        short_name: 'Fuxi',
        description: 'Mandarin reviews, a dictionary and a tutor.',
        lang: 'en',
        start_url: '/',
        scope: '/',
        display: 'standalone',
        background_color: '#151615',
        theme_color: '#151615',
        icons: [
          { src: '/icon-192.png', sizes: '192x192', type: 'image/png' },
          { src: '/icon-512.png', sizes: '512x512', type: 'image/png' },
          { src: '/icon-maskable-512.png', sizes: '512x512', type: 'image/png', purpose: 'maskable' },
        ],
      },
      workbox: {
        // The app shell works offline; API data does not until M5.
        globPatterns: ['**/*.{js,css,html,png,webmanifest}'],
        navigateFallback: '/index.html',
        // Never answer these from the cache: the API, Cloudflare Access's own
        // pages, and /sign-in, which must reach the network so Access can
        // renew an expired session (src/api/client.ts).
        navigateFallbackDenylist: [/^\/v1\//, /^\/cdn-cgi\//, /^\/sign-in/, /^\/healthz/, /^\/readyz/],
        runtimeCaching: [
          {
            urlPattern: ({ url }) => url.origin === 'https://fonts.googleapis.com',
            handler: 'StaleWhileRevalidate',
            options: { cacheName: 'font-css' },
          },
          {
            urlPattern: ({ url }) => url.origin === 'https://fonts.gstatic.com',
            handler: 'CacheFirst',
            options: {
              cacheName: 'font-files',
              expiration: { maxEntries: 300, maxAgeSeconds: 365 * 24 * 60 * 60 },
              cacheableResponse: { statuses: [0, 200] },
            },
          },
        ],
      },
    }),
  ],
  server: {
    proxy: { '/v1': 'http://localhost:8080' },
  },
  build: {
    target: 'es2022',
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    css: false,
  },
})
