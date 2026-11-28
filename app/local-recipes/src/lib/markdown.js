// Recipe bodies are free-form Markdown and often repeat the ingredient
// list (imports especially). When the recipe has structured ingredients,
// the detail page shows those in an interactive checklist instead, so the
// duplicate "Ingredients" section is dropped from the rendered body.
export function stripIngredientsSection(markdown) {
  if (!markdown) return ''
  const lines = markdown.split('\n')
  const out = []
  let skipLevel = 0
  for (const line of lines) {
    const m = /^(#{1,6})\s+(.*)$/.exec(line)
    if (m) {
      const level = m[1].length
      if (skipLevel && level <= skipLevel) skipLevel = 0
      if (!skipLevel && /^ingredients\b/i.test(m[2].trim())) {
        skipLevel = level
        continue
      }
    }
    if (!skipLevel) out.push(line)
  }
  return out.join('\n').trim()
}
