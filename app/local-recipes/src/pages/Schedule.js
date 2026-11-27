import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../lib/api'
import { toDateParam, startOfWeek, addDays, WEEKDAY_NAMES } from '../lib/dates'

export function Schedule() {
  const [config, setConfig] = useState(null)
  const [weekStart, setWeekStart] = useState(() => startOfWeek(new Date()))
  const [meals, setMeals] = useState(null)
  const [error, setError] = useState(null)
  const [generating, setGenerating] = useState(false)

  const weekEnd = useMemo(() => addDays(weekStart, 6), [weekStart])

  useEffect(() => {
    api.getScheduleConfig().then(setConfig).catch((e) => setError(e.message))
  }, [])

  const loadMeals = () => {
    api.listSchedule(toDateParam(weekStart), toDateParam(weekEnd))
      .then((d) => setMeals(d.meals))
      .catch((e) => setError(e.message))
  }

  useEffect(loadMeals, [weekStart]) // eslint-disable-line react-hooks/exhaustive-deps

  const saveConfig = async (e) => {
    e.preventDefault()
    try {
      const saved = await api.saveScheduleConfig({
        meals_per_week: Number(config.meals_per_week) || 0,
        meals_to_cook: Number(config.meals_to_cook) || 0,
        leftover_days: Number(config.leftover_days) || 0,
      })
      setConfig(saved)
    } catch (err) {
      setError(err.message)
    }
  }

  const generate = async () => {
    setGenerating(true)
    setError(null)
    try {
      await api.generateSchedule(toDateParam(weekStart))
      loadMeals()
    } catch (err) {
      setError(err.message)
    } finally {
      setGenerating(false)
    }
  }

  const mealsByDate = useMemo(() => {
    const m = {}
    ;(meals || []).forEach((meal) => { m[meal.date.slice(0, 10)] = meal })
    return m
  }, [meals])

  const days = useMemo(() => Array.from({ length: 7 }, (_, i) => addDays(weekStart, i)), [weekStart])

  return (
    <div className="page">
      <h1>Meal schedule</h1>
      {error && <p className="error">{error}</p>}

      {config && (
        <form className="schedule-config-form" onSubmit={saveConfig}>
          <label>
            Meals per week
            <input type="number" min="0" value={config.meals_per_week}
              onChange={(e) => setConfig({ ...config, meals_per_week: e.target.value })} />
          </label>
          <label>
            Meals to cook
            <input type="number" min="0" value={config.meals_to_cook}
              onChange={(e) => setConfig({ ...config, meals_to_cook: e.target.value })} />
          </label>
          <label>
            Leftover days (per cooked meal)
            <input type="number" min="0" value={config.leftover_days}
              onChange={(e) => setConfig({ ...config, leftover_days: e.target.value })} />
          </label>
          <button className="button" type="submit">Save</button>
        </form>
      )}

      <div className="schedule-week-controls">
        <button className="button" onClick={() => setWeekStart(addDays(weekStart, -7))}>← Previous week</button>
        <span className="schedule-week-label">
          Week of {toDateParam(weekStart)} – {toDateParam(weekEnd)}
        </span>
        <button className="button" onClick={() => setWeekStart(addDays(weekStart, 7))}>Next week →</button>
        <button className="button button-primary" onClick={generate} disabled={generating}>
          {generating ? 'Generating…' : 'Generate / regenerate this week'}
        </button>
      </div>

      <div className="schedule-grid">
        {days.map((day, idx) => {
          const key = toDateParam(day)
          const meal = mealsByDate[key]
          return (
            <div className="schedule-day" key={key}>
              <div className="schedule-day-header">
                <strong>{WEEKDAY_NAMES[idx]}</strong>
                <span>{key}</span>
              </div>
              {meal ? (
                <Link to={`/recipes/${meal.recipe_id}`} className="schedule-meal">
                  <span className="schedule-meal-title">{meal.recipe ? meal.recipe.title : meal.recipe_id}</span>
                  {meal.is_leftover && <span className="schedule-meal-badge">leftovers</span>}
                </Link>
              ) : (
                <div className="schedule-meal schedule-meal-empty">no meal planned</div>
              )}
            </div>
          )
        })}
      </div>
    </div>
  )
}
