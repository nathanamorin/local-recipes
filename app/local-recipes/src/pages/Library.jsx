import { useDeferredValue, useMemo, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { api } from '../lib/api'
import { useResource } from '../lib/resource'
import { formatMinutes, pluralize, totalMinutes } from '../lib/format'
import { Icon } from '../components/Icon'
import { EmptyState, ErrorState, PageHeader, RecipeCover, Skeleton, Stars } from '../components/ui'

const SORTS = {
  recent: { label: 'Recently updated', fn: (a, b) => new Date(b.updated_at) - new Date(a.updated_at) },
  rating: { label: 'Highest rated', fn: (a, b) => (b.average_rating || 0) - (a.average_rating || 0) },
  quickest: {
    label: 'Quickest',
    fn: (a, b) => (totalMinutes(a) || Infinity) - (totalMinutes(b) || Infinity),
  },
  title: { label: 'A–Z', fn: (a, b) => a.title.localeCompare(b.title) },
}

const TIME_FILTERS = [
  { id: 'all', label: 'Any time' },
  { id: '30', label: '≤ 30 min', max: 30 },
  { id: '60', label: '≤ 1 hr', max: 60 },
]

function RecipeCard({ recipe }) {
  const total = totalMinutes(recipe)
  return (
    <li>
      <Link className="recipe-card" to={`/recipes/${recipe.id}`}>
        <RecipeCover recipe={recipe} className="recipe-card-cover" />
        <div className="recipe-card-body">
          <h3 className="recipe-card-title">{recipe.title}</h3>
          {recipe.description && <p className="recipe-card-desc">{recipe.description}</p>}
          <div className="recipe-card-meta">
            {total > 0 && (
              <span className="meta-item">
                <Icon name="clock" size={15} /> {formatMinutes(total)}
              </span>
            )}
            {recipe.servings > 0 && (
              <span className="meta-item">
                <Icon name="users" size={15} /> {recipe.servings}
              </span>
            )}
            <Stars value={recipe.average_rating} count={recipe.rating_count} />
          </div>
        </div>
      </Link>
    </li>
  )
}

function LibrarySkeleton() {
  return (
    <ul className="recipe-grid" aria-hidden="true">
      {Array.from({ length: 6 }, (_, i) => (
        <li key={i} className="recipe-card recipe-card-skeleton">
          <Skeleton className="recipe-card-cover" />
          <div className="recipe-card-body">
            <Skeleton style={{ height: 20, width: '70%' }} />
            <Skeleton style={{ height: 14, width: '95%', marginTop: 10 }} />
            <Skeleton style={{ height: 14, width: '60%', marginTop: 6 }} />
          </div>
        </li>
      ))}
    </ul>
  )
}

export function Library() {
  const { data: recipes, error, loading, reload } = useResource('recipes', api.listRecipes)
  const [params, setParams] = useSearchParams()
  const [query, setQuery] = useState(params.get('q') || '')
  const deferredQuery = useDeferredValue(query)
  const sort = SORTS[params.get('sort')] ? params.get('sort') : 'recent'
  const time = TIME_FILTERS.find((t) => t.id === params.get('time')) || TIME_FILTERS[0]

  const setParam = (key, value, fallback) => {
    const next = new URLSearchParams(params)
    if (!value || value === fallback) next.delete(key)
    else next.set(key, value)
    setParams(next, { replace: true })
  }

  const visible = useMemo(() => {
    if (!recipes) return []
    const q = deferredQuery.trim().toLowerCase()
    return recipes
      .filter((r) => !q || r.title.toLowerCase().includes(q) || (r.description || '').toLowerCase().includes(q))
      .filter((r) => !time.max || (totalMinutes(r) > 0 && totalMinutes(r) <= time.max))
      .sort(SORTS[sort].fn)
  }, [recipes, deferredQuery, sort, time])

  const hasRecipes = recipes && recipes.length > 0

  return (
    <div className="page">
      <PageHeader
        title="Recipes"
        subtitle={hasRecipes ? pluralize(recipes.length, 'recipe') + ' in your library' : undefined}
        actions={
          <Link className="btn btn-primary" to="/recipes/new">
            <Icon name="plus" size={18} /> New recipe
          </Link>
        }
      />

      {hasRecipes && (
        <div className="toolbar">
          <label className="search-field">
            <Icon name="search" size={18} />
            <span className="visually-hidden">Search recipes</span>
            <input
              type="search"
              placeholder="Search recipes"
              value={query}
              onChange={(e) => {
                setQuery(e.target.value)
                setParam('q', e.target.value, '')
              }}
            />
          </label>
          <div className="segmented" role="group" aria-label="Total time">
            {TIME_FILTERS.map((t) => (
              <button
                key={t.id}
                className="segmented-btn"
                aria-pressed={time.id === t.id}
                onClick={() => setParam('time', t.id, 'all')}
              >
                {t.label}
              </button>
            ))}
          </div>
          <label className="select-field">
            <Icon name="sort" size={16} />
            <span className="visually-hidden">Sort by</span>
            <select value={sort} onChange={(e) => setParam('sort', e.target.value, 'recent')}>
              {Object.entries(SORTS).map(([id, s]) => (
                <option key={id} value={id}>
                  {s.label}
                </option>
              ))}
            </select>
          </label>
        </div>
      )}

      {loading && <LibrarySkeleton />}
      {error && <ErrorState error={error} onRetry={reload} />}

      {recipes && recipes.length === 0 && (
        <EmptyState
          title="Your recipe box is empty"
          action={
            <Link className="btn btn-primary" to="/recipes/new">
              <Icon name="plus" size={18} /> Add your first recipe
            </Link>
          }
        >
          Add a recipe by hand, or ask your AI assistant to import one through the MCP server.
        </EmptyState>
      )}

      {hasRecipes && visible.length === 0 && (
        <EmptyState icon="search" title="No matching recipes">
          Try a different search or time filter.
        </EmptyState>
      )}

      {visible.length > 0 && (
        <ul className="recipe-grid">
          {visible.map((r) => (
            <RecipeCard key={r.id} recipe={r} />
          ))}
        </ul>
      )}
    </div>
  )
}
