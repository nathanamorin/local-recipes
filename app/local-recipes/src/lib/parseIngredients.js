// Parses pasted ingredient lines ("1 ½ cups flour, sifted") into the
// structured rows the editor uses. Best-effort: anything it can't place
// stays in the name so nothing is lost.

const UNICODE_FRACTIONS = { '¼': 0.25, '½': 0.5, '¾': 0.75, '⅓': 1 / 3, '⅔': 2 / 3, '⅛': 0.125, '⅜': 0.375, '⅝': 0.625, '⅞': 0.875 }

const UNITS = [
  'cups', 'cup', 'c', 'tablespoons', 'tablespoon', 'tbsp', 'tbs', 'teaspoons', 'teaspoon', 'tsp',
  'grams', 'gram', 'g', 'kilograms', 'kg', 'milliliters', 'ml', 'liters', 'l', 'ounces', 'ounce', 'oz',
  'pounds', 'pound', 'lbs', 'lb', 'cloves', 'clove', 'cans', 'can', 'pinch', 'pinches', 'sticks', 'stick',
  'slices', 'slice', 'bunch', 'bunches', 'packet', 'packets', 'package', 'packages', 'sprigs', 'sprig',
]

function parseNumber(token) {
  if (!token) return null
  let total = 0
  let matched = false
  for (const part of token.split(/\s+/)) {
    if (UNICODE_FRACTIONS[part] !== undefined) {
      total += UNICODE_FRACTIONS[part]
      matched = true
      continue
    }
    const mixed = /^(\d+)([¼½¾⅓⅔⅛⅜⅝⅞])$/.exec(part)
    if (mixed) {
      total += Number(mixed[1]) + UNICODE_FRACTIONS[mixed[2]]
      matched = true
      continue
    }
    const frac = /^(\d+)\/(\d+)$/.exec(part)
    if (frac) {
      total += Number(frac[1]) / Number(frac[2])
      matched = true
      continue
    }
    if (/^\d+(\.\d+)?$/.test(part)) {
      total += Number(part)
      matched = true
      continue
    }
    return null
  }
  return matched ? total : null
}

export function parseIngredientLine(raw) {
  let line = raw.replace(/^\s*([-*•]|\d+[.)])\s+/, '').trim()
  if (!line) return null
  let optional = false
  if (/\(optional\)|,?\s*optional$/i.test(line)) {
    optional = true
    line = line.replace(/\(optional\)|,?\s*optional$/i, '').trim()
  }

  const qtyMatch = /^((?:\d+\s+)?(?:\d+\/\d+|\d+(?:\.\d+)?[¼½¾⅓⅔⅛⅜⅝⅞]?|[¼½¾⅓⅔⅛⅜⅝⅞])(?:\s+\d+\/\d+)?)\s*(.*)$/.exec(line)
  let quantity = ''
  let rest = line
  if (qtyMatch) {
    const n = parseNumber(qtyMatch[1].trim())
    if (n !== null) {
      quantity = String(Math.round(n * 1000) / 1000)
      rest = qtyMatch[2]
    }
  }

  let unit = ''
  const unitMatch = /^([A-Za-z]+)\.?\s+(.*)$/.exec(rest)
  if (quantity && unitMatch && UNITS.includes(unitMatch[1].toLowerCase())) {
    unit = unitMatch[1].toLowerCase()
    rest = unitMatch[2]
  }
  rest = rest.replace(/^of\s+/i, '')

  let name = rest
  let notes = ''
  const comma = rest.indexOf(',')
  if (comma > 0) {
    name = rest.slice(0, comma)
    notes = rest.slice(comma + 1).trim()
  }
  return { name: name.trim(), quantity, unit, notes, optional }
}

export function parseIngredientList(text) {
  return text
    .split('\n')
    .map(parseIngredientLine)
    .filter((r) => r && r.name)
}
