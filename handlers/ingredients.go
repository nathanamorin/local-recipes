package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/nathanamorin/local-recipes/internal/recipes"
	"k8s.io/klog/v2"
)

type ingredientListResponse struct {
	Ingredients []recipes.Ingredient `json:"ingredients"`
}

// ListIngredients godoc
// @Summary      List the current user's ingredients and stock levels
// @Tags         ingredients
// @Produce      json
// @Success      200  {object}  ingredientListResponse
// @Router       /ingredients [get]
func (h *Handler) ListIngredients(c echo.Context) error {
	list, err := recipes.ListIngredients(h.db, currentUserID(c))
	if err != nil {
		klog.Errorf("error listing ingredients: %s", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error listing ingredients"})
	}
	if list == nil {
		list = []recipes.Ingredient{}
	}
	return c.JSON(http.StatusOK, ingredientListResponse{Ingredients: list})
}

// ListLowStockIngredients godoc
// @Summary      List ingredients at or below their low-stock threshold
// @Tags         ingredients
// @Produce      json
// @Success      200  {object}  ingredientListResponse
// @Router       /ingredients/low-stock [get]
func (h *Handler) ListLowStockIngredients(c echo.Context) error {
	list, err := recipes.ListLowStock(h.db, currentUserID(c))
	if err != nil {
		klog.Errorf("error listing low-stock ingredients: %s", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error listing low-stock ingredients"})
	}
	if list == nil {
		list = []recipes.Ingredient{}
	}
	return c.JSON(http.StatusOK, ingredientListResponse{Ingredients: list})
}

type createIngredientRequest struct {
	Name        string `json:"name"`
	Category    string `json:"category"`
	DefaultUnit string `json:"default_unit"`
}

// AddIngredient godoc
// @Summary      Create (or find) an ingredient
// @Tags         ingredients
// @Accept       json
// @Produce      json
// @Param        ingredient  body  createIngredientRequest  true  "Ingredient"
// @Success      200  {object}  recipes.Ingredient
// @Router       /ingredients [post]
func (h *Handler) AddIngredient(c echo.Context) error {
	var req createIngredientRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	ing, err := recipes.FindOrCreateIngredient(h.db, currentUserID(c), req.Name, req.Category, req.DefaultUnit)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, ing)
}

// DeleteIngredient godoc
// @Summary      Delete an ingredient
// @Tags         ingredients
// @Produce      json
// @Param        ingredient_id  path  string  true  "Ingredient ID"
// @Success      200  {object}  map[string]string
// @Router       /ingredients/{ingredient_id} [delete]
func (h *Handler) DeleteIngredient(c echo.Context) error {
	if err := recipes.DeleteIngredient(h.db, currentUserID(c), c.Param("ingredient_id")); err != nil {
		klog.Errorf("error deleting ingredient: %s", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error deleting ingredient"})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "success"})
}

type stockAdjustRequest struct {
	// Delta is a relative change (e.g. -2 after using 2 cups, +12 after a
	// grocery run). Mutually exclusive with Quantity.
	Delta *float64 `json:"delta,omitempty"`
	// Quantity sets the absolute on-hand amount (e.g. after a pantry
	// recount). Mutually exclusive with Delta.
	Quantity *float64 `json:"quantity,omitempty"`
	Unit     string   `json:"unit"`
}

// UpdateIngredientStock godoc
// @Summary      Adjust or set an ingredient's on-hand stock level
// @Tags         ingredients
// @Accept       json
// @Produce      json
// @Param        ingredient_id  path  string  true  "Ingredient ID"
// @Param        stock  body  stockAdjustRequest  true  "Stock change"
// @Success      200  {object}  recipes.IngredientStock
// @Router       /ingredients/{ingredient_id}/stock [put]
func (h *Handler) UpdateIngredientStock(c echo.Context) error {
	var req stockAdjustRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	userID := currentUserID(c)
	ingredientID := c.Param("ingredient_id")

	var stock *recipes.IngredientStock
	var err error
	switch {
	case req.Quantity != nil:
		stock, err = recipes.SetStock(h.db, userID, ingredientID, *req.Quantity, req.Unit)
	case req.Delta != nil:
		stock, err = recipes.AdjustStock(h.db, userID, ingredientID, *req.Delta, req.Unit)
	default:
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "either delta or quantity is required"})
	}
	if err != nil {
		klog.Errorf("error updating stock: %s", err)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, stock)
}

type lowStockThresholdRequest struct {
	Threshold float64 `json:"threshold"`
}

// SetLowStockThreshold godoc
// @Summary      Set the quantity at/below which an ingredient is "low stock"
// @Tags         ingredients
// @Accept       json
// @Produce      json
// @Param        ingredient_id  path  string  true  "Ingredient ID"
// @Param        threshold  body  lowStockThresholdRequest  true  "Threshold"
// @Success      200  {object}  map[string]string
// @Router       /ingredients/{ingredient_id}/low-stock-threshold [put]
func (h *Handler) SetLowStockThreshold(c echo.Context) error {
	var req lowStockThresholdRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	if err := recipes.SetLowStockThreshold(h.db, currentUserID(c), c.Param("ingredient_id"), req.Threshold); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "success"})
}
