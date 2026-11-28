import { useEffect, useState } from 'react'
import { NavLink, useLocation } from 'react-router-dom'
import { Icon } from './Icon'
import { api } from '../lib/api'
import { useResource } from '../lib/resource'
import { personInitials } from '../lib/format'

const NAV = [
  { to: '/', label: 'Recipes', icon: 'book', end: true },
  { to: '/pantry', label: 'Pantry', icon: 'basket' },
  { to: '/plan', label: 'Meal plan', icon: 'calendar' },
]

function UserBadge() {
  const { data: user } = useResource('me', api.getCurrentUser)
  const [pictureFailed, setPictureFailed] = useState(false)
  if (!user)
    return (
      <NavLink to="/settings" className="user-badge" aria-label="Settings">
        <span className="user-badge-placeholder" aria-hidden="true" />
      </NavLink>
    )
  const label = user.name || user.username || user.email || user.user_id
  const tooltip = [user.name, user.username && `@${user.username}`, user.email].filter(Boolean).join(' · ') || user.user_id
  return (
    <NavLink to="/settings" className="user-badge" title={`${tooltip} · Settings`} aria-label="Settings">
      {user.picture && !pictureFailed ? (
        <img className="avatar" src={user.picture} alt="" referrerPolicy="no-referrer" onError={() => setPictureFailed(true)} />
      ) : (
        <span className="avatar avatar-fallback" aria-hidden="true">
          {personInitials(label)}
        </span>
      )}
      <span className="user-badge-text">
        <span className="user-badge-name">{label}</span>
        {user.name && user.email && <span className="user-badge-email">{user.email}</span>}
      </span>
      <Icon name="settings" size={18} className="user-badge-gear" />
    </NavLink>
  )
}

export function AppShell({ children }) {
  const location = useLocation()

  // Move focus to the main region on navigation so keyboard and
  // screen-reader users land on the new page, and reset scroll.
  useEffect(() => {
    window.scrollTo(0, 0)
    const main = document.getElementById('main')
    if (main) main.focus({ preventScroll: true })
  }, [location.pathname])

  return (
    <div className="shell">
      <a className="skip-link" href="#main">
        Skip to content
      </a>
      <header className="topbar">
        <div className="topbar-inner">
          <NavLink to="/" className="brand" aria-label="Recipes home">
            <span className="brand-mark" aria-hidden="true">
              <Icon name="utensils" size={18} strokeWidth={2} />
            </span>
            <span className="brand-name">Recipes</span>
          </NavLink>
          <nav className="topnav" aria-label="Main">
            {NAV.map((item) => (
              <NavLink key={item.to} to={item.to} end={item.end} className="topnav-link">
                <Icon name={item.icon} size={18} />
                {item.label}
              </NavLink>
            ))}
          </nav>
          <UserBadge />
        </div>
      </header>
      <main id="main" className="main" tabIndex={-1}>
        {children}
      </main>
      <nav className="tabbar" aria-label="Main">
        {NAV.map((item) => (
          <NavLink key={item.to} to={item.to} end={item.end} className="tabbar-link">
            <Icon name={item.icon} size={22} />
            <span>{item.label}</span>
          </NavLink>
        ))}
      </nav>
    </div>
  )
}
