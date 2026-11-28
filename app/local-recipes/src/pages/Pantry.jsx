import { useDeferredValue, useMemo, useState } from 'react'
import { api } from '../lib/api'
import { invalidate, setCached, useResource } from '../lib/resource'
import { displayUnit, pluralize } from '../lib/format'
import { Icon } from '../components/Icon'
import { ConfirmDialog, Dialog } from '../components/Dialog'
import { useToast } from '../components/Toast'
import { EmptyState, ErrorState, PageHeader, Skeleton } from '../components/ui'

function statusOf(ing) {
  const s = ing.stock
  if (!s) return 'untracked'
  if (s.quantity_on_hand <= 0) return 'out'
  if (s.low_stock_threshold > 0 && s.quantity_on_hand <= s.low_stock_threshold) return 'low'
  return 'ok'
}

const STATUS = {
  ok: { label: 'In stock', tone: 'ok' },
  low: { label: 'Low', tone: 'warn' },
  out: { label: 'Out', tone: 'danger' },
  untracked: { label: 'Not tracked', tone: 'muted' },
}

const FILTERS = [
  { id: 'all', label: 'All' },
  { id: 'attention', label: 'Low & out' },
  { id: 'ok', label: 'In stock' },
]

function fmtQty(n) {
  return String(Math.round(n * 100) / 100)
}

function unitOf(ing) {
  return (ing.stock && ing.stock.unit) || ing.default_unit || ''
}

function usePantryMutations() {
  const toast = useToast()
  const patch = (id, fn) => setCached('ingredients', (list) => (list || []).map((i) => (i.id === id ? fn(i) : i)))
  const run = async (promise, okMessage) => {
    try {
      await promise
      if (okMessage) toast(okMessage)
    } catch (e) {
      toast(e.message, { tone: 'error' })
    } finally {
      invalidate('ingredients')
      invalidate('shopping')
    }
  }
  return {
    adjust: (ing, delta) => {
      const unit = unitOf(ing)
      patch(ing.id, (i) => ({
        ...i,
        stock: {
          ...(i.stock || { low_stock_threshold: 0, unit }),
          quantity_on_hand: Math.max(0, ((i.stock && i.stock.quantity_on_hand) || 0) + delta),
        },
      }))
      return run(api.adjustStock(ing.id, delta, unit))
    },
    set: (ing, quantity, threshold) => {
      const unit = unitOf(ing)
      patch(ing.id, (i) => ({
        ...i,
        stock: { ...(i.stock || { unit }), quantity_on_hand: quantity, low_stock_threshold: threshold },
      }))
      return run(
        api.setStock(ing.id, quantity, unit).then(() => api.setLowStockThreshold(ing.id, threshold)),
        `Updated ${ing.name}`,
      )
    },
    remove: (ing) => {
      setCached('ingredients', (list) => (list || []).filter((i) => i.id !== ing.id))
      return run(api.deleteIngredient(ing.id), `Removed ${ing.name}`)
    },
  }
}

function PantryItem({ ing, onOpen, mutations }) {
  const status = statusOf(ing)
  const qty = ing.stock ? ing.stock.quantity_on_hand : null
  const unit = displayUnit(unitOf(ing))
  return (
    <li className={`pantry-item status-${status}`}>
      <button className="pantry-item-main" onClick={() => onOpen(ing)} aria-label={`${ing.name}: edit stock`}>
        <span className="pantry-item-name">{ing.name}</span>
        <span className={`pill pill-${STATUS[status].tone}`}>{STATUS[status].label}</span>
      </button>
      {qty === null ? (
        <button className="track-btn" onClick={() => onOpen(ing)}>
          Set amount
        </button>
      ) : (
        <div className="pantry-item-qty">
          <button
            className="qty-btn"
            aria-label={`Use one ${unit || 'unit'} of ${ing.name}`}
            onClick={() => mutations.adjust(ing, -1)}
            disabled={qty <= 0}
          >
            <Icon name="minus" size={16} />
          </button>
          <span className="qty-value" aria-live="polite">
            {fmtQty(qty)}
            {unit && <span className="qty-unit"> {unit}</span>}
          </span>
          <button className="qty-btn" aria-label={`Add one ${unit || 'unit'} of ${ing.name}`} onClick={() => mutations.adjust(ing, 1)}>
            <Icon name="plus" size={16} />
          </button>
        </div>
      )}
    </li>
  )
}

