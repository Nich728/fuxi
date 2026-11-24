import { useCallback, useMemo, useRef, useState, type ReactNode } from 'react'
import type { Me } from '../api/types'
import { AppContext, type Toast } from './state'

export function AppProvider({ me: initial, children }: { me: Me; children: ReactNode }) {
  const [me, setMe] = useState(initial)
  const [toasts, setToasts] = useState<Toast[]>([])
  const next = useRef(1)

  const dismiss = useCallback((id: number) => setToasts((ts) => ts.filter((t) => t.id !== id)), [])
  const toast = useCallback(
    (message: string, action?: Toast['action']) => {
      const id = next.current++
      setToasts((ts) => [...ts.slice(-2), { id, message, action }])
      setTimeout(() => dismiss(id), action ? 6000 : 3500)
    },
    [dismiss],
  )

  const value = useMemo(() => ({ me, setMe, toasts, toast, dismiss }), [me, toasts, toast, dismiss])
  return <AppContext.Provider value={value}>{children}</AppContext.Provider>
}
