// dates.js holds the small set of date-math helpers the Schedule page
// needs - formatting as the YYYY-MM-DD strings the API expects, and
// finding the Monday that starts the week containing a given date.
export function toDateParam(date) {
  const y = date.getFullYear()
  const m = String(date.getMonth() + 1).padStart(2, '0')
  const d = String(date.getDate()).padStart(2, '0')
  return `${y}-${m}-${d}`
}

export function startOfWeek(date) {
  const d = new Date(date)
  const day = d.getDay() // 0 = Sunday ... 6 = Saturday
  const diff = (day === 0 ? -6 : 1) - day // shift so Monday is day 0
  d.setDate(d.getDate() + diff)
  d.setHours(0, 0, 0, 0)
  return d
}

export function addDays(date, days) {
  const d = new Date(date)
  d.setDate(d.getDate() + days)
  return d
}

export const WEEKDAY_NAMES = ['Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday', 'Sunday']