function EditStockDialog({ ing, onClose, mutations }) {
  const [qty, setQty] = useState(ing && ing.stock ? fmtQty(ing.stock.quantity_on_hand) : '0')
  const [threshold, setThreshold] = useState(ing && ing.stock ? fmtQty(ing.stock.low_stock_threshold) : '0')
  const [confirm, setConfirm] = useState(false)
  if (!ing) return null
  const unit = displayUnit(unitOf(ing))
  return (
    <>
      <Dialog
        open={!confirm}
        onClose={onClose}
        title={ing.name}
        footer={
          <>
            <button type="button" className="btn btn-ghost btn-danger-text" onClick={() => setConfirm(true)}>
              <Icon name="trash" size={16} /> Remove
            </button>
            <span className="spacer" />
            <button type="button" className="btn btn-ghost" onClick={onClose}>
              Cancel
            </button>
            <button
              type="submit"
              form="stock-form"
              className="btn btn-primary"
            >
              Save
            </button>
          </>
        }
      >
        <form
          id="stock-form"
          className="stack"
          onSubmit={(e) => {
            e.preventDefault()
            mutations.set(ing, Math.max(0, Number(qty) || 0), Math.max(0, Number(threshold) || 0))
            onClose()
          }}
        >
          {ing.category && <p className="muted dialog-subtitle">{ing.category}</p>}
          <label className="field">
            <span className="field-label">On hand{unit ? ` (${unit})` : ''}</span>
            <input inputMode="decimal" value={qty} onChange={(e) => setQty(e.target.value.replace(/[^\d.]/g, ''))} autoFocus />
          </label>
          <label className="field">
            <span className="field-label">Warn me when at or below</span>
            <input inputMode="decimal" value={threshold} onChange={(e) => setThreshold(e.target.value.replace(/[^\d.]/g, ''))} />
            <span className="field-hint">Set to 0 to turn off low-stock alerts.</span>
          </label>
        </form>
      </Dialog>
      <ConfirmDialog
        open={confirm}
        onClose={() => setConfirm(false)}
        onConfirm={() => {
          mutations.remove(ing)
          onClose()
        }}
        title={`Remove ${ing.name}?`}
        message="It will be removed from your pantry. Recipes that use it keep their ingredient lists."
        confirmLabel="Remove"
      />
    </>
  )
}

function AddItemDialog({ open, onClose, categories }) {
  const toast = useToast()
  const [form, setForm] = useState({ name: '', category: '', default_unit: '' })
  const [busy, setBusy] = useState(false)
  const set = (k) => (e) => setForm((f) => ({ ...f, [k]: e.target.value }))
  const submit = async (e) => {
    e.preventDefault()
    if (!form.name.trim()) return
    setBusy(true)
    try {
      await api.addIngredient({ name: form.name.trim(), category: form.category.trim(), default_unit: form.default_unit.trim() })
      invalidate('ingredients')
      invalidate('shopping')
      toast(`Added ${form.name.trim()}`)
      setForm({ name: '', category: '', default_unit: '' })
      onClose()
    } catch (err) {
      toast(err.message, { tone: 'error' })
    } finally {
      setBusy(false)
    }
  }
  return (
    <Dialog
      open={open}
      onClose={onClose}
      title="Add to pantry"
      footer={
        <>
          <button type="button" className="btn btn-ghost" onClick={onClose}>
            Cancel
          </button>
          <button type="submit" form="add-item-form" className="btn btn-primary" disabled={busy || !form.name.trim()}>
            {busy ? 'Adding…' : 'Add item'}
          </button>
        </>
      }
    >
      <form id="add-item-form" className="stack" onSubmit={submit}>
        <label className="field">
          <span className="field-label">Name</span>
          <input value={form.name} onChange={set('name')} placeholder="eggs" autoFocus />
        </label>
        <div className="field-grid field-grid-2">
          <label className="field">
            <span className="field-label">Category</span>
            <input value={form.category} onChange={set('category')} list="pantry-categories" placeholder="dairy" />
            <datalist id="pantry-categories">
              {categories.map((c) => (
                <option key={c} value={c} />
              ))}
            </datalist>
          </label>
          <label className="field">
            <span className="field-label">Unit</span>
            <input value={form.default_unit} onChange={set('default_unit')} placeholder="count" />
          </label>
        </div>
      </form>
    </Dialog>
  )
}

function PantrySkeleton() {
  return (
    <div className="pantry-group" aria-hidden="true">
      <Skeleton style={{ height: 14, width: 80, marginBottom: 12 }} />
      <ul className="pantry-list">
        {Array.from({ length: 6 }, (_, i) => (
          <li key={i} className="pantry-item">
            <Skeleton style={{ height: 18, width: '50%' }} />
            <Skeleton style={{ height: 32, width: 120 }} />
          </li>
        ))}
      </ul>
    </div>
  )
}

