// Package handlers wires local-recipes' domain logic (internal/recipes)
// to Echo HTTP routes, mirroring local-podcasts' handlers package
// structure (one file per resource area, a shared Handler struct holding
// dependencies, Register attaching routes).
package handlers

import (
	"github.com/labstack/echo/v4"
	"github.com/nathanamorin/local-recipes/internal/identity"
	"gorm.io/gorm"
)

// Handler holds shared dependencies for all route handlers.
type Handler struct {
	db              *gorm.DB
	profiles        *identity.ProfileFetcher
	pictureTemplate string
}

// WithPictureURLTemplate sets a {sub} URL template for the user's
// profile picture, used when the userinfo lookup doesn't provide one.
func WithPictureURLTemplate(t string) Option {
	return func(h *Handler) { h.pictureTemplate = t }
}

// Option configures optional Handler dependencies.
type Option func(*Handler)

// WithProfiles enables looking up the signed-in user's display profile
// (name, picture, ...) for GET /api/me.
func WithProfiles(f *identity.ProfileFetcher) Option {
	return func(h *Handler) { h.profiles = f }
}

// New constructs a Handler backed by the single shared PostgreSQL
// connection pool db.
func New(db *gorm.DB, opts ...Option) *Handler {
	h := &Handler{db: db}
	for _, o := range opts {
		o(h)
	}
	return h
}

// APIPrefix is the path prefix every API route is mounted under.
const APIPrefix = "/api"

// Register attaches all API routes to the given Echo instance under
// APIPrefix, scoped to a group so that userIDMiddleware (which fails
// closed - see its doc comment) only guards the API itself, not the static
// SPA asset/fallback route main.go registers directly on e.
//
// The prefix must be non-empty: Echo's Group.Use registers catch-all
// routes for the group's prefix and prefix+"/*" so group middleware runs
// even on unmatched paths. With an empty prefix those catch-alls are "/"
// and "/*", which shadow the SPA's root and make it 401/404. A shared
// prefix also keeps API paths (/api/ingredients) from colliding with the
// SPA's client-side routes (/ingredients) on refresh or deep link.
func (h *Handler) Register(e *echo.Echo) {
	g := e.Group(APIPrefix, userIDMiddleware)

	g.GET("/me", h.GetCurrentUser)

	g.GET("/recipes", h.ListRecipes)
	g.POST("/recipes", h.AddRecipe)
	g.GET("/recipes/:recipe_id", h.GetRecipe)
	g.PUT("/recipes/:recipe_id", h.UpdateRecipe)
	g.DELETE("/recipes/:recipe_id", h.DeleteRecipe)
	g.POST("/recipes/:recipe_id/archive", h.ArchiveRecipe)
	g.POST("/recipes/:recipe_id/restore", h.RestoreRecipe)
	g.POST("/recipes/:recipe_id/cook", h.CookRecipe)
	g.GET("/shopping", h.GetShoppingList)
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
