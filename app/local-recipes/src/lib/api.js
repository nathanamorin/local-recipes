// api.js is a thin fetch wrapper around local-recipes' HTTP API. Requests
// go through the dev server's proxy (see package.json's "proxy" field) in
// development, or are same-origin in production (the Go server serves
// both the API and the built frontend - see cmd/server/main.go), so paths
// here are always relative, with no host.
//
// There's intentionally no client-side auth of any kind: identity comes
// from the X-Auth-Request-User header Istio injects upstream of this app
// (see internal/identity), which the browser never sees or needs to
// handle directly.
// All API routes live under /api (see handlers.APIPrefix) so they never
// collide with the SPA's own client-side routes.
const API_PREFIX = '/api'

async function request(path, options = {}) {
  const res = await fetch(API_PREFIX + path, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  })
  const text = await res.text()
  const data = text ? JSON.parse(text) : null
  if (!res.ok) {
    const message = (data && data.error) || `request to ${path} failed (${res.status})`
    throw new Error(message)
  }
  return data
}

const get = (path) => request(path)
const del = (path) => request(path, { method: 'DELETE' })
const post = (path, body) => request(path, { method: 'POST', body: body !== undefined ? JSON.stringify(body) : undefined })
const put = (path, body) => request(path, { method: 'PUT', body: body !== undefined ? JSON.stringify(body) : undefined })

export const api = {
  getCurrentUser: () => get('/me'),

  listRecipes: () => get('/recipes'),
  getRecipe: (id) => get(`/recipes/${encodeURIComponent(id)}`),
  createRecipe: (recipe) => post('/recipes', recipe),
  updateRecipe: (id, recipe) => put(`/recipes/${encodeURIComponent(id)}`, recipe),
  deleteRecipe: (id) => del(`/recipes/${encodeURIComponent(id)}`),

  getRating: (recipeId) => get(`/recipes/${encodeURIComponent(recipeId)}/rating`),
  rateRecipe: (recipeId, score, notes) => put(`/recipes/${encodeURIComponent(recipeId)}/rating`, { score, notes }),

  listIngredients: () => get('/ingredients'),
  listLowStockIngredients: () => get('/ingredients/low-stock'),
  addIngredient: (ingredient) => post('/ingredients', ingredient),
  deleteIngredient: (id) => del(`/ingredients/${encodeURIComponent(id)}`),
  adjustStock: (id, delta, unit) => put(`/ingredients/${encodeURIComponent(id)}/stock`, { delta, unit }),
  setStock: (id, quantity, unit) => put(`/ingredients/${encodeURIComponent(id)}/stock`, { quantity, unit }),
  setLowStockThreshold: (id, threshold) => put(`/ingredients/${encodeURIComponent(id)}/low-stock-threshold`, { threshold }),

  getScheduleConfig: () => get('/schedule/config'),
  saveScheduleConfig: (config) => put('/schedule/config', config),
  generateSchedule: (weekStart) => post(`/schedule/generate?week_start=${encodeURIComponent(weekStart)}`),
  listSchedule: (from, to) => get(`/schedule?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`),
}
