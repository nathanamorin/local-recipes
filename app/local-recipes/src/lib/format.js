// Display formatting. Quantities are stored as plain numbers (so they can
// be scaled) and only rendered as kitchen fractions here.

const FRACTIONS = [
  [1 / 8, '⅛'],
  [1 / 4, '¼'],
  [1 / 3, '⅓'],
  [3 / 8, '⅜'],
  [1 / 2, '½'],
  [5 / 8, '⅝'],
  [2 / 3, '⅔'],
  [3 / 4, '¾'],
  [7 / 8, '⅞'],
]

// Units where fractions read naturally; metric weights/volumes stay decimal.
const FRACTION_UNITS = new Set([
  '', 'cup', 'cups', 'c', 'tbsp', 'tablespoon', 'tablespoons', 'tsp', 'teaspoon', 'teaspoons',
  'count', 'whole', 'clove', 'cloves', 'can', 'cans', 'pinch', 'stick', 'sticks', 'lb', 'lbs', 'oz',
  'piece', 'pieces', 'slice', 'slices', 'bunch', 'large', 'medium', 'small', 'egg', 'eggs',
])

export function formatQuantity(value, unit = '') {
  if (!value || value <= 0) return ''
  const u = unit.trim().toLowerCase()
  if (!FRACTION_UNITS.has(u)) {
    if (value >= 100) return String(Math.round(value))
    return String(Math.round(value * 10) / 10)
  }
  const whole = Math.floor(value)
  const rest = value - whole
  if (rest < 0.04) return String(whole || '')
  if (rest > 0.96) return String(whole + 1)
  let best = null
  for (const [f, glyph] of FRACTIONS) {
    const diff = Math.abs(rest - f)
    if (diff < 0.04 && (!best || diff < best.diff)) best = { diff, glyph }
  }
  if (best) return `${whole || ''}${best.glyph}`
  return String(Math.round(value * 100) / 100)
}

// "count" is how the stock model spells "no unit" - don't print it.
export function displayUnit(unit, quantity) {
  const u = (unit || '').trim()
  if (!u || u.toLowerCase() === 'count') return ''
  return u
}

export function formatMinutes(min) {
  if (!min || min <= 0) return ''
  if (min < 60) return `${min} min`
  const h = Math.floor(min / 60)
  const m = min % 60
  return m ? `${h} hr ${m} min` : `${h} hr`
}

export function totalMinutes(recipe) {
  return (recipe.prep_minutes || 0) + (recipe.cook_minutes || 0)
}

export function pluralize(n, one, many = `${one}s`) {
  return `${n} ${n === 1 ? one : many}`
}

export function relativeDate(iso) {
  if (!iso) return ''
  const then = new Date(iso)
  const days = Math.round((Date.now() - then.getTime()) / 86_400_000)
  if (days <= 0) return 'today'
  if (days === 1) return 'yesterday'
  if (days < 7) return `${days} days ago`
  if (days < 30) return `${Math.round(days / 7)} wk ago`
  return then.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' })
}

// Initials for a person: "Jane Doe" -> "JD", "dev-user@x.com" -> "DU".
export function personInitials(label) {
  const parts = (label || '?').replace(/@.*/, '').split(/[\s._-]+/).filter(Boolean)
  return (parts.length > 1 ? parts[0][0] + parts[1][0] : (parts[0] || '?').slice(0, 2)).toUpperCase()
}

// Stable warm hue per title, for recipe placeholders.
export function hueFor(text) {
  let h = 0
  for (let i = 0; i < text.length; i++) h = (h * 31 + text.charCodeAt(i)) >>> 0
  const palette = [18, 28, 38, 95, 150, 175, 205, 340, 8]
  return palette[h % palette.length]
}

export function initialsFor(text) {
  const words = text.replace(/[^\p{L}\p{N}\s]/gu, '').split(/\s+/).filter(Boolean)
  if (words.length === 0) return '?'
  if (words.length === 1) return words[0].slice(0, 2).toUpperCase()
  return (words[0][0] + words[words.length - 1][0]).toUpperCase()
}
