package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/nathanamorin/local-recipes/internal/recipes"
	"gorm.io/gorm"
	"k8s.io/klog/v2"
)

// GetShoppingList godoc
// @Summary      What to buy for planned meals and low stock
// @Description  Covers cooked (non-leftover) meals planned between from and to, plus restocking anything that would end at or below its low-stock threshold. Defaults: from = today, to = the Sunday ending from's week.
// @Tags         shopping
// @Produce      json
// @Param        from  query  string  false  "YYYY-MM-DD"
// @Param        to    query  string  false  "YYYY-MM-DD"
// @Success      200  {object}  recipes.ShoppingList
// @Router       /shopping [get]
func (h *Handler) GetShoppingList(c echo.Context) error {
	from, to, err := shoppingRange(c.QueryParam("from"), c.QueryParam("to"), time.Now())
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	list, err := recipes.BuildShoppingList(h.db, currentUserID(c), from, to)
	if err != nil {
		klog.Errorf("error building shopping list: %s", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error building shopping list"})
	}
	return c.JSON(http.StatusOK, list)
}

func shoppingRange(fromParam, toParam string, now time.Time) (time.Time, time.Time, error) {
	from := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	if fromParam != "" {
		var err error
		if from, err = parseDateParam(fromParam); err != nil {
			return from, from, errors.New("from must be YYYY-MM-DD")
		}
	}
	// End of from's Monday-start week.
	to := from.AddDate(0, 0, (7-int(from.Weekday()))%7)
	if toParam != "" {
		var err error
		if to, err = parseDateParam(toParam); err != nil {
			return from, to, errors.New("to must be YYYY-MM-DD")
		}
	}
	if to.Before(from) {
		return from, to, errors.New("to must not be before from")
	}
	return from, to, nil
}

type cookRequest struct {
	Servings int  `json:"servings"`
	DryRun   bool `json:"dry_run"`
}

// CookRecipe godoc
// @Summary      Use one batch of a recipe's ingredients from the pantry
// @Description  Deducts the recipe's (non-optional) ingredients, scaled to servings, from tracked pantry stock. dry_run previews without changing anything.
// @Tags         recipes
// @Accept       json
// @Produce      json
// @Param        recipe_id  path  string       true  "Recipe ID"
// @Param        body       body  cookRequest  false "servings (0 = as written) and dry_run"
// @Success      200  {object}  recipes.CookResult
// @Router       /recipes/{recipe_id}/cook [post]
func (h *Handler) CookRecipe(c echo.Context) error {
	var req cookRequest
	if c.Request().ContentLength != 0 {
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		}
	}
	if req.Servings < 0 || req.Servings > 1000 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "servings must be between 0 and 1000"})
	}
	res, err := recipes.CookRecipe(h.db, currentUserID(c), c.Param("recipe_id"), req.Servings, req.DryRun)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "recipe not found"})
	}
	if err != nil {
		klog.Errorf("error cooking recipe: %s", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error updating pantry"})
	}
	return c.JSON(http.StatusOK, res)
}
