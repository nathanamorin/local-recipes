import { Link } from 'react-router-dom'
import { Icon } from './Icon'
import { hueFor, initialsFor } from '../lib/format'

export function PageHeader({ title, eyebrow, subtitle, actions, back }) {
  return (
    <header className="page-header">
      <div className="page-header-text">
        {back && (
          <Link to={back.to} className="back-link">
            <Icon name="arrowLeft" size={16} /> {back.label}
          </Link>
        )}
        {eyebrow && <p className="eyebrow">{eyebrow}</p>}
        <h1>{title}</h1>
        {subtitle && <p className="page-subtitle">{subtitle}</p>}
      </div>
      {actions && <div className="page-actions">{actions}</div>}
    </header>
  )
}

export function EmptyState({ icon = 'book', title, children, action }) {
  return (
    <div className="empty-state">
      <div className="empty-state-icon">
        <Icon name={icon} size={28} />
      </div>
      <h2>{title}</h2>
      {children && <p>{children}</p>}
      {action}
    </div>
  )
}

export function ErrorState({ error, onRetry }) {
  return (
    <div className="empty-state empty-state-error" role="alert">
      <div className="empty-state-icon">
        <Icon name="alert" size={28} />
      </div>
      <h2>Something went wrong</h2>
      <p>{error && error.message ? error.message : 'The request failed.'}</p>
      {onRetry && (
        <button className="btn btn-secondary" onClick={onRetry}>
          <Icon name="refresh" size={18} /> Try again
        </button>
      )}
    </div>
  )
}

export function Stars({ value, count, size = 14 }) {
  if (!value) return <span className="stars stars-empty">Not rated</span>
  const rounded = Math.round(value)
  return (
    <span className="stars" aria-label={`Rated ${value.toFixed(1)} out of 5`}>
      {[1, 2, 3, 4, 5].map((n) => (
        <Icon key={n} name="star" size={size} filled={n <= rounded} strokeWidth={1.5} />
      ))}
      {count > 0 && <span className="stars-count">{value.toFixed(1)}</span>}
    </span>
  )
}

// Cover art: the recipe's photo when it has one, otherwise a warm
// generated tile so a photo-less library still looks intentional.
export function RecipeCover({ recipe, className = '', eager = false }) {
  const hue = hueFor(recipe.title || '')
  return (
    <div className={`cover ${className}`} style={{ '--cover-hue': hue }}>
      {recipe.image_url ? (
        <img
          src={recipe.image_url}
          alt=""
          loading={eager ? 'eager' : 'lazy'}
          decoding="async"
          referrerPolicy="no-referrer"
          onError={(e) => {
            e.currentTarget.style.display = 'none'
          }}
        />
      ) : null}
      <span className="cover-initials" aria-hidden="true">
        {initialsFor(recipe.title || '')}
      </span>
    </div>
  )
}

export function Skeleton({ className = '', style }) {
  return <div className={`skeleton ${className}`} style={style} aria-hidden="true" />
}

export function Chip({ icon, children, tone }) {
  return (
    <span className={`chip ${tone ? `chip-${tone}` : ''}`}>
      {icon && <Icon name={icon} size={15} />}
      {children}
    </span>
  )
}

export function Stepper({ value, onChange, min = 1, max = 99, label }) {
  return (
    <div className="stepper" role="group" aria-label={label}>
      <button
        type="button"
        className="stepper-btn"
        aria-label={`Decrease ${label}`}
        onClick={() => onChange(Math.max(min, value - 1))}
        disabled={value <= min}
      >
        <Icon name="minus" size={16} />
      </button>
      <span className="stepper-value" aria-live="polite">
        {value}
      </span>
      <button
        type="button"
        className="stepper-btn"
        aria-label={`Increase ${label}`}
        onClick={() => onChange(Math.min(max, value + 1))}
        disabled={value >= max}
      >
        <Icon name="plus" size={16} />
      </button>
    </div>
  )
}
