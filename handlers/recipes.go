package handlers

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/nathanamorin/local-recipes/internal/recipes"
	"gorm.io/gorm"
	"k8s.io/klog/v2"
)

type recipeListResponse struct {
	Recipes []recipes.Recipe `json:"recipes"`
}

// ListRecipes godoc
// @Summary      List the current user's recipes
// @Tags         recipes
// @Produce      json
// @Success      200  {object}  recipeListResponse
// @Router       /recipes [get]
func (h *Handler) ListRecipes(c echo.Context) error {
	list, err := recipes.ListRecipes(h.db, currentUserID(c))
	if err != nil {
		klog.Errorf("error listing recipes: %s", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error listing recipes"})
	}
	if list == nil {
		list = []recipes.Recipe{}
	}
	return c.JSON(http.StatusOK, recipeListResponse{Recipes: list})
}

type recipeRequest struct {
	Title        string                    `json:"title"`
	Description  string                    `json:"description"`
	BodyMarkdown string                    `json:"body_markdown"`
	Servings     int                       `json:"servings"`
	PrepMinutes  int                       `json:"prep_minutes"`
	CookMinutes  int                       `json:"cook_minutes"`
	SourceURL    string                    `json:"source_url"`
	ImageURL     string                    `json:"image_url"`
	Ingredients  []recipes.IngredientInput `json:"ingredients"`
}

func (r recipeRequest) toInput() recipes.RecipeInput {
	return recipes.RecipeInput{
		Title:        r.Title,
		Description:  r.Description,
		BodyMarkdown: r.BodyMarkdown,
		Servings:     r.Servings,
		PrepMinutes:  r.PrepMinutes,
		CookMinutes:  r.CookMinutes,
		SourceURL:    r.SourceURL,
		ImageURL:     r.ImageURL,
		Ingredients:  r.Ingredients,
	}
}

// AddRecipe godoc
// @Summary      Create (or import) a recipe
// @Tags         recipes
// @Accept       json
// @Produce      json
// @Param        recipe  body  recipeRequest  true  "Recipe"
// @Success      200  {object}  recipes.Recipe
// @Router       /recipes [post]
func (h *Handler) AddRecipe(c echo.Context) error {
	var req recipeRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	recipe, err := recipes.CreateRecipe(h.db, currentUserID(c), req.toInput())
	if err != nil {
		klog.Errorf("error creating recipe: %s", err)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, recipe)
}

// GetRecipe godoc
// @Summary      Get a single recipe
// @Tags         recipes
// @Produce      json
// @Param        recipe_id  path  string  true  "Recipe ID"
// @Success      200  {object}  recipes.Recipe
// @Failure      404  {object}  map[string]string
// @Router       /recipes/{recipe_id} [get]
func (h *Handler) GetRecipe(c echo.Context) error {
	recipe, err := recipes.GetRecipe(h.db, currentUserID(c), c.Param("recipe_id"))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "recipe not found"})
	}
	if err != nil {
		klog.Errorf("error getting recipe: %s", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error getting recipe"})
	}
	return c.JSON(http.StatusOK, recipe)
}

// UpdateRecipe godoc
// @Summary      Update a recipe
// @Tags         recipes
// @Accept       json
// @Produce      json
// @Param        recipe_id  path  string  true  "Recipe ID"
// @Param        recipe  body  recipeRequest  true  "Recipe"
// @Success      200  {object}  recipes.Recipe
// @Router       /recipes/{recipe_id} [put]
func (h *Handler) UpdateRecipe(c echo.Context) error {
	var req recipeRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	recipe, err := recipes.UpdateRecipe(h.db, currentUserID(c), c.Param("recipe_id"), req.toInput())
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "recipe not found"})
	}
	if err != nil {
		klog.Errorf("error updating recipe: %s", err)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, recipe)
}

// DeleteRecipe godoc
// @Summary      Delete a recipe
// @Tags         recipes
// @Produce      json
// @Param        recipe_id  path  string  true  "Recipe ID"
// @Success      200  {object}  map[string]string
// @Router       /recipes/{recipe_id} [delete]
func (h *Handler) DeleteRecipe(c echo.Context) error {
	if err := recipes.DeleteRecipe(h.db, currentUserID(c), c.Param("recipe_id")); err != nil {
		klog.Errorf("error deleting recipe: %s", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error deleting recipe"})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "success"})
}

type ratingRequest struct {
	Score int    `json:"score"`
	Notes string `json:"notes"`
}

// RateRecipe godoc
// @Summary      Rate a recipe (1-5)
// @Tags         recipes
// @Accept       json
// @Produce      json
// @Param        recipe_id  path  string  true  "Recipe ID"
// @Param        rating  body  ratingRequest  true  "Rating"
// @Success      200  {object}  recipes.Rating
// @Router       /recipes/{recipe_id}/rating [put]
func (h *Handler) RateRecipe(c echo.Context) error {
	var req ratingRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	rating, err := recipes.UpsertRating(h.db, currentUserID(c), c.Param("recipe_id"), req.Score, req.Notes)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, rating)
}

// GetRecipeRating godoc
// @Summary      Get the current user's rating for a recipe
// @Tags         recipes
// @Produce      json
// @Param        recipe_id  path  string  true  "Recipe ID"
// @Success      200  {object}  recipes.Rating
// @Failure      404  {object}  map[string]string
// @Router       /recipes/{recipe_id}/rating [get]
func (h *Handler) GetRecipeRating(c echo.Context) error {
	rating, err := recipes.GetRating(h.db, currentUserID(c), c.Param("recipe_id"))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "no rating yet"})
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error getting rating"})
	}
	return c.JSON(http.StatusOK, rating)
}
