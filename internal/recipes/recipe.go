package recipes

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var slugDisallowed = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(title string) string {
	s := slugDisallowed.ReplaceAllString(strings.ToLower(strings.TrimSpace(title)), "-")
	return strings.Trim(s, "-")
}

// IngredientInput describes one ingredient line when creating/importing a
// recipe: the free-text ingredient name (resolved/created via
// FindOrCreateIngredient) plus the quantity/unit/notes needed for that
// recipe specifically.
type IngredientInput struct {
	Name     string  `json:"name"`
	Quantity float64 `json:"quantity"`
	Unit     string  `json:"unit"`
	Notes    string  `json:"notes"`
	Optional bool    `json:"optional"`
	Category string  `json:"category,omitempty"`
}

// RecipeInput is the common shape for creating or importing a recipe,
// shared by the HTTP handlers (handlers/recipes.go) and the MCP
// import_recipe tool (cmd/mcpserver), so both front doors produce
// identically-structured recipes.
type RecipeInput struct {
	Title        string
	Description  string
	BodyMarkdown string
	Servings     int
	PrepMinutes  int
	CookMinutes  int
	SourceURL    string
	ImageURL     string
	Ingredients  []IngredientInput
}

// CreateRecipe creates a new recipe for userID from in, resolving (and
// creating, if new) each tagged ingredient via FindOrCreateIngredient.
// This is the single code path both manual recipe creation and MCP-driven
// auto-import go through, so tagging/ingredient-dedup behavior can't
// drift between the two.
func CreateRecipe(db *gorm.DB, userID string, in RecipeInput) (*Recipe, error) {
	if strings.TrimSpace(in.Title) == "" {
		return nil, fmt.Errorf("title is required")
	}

	recipe := &Recipe{
		ID:           uuid.NewString(),
		UserID:       userID,
		Title:        in.Title,
		Slug:         slugify(in.Title),
		Description:  in.Description,
		BodyMarkdown: in.BodyMarkdown,
		Servings:     in.Servings,
		PrepMinutes:  in.PrepMinutes,
		CookMinutes:  in.CookMinutes,
		SourceURL:    in.SourceURL,
		ImageURL:     in.ImageURL,
	}

	err := db.Transaction(func(tx *gorm.DB) error {
		// Disambiguate the slug if this user already has a recipe with
		// the same title, rather than failing the whole import outright.
		base := recipe.Slug
		for i := 2; ; i++ {
			var count int64
			if err := tx.Model(&Recipe{}).Where("user_id = ? AND slug = ?", userID, recipe.Slug).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				break
			}
			recipe.Slug = fmt.Sprintf("%s-%d", base, i)
		}

		if err := tx.Create(recipe).Error; err != nil {
			return err
		}

		for _, ii := range in.Ingredients {
			if strings.TrimSpace(ii.Name) == "" {
				continue
			}
			ing, err := FindOrCreateIngredient(tx, userID, ii.Name, ii.Category, ii.Unit)
			if err != nil {
				return fmt.Errorf("resolve ingredient %q: %w", ii.Name, err)
			}
			ri := RecipeIngredient{
				ID:           uuid.NewString(),
				RecipeID:     recipe.ID,
				IngredientID: ing.ID,
				Quantity:     ii.Quantity,
				Unit:         ii.Unit,
				Notes:        ii.Notes,
				Optional:     ii.Optional,
			}
			if err := tx.Create(&ri).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return GetRecipe(db, userID, recipe.ID)
}

// GetRecipe fetches a single recipe (with ingredients and rating summary)
// scoped to userID.
func GetRecipe(db *gorm.DB, userID, recipeID string) (*Recipe, error) {
	var recipe Recipe
	err := db.Where("user_id = ? AND id = ?", userID, recipeID).
		Preload("Ingredients").
		Preload("Ingredients.Ingredient").
		First(&recipe).Error
	if err != nil {
		return nil, err
	}
	if err := loadRatingSummary(db, userID, []*Recipe{&recipe}); err != nil {
		return nil, err
	}
	return &recipe, nil
}

// ListRecipes returns every recipe for userID (without the full
// ingredient list, for a lighter-weight library view), most recently
// updated first, each annotated with its rating summary.
func ListRecipes(db *gorm.DB, userID string) ([]Recipe, error) {
	var recipeList []Recipe
	if err := db.Where("user_id = ?", userID).Order("updated_at desc").Find(&recipeList).Error; err != nil {
		return nil, err
	}
	ptrs := make([]*Recipe, len(recipeList))
	for i := range recipeList {
		ptrs[i] = &recipeList[i]
	}
	if err := loadRatingSummary(db, userID, ptrs); err != nil {
		return nil, err
	}
	return recipeList, nil
}

// loadRatingSummary populates AverageRating/RatingCount on each recipe in
// recipeList in a single query, rather than one query per recipe.
func loadRatingSummary(db *gorm.DB, userID string, recipeList []*Recipe) error {
	if len(recipeList) == 0 {
		return nil
	}
	ids := make([]string, len(recipeList))
	byID := make(map[string]*Recipe, len(recipeList))
	for i, r := range recipeList {
		ids[i] = r.ID
		byID[r.ID] = r
	}

	var rows []struct {
		RecipeID string
		Avg      float64
		Count    int
	}
	err := db.Model(&Rating{}).
		Select("recipe_id, avg(score) as avg, count(*) as count").
		Where("user_id = ? AND recipe_id IN ?", userID, ids).
		Group("recipe_id").
		Scan(&rows).Error
	if err != nil {
		return err
	}
	for _, row := range rows {
		if r, ok := byID[row.RecipeID]; ok {
			r.AverageRating = row.Avg
			r.RatingCount = row.Count
		}
	}
	return nil
}

// UpdateRecipe applies a full RecipeInput over an existing recipe,
// replacing its ingredient tags entirely with in.Ingredients.
func UpdateRecipe(db *gorm.DB, userID, recipeID string, in RecipeInput) (*Recipe, error) {
	err := db.Transaction(func(tx *gorm.DB) error {
		var recipe Recipe
		if err := tx.Where("user_id = ? AND id = ?", userID, recipeID).First(&recipe).Error; err != nil {
			return err
		}

		recipe.Title = in.Title
		recipe.Description = in.Description
		recipe.BodyMarkdown = in.BodyMarkdown
		recipe.Servings = in.Servings
		recipe.PrepMinutes = in.PrepMinutes
		recipe.CookMinutes = in.CookMinutes
		recipe.SourceURL = in.SourceURL
		recipe.ImageURL = in.ImageURL
		if err := tx.Save(&recipe).Error; err != nil {
			return err
		}

		if err := tx.Where("recipe_id = ?", recipeID).Delete(&RecipeIngredient{}).Error; err != nil {
			return err
		}
		for _, ii := range in.Ingredients {
			if strings.TrimSpace(ii.Name) == "" {
				continue
			}
			ing, err := FindOrCreateIngredient(tx, userID, ii.Name, ii.Category, ii.Unit)
			if err != nil {
				return err
			}
			ri := RecipeIngredient{
				ID:           uuid.NewString(),
				RecipeID:     recipeID,
				IngredientID: ing.ID,
				Quantity:     ii.Quantity,
				Unit:         ii.Unit,
				Notes:        ii.Notes,
				Optional:     ii.Optional,
			}
			if err := tx.Create(&ri).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return GetRecipe(db, userID, recipeID)
}

// DeleteRecipe removes a recipe along with its ingredient tags and
// ratings.
func DeleteRecipe(db *gorm.DB, userID, recipeID string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("recipe_id = ?", recipeID).Delete(&RecipeIngredient{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ? AND recipe_id = ?", userID, recipeID).Delete(&Rating{}).Error; err != nil {
			return err
		}
		return tx.Where("user_id = ? AND id = ?", userID, recipeID).Delete(&Recipe{}).Error
	})
}
