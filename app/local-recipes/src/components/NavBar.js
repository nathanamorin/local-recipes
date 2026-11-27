import { NavLink } from 'react-router-dom'
import { useEffect, useState } from 'react'
import { api } from '../lib/api'

export function NavBar() {
  const [userID, setUserID] = useState(null)

  useEffect(() => {
    api.getCurrentUser().then((d) => setUserID(d.user_id)).catch(() => setUserID(null))
  }, [])

  return (
    <header className="navbar">
      <div className="navbar-brand">local-recipes</div>
      <nav className="navbar-links">
        <NavLink to="/" end>Recipes</NavLink>
        <NavLink to="/ingredients">Ingredients</NavLink>
        <NavLink to="/schedule">Schedule</NavLink>
      </nav>
      {userID && <div className="navbar-user">{userID}</div>}
    </header>
  )
}
