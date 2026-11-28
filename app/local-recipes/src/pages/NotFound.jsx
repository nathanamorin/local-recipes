import { Link } from 'react-router-dom'
import { EmptyState } from '../components/ui'

export default function NotFound() {
  return (
    <EmptyState
      icon="search"
      title="Page not found"
      action={
        <Link className="btn btn-primary" to="/">
          Back to recipes
        </Link>
      }
    >
      That page doesn't exist, or it may have moved.
    </EmptyState>
  )
}
