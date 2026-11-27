import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../lib/api'

function Stars({ value }) {
  if (!value) return <span className="stars stars-none">not yet rated</span>
  const rounded = Math.round(value)
  return (
    <span className="stars" title={`${value.toFixed(1)} / 5`}>
      {'★'.repeat(rounded)}{'☆'.repeat(5 - rounded)}
    </span>
  )
}

export function RecipeLibrary() {
  const [recipes, setRecipes] = useState(null)
  const [error, setError] = useState(null)

  useEffect(() => {
    api.listRecipes().then((d) => setRecipes(d.recipes)).catch((e) => setError(e.message))
  }, [])

  return (
    <div className="page">
      <div className="page-header">
        <h1>Recipes</h1>
        <Link className="button button-primary" to="/recipes/new">+ New recipe</Link>
      </div>
      {error && <p className="error">{error}</p>}
      {recipes === null && !error && <p>Loading…</p>}
      {recipes && recipes.length === 0 && (
        <div className="empty-state">
          <p>No recipes yet. Add one manually, or import one via the MCP server.</p>
        </div>
      )}
      <div className="recipe-grid">
        {recipes && recipes.map((r) => (
          <Link className="recipe-card" key={r.id} to={`/recipes/${r.id}`}>
            {r.image_url && <img className="recipe-card-image" src={r.image_url} alt={r.title} />}
            <div className="recipe-card-body">
              <h3>{r.title}</h3>
              {r.description && <p className="recipe-card-description">{r.description}</p>}
              <div className="recipe-card-meta">
                <Stars value={r.average_rating} />
                {r.rating_count > 0 && <span className="rating-count">({r.rating_count})</span>}
              </div>
            </div>
          </Link>
        ))}
      </div>
    </div>
  )
}
