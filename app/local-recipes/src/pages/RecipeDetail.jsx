import { createContext, useContext, useEffect, useMemo, useRef, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { api } from '../lib/api'
import { getCached, invalidate, setCached, useResource } from '../lib/resource'
import { displayUnit, formatMinutes, formatQuantity, totalMinutes } from '../lib/format'
import { stripIngredientsSection } from '../lib/markdown'
import { Icon } from '../components/Icon'
import { archiveRecipe, restoreRecipe } from '../lib/recipeActions'
import { Dialog } from '../components/Dialog'
import { useToast } from '../components/Toast'
import { Chip, ErrorState, RecipeCover, Skeleton, Stars, Stepper } from '../components/ui'

// Checked ingredients/steps are per-cooking-session state: kept for the
// browser session (survives a reload mid-cook) but never persisted.
function useSessionSet(key) {
  const [set, setSet] = useState(() => {
    try {
      return new Set(JSON.parse(sessionStorage.getItem(key) || '[]'))
    } catch {
      return new Set()
    }
  })
  useEffect(() => {
    try {
      sessionStorage.setItem(key, JSON.stringify([...set]))
    } catch {
      /* storage unavailable - state just won't survive reloads */
    }
  }, [key, set])
  const toggle = (id) =>
    setSet((prev) => {
      const next = new Set(prev)
      next.has(id) ? next.delete(id) : next.add(id)
      return next
    })
  return [set, toggle, () => setSet(new Set())]
}

function useWakeLock(active) {
  const [supported] = useState(() => typeof navigator !== 'undefined' && 'wakeLock' in navigator)
  useEffect(() => {
    if (!active || !supported) return
    let lock = null
    let cancelled = false
    const acquire = () =>
      navigator.wakeLock
        .request('screen')
        .then((l) => {
          if (cancelled) l.release()
          else lock = l
        })
        .catch(() => {})
    acquire()
    // The lock is dropped whenever the tab is hidden; re-take it on return.
    const onVisible = () => document.visibilityState === 'visible' && acquire()
    document.addEventListener('visibilitychange', onVisible)
    return () => {
      cancelled = true
      document.removeEventListener('visibilitychange', onVisible)
      if (lock) lock.release().catch(() => {})
    }
  }, [active, supported])
  return supported
}

function stockStatus(ingredient) {
  const stock = ingredient && ingredient.stock
  if (!stock) return 'untracked'
  if (stock.quantity_on_hand <= 0) return 'out'
  if (stock.low_stock_threshold > 0 && stock.quantity_on_hand <= stock.low_stock_threshold) return 'low'
  return 'ok'
}

const STATUS_LABEL = { ok: 'In pantry', low: 'Running low', out: 'Out of stock' }

function IngredientsPanel({ recipe, pantryById, servings, setServings }) {
  const base = recipe.servings > 0 ? recipe.servings : 0
  const [checked, toggle, reset] = useSessionSet(`checked:${recipe.id}`)
  const factor = base ? servings / base : 1
  const items = recipe.ingredients || []

  const tracked = items.filter((ri) => stockStatus(pantryById.get(ri.ingredient_id)) !== 'untracked')
  const missing = tracked.filter((ri) => stockStatus(pantryById.get(ri.ingredient_id)) === 'out' && !ri.optional)

  return (
    <section className="card ingredients-card" aria-labelledby="ingredients-heading">
      <div className="card-header">
        <h2 id="ingredients-heading">Ingredients</h2>
        {checked.size > 0 && (
          <button className="link-button" onClick={reset}>
            Clear checks
          </button>
        )}
      </div>

      {base > 0 && (
        <div className="servings-row">
          <span className="servings-label">
            <Icon name="users" size={16} /> Servings
          </span>
          <Stepper value={servings} onChange={setServings} label="servings" />
        </div>
      )}
      {base > 0 && servings !== base && (
        <p className="scale-note">
          Scaled ×{Math.round(factor * 100) / 100} from {base}.{' '}
          <button className="link-button" onClick={() => setServings(base)}>
            Reset
          </button>
        </p>
      )}

      {tracked.length > 0 && (
        <p className={`pantry-summary ${missing.length ? 'pantry-summary-warn' : ''}`}>
          <Icon name={missing.length ? 'alert' : 'basket'} size={16} />
          {missing.length
            ? `Missing ${missing.length} of ${items.length} from your pantry`
            : `Everything tracked is in your pantry`}
        </p>
      )}

      <ul className="ingredient-checklist">
        {items.map((ri) => {
          const name = (ri.ingredient && ri.ingredient.name) || 'Ingredient'
          const qty = formatQuantity(ri.quantity * factor, ri.unit)
          const unit = displayUnit(ri.unit, ri.quantity)
          const status = stockStatus(pantryById.get(ri.ingredient_id))
          const isChecked = checked.has(ri.id)
          return (
            <li key={ri.id} className={isChecked ? 'is-checked' : ''}>
              <label className="check-row">
                <input type="checkbox" checked={isChecked} onChange={() => toggle(ri.id)} />
                <span className="check-box" aria-hidden="true">
                  <Icon name="check" size={14} strokeWidth={2.5} />
                </span>
                <span className="ingredient-text">
                  {(qty || unit) && (
                    <span className="ingredient-qty">
                      {qty}
                      {qty && unit ? ' ' : ''}
                      {unit}
                    </span>
                  )}{' '}
                  <span className="ingredient-name">{name}</span>
                  {ri.notes && <span className="ingredient-notes">, {ri.notes}</span>}
                  {ri.optional && <span className="tag">optional</span>}
                </span>
                {status !== 'untracked' && (
                  <span className={`stock-dot stock-${status}`} title={STATUS_LABEL[status]}>
                    <span className="visually-hidden">{STATUS_LABEL[status]}</span>
                  </span>
                )}
              </label>
            </li>
          )
        })}
      </ul>
      {items.length === 0 && <p className="muted">No structured ingredients - see the method.</p>}
    </section>
  )
}

// Steps state is provided once per method body; list renderers live at
// module level so their component identity is stable and toggling a step
// doesn't remount the list (which would drop keyboard focus).
const StepState = createContext(null)
const InOrderedList = createContext(false)

function MdOl({ node, ...props }) {
  return (
    <InOrderedList.Provider value={true}>
      <ol className="steps" {...props} />
    </InOrderedList.Provider>
  )
}

function MdUl({ node, ...props }) {
  return (
    <InOrderedList.Provider value={false}>
      <ul {...props} />
    </InOrderedList.Provider>
  )
}

function MdLi({ node, children, ...props }) {
  const inOl = useContext(InOrderedList)
  const state = useContext(StepState)
  if (!inOl || !state) return <li {...props}>{children}</li>
  const id = node && node.position ? String(node.position.start.line) : ''
  const isDone = state.done.has(id)
  return (
    <li
      {...props}
      className={`step ${isDone ? 'is-done' : ''}`}
      onClick={(e) => {
        if (e.target.closest('a')) return
        state.toggle(id)
      }}
      onKeyDown={(e) => {
        if (e.target !== e.currentTarget) return
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault()
          state.toggle(id)
        }
      }}
      role="checkbox"
      aria-checked={isDone}
      tabIndex={0}
    >
      {children}
    </li>
  )
}

