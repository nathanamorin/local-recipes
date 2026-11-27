import { BrowserRouter as Router, Routes, Route } from 'react-router-dom'
import { NavBar } from './components/NavBar'
import { RecipeLibrary } from './pages/RecipeLibrary'
import { RecipeDetail } from './pages/RecipeDetail'
import { RecipeForm } from './pages/RecipeForm'
import { Ingredients } from './pages/Ingredients'
import { Schedule } from './pages/Schedule'
import { NotFound } from './pages/misc'

export default function App() {
  return (
    <Router>
      <div className="app">
        <NavBar />
        <main className="main">
          <Routes>
            <Route path="/" element={<RecipeLibrary />} />
            <Route path="/recipes/new" element={<RecipeForm mode="create" />} />
            <Route path="/recipes/:recipeId" element={<RecipeDetail />} />
            <Route path="/recipes/:recipeId/edit" element={<RecipeForm mode="edit" />} />
            <Route path="/ingredients" element={<Ingredients />} />
            <Route path="/schedule" element={<Schedule />} />
            <Route path="*" element={<NotFound />} />
          </Routes>
        </main>
      </div>
    </Router>
  )
}