export default function Pantry() {
  const { data: items, error, loading, reload } = useResource('ingredients', api.listIngredients)
  const mutations = usePantryMutations()
  const [query, setQuery] = useState('')
  const deferredQuery = useDeferredValue(query)
  const [filter, setFilter] = useState('all')
  const [category, setCategory] = useState('')
  const [editing, setEditing] = useState(null)
  const [adding, setAdding] = useState(false)

  const categories = useMemo(
    () => [...new Set((items || []).map((i) => i.category).filter(Boolean))].sort((a, b) => a.localeCompare(b)),
    [items],
  )
  const attentionCount = useMemo(() => (items || []).filter((i) => ['low', 'out'].includes(statusOf(i))).length, [items])

  const groups = useMemo(() => {
    const q = deferredQuery.trim().toLowerCase()
    const filtered = (items || []).filter((i) => {
      const st = statusOf(i)
      if (q && !i.name.toLowerCase().includes(q)) return false
      if (category && i.category !== category) return false
      if (filter === 'attention' && !['low', 'out'].includes(st)) return false
      if (filter === 'ok' && st !== 'ok') return false
      return true
    })
    const byCat = new Map()
    for (const i of filtered) {
      const c = i.category || 'Uncategorized'
      if (!byCat.has(c)) byCat.set(c, [])
      byCat.get(c).push(i)
    }
    return [...byCat.entries()]
      .sort(([a], [b]) => (a === 'Uncategorized') - (b === 'Uncategorized') || a.localeCompare(b))
      .map(([c, list]) => [c, list.sort((a, b) => a.name.localeCompare(b.name))])
  }, [items, deferredQuery, filter, category])

  const hasItems = items && items.length > 0

  return (
    <div className="page">
      <PageHeader
        title="Pantry"
        subtitle={
          hasItems
            ? `${pluralize(items.length, 'item')}${attentionCount ? ` · ${attentionCount} need restocking` : ''}`
            : undefined
        }
        actions={
          <button className="btn btn-primary" onClick={() => setAdding(true)}>
            <Icon name="plus" size={18} /> Add item
          </button>
        }
      />

      {hasItems && (
        <div className="toolbar">
          <label className="search-field">
            <Icon name="search" size={18} />
            <span className="visually-hidden">Search pantry</span>
            <input type="search" placeholder="Search pantry" value={query} onChange={(e) => setQuery(e.target.value)} />
          </label>
          <div className="segmented" role="group" aria-label="Stock filter">
            {FILTERS.map((f) => (
              <button key={f.id} className="segmented-btn" aria-pressed={filter === f.id} onClick={() => setFilter(f.id)}>
                {f.label}
                {f.id === 'attention' && attentionCount > 0 && <span className="count-badge">{attentionCount}</span>}
              </button>
            ))}
          </div>
        </div>
      )}

      {categories.length > 1 && (
        <div className="chip-scroller" role="group" aria-label="Category">
          <button className="filter-chip" aria-pressed={!category} onClick={() => setCategory('')}>
            All categories
          </button>
          {categories.map((c) => (
            <button key={c} className="filter-chip" aria-pressed={category === c} onClick={() => setCategory(category === c ? '' : c)}>
              {c}
            </button>
          ))}
        </div>
      )}

      {loading && <PantrySkeleton />}
      {error && <ErrorState error={error} onRetry={reload} />}

      {items && items.length === 0 && (
        <EmptyState
          icon="basket"
          title="Your pantry is empty"
          action={
            <button className="btn btn-primary" onClick={() => setAdding(true)}>
              <Icon name="plus" size={18} /> Add your first item
            </button>
          }
        >
          Ingredients are added automatically when you save recipes. Track how much you have to see what's missing at a glance.
        </EmptyState>
      )}

      {hasItems && groups.length === 0 && (
        <EmptyState icon="search" title="Nothing matches">
          Try a different search or filter.
        </EmptyState>
      )}

      {groups.map(([cat, list]) => (
        <section key={cat} className="pantry-group" aria-label={cat}>
          <h2 className="group-title">
            {cat} <span className="group-count">{list.length}</span>
          </h2>
          <ul className="pantry-list">
            {list.map((ing) => (
              <PantryItem key={ing.id} ing={ing} onOpen={setEditing} mutations={mutations} />
            ))}
          </ul>
        </section>
      ))}

      {editing && (
        <EditStockDialog key={editing.id} ing={editing} onClose={() => setEditing(null)} mutations={mutations} />
      )}
      <AddItemDialog open={adding} onClose={() => setAdding(false)} categories={categories} />
    </div>
  )
}
