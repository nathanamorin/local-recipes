import { createContext, useCallback, useContext, useMemo, useRef, useState } from 'react'
import { Icon } from './Icon'

const ToastContext = createContext(() => {})

export function useToast() {
  return useContext(ToastContext)
}

// Non-blocking feedback for saves/deletes/errors. Announced via a polite
// live region; an optional action (e.g. Undo) gets its own button.
export function ToastProvider({ children }) {
  const [toasts, setToasts] = useState([])
  const nextId = useRef(1)

  const dismiss = useCallback((id) => setToasts((t) => t.filter((x) => x.id !== id)), [])

  const toast = useCallback(
    (message, { tone = 'default', action, duration = 4000 } = {}) => {
      const id = nextId.current++
      setToasts((t) => [...t.slice(-2), { id, message, tone, action }])
      window.setTimeout(() => dismiss(id), duration)
    },
    [dismiss],
  )

  const value = useMemo(() => toast, [toast])

  return (
    <ToastContext.Provider value={value}>
      {children}
      <div className="toast-region" role="status" aria-live="polite">
        {toasts.map((t) => (
          <div key={t.id} className={`toast toast-${t.tone}`}>
            {t.tone === 'error' ? <Icon name="alert" size={18} /> : <Icon name="check" size={18} />}
            <span className="toast-message">{t.message}</span>
            {t.action && (
              <button
                className="toast-action"
                onClick={() => {
                  t.action.onClick()
                  dismiss(t.id)
                }}
              >
                {t.action.label}
              </button>
            )}
            <button className="icon-button icon-button-sm" aria-label="Dismiss" onClick={() => dismiss(t.id)}>
              <Icon name="x" size={16} />
            </button>
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  )
}
