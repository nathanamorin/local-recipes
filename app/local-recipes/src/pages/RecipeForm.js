import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { api } from '../lib/api'

const emptyIngredient = () => ({ name: '', quantity: '', unit: '', notes: '', optional: false })

const emptyForm = () => ({
  title: '',
  description: '',
  body_markdown: '',
  servings: '',
  prep_minutes: '',
  cook_minutes: '',
  source_url: '',
  image_url: '',
  ingredients: [emptyIngredient()],
})

function toFormState(recipe) {
  return {
    title: recipe.title || '',
    description: recipe.description || '',
    body_markdown: recipe.body_markdown || '',
    servings: recipe.servings || '',
    prep_minutes: recipe.prep_minutes || '',
    cook_minutes: recipe.cook_minutes || '',
    source_url: recipe.source_url || '',
    image_url: recipe.image_url || '',
    ingredients: (recipe.ingredients || []).map((ri) => ({
      name: (ri.ingredient && ri.ingredient.name) || '',
      quantity: ri.quantity || '',
      unit: ri.unit || '',
      notes: ri.notes || '',
      optional: !!ri.optional,
    })),
  }
}

function toPayload(form) {
  return {
    title: form.title.trim(),
    description: form.description.trim(),
    body_markdown: form.body_markdown,
    servings: Number(form.servings) || 0,
    prep_minutes: Number(form.prep_minutes) || 0,
    cook_minutes: Number(form.cook_minutes) || 0,
    source_url: form.source_url.trim(),
    image_url: form.image_url.trim(),
    ingredients: form.ingredients
      .filter((i) => i.name.trim() !== '')
      .map((i) => ({
        name: i.name.trim(),
        quantity: Number(i.quantity) || 0,
        unit: i.unit.trim(),
        notes: i.notes.trim(),
        optional: !!i.optional,
      })),
  }
}

export function RecipeForm({ mode }) {
  const { recipeId } = useParams()
  const navigate = useNavigate()
  const [form, setForm] = useState(emptyForm())
  const [error, setError] = useState(null)
  const [loading, setLoading] = useState(mode === 'edit')
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (mode !== 'edit') return
    api.getRecipe(recipeId)
      .then((r) => setForm(toFormState(r)))
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false))
  }, [mode, recipeId])

  const setField = (field, value) => setForm((f) => ({ ...f, [field]: value }))

  const setIngredientField = (idx, field, value) => setForm((f) => {
    const ingredients = f.ingredients.slice()
    ingredients[idx] = { ...ingredients[idx], [field]: value }
    return { ...f, ingredients }
  })

  const addIngredientRow = () => setForm((f) => ({ ...f, ingredients: [...f.ingredients, emptyIngredient()] }))
  const removeIngredientRow = (idx) => setForm((f) => ({ ...f, ingredients: f.ingredients.filter((_, i) => i !== idx) }))

  const onSubmit = async (e) => {
    e.preventDefault()
    setSaving(true)
    setError(null)
    try {
      const payload = toPayload(form)
      const saved = mode === 'edit'
        ? await api.updateRecipe(recipeId, payload)
        : await api.createRecipe(payload)
      navigate(`/recipes/${saved.id}`)
    } catch (err) {
      setError(err.message)
    } finally {
      setSaving(false)
    }
  }

  if (loading) return <p>Loading…</p>

  return (
    <div className="page">
      <h1>{mode === 'edit' ? 'Edit recipe' : 'New recipe'}</h1>
      {error && <p className="error">{error}</p>}
      <form className="recipe-form" onSubmit={onSubmit}>
        <label>
          Title
          <input value={form.title} onChange={(e) => setField('title', e.target.value)} required />
        </label>
        <label>
          Description
          <input value={form.description} onChange={(e) => setField('description', e.target.value)} />
        </label>
        <div className="form-row">
          <label>
            Servings
            <input type="number" min="0" value={form.servings} onChange={(e) => setField('servings', e.target.value)} />
          </label>
          <label>
            Prep (min)
            <input type="number" min="0" value={form.prep_minutes} onChange={(e) => setField('prep_minutes', e.target.value)} />
          </label>
          <label>
            Cook (min)
            <input type="number" min="0" value={form.cook_minutes} onChange={(e) => setField('cook_minutes', e.target.value)} />
          </label>
        </div>
        <div className="form-row">
          <label>
            Source URL
            <input value={form.source_url} onChange={(e) => setField('source_url', e.target.value)} />
          </label>
          <label>
            Image URL
            <input value={form.image_url} onChange={(e) => setField('image_url', e.target.value)} />
          </label>
        </div>

        <fieldset>
          <legend>Ingredients</legend>
          {form.ingredients.map((ing, idx) => (
            <div className="ingredient-row" key={idx}>
              <input placeholder="name" value={ing.name} onChange={(e) => setIngredientField(idx, 'name', e.target.value)} />
              <input placeholder="qty" type="number" step="any" value={ing.quantity} onChange={(e) => setIngredientField(idx, 'quantity', e.target.value)} />
              <input placeholder="unit" value={ing.unit} onChange={(e) => setIngredientField(idx, 'unit', e.target.value)} />
              <input placeholder="notes" value={ing.notes} onChange={(e) => setIngredientField(idx, 'notes', e.target.value)} />
              <label className="ingredient-optional-toggle">
                <input type="checkbox" checked={ing.optional} onChange={(e) => setIngredientField(idx, 'optional', e.target.checked)} />
                optional
              </label>
              <button type="button" className="button button-danger" onClick={() => removeIngredientRow(idx)}>Remove</button>
            </div>
          ))}
          <button type="button" className="button" onClick={addIngredientRow}>+ Add ingredient</button>
        </fieldset>

        <label>
          Instructions (Markdown)
          <textarea
            className="markdown-editor"
            rows={16}
            value={form.body_markdown}
            onChange={(e) => setField('body_markdown', e.target.value)}
          />
        </label>

        <div className="form-actions">
          <button className="button button-primary" type="submit" disabled={saving}>
            {saving ? 'Saving…' : 'Save recipe'}
          </button>
        </div>
      </form>
    </div>
  )
}
