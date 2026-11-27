import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import ReactMarkdown from 'react-markdown'
import { api } from '../lib/api'

function RatingWidget({ recipeId, onRated }) {
  const [current, setCurrent] = useState(null)
  const [notes, setNotes] = useState('')
  const [hover, setHover] = useState(0)

  useEffect(() => {
    api.getRating(recipeId)
      .then((r) => { setCurrent(r.score); setNotes(r.notes || '') })
      .catch(() => {})
  }, [recipeId])

  const rate = async (score) => {
    await api.rateRecipe(recipeId, score, notes)
    setCurrent(score)
    onRated && onRated()
  }

  return (
    <div className="rating-widget">
      <div className="rating-widget-stars">
        {[1, 2, 3, 4, 5].map((n) => (
          <button
            key={n}
            type="button"
            className={`rating-star ${(hover || current) >= n ? 'rating-star-on' : ''}`}
            onMouseEnter={() => setHover(n)}
            onMouseLeave={() => setHover(0)}
            onClick={() => rate(n)}
            aria-label={`Rate ${n} stars`}
          >★</button>
        ))}
      </div>
      <textarea
        className="rating-notes"
        placeholder="Notes about this attempt (optional)"
        value={notes}
        onChange={(e) => setNotes(e.target.value)}
        onBlur={() => current && rate(current)}
      />
    </div>
  )
}

export function RecipeDetail() {
  const { recipeId } = useParams()
  const navigate = useNavigate()
  const [recipe, setRecipe] = useState(null)
  const [error, setError] = useState(null)

  const load = () => api.getRecipe(recipeId).then(setRecipe).catch((e) => setError(e.message))

  useEffect(() => { load() }, [recipeId]) // eslint-disable-line react-hooks/exhaustive-deps

  const onDelete = async () => {
    if (!window.confirm(`Delete "${recipe.title}"? This cannot be undone.`)) return
    await api.deleteRecipe(recipeId)
    navigate('/')
  }

  if (error) return <p className="error">{error}</p>
  if (!recipe) return <p>Loading…</p>

  return (
    <div className="page recipe-detail">
      <div className="page-header">
        <h1>{recipe.title}</h1>
        <div className="page-actions">
          <Link className="button" to={`/recipes/${recipe.id}/edit`}>Edit</Link>
          <button className="button button-danger" onClick={onDelete}>Delete</button>
        </div>
      </div>

      {recipe.description && <p className="recipe-description">{recipe.description}</p>}

      <div className="recipe-detail-meta">
        {recipe.prep_minutes > 0 && <span>Prep: {recipe.prep_minutes}m</span>}
        {recipe.cook_minutes > 0 && <span>Cook: {recipe.cook_minutes}m</span>}
        {recipe.servings > 0 && <span>Servings: {recipe.servings}</span>}
        {recipe.source_url && <a href={recipe.source_url} target="_blank" rel="noreferrer">Source</a>}
      </div>

      {recipe.ingredients && recipe.ingredients.length > 0 && (
        <section>
          <h2>Ingredients</h2>
          <ul className="ingredient-list">
            {recipe.ingredients.map((ri) => (
              <li key={ri.id}>
                {ri.quantity ? `${ri.quantity} ` : ''}{ri.unit ? `${ri.unit} ` : ''}
                {(ri.ingredient && ri.ingredient.name) || ri.ingredient_id}
                {ri.optional && <span className="ingredient-optional"> (optional)</span>}
                {ri.notes && <span className="ingredient-notes"> ({ri.notes})</span>}
              </li>
            ))}
          </ul>
        </section>
      )}

      <section className="recipe-body">
        <ReactMarkdown>{recipe.body_markdown || ''}</ReactMarkdown>
      </section>

      <section>
        <h2>Rate this recipe</h2>
        <RatingWidget recipeId={recipeId} onRated={load} />
      </section>
    </div>
  )
}
