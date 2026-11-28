// Package recipes is the core domain package for local-recipes: recipe
// storage (Markdown-bodied, for rich formatting), ingredient tagging and
// stock tracking, ratings, and meal scheduling. It is used by both the
// public HTTP handlers and the MCP server, so recipe import/ingredient
// logging behaves identically whether it comes from the web UI or from
// an LLM driving the MCP tools.
//
// Every table is scoped by UserID: local-recipes keeps a single shared
// PostgreSQL database (unlike per-tenant-database designs), with each
// user's data partitioned by the UserID column instead - the same
// pattern local-podcasts uses for UserFavorite, keyed off the
// X-Auth-Request-User header Istio's ext_authz gating injects (see
// internal/identity).
package recipes

import (
	"time"

	"gorm.io/gorm"
)

// Recipe is a single recipe, scoped to the user who owns it. BodyMarkdown
// holds the recipe's rich-formatted instructions/notes (headings, lists,
// links, etc.) as Markdown rather than plain text or HTML - Markdown
// keeps imported recipes diffable/editable as plain text while still
// rendering richly in the frontend (see app/local-recipes's use of
// react-markdown), and is trivial for an LLM to produce when importing a
// recipe via the MCP server.
type Recipe struct {
	ID           string    `json:"id" gorm:"primaryKey"`
	UserID       string    `json:"-" gorm:"index:idx_recipes_user,priority:1"`
	Title        string    `json:"title" gorm:"index:idx_recipes_user,priority:2"`
	Slug         string    `json:"slug" gorm:"uniqueIndex:idx_recipes_user_slug,priority:2"`
	Description  string    `json:"description"`
	BodyMarkdown string    `json:"body_markdown" gorm:"type:text"`
	Servings     int       `json:"servings"`
	PrepMinutes  int       `json:"prep_minutes"`
	CookMinutes  int       `json:"cook_minutes"`
	SourceURL    string    `json:"source_url"`
	ImageURL     string    `json:"image_url"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	// ArchivedAt is set when the user archives (soft-removes) a recipe.
	// Archived recipes are hidden from the library, the meal planner's
	// pool and the MCP tools, but keep their ingredients and ratings so
	// they can be restored from Settings.
	ArchivedAt *time.Time `json:"archived_at,omitempty" gorm:"index"`

	Ingredients []RecipeIngredient `json:"ingredients,omitempty" gorm:"foreignKey:RecipeID;references:ID"`

	// AverageRating/RatingCount are computed by loadRatingSummary, never
	// persisted - GORM's "-" tag keeps them out of AutoMigrate/Save while
	// still letting them round-trip through JSON for API responses.
	AverageRating float64 `json:"average_rating,omitempty" gorm:"-"`
	RatingCount   int     `json:"rating_count,omitempty" gorm:"-"`
}

// Ingredient is a named ingredient a user's recipes can be tagged with,
// and whose on-hand stock level is tracked (see IngredientStock). Each
// user maintains their own ingredient catalog - "flour" for one user is
// a distinct row (and stock level) from "flour" for another.
type Ingredient struct {
	ID     string `json:"id" gorm:"primaryKey"`
	UserID string `json:"-" gorm:"uniqueIndex:idx_ingredients_user_name,priority:1"`
	Name   string `json:"name" gorm:"uniqueIndex:idx_ingredients_user_name,priority:2"`
	// NormalizedName is Name lowercased/trimmed, used to dedupe
	// case/whitespace variants (e.g. "Flour" vs "flour ") when tagging
	// recipes or logging stock from free-text MCP input, without forcing
	// exact-case matches on API callers.
	NormalizedName string `json:"-" gorm:"index"`
	Category       string `json:"category"`
	DefaultUnit    string `json:"default_unit"`

	Stock *IngredientStock `json:"stock,omitempty" gorm:"foreignKey:IngredientID;references:ID"`
}

// IngredientStock is the current quantity of an ingredient a user has on
// hand. Kept as its own table (rather than columns on Ingredient) so
// stock-level writes (frequent, from MCP "log ingredient" calls) are
// decoupled from the ingredient catalog itself (rare writes, mostly at
// recipe-import time).
type IngredientStock struct {
	ID                string    `json:"-" gorm:"primaryKey"`
	UserID            string    `json:"-" gorm:"uniqueIndex:idx_stock_user_ingredient,priority:1"`
	IngredientID      string    `json:"ingredient_id" gorm:"uniqueIndex:idx_stock_user_ingredient,priority:2"`
	QuantityOnHand    float64   `json:"quantity_on_hand"`
	Unit              string    `json:"unit"`
	LowStockThreshold float64   `json:"low_stock_threshold"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// RecipeIngredient tags a Recipe with a required (or optional) quantity
// of an Ingredient - the join that both drives the recipe's displayed
// ingredient list and lets ingredient stock be cross-referenced against
// what a recipe needs.
type RecipeIngredient struct {
	ID           string  `json:"id" gorm:"primaryKey"`
	RecipeID     string  `json:"-" gorm:"index:idx_recipe_ingredients_recipe,priority:1"`
	IngredientID string  `json:"ingredient_id" gorm:"index"`
	Quantity     float64 `json:"quantity"`
	Unit         string  `json:"unit"`
	Notes        string  `json:"notes"`
	Optional     bool    `json:"optional"`

	Ingredient *Ingredient `json:"ingredient,omitempty" gorm:"foreignKey:IngredientID;references:ID"`
}

// Rating is a single user's 1-5 rating (with optional free-text notes)
// of a recipe. There is at most one Rating per (UserID, RecipeID) pair -
// re-rating a recipe updates the existing row (see UpsertRating) rather
// than accumulating a rating history, since the feature this supports is
// "how good is this recipe", not a review feed.
type Rating struct {
	ID        string    `json:"id" gorm:"primaryKey"`
	UserID    string    `json:"-" gorm:"uniqueIndex:idx_ratings_user_recipe,priority:1"`
	RecipeID  string    `json:"recipe_id" gorm:"uniqueIndex:idx_ratings_user_recipe,priority:2"`
	Score     int       `json:"score"`
	Notes     string    `json:"notes"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// MealScheduleConfig holds one user's meal-planning preferences: how
// many meals per week they plan for, how many of those are freshly
// cooked (the rest being leftovers), and how many consecutive days a
// cooked meal's leftovers are expected to last. See internal/recipes
// schedule.go (GenerateWeek) for how these drive schedule generation.
type MealScheduleConfig struct {
	UserID       string    `json:"-" gorm:"primaryKey"`
	MealsPerWeek int       `json:"meals_per_week"`
	MealsToCook  int       `json:"meals_to_cook"`
	LeftoverDays int       `json:"leftover_days"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ScheduledMeal is one planned meal on one calendar date: either a
// freshly-cooked recipe, or a leftover day pointing back (via
// CookedFromID) to the ScheduledMeal whose cooking produced it.
type ScheduledMeal struct {
	ID           string    `json:"id" gorm:"primaryKey"`
	UserID       string    `json:"-" gorm:"index:idx_scheduled_meals_user_date,priority:1"`
	WeekStart    time.Time `json:"week_start" gorm:"type:date;index"`
	Date         time.Time `json:"date" gorm:"type:date;index:idx_scheduled_meals_user_date,priority:2"`
	RecipeID     string    `json:"recipe_id"`
	IsLeftover   bool      `json:"is_leftover"`
	CookedFromID *string   `json:"cooked_from_id,omitempty"`
	CreatedAt    time.Time `json:"created_at"`

	Recipe *Recipe `json:"recipe,omitempty" gorm:"foreignKey:RecipeID;references:ID"`
}

// AutoMigrate creates/updates every table this package owns. Called both
// at server startup (cmd/server) and from the MCP server (cmd/mcpserver),
// since each is a separate binary/process that may be the first to touch
// a brand-new deployment's database.
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&Ingredient{},
		&IngredientStock{},
		&Recipe{},
		&RecipeIngredient{},
		&Rating{},
		&MealScheduleConfig{},
		&ScheduledMeal{},
	)
}
