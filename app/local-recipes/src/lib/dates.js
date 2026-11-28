// Date helpers for the meal planner. The API speaks YYYY-MM-DD in the
// user's local calendar, and weeks start on Monday.

export function toDateParam(date) {
  const y = date.getFullYear()
  const m = String(date.getMonth() + 1).padStart(2, '0')
  const d = String(date.getDate()).padStart(2, '0')
  return `${y}-${m}-${d}`
}

export function startOfWeek(date) {
  const d = new Date(date)
  const day = d.getDay()
  d.setDate(d.getDate() + ((day === 0 ? -6 : 1) - day))
  d.setHours(0, 0, 0, 0)
  return d
}

export function addDays(date, days) {
  const d = new Date(date)
  d.setDate(d.getDate() + days)
  return d
}

export function isSameDay(a, b) {
  return toDateParam(a) === toDateParam(b)
}

export function weekLabel(start) {
  const end = addDays(start, 6)
  const sameMonth = start.getMonth() === end.getMonth()
  const fmt = (d, opts) => d.toLocaleDateString(undefined, opts)
  const left = fmt(start, { month: 'short', day: 'numeric' })
  const right = sameMonth ? fmt(end, { day: 'numeric' }) : fmt(end, { month: 'short', day: 'numeric' })
  return `${left} – ${right}`
}

export function relativeWeek(start) {
  const diff = Math.round((start - startOfWeek(new Date())) / (7 * 86_400_000))
  if (diff === 0) return 'This week'
  if (diff === 1) return 'Next week'
  if (diff === -1) return 'Last week'
  return diff > 0 ? `In ${diff} weeks` : `${-diff} weeks ago`
}
