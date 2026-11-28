import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../lib/api'
import { invalidate, useResource } from '../lib/resource'
import { addDays, isSameDay, relativeWeek, startOfWeek, toDateParam, weekLabel } from '../lib/dates'
import { displayUnit, formatQuantity, pluralize } from '../lib/format'
import { Icon } from '../components/Icon'
import { useToast } from '../components/Toast'
import { EmptyState, ErrorState, PageHeader, Skeleton } from '../components/ui'

// Round purchase amounts up so the list never under-buys (0.31 cup -> ⅓).
function buyAmount(q, unit) {
  if (!q) return ''
  const u = displayUnit(unit)
  const rounded = q >= 10 ? Math.ceil(q) : Math.ceil(q * 100) / 100
  const amount = formatQuantity(rounded, unit) || String(rounded)
  return u ? `${amount} ${u}` : amount
}

const weekday = (iso) => new Date(iso + 'T00:00').toLocaleDateString(undefined, { weekday: 'short' })

function reasonText(item) {
  const reasons = item.reasons || []
  const planned = reasons.filter((r) => r.kind === 'planned')
  const parts = []
  if (planned.length) {
    const byRecipe = new Map()
    for (const r of planned) {
      if (!byRecipe.has(r.recipe_title)) byRecipe.set(r.recipe_title, [])
      byRecipe.get(r.recipe_title).push(weekday(r.date))
    }
    parts.push(
      'For ' +
        [...byRecipe.entries()].map(([title, days]) => `${title} (${days.join(', ')})`).join(', '),
    )
  }
  if (reasons.some((r) => r.kind === 'restock')) {
    if (item.threshold > 0) parts.push(`keeps ${buyAmount(item.threshold, item.unit)} on hand`)
    else if (!planned.length) parts.push('out of stock')
  }
  return parts.join(' · ')
}

function useStoredSet(key) {
  const read = () => {
    try {
      return new Set(JSON.parse(localStorage.getItem(key) || '[]'))
    } catch {
      return new Set()
    }
  }
  const [set, setSet] = useState(read)
  useEffect(() => setSet(read()), [key]) // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => {
    try {
      localStorage.setItem(key, JSON.stringify([...set]))
    } catch {
      /* storage unavailable: checks just won't persist */
    }
  }, [key, set])
  const toggle = (id) =>
    setSet((prev) => {
      const next = new Set(prev)
      next.has(id) ? next.delete(id) : next.add(id)
      return next
    })
  return [set, toggle, (ids) => setSet((prev) => new Set([...prev].filter((x) => !ids.includes(x))))]
}

function ShoppingRow({ item, checked, onToggle }) {
  const amount = buyAmount(item.quantity, item.unit)
  const why = reasonText(item)
  return (
    <li className={`shop-row ${checked ? 'is-checked' : ''}`}>
      <label className="check-row">
        <input type="checkbox" checked={checked} onChange={onToggle} />
        <span className="check-box" aria-hidden="true">
          <Icon name="check" size={14} strokeWidth={2.5} />
        </span>
        <span className="shop-text">
          <span className="shop-line">
            <span className="shop-name">{item.name}</span>
            <span className="shop-amount">{amount || (item.tracked ? 'Restock' : '')}</span>
          </span>
          {why && <span className="shop-why">{why}</span>}
          {item.unconverted && item.unconverted.length > 0 && (
            <span className="shop-why shop-warn">
              {item.unconverted.map((u) => `${formatQuantity(u.quantity, u.unit) || u.quantity} ${displayUnit(u.unit)}`.trim()).join(' + ')}{' '}
              needed for {[...new Set(item.unconverted.map((u) => u.recipe_title))].join(', ')} — your pantry tracks{' '}
              {displayUnit(item.unit) || 'a count'} ({buyAmount(item.on_hand, item.unit) || '0'} on hand)
            </span>
          )}
        </span>
      </label>
    </li>
  )
}