function MdA({ node, ...props }) {
  return <a {...props} target="_blank" rel="noreferrer" />
}

const MD_COMPONENTS = { ol: MdOl, ul: MdUl, li: MdLi, a: MdA }
const REMARK_PLUGINS = [remarkGfm]

function MethodBody({ recipe }) {
  const [done, toggle] = useSessionSet(`steps:${recipe.id}`)
  const body = useMemo(
    () => (recipe.ingredients && recipe.ingredients.length ? stripIngredientsSection(recipe.body_markdown) : recipe.body_markdown || ''),
    [recipe],
  )
  const state = useMemo(() => ({ done, toggle }), [done]) // eslint-disable-line react-hooks/exhaustive-deps

  // Markdown parsing is the expensive part; it only re-runs when the body changes.
  const rendered = useMemo(
    () => (
      <ReactMarkdown remarkPlugins={REMARK_PLUGINS} components={MD_COMPONENTS}>
        {body}
      </ReactMarkdown>
    ),
    [body],
  )

  if (!body.trim()) return <p className="muted">No method written yet.</p>
  return (
    <StepState.Provider value={state}>
      <div className="prose">{rendered}</div>
    </StepState.Provider>
  )
}

function RatingCard({ recipeId, onRated }) {
  const toast = useToast()
  const { data: rating, loading } = useResource(`rating:${recipeId}`, () => api.getRating(recipeId))
  const [hover, setHover] = useState(0)
  const [notes, setNotes] = useState(null)
  const score = rating ? rating.score : 0
  const notesValue = notes ?? (rating ? rating.notes || '' : '')
  const dirty = notes !== null && notes !== (rating ? rating.notes || '' : '')

  const save = async (newScore, newNotes) => {
    try {
      const saved = await api.rateRecipe(recipeId, newScore, newNotes)
      setCached(`rating:${recipeId}`, saved)
      setNotes(null)
      onRated()
      toast(newScore !== score ? `Rated ${newScore} star${newScore === 1 ? '' : 's'}` : 'Notes saved')
    } catch (e) {
      toast(e.message, { tone: 'error' })
    }
  }

  return (
    <section className="card" aria-labelledby="rating-heading">
      <div className="card-header">
        <h2 id="rating-heading">Your rating</h2>
      </div>
      <div className="rating-stars" role="radiogroup" aria-label="Rating" onMouseLeave={() => setHover(0)}>
        {[1, 2, 3, 4, 5].map((n) => (
          <button
            key={n}
            type="button"
            role="radio"
            aria-checked={score === n}
            aria-label={`${n} star${n === 1 ? '' : 's'}`}
            className={`rating-star ${(hover || score) >= n ? 'is-on' : ''}`}
            onMouseEnter={() => setHover(n)}
            onFocus={() => setHover(n)}
            onBlur={() => setHover(0)}
            onClick={() => save(n, notesValue)}
            disabled={loading}
          >
            <Icon name="star" size={28} filled={(hover || score) >= n} strokeWidth={1.5} />
          </button>
        ))}
      </div>
      <label className="field">
        <span className="field-label">Notes for next time</span>
        <textarea
          rows={3}
          placeholder={score ? 'What would you change?' : 'Rate the recipe to save notes'}
          value={notesValue}
          onChange={(e) => setNotes(e.target.value)}
          disabled={!score}
        />
      </label>
      {dirty && (
        <div className="card-actions">
          <button className="btn btn-ghost btn-sm" onClick={() => setNotes(null)}>
            Discard
          </button>
          <button className="btn btn-primary btn-sm" onClick={() => save(score, notesValue)}>
            Save notes
          </button>
        </div>
      )}
    </section>
  )
}

