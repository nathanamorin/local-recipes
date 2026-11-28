import { Component, Suspense, lazy } from 'react'
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { AppShell } from './components/AppShell'
import { ToastProvider } from './components/Toast'
import { ErrorState } from './components/ui'
import { Library } from './pages/Library'

// The library is the landing page, so it ships in the main bundle; every
// other page (and react-markdown with it) loads on demand.
const RecipeDetail = lazy(() => import('./pages/RecipeDetail'))
const RecipeEditor = lazy(() => import('./pages/RecipeEditor'))
const Pantry = lazy(() => import('./pages/Pantry'))
const Planner = lazy(() => import('./pages/Planner'))
const Settings = lazy(() => import('./pages/Settings'))
const Shopping = lazy(() => import('./pages/Shopping'))
const NotFound = lazy(() => import('./pages/NotFound'))

class ErrorBoundary extends Component {
  state = { error: null }
  static getDerivedStateFromError(error) {
    return { error }
  }
  render() {
    if (this.state.error) {
      return <ErrorState error={this.state.error} onRetry={() => window.location.reload()} />
    }
    return this.props.children
  }
}

function PageFallback() {
  return <div className="page-loading" aria-busy="true" aria-label="Loading" />
}

export default function App() {
  return (
    <BrowserRouter>
      <ToastProvider>
        <AppShell>
          <ErrorBoundary>
            <Suspense fallback={<PageFallback />}>
              <Routes>
                <Route path="/" element={<Library />} />
                <Route path="/recipes/new" element={<RecipeEditor mode="create" />} />
                <Route path="/recipes/:recipeId" element={<RecipeDetail />} />
                <Route path="/recipes/:recipeId/edit" element={<RecipeEditor mode="edit" />} />
                <Route path="/pantry" element={<Pantry />} />
                <Route path="/plan" element={<Planner />} />
                <Route path="/shopping" element={<Shopping />} />
                <Route path="/settings" element={<Settings />} />
                {/* Old paths, kept so existing bookmarks still work. */}
                <Route path="/ingredients" element={<Navigate to="/pantry" replace />} />
                <Route path="/schedule" element={<Navigate to="/plan" replace />} />
                <Route path="*" element={<NotFound />} />
              </Routes>
            </Suspense>
          </ErrorBoundary>
        </AppShell>
      </ToastProvider>
    </BrowserRouter>
  )
}
