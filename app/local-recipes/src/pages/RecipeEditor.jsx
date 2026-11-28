import { useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { api } from '../lib/api'
import { invalidate, setCached, useResource } from '../lib/resource'
import { parseIngredientList } from '../lib/parseIngredients'
import { Icon } from '../components/Icon'
import { Dialog } from '../components/Dialog'
import { useToast } from '../components/Toast'
import { ErrorState, PageHeader, Skeleton } from '../components/ui'

let rowSeq = 0
const newRow = (r = {}) => ({ key: ++rowSeq, name: '', quantity: '', unit: '', notes: '', optional: false, ...r })

const emptyForm = () => ({
  title: '',
  description: '',
  body_markdown: '',
  servings: '',
  prep_minutes: '',
  cook_minutes: '',
  source_url: '',
  image_url: '',
  ingredients: [newRow()],
})

function toForm(recipe) {
  const rows = (recipe.ingredients || []).map((ri) =>
    newRow({
      name: (ri.ingredient && ri.ingredient.name) || '',
      quantity: ri.quantity ? String(ri.quantity) : '',
      unit: ri.unit || '',
      notes: ri.notes || '',
      optional: !!ri.optional,
    }),
  )
  return {
    title: recipe.title || '',
    description: recipe.description || '',
    body_markdown: recipe.body_markdown || '',
    servings: recipe.servings ? String(recipe.servings) : '',
    prep_minutes: recipe.prep_minutes ? String(recipe.prep_minutes) : '',
    cook_minutes: recipe.cook_minutes ? String(recipe.cook_minutes) : '',
    source_url: recipe.source_url || '',
    image_url: recipe.image_url || '',
    ingredients: rows.length ? rows : [newRow()],
  }
}

function toPayload(form) {
  const num = (v) => Math.max(0, Number(v) || 0)
  return {
    title: form.title.trim(),
    description: form.description.trim(),
    body_markdown: form.body_markdown,
    servings: Math.round(num(form.servings)),
    prep_minutes: Math.round(num(form.prep_minutes)),
    cook_minutes: Math.round(num(form.cook_minutes)),
    source_url: form.source_url.trim(),
    image_url: form.image_url.trim(),
    ingredients: form.ingredients
      .filter((i) => i.name.trim())
      .map((i) => ({
        name: i.name.trim(),
        quantity: num(i.quantity),
        unit: i.unit.trim(),
        notes: i.notes.trim(),
        optional: i.optional,
      })),
  }
}

function IngredientRows({ rows, onChange }) {
  const lastNameRef = useRef(null)
  const [focusLast, setFocusLast] = useState(false)

  useEffect(() => {
    if (focusLast && lastNameRef.current) {
      lastNameRef.current.focus()
      setFocusLast(false)
    }
  }, [focusLast, rows.length])

  const update = (key, field, value) => onChange(rows.map((r) => (r.key === key ? { ...r, [field]: value } : r)))
  const remove = (key) => {
    const next = rows.filter((r) => r.key !== key)
    onChange(next.length ? next : [newRow()])
  }
  const move = (idx, dir) => {
    const next = rows.slice()
    const [item] = next.splice(idx, 1)
    next.splice(idx + dir, 0, item)
    onChange(next)
  }
  const addRow = () => {
    onChange([...rows, newRow()])
    setFocusLast(true)
  }
  const onKeyDown = (e, idx) => {
    if (e.key === 'Enter') {
      e.preventDefault()
      if (idx === rows.length - 1) addRow()
    }
  }

  return (
    <div className="ingredient-editor">
      <div className="ingredient-editor-head" aria-hidden="true">
        <span>Qty</span>
        <span>Unit</span>
        <span>Ingredient</span>
        <span>Notes</span>
        <span />
      </div>
      <ol className="ingredient-editor-rows">
        {rows.map((row, idx) => (
          <li key={row.key} className="ingredient-editor-row">
            <input
              aria-label={`Quantity for ingredient ${idx + 1}`}
              inputMode="decimal"
              placeholder="1.5"
              value={row.quantity}
              onChange={(e) => update(row.key, 'quantity', e.target.value.replace(/[^\d.]/g, ''))}
              onKeyDown={(e) => onKeyDown(e, idx)}
            />
            <input
              aria-label={`Unit for ingredient ${idx + 1}`}
              placeholder="cup"
              value={row.unit}
              onChange={(e) => update(row.key, 'unit', e.target.value)}
              onKeyDown={(e) => onKeyDown(e, idx)}
            />
            <input
              ref={idx === rows.length - 1 ? lastNameRef : null}
              aria-label={`Name of ingredient ${idx + 1}`}
              placeholder="all-purpose flour"
              value={row.name}
              onChange={(e) => update(row.key, 'name', e.target.value)}
              onKeyDown={(e) => onKeyDown(e, idx)}
            />
            <input
              aria-label={`Notes for ingredient ${idx + 1}`}
              placeholder="sifted"
              value={row.notes}
              onChange={(e) => update(row.key, 'notes', e.target.value)}
              onKeyDown={(e) => onKeyDown(e, idx)}
            />
            <div className="ingredient-editor-actions">
              <label className="tag-toggle" title="Optional ingredient">
                <input type="checkbox" checked={row.optional} onChange={(e) => update(row.key, 'optional', e.target.checked)} />
                <span>Optional</span>
              </label>
              <button
                type="button"
                className="icon-button icon-button-sm"
                aria-label={`Move ingredient ${idx + 1} up`}
                onClick={() => move(idx, -1)}
                disabled={idx === 0}
              >
                <Icon name="chevronLeft" size={16} className="rot-90" />
              </button>
              <button
                type="button"
                className="icon-button icon-button-sm"
                aria-label={`Move ingredient ${idx + 1} down`}
                onClick={() => move(idx, 1)}
                disabled={idx === rows.length - 1}
              >
                <Icon name="chevronRight" size={16} className="rot-90" />
              </button>
              <button
                type="button"
                className="icon-button icon-button-sm icon-button-danger"
                aria-label={`Remove ingredient ${idx + 1}`}
                onClick={() => remove(row.key)}
              >
                <Icon name="trash" size={16} />
              </button>
            </div>
          </li>
        ))}
      </ol>
      <button type="button" className="btn btn-ghost btn-sm" onClick={addRow}>
        <Icon name="plus" size={16} /> Add ingredient
      </button>
    </div>
  )
}

function PasteDialog({ open, onClose, onAdd }) {
  const [text, setText] = useState('')
  const parsed = useMemo(() => parseIngredientList(text), [text])
  return (
    <Dialog
      open={open}
      onClose={onClose}
      title="Paste ingredients"
      size="md"
      footer={
        <>
          <button type="button" className="btn btn-ghost" onClick={onClose}>
            Cancel
          </button>
          <button type="button"
            className="btn btn-primary"
            disabled={!parsed.length}
            onClick={() => {
              onAdd(parsed)
              setText('')
            }}
          >
            Add {parsed.length || ''} ingredient{parsed.length === 1 ? '' : 's'}
          </button>
        </>
      }
    >
      <p className="muted">One ingredient per line. Quantities, common units and notes after a comma are picked up automatically.</p>
      <textarea
        className="paste-area"
        rows={8}
        autoFocus
        placeholder={'2 ¼ tsp active dry yeast\n4 ½ cups all-purpose flour\n4 tbsp butter, melted'}
        value={text}
        onChange={(e) => setText(e.target.value)}
      />
      {parsed.length > 0 && (
        <ul className="paste-preview">
          {parsed.slice(0, 6).map((p, i) => (
            <li key={i}>
              <strong>{[p.quantity, p.unit].filter(Boolean).join(' ') || '—'}</strong> {p.name}
              {p.notes && <span className="muted">, {p.notes}</span>}
            </li>
          ))}
          {parsed.length > 6 && <li className="muted">…and {parsed.length - 6} more</li>}
        </ul>
      )}
    </Dialog>
  )
}

const REMARK_PLUGINS = [remarkGfm]

function MethodEditor({ value, onChange }) {
  const [tab, setTab] = useState('write')
  return (
    <div className="method-editor">
      <div className="tabs" role="tablist" aria-label="Method editor">
        <button type="button" role="tab" aria-selected={tab === 'write'} className="tab" onClick={() => setTab('write')}>
          Write
        </button>
        <button type="button" role="tab" aria-selected={tab === 'preview'} className="tab" onClick={() => setTab('preview')}>
          Preview
        </button>
        <span className="tabs-hint">Markdown supported · number your steps</span>
      </div>
      {tab === 'write' ? (
        <textarea
          className="method-textarea"
          aria-label="Method"
          rows={14}
          placeholder={'1. Preheat the oven to 450°F.\n2. Mix the dough…\n\n## Tips\n- …'}
          value={value}
          onChange={(e) => onChange(e.target.value)}
        />
      ) : (
        <div className="prose method-preview">
          {value.trim() ? <ReactMarkdown remarkPlugins={REMARK_PLUGINS}>{value}</ReactMarkdown> : <p className="muted">Nothing to preview yet.</p>}
        </div>
      )}
    </div>
  )
}

function Field({ label, hint, error, children, className = '' }) {
  return (
    <label className={`field ${error ? 'has-error' : ''} ${className}`}>
      <span className="field-label">{label}</span>
      {children}
      {error ? <span className="field-error">{error}</span> : hint ? <span className="field-hint">{hint}</span> : null}
    </label>
  )
}

export default function RecipeEditor({ mode }) {
  const { recipeId } = useParams()
  const navigate = useNavigate()
  const toast = useToast()
  const isEdit = mode === 'edit'
  const { data: recipe, error: loadError, loading, reload } = useResource(isEdit ? `recipe:${recipeId}` : null, () =>
    api.getRecipe(recipeId),
  )
  const [form, setForm] = useState(() => (isEdit ? null : emptyForm()))
  const [initial, setInitial] = useState(null)
  const [saving, setSaving] = useState(false)
  const [submitted, setSubmitted] = useState(false)
  const [pasteOpen, setPasteOpen] = useState(false)
  const titleRef = useRef(null)

  useEffect(() => {
    if (isEdit && recipe && !form) {
      const f = toForm(recipe)
      setForm(f)
      setInitial(JSON.stringify(toPayload(f)))
    }
    if (!isEdit && initial === null) setInitial(JSON.stringify(toPayload(emptyForm())))
  }, [isEdit, recipe, form, initial])

  const dirty = form && initial !== null && JSON.stringify(toPayload(form)) !== initial

  useEffect(() => {
    if (!dirty) return
    const onBeforeUnload = (e) => {
      e.preventDefault()
      e.returnValue = ''
    }
    window.addEventListener('beforeunload', onBeforeUnload)
    return () => window.removeEventListener('beforeunload', onBeforeUnload)
  }, [dirty])

  if (isEdit && loading) {
    return (
      <div className="page page-narrow">
        <Skeleton style={{ height: 40, width: '40%' }} />
        <Skeleton style={{ height: 300, marginTop: 24 }} />
      </div>
    )
  }
  if (isEdit && loadError) return <ErrorState error={loadError} onRetry={reload} />
  if (!form) return null

  const set = (field) => (e) => setForm((f) => ({ ...f, [field]: e.target.value }))
  const titleError = submitted && !form.title.trim() ? 'Give your recipe a name' : null
  const imageOk = /^https?:\/\//i.test(form.image_url.trim())

  const onSubmit = async (e) => {
    e.preventDefault()
    setSubmitted(true)
    if (!form.title.trim()) {
      titleRef.current && titleRef.current.focus()
      return
    }
    setSaving(true)
    try {
      const payload = toPayload(form)
      const saved = isEdit ? await api.updateRecipe(recipeId, payload) : await api.createRecipe(payload)
      setInitial(JSON.stringify(payload))
      invalidate('recipes')
      invalidate(`recipe:${saved.id}`)
      if (isEdit) setCached(`recipe:${saved.id}`, (prev) => ({ ...(prev || {}), ...saved }))
      toast(isEdit ? 'Recipe saved' : 'Recipe added')
      navigate(`/recipes/${saved.id}`)
    } catch (err) {
      toast(err.message, { tone: 'error' })
    } finally {
      setSaving(false)
    }
  }

  const cancelTo = isEdit ? `/recipes/${recipeId}` : '/'

  return (
    <form className="page page-narrow editor" onSubmit={onSubmit} noValidate>
      <PageHeader
        title={isEdit ? 'Edit recipe' : 'New recipe'}
        back={{ to: cancelTo, label: isEdit ? 'Back to recipe' : 'All recipes' }}
      />

      <section className="card form-section" aria-labelledby="sec-basics">
        <h2 id="sec-basics" className="form-section-title">
          Basics
        </h2>
        <Field label="Title" error={titleError}>
          <input ref={titleRef} value={form.title} onChange={set('title')} placeholder="Homemade soft pretzels" autoFocus={!isEdit} />
        </Field>
        <Field label="Short description" hint="One or two sentences shown on the recipe card.">
          <textarea rows={2} value={form.description} onChange={set('description')} />
        </Field>
        <div className="field-grid field-grid-3">
          <Field label="Servings">
            <input inputMode="numeric" value={form.servings} onChange={set('servings')} placeholder="4" />
          </Field>
          <Field label="Prep (minutes)">
            <input inputMode="numeric" value={form.prep_minutes} onChange={set('prep_minutes')} placeholder="20" />
          </Field>
          <Field label="Cook (minutes)">
            <input inputMode="numeric" value={form.cook_minutes} onChange={set('cook_minutes')} placeholder="35" />
          </Field>
        </div>
      </section>

      <section className="card form-section" aria-labelledby="sec-ingredients">
        <div className="form-section-head">
          <h2 id="sec-ingredients" className="form-section-title">
            Ingredients
          </h2>
          <button type="button" className="btn btn-ghost btn-sm" onClick={() => setPasteOpen(true)}>
            Paste a list
          </button>
        </div>
        <IngredientRows rows={form.ingredients} onChange={(ingredients) => setForm((f) => ({ ...f, ingredients }))} />
      </section>

      <section className="card form-section" aria-labelledby="sec-method">
        <h2 id="sec-method" className="form-section-title">
          Method
        </h2>
        <MethodEditor value={form.body_markdown} onChange={(v) => setForm((f) => ({ ...f, body_markdown: v }))} />
      </section>

      <section className="card form-section" aria-labelledby="sec-source">
        <h2 id="sec-source" className="form-section-title">
          Source &amp; photo
        </h2>
        <div className="field-grid field-grid-2">
          <Field label="Source URL">
            <input type="url" inputMode="url" value={form.source_url} onChange={set('source_url')} placeholder="https://" />
          </Field>
          <Field label="Photo URL">
            <input type="url" inputMode="url" value={form.image_url} onChange={set('image_url')} placeholder="https://" />
          </Field>
        </div>
        {imageOk && <img className="image-preview" src={form.image_url.trim()} alt="Recipe photo preview" referrerPolicy="no-referrer" />}
      </section>

      <div className="savebar">
        <span className="savebar-status">{dirty ? 'Unsaved changes' : isEdit ? 'No changes' : ''}</span>
        <button type="button" className="btn btn-ghost" onClick={() => navigate(cancelTo)}>
          Cancel
        </button>
        <button type="submit" className="btn btn-primary" disabled={saving}>
          {saving ? 'Saving…' : isEdit ? 'Save changes' : 'Add recipe'}
        </button>
      </div>

      <PasteDialog
        open={pasteOpen}
        onClose={() => setPasteOpen(false)}
        onAdd={(rows) => {
          setForm((f) => {
            const kept = f.ingredients.filter((r) => r.name.trim())
            return { ...f, ingredients: [...kept, ...rows.map((r) => newRow(r))] }
          })
          setPasteOpen(false)
          toast(`Added ${rows.length} ingredient${rows.length === 1 ? '' : 's'}`)
        }}
      />
    </form>
  )
}