function fmtStock(n, unit) {
  const u = displayUnit(unit)
  const q = Math.round(n * 100) / 100
  return u ? `${q} ${u}` : String(q)
}

// Preview (dry run) then apply one batch's worth of pantry deductions,
// with an Undo that re-adds exactly what was removed.
function CookDialog({ open, onClose, recipe, servings }) {
  const toast = useToast()
  const [preview, setPreview] = useState(null)
  const [error, setError] = useState(null)
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (!open) return
    setPreview(null)
    setError(null)
    api
      .cookRecipe(recipe.id, servings, true)
      .then(setPreview)
      .catch(setError)
  }, [open, recipe.id, servings])

  const changes = preview ? preview.changes.filter((c) => c.quantity > 0) : []

  const apply = async () => {
    setBusy(true)
    try {
      const res = await api.cookRecipe(recipe.id, servings, false)
      const applied = res.changes.filter((c) => c.quantity > 0)
      invalidate('ingredients')
      invalidate('shopping')
      onClose()
      toast(`Used ingredients for ${recipe.title}`, {
        duration: 8000,
        action: applied.length
          ? {
              label: 'Undo',
              onClick: async () => {
                try {
                  for (const c of applied) await api.adjustStock(c.ingredient_id, c.quantity, c.unit)
                  invalidate('ingredients')
                  invalidate('shopping')
                  toast('Pantry restored')
                } catch (e) {
                  toast(e.message, { tone: 'error' })
                }
              },
            }
          : undefined,
      })
    } catch (e) {
      toast(e.message, { tone: 'error' })
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog
      open={open}
      onClose={onClose}
      title="Use ingredients"
      size="md"
      footer={
        <>
          <button type="button" className="btn btn-ghost" onClick={onClose}>
            Cancel
          </button>
          <button type="button" className="btn btn-primary" onClick={apply} disabled={busy || !changes.length}>
            {busy ? 'Updating…' : `Remove from pantry`}
          </button>
        </>
      }
    >
      <p className="muted">
        Subtracts {servings ? `${servings} serving${servings === 1 ? '' : 's'} of ` : ''}this recipe from your pantry.
      </p>
      {error && <p className="field-error">{error.message}</p>}
      {!preview && !error && <Skeleton style={{ height: 120, marginTop: 12 }} />}
      {preview && (
        <>
          {changes.length > 0 ? (
            <ul className="cook-list">
              {changes.map((c) => (
                <li key={c.ingredient_id + c.before}>
                  <span className="cook-name">{c.name}</span>
                  <span className="cook-delta">−{fmtStock(c.quantity, c.unit)}</span>
                  <span className="cook-after">
                    {fmtStock(c.before, c.unit)} → <strong>{fmtStock(c.after, c.unit)}</strong>
                    {c.short && <span className="pill pill-warn">not enough</span>}
                  </span>
                </li>
              ))}
            </ul>
          ) : (
            <p className="cook-empty">None of this recipe's ingredients are tracked in your pantry.</p>
          )}
          {preview.skipped.length > 0 && (
            <details className="cook-skipped">
              <summary>
                {preview.skipped.length} not changed
              </summary>
              <ul>
                {preview.skipped.map((s, i) => (
                  <li key={i}>
                    <span className="cook-name">{s.name}</span> <span className="muted">— {s.reason}</span>
                  </li>
                ))}
              </ul>
            </details>
          )}
        </>
      )}
    </Dialog>
  )
}

