import { NavLink } from 'react-router-dom'
import { useEffect, useState } from 'react'
import { api } from '../lib/api'

function initials(label) {
  const parts = label.replace(/@.*/, '').split(/[\s._-]+/).filter(Boolean)
  return (parts.length > 1 ? parts[0][0] + parts[1][0] : label.slice(0, 2)).toUpperCase()
}

function UserBadge({ user }) {
  const [pictureFailed, setPictureFailed] = useState(false)
  const label = user.name || user.username || user.email || user.user_id
  const secondary = user.name && user.email ? user.email : null
  const tooltip = [user.name, user.username && `@${user.username}`, user.email].filter(Boolean).join(' · ') || user.user_id

  return (
    <div className="navbar-user" title={tooltip}>
      {user.picture && !pictureFailed ? (
        <img className="navbar-avatar" src={user.picture} alt="" referrerPolicy="no-referrer" onError={() => setPictureFailed(true)} />
      ) : (
        <span className="navbar-avatar navbar-avatar-fallback" aria-hidden="true">{initials(label)}</span>
      )}
      <span className="navbar-user-text">
        <span className="navbar-user-name">{label}</span>
        {secondary && <span className="navbar-user-email">{secondary}</span>}
      </span>
    </div>
  )
}

export function NavBar() {
  const [user, setUser] = useState(null)

  useEffect(() => {
    api.getCurrentUser().then(setUser).catch(() => setUser(null))
  }, [])

  return (
    <header className="navbar">
      <div className="navbar-brand">local-recipes</div>
      <nav className="navbar-links">
        <NavLink to="/" end>Recipes</NavLink>
        <NavLink to="/ingredients">Ingredients</NavLink>
        <NavLink to="/schedule">Schedule</NavLink>
      </nav>
      {user && <UserBadge user={user} />}
    </header>
  )
}
