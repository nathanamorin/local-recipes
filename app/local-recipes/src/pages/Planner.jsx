import { useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../lib/api'
import { invalidate, setCached, useResource } from '../lib/resource'
import { addDays, isSameDay, relativeWeek, startOfWeek, toDateParam, weekLabel } from '../lib/dates'
import { formatMinutes, pluralize, totalMinutes } from '../lib/format'
import { Icon } from '../components/Icon'
import { ConfirmDialog, Dialog } from '../components/Dialog'
import { useToast } from '../components/Toast'
import { EmptyState, ErrorState, PageHeader, RecipeCover, Skeleton } from '../components/ui'

function SettingsDialog({ open, onClose, config }) {
  const toast = useToast()
  const [form, setForm] = useState(null)
  const values = form || {
    meals_per_week: String(config ? config.meals_per_week : 7),
    meals_to_cook: String(config ? config.meals_to_cook : 4),
    leftover_days: String(config ? config.leftover_days : 1),
  }
  const set = (k) => (e) => setForm({ ...values, [k]: e.target.value.replace(/\D/g, '') })
  const close = () => {
    setForm(null)
    onClose()
  }
  const submit = async (e) => {
    e.preventDefault()
    try {
      const saved = await api.saveScheduleConfig({
        meals_per_week: Number(values.meals_per_week) || 0,
        meals_to_cook: Number(values.meals_to_cook) || 0,
        leftover_days: Number(values.leftover_days) || 0,
      })
      setCached('schedule-config', saved)
      toast('Plan settings saved')
      close()
    } catch (err) {
      toast(err.message, { tone: 'error' })
    }
  }
  return (
    <Dialog
      open={open}
      onClose={close}
      title="Plan settings"
      footer={
        <>
          <button type="button" className="btn btn-ghost" onClick={close}>
            Cancel
          </button>
          <button type="submit" form="plan-settings" className="btn btn-primary">
            Save
          </button>
        </>
      }
    >
      <form id="plan-settings" className="stack" onSubmit={submit}>
        <p className="muted">Generating a week picks your least-recently-cooked recipes and fills the gaps with leftovers.</p>
        <label className="field">
          <span className="field-label">Dinners to plan per week</span>
          <input inputMode="numeric" value={values.meals_per_week} onChange={set('meals_per_week')} />
        </label>
        <div className="field-grid field-grid-2">
          <label className="field">
            <span className="field-label">Nights you cook</span>
            <input inputMode="numeric" value={values.meals_to_cook} onChange={set('meals_to_cook')} />
          </label>
          <label className="field">
            <span className="field-label">Leftover nights per cook</span>
            <input inputMode="numeric" value={values.leftover_days} onChange={set('leftover_days')} />
          </label>
        </div>
      </form>
    </Dialog>
  )
}

function DayRow({ date, meal, cookedOnLabel }) {
  const today = isSameDay(date, new Date())
  const past = !today && date < new Date()
  const recipe = meal && meal.recipe
  return (
    <li className={`day-row ${today ? 'is-today' : ''} ${past ? 'is-past' : ''}`}>
      <div className="day-date">
        <span className="day-weekday">{date.toLocaleDateString(undefined, { weekday: 'short' })}</span>
        <span className="day-num">{date.getDate()}</span>
        {today && <span className="today-badge">Today</span>}
      </div>
      {meal ? (
        <Link to={`/recipes/${meal.recipe_id}`} className="day-meal">
          <RecipeCover recipe={recipe || { title: '?' }} className="day-meal-cover" />
          <span className="day-meal-text">
            <span className="day-meal-title">{recipe ? recipe.title : 'Recipe'}</span>
            <span className="day-meal-meta">
              {meal.is_leftover ? (
                <span className="pill pill-muted">
                  <Icon name="leftovers" size={13} /> Leftovers{cookedOnLabel ? ` from ${cookedOnLabel}` : ''}
                </span>
              ) : (
                <span className="pill pill-accent">
                  <Icon name="flame" size={13} /> Cook
                </span>
              )}
              {!meal.is_leftover && recipe && totalMinutes(recipe) > 0 && (
                <span className="meta-item">
                  <Icon name="clock" size={13} /> {formatMinutes(totalMinutes(recipe))}
                </span>
              )}
            </span>
          </span>
          <Icon name="chevronRight" size={18} className="day-meal-chevron" />
        </Link>
      ) : (
        <div className="day-empty">Nothing planned</div>
      )}
    </li>
  )
}

export default function Planner() {
  const toast = useToast()
  const [weekStart, setWeekStart] = useState(() => startOfWeek(new Date()))
  const from = toDateParam(weekStart)
  const to = toDateParam(addDays(weekStart, 6))
  const { data: meals, error, loading, reload } = useResource(`schedule:${from}`, () => api.listSchedule(from, to))
  const { data: config } = useResource('schedule-config', api.getScheduleConfig)
  const { data: recipes } = useResource('recipes', api.listRecipes)
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [confirmRegen, setConfirmRegen] = useState(false)
  const [generating, setGenerating] = useState(false)

  const days = useMemo(() => Array.from({ length: 7 }, (_, i) => addDays(weekStart, i)), [weekStart])
  const byDate = useMemo(() => {
    const m = new Map()
    for (const meal of meals || []) m.set(meal.date.slice(0, 10), meal)
    return m
  }, [meals])
  const byId = useMemo(() => new Map((meals || []).map((m) => [m.id, m])), [meals])

  const cooked = (meals || []).filter((m) => !m.is_leftover).length
  const leftovers = (meals || []).filter((m) => m.is_leftover).length
  const isCurrentWeek = isSameDay(weekStart, startOfWeek(new Date()))
  const noRecipes = recipes && recipes.length === 0

  const generate = async () => {
    setConfirmRegen(false)
    setGenerating(true)
    try {
      const fresh = await api.generateSchedule(from)
      setCached(`schedule:${from}`, fresh)
      invalidate(`schedule:${from}`)
      toast(`Planned ${relativeWeek(weekStart).toLowerCase()}`)
    } catch (e) {
      toast(e.message, { tone: 'error' })
    } finally {
      setGenerating(false)
    }
  }

  const cookedOnLabel = (meal) => {
    if (!meal.is_leftover || !meal.cooked_from_id) return ''
    const src = byId.get(meal.cooked_from_id)
    return src ? new Date(src.date.slice(0, 10) + 'T00:00').toLocaleDateString(undefined, { weekday: 'long' }) : ''
  }

  return (
    <div className="page page-narrow">
      <PageHeader
        title="Meal plan"
        subtitle={
          meals && meals.length
            ? `${pluralize(cooked, 'night')} cooking · ${pluralize(leftovers, 'leftover night')}`
            : config
              ? `${config.meals_to_cook} cooked dinners, ${config.leftover_days} leftover night${config.leftover_days === 1 ? '' : 's'} each`
              : undefined
        }
        actions={
          <>
            <button className="btn btn-secondary" onClick={() => setSettingsOpen(true)}>
              <Icon name="sliders" size={18} /> Settings
            </button>
            <button
              className="btn btn-primary"
              onClick={() => (meals && meals.length ? setConfirmRegen(true) : generate())}
              disabled={generating || noRecipes}
            >
              <Icon name="sparkles" size={18} /> {generating ? 'Planning…' : meals && meals.length ? 'Regenerate' : 'Plan this week'}
            </button>
          </>
        }
      />

      <div className="week-nav">
        <button className="icon-button" aria-label="Previous week" onClick={() => setWeekStart(addDays(weekStart, -7))}>
          <Icon name="chevronLeft" />
        </button>
        <div className="week-nav-label" aria-live="polite">
          <span className="week-nav-relative">{relativeWeek(weekStart)}</span>
          <span className="week-nav-range">{weekLabel(weekStart)}</span>
        </div>
        <button className="icon-button" aria-label="Next week" onClick={() => setWeekStart(addDays(weekStart, 7))}>
          <Icon name="chevronRight" />
        </button>
        {!isCurrentWeek && (
          <button className="btn btn-ghost btn-sm week-nav-today" onClick={() => setWeekStart(startOfWeek(new Date()))}>
            Today
          </button>
        )}
      </div>

      {error && <ErrorState error={error} onRetry={reload} />}

      {loading && (
        <ul className="day-list" aria-hidden="true">
          {days.map((d) => (
            <li key={d.toISOString()} className="day-row">
              <Skeleton style={{ height: 44, width: 44 }} />
              <Skeleton style={{ height: 56, flex: 1 }} />
            </li>
          ))}
        </ul>
      )}

      {meals && meals.length === 0 && (
        <EmptyState
          icon="calendar"
          title={noRecipes ? 'Add recipes to start planning' : 'Nothing planned yet'}
          action={
            noRecipes ? (
              <Link className="btn btn-primary" to="/recipes/new">
                <Icon name="plus" size={18} /> Add a recipe
              </Link>
            ) : (
              <button className="btn btn-primary" onClick={generate} disabled={generating}>
                <Icon name="sparkles" size={18} /> {generating ? 'Planning…' : 'Plan this week'}
              </button>
            )
          }
        >
          {noRecipes
            ? 'The planner picks dinners from your recipe library.'
            : 'Generate a week of dinners from your least-recently-cooked recipes, with leftovers built in.'}
        </EmptyState>
      )}

      {meals && meals.length > 0 && (
        <ul className="day-list">
          {days.map((d) => {
            const meal = byDate.get(toDateParam(d))
            return <DayRow key={d.toISOString()} date={d} meal={meal} cookedOnLabel={meal ? cookedOnLabel(meal) : ''} />
          })}
        </ul>
      )}

      <SettingsDialog open={settingsOpen} onClose={() => setSettingsOpen(false)} config={config} />
      <ConfirmDialog
        open={confirmRegen}
        onClose={() => setConfirmRegen(false)}
        onConfirm={generate}
        title="Regenerate this week?"
        message="The current plan for this week will be replaced with a fresh one."
        confirmLabel="Regenerate"
      />
    </div>
  )
}
