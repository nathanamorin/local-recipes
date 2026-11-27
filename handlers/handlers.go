// Package handlers wires local-recipes' domain logic (internal/recipes)
// to Echo HTTP routes, mirroring local-podcasts' handlers package
// structure (one file per resource area, a shared Handler struct holding
// dependencies, Register attaching routes).
package handlers

import (
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// Handler holds shared dependencies for all route handlers.
type Handler struct {
	db *gorm.DB
}

// New constructs a Handler backed by the single shared PostgreSQL
// connection pool db.
func New(db *gorm.DB) *Handler {
	return &Handler{db: db}
}

// Register attaches all API routes to the given Echo instance, scoped to
// a group so that userIDMiddleware (which fails closed - see its doc
// comment) only guards the API itself, not the static SPA asset/fallback
// route main.go registers directly on e - those must stay reachable
// without an identity header, or the frontend's own JS/CSS bundle could
// never load in the first place.
func (h *Handler) Register(e *echo.Echo) {
	g := e.Group("", userIDMiddleware)

	g.GET("/me", h.GetCurrentUser)

	g.GET("/recipes", h.ListRecipes)
	g.POST("/recipes", h.AddRecipe)
	g.GET("/recipes/:recipe_id", h.GetRecipe)
	g.PUT("/recipes/:recipe_id", h.UpdateRecipe)
	g.DELETE("/recipes/:recipe_id", h.DeleteRecipe)
	g.GET("/recipes/:recipe_id/rating", h.GetRecipeRating)
	g.PUT("/recipes/:recipe_id/rating", h.RateRecipe)

	g.GET("/ingredients", h.ListIngredients)
	g.GET("/ingredients/low-stock", h.ListLowStockIngredients)
	g.POST("/ingredients", h.AddIngredient)
	g.DELETE("/ingredients/:ingredient_id", h.DeleteIngredient)
	g.PUT("/ingredients/:ingredient_id/stock", h.UpdateIngredientStock)
	g.PUT("/ingredients/:ingredient_id/low-stock-threshold", h.SetLowStockThreshold)

	g.GET("/schedule/config", h.GetScheduleConfig)
	g.PUT("/schedule/config", h.SaveScheduleConfig)
	g.POST("/schedule/generate", h.GenerateSchedule)
	g.GET("/schedule", h.ListSchedule)
}
