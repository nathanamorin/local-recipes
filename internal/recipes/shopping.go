package recipes

import (
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

// ShoppingReason explains why an item is on the shopping list.
type ShoppingReason struct {
	// Kind is "planned" (a scheduled meal needs it) or "restock" (on-hand
	// stock would end up at or below the low-stock threshold, or it's out).
	Kind        string  `json:"kind"`
	RecipeID    string  `json:"recipe_id,omitempty"`
	RecipeTitle string  `json:"recipe_title,omitempty"`
	Date        string  `json:"date,omitempty"`
	Quantity    float64 `json:"quantity,omitempty"`
	Unit        string  `json:"unit,omitempty"`
}

// ShoppingItem is one ingredient to buy (or, when Tracked is false, to
// check for - it isn't tracked in the pantry so its stock is unknown).
type ShoppingItem struct {
	IngredientID string `json:"ingredient_id"`
	Name         string `json:"name"`
	Category     string `json:"category"`
	// Quantity to buy in Unit. Zero means the amount is unknown (e.g. an
	// out-of-stock item with no low-stock threshold to restock to).
	Quantity float64 `json:"quantity"`
	Unit     string  `json:"unit"`
	Tracked  bool    `json:"tracked"`
	OnHand   float64 `json:"on_hand"`
	// Planned is how much the planned meals need, in Unit.
	Planned   float64          `json:"planned"`
	Threshold float64          `json:"threshold"`
	Reasons   []ShoppingReason `json:"reasons"`
	// Unconverted lists planned needs whose unit can't be compared to the
	// pantry's (e.g. cups vs grams), so they're shown but not counted.
	Unconverted []ShoppingReason `json:"unconverted,omitempty"`
}

// ShoppingList is what to buy to cover the planned meals in [From, To]
// and bring low stock back above its threshold.
type ShoppingList struct {
	From         string         `json:"from"`
	To           string         `json:"to"`
	PlannedMeals int            `json:"planned_meals"`
	Items        []ShoppingItem `json:"items"`
}

const epsilon = 1e-6

// BuildShoppingList computes userID's shopping list for meals cooked
// (not leftover nights) between from and to inclusive.
func BuildShoppingList(db *gorm.DB, userID string, from, to time.Time) (*ShoppingList, error) {
	ingredients, err := ListIngredients(db, userID)
	if err != nil {
		return nil, err
	}

	var meals []ScheduledMeal
	err = db.Where("user_id = ? AND date >= ? AND date <= ? AND is_leftover = ?", userID, from, to, false).
		Preload("Recipe").
		Preload("Recipe.Ingredients").
		Order("date asc").
		Find(&meals).Error
	if err != nil {
		return nil, err
	}

	items := map[string]*ShoppingItem{}
	byID := map[string]*Ingredient{}
	for i := range ingredients {
		ing := &ingredients[i]
		byID[ing.ID] = ing
		item := &ShoppingItem{IngredientID: ing.ID, Name: ing.Name, Category: ing.Category, Unit: ing.DefaultUnit, Reasons: []ShoppingReason{}}
		if ing.Stock != nil {
			item.Tracked = true
			item.OnHand = ing.Stock.QuantityOnHand
			item.Threshold = ing.Stock.LowStockThreshold
			if ing.Stock.Unit != "" {
				item.Unit = ing.Stock.Unit
			}
		}
		items[ing.ID] = item
	}

	planned := 0
	for _, meal := range meals {
		if meal.Recipe == nil {
			continue
		}
		planned++
		for _, ri := range meal.Recipe.Ingredients {
			item, ok := items[ri.IngredientID]
			if !ok || ri.Optional {
				continue
			}
			reason := ShoppingReason{
				Kind: "planned", RecipeID: meal.RecipeID, RecipeTitle: meal.Recipe.Title,
				Date: meal.Date.Format("2006-01-02"), Quantity: ri.Quantity, Unit: ri.Unit,
			}
			// An untracked item with no unit yet adopts the first recipe unit.
			if !item.Tracked && item.Planned == 0 && len(item.Reasons) == 0 && ri.Unit != "" {
				if _, convertible := convertQuantity(1, item.Unit, ri.Unit); !convertible {
					item.Unit = ri.Unit
				}
			}
			if ri.Quantity <= 0 {
				item.Reasons = append(item.Reasons, reason) // "to taste" - listed, not counted
				continue
			}
			if q, ok := convertQuantity(ri.Quantity, ri.Unit, item.Unit); ok {
				item.Planned += q
				item.Reasons = append(item.Reasons, reason)
			} else {
				item.Unconverted = append(item.Unconverted, reason)
			}
		}
	}

	list := &ShoppingList{From: from.Format("2006-01-02"), To: to.Format("2006-01-02"), PlannedMeals: planned, Items: []ShoppingItem{}}
	for _, item := range items {
		needed := len(item.Reasons) > 0 || len(item.Unconverted) > 0
		if item.Tracked {
			short := item.Planned + item.Threshold - item.OnHand
			switch {
			case short > epsilon:
				item.Quantity = short
				if item.Threshold > 0 {
					item.Reasons = append(item.Reasons, ShoppingReason{Kind: "restock", Quantity: item.Threshold, Unit: item.Unit})
				}
			case item.OnHand <= epsilon:
				// Out of stock with no amount to aim for: list it, amount unknown.
				item.Reasons = append(item.Reasons, ShoppingReason{Kind: "restock"})
			case len(item.Unconverted) > 0:
				// Can't tell whether stock covers it; keep it so the user decides.
			default:
				continue // enough on hand
			}
		} else {
			if !needed {
				continue
			}
			item.Quantity = item.Planned
		}
		list.Items = append(list.Items, *item)
	}
	sort.Slice(list.Items, func(i, j int) bool {
		a, b := list.Items[i], list.Items[j]
		if a.Tracked != b.Tracked {
			return a.Tracked
		}
		ca, cb := strings.ToLower(a.Category), strings.ToLower(b.Category)
		if ca != cb {
			return ca < cb
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
	return list, nil
}
