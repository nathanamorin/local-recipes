import { Link } from 'react-router-dom'

export function NotFound() {
  return (
    <div className="empty-state">
      <h2>Not found</h2>
      <p><Link to="/">Back to recipes</Link></p>
    </div>
  )
}
