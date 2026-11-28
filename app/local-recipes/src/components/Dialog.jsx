import { useEffect, useRef } from 'react'
import { Icon } from './Icon'

// Native <dialog> gives focus trapping, Escape-to-close and an inert
// background for free; this just syncs it with React state.
export function Dialog({ open, onClose, title, children, footer, size = 'sm' }) {
  const ref = useRef(null)

  useEffect(() => {
    const el = ref.current
    if (!el) return
    if (open && !el.open) el.showModal()
    if (!open && el.open) el.close()
  }, [open])

  return (
    <dialog
      ref={ref}
      className={`dialog dialog-${size}`}
      // Only user dismissals (Escape, backdrop click) call onClose. The
      // native "close" event also fires when we close it programmatically
      // (open -> false), which must not re-trigger the parent's handler.
      onCancel={(e) => {
        e.preventDefault()
        onClose()
      }}
      onClick={(e) => {
        if (e.target === ref.current) onClose()
      }}
      aria-labelledby="dialog-title"
    >
      {open && (
        <div className="dialog-inner">
          <header className="dialog-header">
            <h2 id="dialog-title">{title}</h2>
            <button type="button" className="icon-button" aria-label="Close" onClick={onClose}>
              <Icon name="x" />
            </button>
          </header>
          <div className="dialog-body">{children}</div>
          {footer && <footer className="dialog-footer">{footer}</footer>}
        </div>
      )}
    </dialog>
  )
}

export function ConfirmDialog({ open, onClose, onConfirm, title, message, confirmLabel = 'Delete', busy }) {
  return (
    <Dialog
      open={open}
      onClose={onClose}
      title={title}
      footer={
        <>
          <button type="button" className="btn btn-ghost" onClick={onClose} disabled={busy}>
            Cancel
          </button>
          <button type="button" className="btn btn-danger" onClick={onConfirm} disabled={busy} autoFocus>
            {busy ? 'Working…' : confirmLabel}
          </button>
        </>
      }
    >
      <p className="muted">{message}</p>
    </Dialog>
  )
}
