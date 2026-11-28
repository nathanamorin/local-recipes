import { useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../lib/api'
import { useResource } from '../lib/resource'
import { deleteRecipeForever, restoreRecipe } from '../lib/recipeActions'
import { personInitials, pluralize, relativeDate } from '../lib/format'
import { Icon } from '../components/Icon'
import { ConfirmDialog } from '../components/Dialog'
import { useToast } from '../components/Toast'
import { EmptyState, ErrorState, PageHeader, RecipeCover, Skeleton } from '../components/ui'

function AccountCard() {
  const { data: user } = useResource('me', api.getCurrentUser)
  const [pictureFailed, setPictureFailed] = useState(false)
  if (!user) return <Skeleton style={{ height: 96 }} />
  const label = user.name || user.username || user.email || user.user_id
  return (
    <section className="card account-card" aria-labelledby="account-heading">
      <h2 id="account-heading" className="visually-hidden">
        Account
      </h2>
      {user.picture && !pictureFailed ? (
        <img className="avatar avatar-lg" src={user.picture} alt="" referrerPolicy="no-referrer" onError={() => setPictureFailed(true)} />
      ) : (
        <span className="avatar avatar-lg avatar-fallback" aria-hidden="true">
          {personInitials(label)}
        </span>
      )}
      <div className="account-text">
        <span className="account-name">{label}</span>
        {user.email && user.email !== label && <span className="muted">{user.email}</span>}
        <span className="account-meta">Signed in with Pocket ID</span>
      </div>
    </section>
  )
}

function ArchivedRow({ recipe, onDelete }) {
  const toast = useToast()
  const [busy, setBusy] = useState(false)
  const restore = async () => {
    setBusy(true)
    try {
      await restoreRecipe(recipe.id)
      toast(`Restored "${recipe.title}" to your library`)
    } catch (e) {
      toast(e.message, { tone: 'error' })
      setBusy(false)
    }
  }
  return (
    <li className="archived-row">
      <Link to={`/recipes/${recipe.id}`} className="archived-link">
        <RecipeCover recipe={recipe} className="archived-cover" />
        <span className="archived-text">
          <span className="archived-title">{recipe.title}</span>
          <span className="muted archived-meta">Archived {relativeDate(recipe.archived_at)}</span>
        </span>
      </Link>
      <div className="archived-actions">
        <button className="btn btn-secondary btn-sm" onClick={restore} disabled={busy}>
          <Icon name="restore" size={16} /> <span className="hide-narrow">Restore</span>
        </button>
        <button
          className="icon-button icon-button-sm icon-button-danger"
          aria-label={`Delete "${recipe.title}" forever`}
          title="Delete forever"
          onClick={() => onDelete(recipe)}
          disabled={busy}
        >
          <Icon name="trash" size={16} />
        </button>
      </div>
    </li>
  )
}

function ArchivedRecipes() {
  const toast = useToast()
  const { data: archived, error, loading, reload } = useResource('recipes-archived', api.listArchivedRecipes)
  const [deleting, setDeleting] = useState(null)
  const [busy, setBusy] = useState(false)

  const confirmDelete = async () => {
    setBusy(true)
    try {
      await deleteRecipeForever(deleting.id)
      toast(`Deleted "${deleting.title}" permanently`)
      setDeleting(null)
    } catch (e) {
      toast(e.message, { tone: 'error' })
    } finally {
      setBusy(false)
    }
  }

  return (
    <section className="settings-section" aria-labelledby="archived-heading">
      <div className="settings-section-head">
        <h2 id="archived-heading">Archived recipes</h2>
        {archived && archived.length > 0 && <span className="muted">{pluralize(archived.length, 'recipe')}</span>}
      </div>
      <p className="muted settings-section-desc">
        Archived recipes are hidden from your library, meal plans and AI assistant, but keep their ingredients and ratings.
      </p>

      {loading && <Skeleton style={{ height: 160 }} />}
      {error && <ErrorState error={error} onRetry={reload} />}
      {archived && archived.length === 0 && (
        <EmptyState icon="archive" title="Nothing archived">
          Use Archive on a recipe to hide it without losing it.
        </EmptyState>
      )}
      {archived && archived.length > 0 && (
        <ul className="card archived-list">
          {archived.map((r) => (
            <ArchivedRow key={r.id} recipe={r} onDelete={setDeleting} />
          ))}
        </ul>
      )}

      <ConfirmDialog
        open={!!deleting}
        onClose={() => setDeleting(null)}
        onConfirm={confirmDelete}
        busy={busy}
        title="Delete forever?"
        confirmLabel="Delete forever"
        message={
          deleting
            ? `"${deleting.title}", its ratings and any meals planned from it will be permanently removed. This can't be undone.`
            : ''
        }
      />
    </section>
  )
}

export default function Settings() {
  return (
    <div className="page page-narrow settings">
      <PageHeader title="Settings" />
      <AccountCard />
      <ArchivedRecipes />
    </div>
  )
}
