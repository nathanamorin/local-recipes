package handlers

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/nathanamorin/local-recipes/internal/recipes"
	"k8s.io/klog/v2"
)

// GetScheduleConfig godoc
// @Summary      Get the current user's meal-schedule configuration
// @Tags         schedule
// @Produce      json
// @Success      200  {object}  recipes.MealScheduleConfig
// @Router       /schedule/config [get]
func (h *Handler) GetScheduleConfig(c echo.Context) error {
	cfg, err := recipes.GetScheduleConfig(h.db, currentUserID(c))
	if err != nil {
		klog.Errorf("error getting schedule config: %s", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error getting schedule config"})
	}
	return c.JSON(http.StatusOK, cfg)
}

type scheduleConfigRequest struct {
	MealsPerWeek int `json:"meals_per_week"`
	MealsToCook  int `json:"meals_to_cook"`
	LeftoverDays int `json:"leftover_days"`
}

// SaveScheduleConfig godoc
// @Summary      Configure meals-per-week / meals-to-cook / leftover-days
// @Tags         schedule
// @Accept       json
// @Produce      json
// @Param        config  body  scheduleConfigRequest  true  "Config"
// @Success      200  {object}  recipes.MealScheduleConfig
// @Router       /schedule/config [put]
func (h *Handler) SaveScheduleConfig(c echo.Context) error {
	var req scheduleConfigRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}
	cfg, err := recipes.SaveScheduleConfig(h.db, currentUserID(c), req.MealsPerWeek, req.MealsToCook, req.LeftoverDays)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, cfg)
}

type scheduledMealListResponse struct {
	Meals []recipes.ScheduledMeal `json:"meals"`
}

// GenerateSchedule godoc
// @Summary      Generate (or regenerate) the meal schedule for a week
// @Description  week_start is any date (YYYY-MM-DD) within the target week; the week is treated as that date through 6 days later.
// @Tags         schedule
// @Produce      json
// @Param        week_start  query  string  true  "Week start date (YYYY-MM-DD)"
// @Success      200  {object}  scheduledMealListResponse
// @Router       /schedule/generate [post]
func (h *Handler) GenerateSchedule(c echo.Context) error {
	weekStart, err := parseDateParam(c.QueryParam("week_start"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "week_start must be YYYY-MM-DD"})
	}
	meals, err := recipes.GenerateWeek(h.db, currentUserID(c), weekStart)
	if err != nil {
		klog.Errorf("error generating schedule: %s", err)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, scheduledMealListResponse{Meals: meals})
}

// ListSchedule godoc
// @Summary      List scheduled meals between two dates
// @Tags         schedule
// @Produce      json
// @Param        from  query  string  true  "Start date (YYYY-MM-DD)"
// @Param        to  query  string  true  "End date (YYYY-MM-DD)"
// @Success      200  {object}  scheduledMealListResponse
// @Router       /schedule [get]
func (h *Handler) ListSchedule(c echo.Context) error {
	from, err := parseDateParam(c.QueryParam("from"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "from must be YYYY-MM-DD"})
	}
	to, err := parseDateParam(c.QueryParam("to"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "to must be YYYY-MM-DD"})
	}
	meals, err := recipes.ListScheduledMeals(h.db, currentUserID(c), from, to)
	if err != nil {
		klog.Errorf("error listing schedule: %s", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error listing schedule"})
	}
	if meals == nil {
		meals = []recipes.ScheduledMeal{}
	}
	return c.JSON(http.StatusOK, scheduledMealListResponse{Meals: meals})
}

func parseDateParam(s string) (time.Time, error) {
	return time.Parse("2006-01-02", s)
}
