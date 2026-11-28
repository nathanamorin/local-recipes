package recipes

import (
	"fmt"
	"math"

	"gorm.io/gorm"
)

// CookChange is one pantry deduction made (or, for a dry run, that would
// be made) by cooking a recipe. Quantity is what was actually removed, in
// the stock's unit - stock never goes below zero, so it can be less than
// the recipe calls for. Re-adding Quantity undoes the change.
type CookChange struct {
	IngredientID string  `json:"ingredient_id"`
	Name         string  `json:"name"`
	Quantity     float64 `json:"quantity"`
	Unit         string  `json:"unit"`
	Before       float64 `json:"before"`
	After        float64 `json:"after"`
	// Short is true when the pantry had less than the recipe needs.
	Short bool `json:"short,omitempty"`
}

// CookSkip is a recipe ingredient that didn't change the pantry, and why.
type CookSkip struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

type CookResult struct {
	Servings int          `json:"servings"`
	Changes  []CookChange `json:"changes"`
	Skipped  []CookSkip   `json:"skipped"`
}

// CookRecipe deducts one batch of recipeID's ingredients - scaled from the
// recipe's servings to `servings` (0 = as written) - from userID's
// tracked pantry stock. With dryRun it only reports what would change.
// Optional and untracked ingredients, and ones whose units can't be
// converted to the stock's unit, are skipped and reported.
func CookRecipe(db *gorm.DB, userID, recipeID string, servings int, dryRun bool) (*CookResult, error) {
	recipe, err := GetRecipe(db, userID, recipeID)
	if err != nil {
		return nil, err
	}
	factor := 1.0
	if servings > 0 && recipe.Servings > 0 {
		factor = float64(servings) / float64(recipe.Servings)
	} else {
		servings = recipe.Servings
	}

	stocks := map[string]IngredientStock{}
	var rows []IngredientStock
	if err := db.Where("user_id = ?", userID).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, s := range rows {
		stocks[s.IngredientID] = s
	}

	result := &CookResult{Servings: servings, Changes: []CookChange{}, Skipped: []CookSkip{}}
	// Several recipe lines can share an ingredient; deduct cumulatively.
	remaining := map[string]float64{}
	for _, ri := range recipe.Ingredients {
		name := "ingredient"
		if ri.Ingredient != nil {
			name = ri.Ingredient.Name
		}
		stock, tracked := stocks[ri.IngredientID]
		switch {
		case ri.Optional:
			result.Skipped = append(result.Skipped, CookSkip{name, "optional"})
			continue
		case !tracked:
			result.Skipped = append(result.Skipped, CookSkip{name, "not tracked in your pantry"})
			continue
		case ri.Quantity <= 0:
			result.Skipped = append(result.Skipped, CookSkip{name, "no quantity in the recipe"})
			continue
		}
		need, ok := convertQuantity(ri.Quantity*factor, ri.Unit, stock.Unit)
		if !ok {
			result.Skipped = append(result.Skipped, CookSkip{name, fmt.Sprintf("can't convert %s to %s", unitLabel(ri.Unit), unitLabel(stock.Unit))})
			continue
		}
		before, seen := remaining[ri.IngredientID]
		if !seen {
			before = stock.QuantityOnHand
		}
		after := math.Max(0, before-need)
		remaining[ri.IngredientID] = after
		result.Changes = append(result.Changes, CookChange{
			IngredientID: ri.IngredientID, Name: name, Quantity: before - after, Unit: stock.Unit,
			Before: before, After: after, Short: need > before+epsilon,
		})
	}

	if dryRun {
		return result, nil
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		for _, c := range result.Changes {
			if c.Quantity <= 0 {
				continue
			}
			// Pass the stock's own unit so AdjustStock doesn't relabel it.
			if _, err := AdjustStock(tx, userID, c.IngredientID, -c.Quantity, c.Unit); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func unitLabel(u string) string {
	if u == "" || u == "count" {
		return "a count"
	}
	return u
}
