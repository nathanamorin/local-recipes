import { useEffect, useState } from 'react'
import { api } from '../lib/api'

function StockRow({ ingredient, onChanged }) {
  const stock = ingredient.stock
  const low = stock && stock.low_stock_threshold > 0 && stock.quantity_on_hand <= stock.low_stock_threshold
  const [delta, setDelta] = useState('')

  const applyDelta = async (sign) => {
    const amount = Number(delta)
    if (!amount) return
    await api.adjustStock(ingredient.id, sign * amount, stock ? stock.unit : ingredient.default_unit)
    setDelta('')
    onChanged()
  }

  const setThreshold = async (value) => {
    await api.setLowStockThreshold(ingredient.id, Number(value) || 0)
    onChanged()
  }

  const remove = async () => {
    if (!window.confirm(`Delete "${ingredient.name}"?`)) return
    await api.deleteIngredient(ingredient.id)
    onChanged()
  }

  return (
    <tr className={low ? 'low-stock-row' : ''}>
      <td>{ingredient.name}</td>
      <td>{ingredient.category}</td>
      <td>{stock ? `${stock.quantity_on_hand} ${stock.unit || ''}` : '—'}</td>
      <td>
        <input
          type="number"
          className="threshold-input"
          defaultValue={stock ? stock.low_stock_threshold : 0}
          onBlur={(e) => setThreshold(e.target.value)}
        />
      </td>
      <td className="stock-actions">
        <input
          type="number"
          step="any"
          className="delta-input"
          placeholder="amount"
          value={delta}
          onChange={(e) => setDelta(e.target.value)}
        />
        <button className="button" onClick={() => applyDelta(1)}>+ add</button>
        <button className="button" onClick={() => applyDelta(-1)}>- use</button>
        <button className="button button-danger" onClick={remove}>Delete</button>
      </td>
    </tr>
  )
}

export function Ingredients() {
  const [ingredients, setIngredients] = useState(null)
  const [error, setError] = useState(null)
  const [showLowStockOnly, setShowLowStockOnly] = useState(false)
  const [newName, setNewName] = useState('')
  const [newCategory, setNewCategory] = useState('')
  const [newUnit, setNewUnit] = useState('')

  const load = () => {
    const fetcher = showLowStockOnly ? api.listLowStockIngredients : api.listIngredients
    fetcher().then((d) => setIngredients(d.ingredients)).catch((e) => setError(e.message))
  }

  useEffect(load, [showLowStockOnly]) // eslint-disable-line react-hooks/exhaustive-deps

  const addIngredient = async (e) => {
    e.preventDefault()
    if (!newName.trim()) return
    await api.addIngredient({ name: newName.trim(), category: newCategory.trim(), default_unit: newUnit.trim() })
    setNewName(''); setNewCategory(''); setNewUnit('')
    load()
  }

  return (
    <div className="page">
      <div className="page-header">
        <h1>Ingredients</h1>
        <label className="toggle">
          <input type="checkbox" checked={showLowStockOnly} onChange={(e) => setShowLowStockOnly(e.target.checked)} />
          Low stock only
        </label>
      </div>
      {error && <p className="error">{error}</p>}

      <form className="inline-form" onSubmit={addIngredient}>
        <input placeholder="name" value={newName} onChange={(e) => setNewName(e.target.value)} required />
        <input placeholder="category" value={newCategory} onChange={(e) => setNewCategory(e.target.value)} />
        <input placeholder="default unit" value={newUnit} onChange={(e) => setNewUnit(e.target.value)} />
        <button className="button button-primary" type="submit">+ Add ingredient</button>
      </form>

      {ingredients === null && !error && <p>Loading…</p>}
      {ingredients && ingredients.length === 0 && <p className="empty-state">No ingredients yet.</p>}
      {ingredients && ingredients.length > 0 && (
        <table className="ingredient-table">
          <thead>
            <tr><th>Name</th><th>Category</th><th>On hand</th><th>Low-stock at</th><th>Adjust</th></tr>
          </thead>
          <tbody>
            {ingredients.map((ing) => <StockRow key={ing.id} ingredient={ing} onChanged={load} />)}
          </tbody>
        </table>
      )}
    </div>
  )
}
