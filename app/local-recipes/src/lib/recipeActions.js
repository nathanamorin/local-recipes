// Archive/restore/delete with consistent cache updates, shared by the
// recipe page and Settings so the library, archive list and recipe views
// never disagree.
import { api } from './api'
import { invalidate, setCached } from './resource'

const without = (id) => (list) => (list ? list.filter((r) => r.id !== id) : list)

export async function archiveRecipe(id) {
  const archived = await api.archiveRecipe(id)
  setCached('recipes', without(id))
  setCached(`recipe:${id}`, archived)
  invalidate('recipes')
  return archived
}

export async function restoreRecipe(id) {
  const restored = await api.restoreRecipe(id)
  setCached('recipes-archived', without(id))
  setCached(`recipe:${id}`, restored)
  invalidate('recipes') // also matches 'recipes-archived'
  return restored
}

export async function deleteRecipeForever(id) {
  await api.deleteRecipe(id)
  setCached('recipes-archived', without(id))
  setCached('recipes', without(id))
  invalidate('recipes')
  invalidate('schedule:') // planned meals for it are removed server-side
}
