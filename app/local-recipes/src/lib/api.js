// Thin fetch wrapper around the local-recipes HTTP API. All routes live
// under /api (see handlers.APIPrefix) so they never collide with the SPA's
// client-side routes. Identity comes from the X-Auth-Request-User header
// the SSO gate injects upstream - the browser never handles auth itself.

const API_PREFIX = '/api'

export class ApiError extends Error {
  constructor(message, status) {
    super(message)
    this.status = status
  }
}

async function request(path, { method = 'GET', body } = {}) {
  const res = await fetch(API_PREFIX + path, {
    method,
    headers: body !== undefined ? { 'Content-Type': 'application/json' } : undefined,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })
  const text = await res.text()
  let data = null
  if (text) {
    try {
      data = JSON.parse(text)
    } catch {
      data = null
    }
  }
  if (!res.ok) {
    throw new ApiError((data && data.error) || `Request failed (${res.status})`, res.status)
  }
  return data
}

const enc = encodeURIComponent

export const api = {
  getCurrentUser: () => request('/me'),

  listRecipes: () => request('/recipes').then((d) => d.recipes || []),
  getRecipe: (id) => request(`/recipes/${enc(id)}`),
  createRecipe: (recipe) => request('/recipes', { method: 'POST', body: recipe }),
  updateRecipe: (id, recipe) => request(`/recipes/${enc(id)}`, { method: 'PUT', body: recipe }),
  deleteRecipe: (id) => request(`/recipes/${enc(id)}`, { method: 'DELETE' }),
  listArchivedRecipes: () => request('/recipes?archived=true').then((d) => d.recipes || []),
  archiveRecipe: (id) => request(`/recipes/${enc(id)}/archive`, { method: 'POST' }),
  restoreRecipe: (id) => request(`/recipes/${enc(id)}/restore`, { method: 'POST' }),

  // 204 (not rated yet) parses to null.
  getRating: (id) => request(`/recipes/${enc(id)}/rating`),
  rateRecipe: (id, score, notes) => request(`/recipes/${enc(id)}/rating`, { method: 'PUT', body: { score, notes } }),

  listIngredients: () => request('/ingredients').then((d) => d.ingredients || []),
  addIngredient: (ingredient) => request('/ingredients', { method: 'POST', body: ingredient }),
  deleteIngredient: (id) => request(`/ingredients/${enc(id)}`, { method: 'DELETE' }),
  adjustStock: (id, delta, unit) => request(`/ingredients/${enc(id)}/stock`, { method: 'PUT', body: { delta, unit } }),
  setStock: (id, quantity, unit) => request(`/ingredients/${enc(id)}/stock`, { method: 'PUT', body: { quantity, unit } }),
  setLowStockThreshold: (id, threshold) =>
    request(`/ingredients/${enc(id)}/low-stock-threshold`, { method: 'PUT', body: { threshold } }),

  getScheduleConfig: () => request('/schedule/config'),
  saveScheduleConfig: (config) => request('/schedule/config', { method: 'PUT', body: config }),
  generateSchedule: (weekStart) =>
    request(`/schedule/generate?week_start=${enc(weekStart)}`, { method: 'POST' }).then((d) => d.meals || []),
  listSchedule: (from, to) => request(`/schedule?from=${enc(from)}&to=${enc(to)}`).then((d) => d.meals || []),
}
