package recipes

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func normalizeIngredientName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// FindOrCreateIngredient returns the user's existing ingredient matching
// name (case/whitespace-insensitively), creating it if none exists yet.
// Used both when tagging a recipe's ingredients and when logging stock
// from free-text MCP input, so "Flour" and "flour" always resolve to the
// same ingredient/stock row for a given user.
func FindOrCreateIngredient(db *gorm.DB, userID, name, category, defaultUnit string) (*Ingredient, error) {
	normalized := normalizeIngredientName(name)
	if normalized == "" {
		return nil, gorm.ErrInvalidData
	}

	var existing Ingredient
	err := db.Where("user_id = ? AND normalized_name = ?", userID, normalized).First(&existing).Error
	if err == nil {
		return &existing, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}

	ing := &Ingredient{
		ID:             uuid.NewString(),
		UserID:         userID,
		Name:           strings.TrimSpace(name),
		NormalizedName: normalized,
		Category:       category,
		DefaultUnit:    defaultUnit,
	}
	if err := db.Create(ing).Error; err != nil {
		return nil, err
	}
	return ing, nil
}

// ListIngredients returns every ingredient the user has, each with its
// current stock level (if any) preloaded, ordered alphabetically.
func ListIngredients(db *gorm.DB, userID string) ([]Ingredient, error) {
	var ingredients []Ingredient
	err := db.Where("user_id = ?", userID).
		Preload("Stock").
		Order("name asc").
		Find(&ingredients).Error
	return ingredients, err
}

// ListLowStock returns ingredients whose on-hand quantity is at or below
// their configured low-stock threshold (thresholds of 0 are treated as
// "not tracked for low-stock" and excluded).
func ListLowStock(db *gorm.DB, userID string) ([]Ingredient, error) {
	var ingredients []Ingredient
	err := db.Where("ingredients.user_id = ?", userID).
		Joins("JOIN ingredient_stocks ON ingredient_stocks.ingredient_id = ingredients.id AND ingredient_stocks.user_id = ingredients.user_id").
		Where("ingredient_stocks.low_stock_threshold > 0 AND ingredient_stocks.quantity_on_hand <= ingredient_stocks.low_stock_threshold").
		Preload("Stock").
		Order("name asc").
		Find(&ingredients).Error
	return ingredients, err
}

// DeleteIngredient removes an ingredient and its stock row. Existing
// RecipeIngredient tags referencing it are left in place (recipes keep
// their historical ingredient list even if the catalog entry is removed).
func DeleteIngredient(db *gorm.DB, userID, ingredientID string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ? AND ingredient_id = ?", userID, ingredientID).
			Delete(&IngredientStock{}).Error; err != nil {
			return err
		}
		return tx.Where("user_id = ? AND id = ?", userID, ingredientID).
			Delete(&Ingredient{}).Error
	})
}

// SetStock sets an ingredient's absolute on-hand quantity (as opposed to
// AdjustStock's relative delta), e.g. after a shelf/pantry recount.
func SetStock(db *gorm.DB, userID, ingredientID string, quantity float64, unit string) (*IngredientStock, error) {
	return upsertStock(db, userID, ingredientID, quantity, unit, false)
}

// AdjustStock changes an ingredient's on-hand quantity by delta (negative
// to consume/use up, positive to add from a grocery run), clamping the
// result at zero. This is the primary way the MCP "log ingredient" tool
// and the web UI both record stock changes, since most real-world updates
// ("used 2 cups", "bought another dozen") are naturally relative rather
// than absolute.
func AdjustStock(db *gorm.DB, userID, ingredientID string, delta float64, unit string) (*IngredientStock, error) {
	return upsertStock(db, userID, ingredientID, delta, unit, true)
}

func upsertStock(db *gorm.DB, userID, ingredientID string, amount float64, unit string, relative bool) (*IngredientStock, error) {
	var stock IngredientStock
	err := db.Where("user_id = ? AND ingredient_id = ?", userID, ingredientID).First(&stock).Error
	switch err {
	case nil:
		if relative {
			stock.QuantityOnHand += amount
		} else {
			stock.QuantityOnHand = amount
		}
		if stock.QuantityOnHand < 0 {
			stock.QuantityOnHand = 0
		}
		if unit != "" {
			stock.Unit = unit
		}
		stock.UpdatedAt = time.Now()
		if err := db.Save(&stock).Error; err != nil {
			return nil, err
		}
		return &stock, nil
	case gorm.ErrRecordNotFound:
		qty := amount
		if relative && qty < 0 {
			qty = 0
		}
		stock = IngredientStock{
			ID:             uuid.NewString(),
			UserID:         userID,
			IngredientID:   ingredientID,
			QuantityOnHand: qty,
			Unit:           unit,
			UpdatedAt:      time.Now(),
		}
		if err := db.Create(&stock).Error; err != nil {
			return nil, err
		}
		return &stock, nil
	default:
		return nil, err
	}
}

// SetLowStockThreshold sets the quantity at or below which an ingredient
// is surfaced by ListLowStock. Creates the stock row (at zero on-hand) if
// the ingredient has no stock history yet.
func SetLowStockThreshold(db *gorm.DB, userID, ingredientID string, threshold float64) error {
	var stock IngredientStock
	err := db.Where("user_id = ? AND ingredient_id = ?", userID, ingredientID).First(&stock).Error
	if err == gorm.ErrRecordNotFound {
		stock = IngredientStock{
			ID:           uuid.NewString(),
			UserID:       userID,
			IngredientID: ingredientID,
		}
	} else if err != nil {
		return err
	}
	stock.LowStockThreshold = threshold
	stock.UpdatedAt = time.Now()
	return db.Save(&stock).Error
}