function DetailSkeleton() {
  return (
    <div className="page" aria-busy="true">
      <Skeleton style={{ height: 16, width: 90 }} />
      <Skeleton style={{ height: 40, width: '60%', marginTop: 16 }} />
      <Skeleton style={{ height: 18, width: '80%', marginTop: 12 }} />
      <div className="detail-layout" style={{ marginTop: 32 }}>
        <Skeleton style={{ height: 420 }} />
        <Skeleton style={{ height: 420 }} />
      </div>
    </div>
  )
}

export default function RecipeDetail() {
  const { recipeId } = useParams()
  const navigate = useNavigate()
  const toast = useToast()
  const key = `recipe:${recipeId}`
  const { data: recipe, error, loading, reload } = useResource(key, () => api.getRecipe(recipeId))
  const { data: pantry } = useResource('ingredients', api.listIngredients)
  const [cookMode, setCookMode] = useState(false)
  const [busy, setBusy] = useState(false)
  const [servingsById, setServingsById] = useState({})
  const [cookOpen, setCookOpen] = useState(false)
  const wakeLockSupported = useWakeLock(cookMode)
  const topRef = useRef(null)

  const pantryById = useMemo(() => new Map((pantry || []).map((i) => [i.id, i])), [pantry])

  useEffect(() => {
    if (recipe) document.title = `${recipe.title} · Recipes`
    return () => {
      document.title = 'Recipes'
    }
  }, [recipe])

  useEffect(() => {
    document.documentElement.classList.toggle('cook-mode', cookMode)
    return () => document.documentElement.classList.remove('cook-mode')
  }, [cookMode])

  if (loading) return <DetailSkeleton />
  if (error) return <ErrorState error={error} onRetry={reload} />

  // Archiving is reversible, so it acts immediately with an Undo toast
  // rather than a confirmation dialog.
  const onArchive = async () => {
    setBusy(true)
    try {
      await archiveRecipe(recipe.id)
      navigate('/')
      toast(`Archived "${recipe.title}"`, {
        duration: 7000,
        action: {
          label: 'Undo',
          onClick: () =>
            restoreRecipe(recipe.id)
              .then(() => toast(`Restored "${recipe.title}"`))
              .catch((e) => toast(e.message, { tone: 'error' })),
        },
      })
    } catch (e) {
      toast(e.message, { tone: 'error' })
      setBusy(false)
    }
  }

  const onRestore = async () => {
    setBusy(true)
    try {
      await restoreRecipe(recipe.id)
      toast(`Restored "${recipe.title}" to your library`)
    } catch (e) {
      toast(e.message, { tone: 'error' })
    } finally {
      setBusy(false)
    }
  }

  const refreshAfterRating = () => {
    reload()
    if (getCached('recipes')) invalidate('recipes')
  }

  const total = totalMinutes(recipe)
  const servings = servingsById[recipe.id] ?? (recipe.servings > 0 ? recipe.servings : 1)
  const setServings = (n) => setServingsById((m) => ({ ...m, [recipe.id]: n }))

  return (
    <article className={`page recipe-page ${cookMode ? 'is-cooking' : ''}`} ref={topRef}>
      <Link to="/" className="back-link">
        <Icon name="arrowLeft" size={16} /> All recipes
      </Link>

      {recipe.archived_at && (
        <div className="banner" role="status">
          <Icon name="archive" size={18} />
          <span className="banner-text">
            This recipe is archived. It's hidden from your library and meal plans.
          </span>
          <button className="btn btn-secondary btn-sm" onClick={onRestore} disabled={busy}>
            <Icon name="restore" size={16} /> Restore
          </button>
        </div>
      )}

      <header className={`recipe-hero ${recipe.image_url ? 'has-image' : ''}`}>
        {recipe.image_url && <RecipeCover recipe={recipe} className="recipe-hero-cover" eager />}
        <div className="recipe-hero-text">
          <h1 className="recipe-title">{recipe.title}</h1>
          {recipe.description && <p className="recipe-lede">{recipe.description}</p>}
          <div className="chip-row">
            {recipe.prep_minutes > 0 && <Chip icon="clock">Prep {formatMinutes(recipe.prep_minutes)}</Chip>}
            {recipe.cook_minutes > 0 && <Chip icon="flame">Cook {formatMinutes(recipe.cook_minutes)}</Chip>}
            {total > 0 && recipe.prep_minutes > 0 && recipe.cook_minutes > 0 && (
              <Chip>Total {formatMinutes(total)}</Chip>
            )}
            {recipe.servings > 0 && <Chip icon="users">Serves {recipe.servings}</Chip>}
            {recipe.average_rating > 0 && (
              <span className="chip">
                <Stars value={recipe.average_rating} count={recipe.rating_count} />
              </span>
            )}
          </div>
          <div className="recipe-actions">
            <button
              className={`btn ${cookMode ? 'btn-secondary' : 'btn-primary'}`}
              onClick={() => setCookMode((v) => !v)}
              aria-pressed={cookMode}
            >
              <Icon name={cookMode ? 'x' : 'flame'} size={18} />
              {cookMode ? 'Exit cook mode' : 'Start cooking'}
            </button>
            <button className="btn btn-secondary" onClick={() => setCookOpen(true)}>
              <Icon name="basket" size={18} /> Use ingredients
            </button>
            <Link className="btn btn-secondary" to={`/recipes/${recipe.id}/edit`}>
              <Icon name="pencil" size={18} /> Edit
            </Link>
            {recipe.source_url && (
              <a className="btn btn-ghost" href={recipe.source_url} target="_blank" rel="noreferrer">
                <Icon name="external" size={18} /> Source
              </a>
            )}
            {!recipe.archived_at && (
              <button className="btn btn-ghost" onClick={onArchive} disabled={busy}>
                <Icon name="archive" size={18} /> Archive
              </button>
            )}
          </div>
          {cookMode && (
            <p className="cook-mode-note" role="status">
              <Icon name="flame" size={16} />
              Cook mode: larger text, tap steps to mark them done
              {wakeLockSupported ? ', and your screen stays on.' : '.'}
            </p>
          )}
        </div>
      </header>

      <div className="detail-layout">
        <aside className="detail-aside">
          <IngredientsPanel recipe={recipe} pantryById={pantryById} servings={servings} setServings={setServings} />
        </aside>
        <div className="detail-main">
          <section className="card method-card" aria-labelledby="method-heading">
            <div className="card-header">
              <h2 id="method-heading">Method</h2>
            </div>
            <MethodBody recipe={recipe} />
          </section>
          {!cookMode && <RatingCard recipeId={recipe.id} onRated={refreshAfterRating} />}
        </div>
      </div>
      <CookDialog
        open={cookOpen}
        onClose={() => setCookOpen(false)}
        recipe={recipe}
        servings={recipe.servings > 0 ? servings : 0}
      />
    </article>
  )
}