export default function Shopping() {
  const toast = useToast()
  const [weekStart, setWeekStart] = useState(() => startOfWeek(new Date()))
  const today = new Date()
  today.setHours(0, 0, 0, 0)
  const isCurrentWeek = isSameDay(weekStart, startOfWeek(today))
  // For the current week, only meals still ahead of you count.
  const fromDate = isCurrentWeek ? today : weekStart
  const from = toDateParam(fromDate)
  const to = toDateParam(addDays(weekStart, 6))
  const key = `shopping:${from}:${to}`
  const { data: list, error, loading, reload } = useResource(key, () => api.getShoppingList(from, to))
  const [checked, toggle, uncheck] = useStoredSet(`shopping-checked:${to}`)
  const [adding, setAdding] = useState(false)
  const [showUntracked, setShowUntracked] = useState(true)

  const { toBuy, untracked } = useMemo(() => {
    const items = (list && list.items) || []
    return { toBuy: items.filter((i) => i.tracked), untracked: items.filter((i) => !i.tracked) }
  }, [list])

  const groups = useMemo(() => {
    const m = new Map()
    for (const i of toBuy) {
      const c = i.category || 'Other'
      if (!m.has(c)) m.set(c, [])
      m.get(c).push(i)
    }
    return [...m.entries()]
  }, [toBuy])

  const checkedItems = [...toBuy, ...untracked].filter((i) => checked.has(i.ingredient_id))
  const addable = checkedItems.filter((i) => i.quantity > 0)
  const remaining = toBuy.filter((i) => !checked.has(i.ingredient_id)).length

  const addToPantry = async () => {
    setAdding(true)
    try {
      for (const i of addable) await api.adjustStock(i.ingredient_id, i.quantity, i.unit)
      uncheck(checkedItems.map((i) => i.ingredient_id))
      invalidate('ingredients')
      invalidate('shopping')
      const unknown = checkedItems.length - addable.length
      toast(
        `Added ${pluralize(addable.length, 'item')} to your pantry` +
          (unknown ? ` · set amounts for ${unknown} in Pantry` : ''),
      )
    } catch (e) {
      toast(e.message, { tone: 'error' })
      reload()
    } finally {
      setAdding(false)
    }
  }

  const rangeLabel =
    isCurrentWeek && !isSameDay(fromDate, weekStart)
      ? `${fromDate.toLocaleDateString(undefined, { weekday: 'short' })} – Sun`
      : weekLabel(weekStart)

  return (
    <div className="page page-narrow shopping">
      <PageHeader
        title="Shopping list"
        subtitle={
          list
            ? `${list.planned_meals ? `${pluralize(list.planned_meals, 'planned meal')} · ` : ''}${
                toBuy.length ? `${remaining} of ${pluralize(toBuy.length, 'item')} left` : 'Nothing to buy'
              }`
            : undefined
        }
        actions={
          <Link className="btn btn-secondary" to="/plan">
            <Icon name="calendar" size={18} /> Meal plan
          </Link>
        }
      />

      <div className="week-nav">
        <button className="icon-button" aria-label="Previous week" onClick={() => setWeekStart(addDays(weekStart, -7))}>
          <Icon name="chevronLeft" />
        </button>
        <div className="week-nav-label" aria-live="polite">
          <span className="week-nav-relative">{relativeWeek(weekStart)}</span>
          <span className="week-nav-range">{rangeLabel}</span>
        </div>
        <button className="icon-button" aria-label="Next week" onClick={() => setWeekStart(addDays(weekStart, 7))}>
          <Icon name="chevronRight" />
        </button>
        {!isCurrentWeek && (
          <button className="btn btn-ghost btn-sm" onClick={() => setWeekStart(startOfWeek(new Date()))}>
            Today
          </button>
        )}
      </div>

      {loading && <Skeleton style={{ height: 280 }} />}
      {error && <ErrorState error={error} onRetry={reload} />}

      {list && list.planned_meals === 0 && (
        <p className="info-note">
          <Icon name="calendar" size={16} />
          No meals planned {isCurrentWeek ? 'for the rest of this week' : 'this week'} — this list only covers restocking.{' '}
          <Link to="/plan">Plan meals</Link>
        </p>
      )}

      {list && toBuy.length === 0 && untracked.length === 0 && (
        <EmptyState icon="basket" title="You're all stocked up">
          Your pantry covers the planned meals and nothing is running low.
        </EmptyState>
      )}

      {groups.map(([cat, items]) => (
        <section key={cat} className="shop-group" aria-label={cat}>
          <h2 className="group-title">
            {cat} <span className="group-count">{items.length}</span>
          </h2>
          <ul className="card shop-list">
            {items.map((i) => (
              <ShoppingRow key={i.ingredient_id} item={i} checked={checked.has(i.ingredient_id)} onToggle={() => toggle(i.ingredient_id)} />
            ))}
          </ul>
        </section>
      ))}

      {untracked.length > 0 && (
        <section className="shop-group" aria-labelledby="untracked-heading">
          <button className="group-title group-toggle" onClick={() => setShowUntracked((v) => !v)} aria-expanded={showUntracked}>
            <span id="untracked-heading">Check you have</span> <span className="group-count">{untracked.length}</span>
            <Icon name="chevronRight" size={14} className={showUntracked ? 'rot-90' : ''} />
          </button>
          {showUntracked && (
            <>
              <p className="muted shop-group-desc">Not tracked in your pantry, so we can't tell if you have enough.</p>
              <ul className="card shop-list">
                {untracked.map((i) => (
                  <ShoppingRow key={i.ingredient_id} item={i} checked={checked.has(i.ingredient_id)} onToggle={() => toggle(i.ingredient_id)} />
                ))}
              </ul>
            </>
          )}
        </section>
      )}

      {checkedItems.length > 0 && (
        <div className="savebar">
          <span className="savebar-status">{pluralize(checkedItems.length, 'item')} in your cart</span>
          <button className="btn btn-primary" onClick={addToPantry} disabled={adding}>
            <Icon name="basket" size={18} /> {adding ? 'Adding…' : 'Add to pantry'}
          </button>
        </div>
      )}
    </div>
  )
}
