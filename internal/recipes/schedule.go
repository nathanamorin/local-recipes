package recipes

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DefaultMealsPerWeek/DefaultMealsToCook/DefaultLeftoverDays are used when
// a user hasn't configured MealScheduleConfig yet - a reasonable starting
// point (cook 3 nights, eat leftovers for one day after each, 5 of 7
// nights planned, 2 nights unplanned/eating out) rather than requiring
// setup before the schedule feature can be used at all.
const (
	DefaultMealsPerWeek = 5
	DefaultMealsToCook  = 3
	DefaultLeftoverDays = 1
)

// GetScheduleConfig returns userID's meal-schedule configuration, falling
// back to the package defaults (without persisting them) if the user
// hasn't saved one yet.
func GetScheduleConfig(db *gorm.DB, userID string) (*MealScheduleConfig, error) {
	var cfg MealScheduleConfig
	err := db.Where("user_id = ?", userID).First(&cfg).Error
	if err == gorm.ErrRecordNotFound {
		return &MealScheduleConfig{
			UserID:       userID,
			MealsPerWeek: DefaultMealsPerWeek,
			MealsToCook:  DefaultMealsToCook,
			LeftoverDays: DefaultLeftoverDays,
		}, nil
	}
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// SaveScheduleConfig creates or updates userID's meal-schedule
// configuration.
func SaveScheduleConfig(db *gorm.DB, userID string, mealsPerWeek, mealsToCook, leftoverDays int) (*MealScheduleConfig, error) {
	if mealsPerWeek < 1 || mealsPerWeek > 7 {
		return nil, fmt.Errorf("meals_per_week must be between 1 and 7")
	}
	if mealsToCook < 1 || mealsToCook > mealsPerWeek {
		return nil, fmt.Errorf("meals_to_cook must be between 1 and meals_per_week")
	}
	if leftoverDays < 0 {
		return nil, fmt.Errorf("leftover_days must be >= 0")
	}

	cfg := MealScheduleConfig{
		UserID:       userID,
		MealsPerWeek: mealsPerWeek,
		MealsToCook:  mealsToCook,
		LeftoverDays: leftoverDays,
		UpdatedAt:    time.Now(),
	}
	err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"meals_per_week", "meals_to_cook", "leftover_days", "updated_at"}),
	}).Create(&cfg).Error
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// spreadIndices returns count indices spread as evenly as possible across
// [0, total), e.g. spreadIndices(7, 3) -> [0, 2, 4] and spreadIndices(7, 5)
// -> [0, 1, 3, 4, 6]. Used both to pick which days of the week have a
// planned meal at all, and (recursively) which of those meal days are
// cook-from-scratch days, so meals/cook days are never all clustered at
// the start of the week.
func spreadIndices(total, count int) []int {
	if count <= 0 || total <= 0 {
		return nil
	}
	if count >= total {
		out := make([]int, total)
		for i := range out {
			out[i] = i
		}
		return out
	}
	out := make([]int, count)
	for i := 0; i < count; i++ {
		out[i] = (i * total) / count
	}
	return out
}

// ListScheduledMeals returns every scheduled meal for userID whose Date
// falls within [from, to], ordered chronologically, with each meal's
// Recipe preloaded.
func ListScheduledMeals(db *gorm.DB, userID string, from, to time.Time) ([]ScheduledMeal, error) {
	var meals []ScheduledMeal
	err := db.Where("user_id = ? AND date >= ? AND date <= ?", userID, from, to).
		Preload("Recipe").
		Order("date asc").
		Find(&meals).Error
	return meals, err
}

// GenerateWeek (re)generates userID's meal schedule for the 7-day week
// starting on weekStart (any existing scheduled meals for that week are
// replaced), per their MealScheduleConfig:
//
//  1. MealsPerWeek days of the week are chosen (evenly spread, via
//     spreadIndices) to have a planned meal at all - the rest are left
//     unplanned (eating out, skipping, etc).
//  2. Of those meal days, MealsToCook are chosen (again evenly spread) as
//     "cook days": a recipe is picked - preferring recipes not cooked
//     recently, then higher-rated ones - and freshly cooked.
//  3. Each other meal day is filled with leftovers from the most recent
//     preceding cook day, up to LeftoverDays days after it. A meal day
//     that precedes every cook day this week (e.g. Monday, if the first
//     cook day is Tuesday) instead carries over leftovers from the most
//     recently cooked recipe overall (including prior weeks); if there's
//     no cooking history at all yet, that day is simply left unplanned.
func GenerateWeek(db *gorm.DB, userID string, weekStart time.Time) ([]ScheduledMeal, error) {
	weekStart = time.Date(weekStart.Year(), weekStart.Month(), weekStart.Day(), 0, 0, 0, 0, time.UTC)

	cfg, err := GetScheduleConfig(db, userID)
	if err != nil {
		return nil, err
	}

	pool, err := recipePoolLeastRecentlyCooked(db, userID)
	if err != nil {
		return nil, err
	}
	if len(pool) == 0 {
		return nil, fmt.Errorf("no recipes to schedule - add some recipes first")
	}

	mealDayIdx := spreadIndices(7, cfg.MealsPerWeek)
	cookPositions := spreadIndices(len(mealDayIdx), cfg.MealsToCook)
	isCookDay := make(map[int]bool, len(cookPositions))
	for _, p := range cookPositions {
		isCookDay[mealDayIdx[p]] = true
	}

	fallbackRecipeID, _ := mostRecentlyCookedRecipeID(db, userID, weekStart)

	var created []ScheduledMeal
	err = db.Transaction(func(tx *gorm.DB) error {
		weekEnd := weekStart.AddDate(0, 0, 6)
		if err := tx.Where("user_id = ? AND date >= ? AND date <= ?", userID, weekStart, weekEnd).
			Delete(&ScheduledMeal{}).Error; err != nil {
			return err
		}

		var lastCookMealID *string
		var lastCookDayOffset = -1 // how many days ago (within this loop) the last cook day was
		recipeCursor := 0

		for _, dayIdx := range mealDayIdx {
			date := weekStart.AddDate(0, 0, dayIdx)

			if isCookDay[dayIdx] {
				recipeID := pool[recipeCursor%len(pool)]
				recipeCursor++
				meal := ScheduledMeal{
					ID:        uuid.NewString(),
					UserID:    userID,
					WeekStart: weekStart,
					Date:      date,
					RecipeID:  recipeID,
					CreatedAt: time.Now(),
				}
				if err := tx.Create(&meal).Error; err != nil {
					return err
				}
				created = append(created, meal)
				lastCookMealID = &meal.ID
				lastCookDayOffset = 0
				continue
			}

			// Leftover day: use the most recent cook day's meal if it's
			// still within LeftoverDays, otherwise fall back to the most
			// recently cooked recipe from history (or skip if neither is
			// available).
			if lastCookMealID != nil && lastCookDayOffset < cfg.LeftoverDays {
				var cookedMeal ScheduledMeal
				if err := tx.First(&cookedMeal, "id = ?", *lastCookMealID).Error; err != nil {
					return err
				}
				meal := ScheduledMeal{
					ID:           uuid.NewString(),
					UserID:       userID,
					WeekStart:    weekStart,
					Date:         date,
					RecipeID:     cookedMeal.RecipeID,
					IsLeftover:   true,
					CookedFromID: lastCookMealID,
					CreatedAt:    time.Now(),
				}
				if err := tx.Create(&meal).Error; err != nil {
					return err
				}
				created = append(created, meal)
			} else if fallbackRecipeID != "" {
				meal := ScheduledMeal{
					ID:         uuid.NewString(),
					UserID:     userID,
					WeekStart:  weekStart,
					Date:       date,
					RecipeID:   fallbackRecipeID,
					IsLeftover: true,
					CreatedAt:  time.Now(),
				}
				if err := tx.Create(&meal).Error; err != nil {
					return err
				}
				created = append(created, meal)
			}
			// else: no cook day has happened yet and there's no history -
			// leave this meal day unplanned.

			if lastCookDayOffset >= 0 {
				lastCookDayOffset++
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

// recipePoolLeastRecentlyCooked returns userID's recipe IDs ordered with
// never-or-longest-ago-cooked recipes first, then by highest average
// rating - so GenerateWeek naturally rotates through the catalog instead
// of repeating a handful of favorites every week, while still preferring
// well-rated recipes among equally-fresh choices.
func recipePoolLeastRecentlyCooked(db *gorm.DB, userID string) ([]string, error) {
	var rows []struct {
		ID string
		// LastCooked is scanned as interface{} rather than time.Time/
		// sql.NullTime because its source column is a MAX(date) inside a
		// subquery - Postgres' driver hands that back as a native
		// time.Time, but some other database/sql drivers (notably the
		// pure-Go SQLite driver this package's tests use) return a plain
		// string for computed/aggregate date columns instead. See
		// parseLastCooked below for the portable conversion.
		LastCooked interface{}
		AvgRating  float64
	}
	err := db.Table("recipes").
		Select("recipes.id as id, last_cooked.max_date as last_cooked, COALESCE(avg(ratings.score), 0) as avg_rating").
		Joins("LEFT JOIN ratings ON ratings.recipe_id = recipes.id AND ratings.user_id = recipes.user_id").
		Joins(`LEFT JOIN (
			SELECT recipe_id, max(date) as max_date FROM scheduled_meals
			WHERE user_id = ? AND is_leftover = false GROUP BY recipe_id
		) as last_cooked ON last_cooked.recipe_id = recipes.id`, userID).
		Where("recipes.user_id = ?", userID).
		Group("recipes.id, last_cooked.max_date").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	// Sort client-side: NULL/never-cooked first (oldest possible),
	// then oldest-cooked-first, then by rating descending as a tiebreak.
	for i := 1; i < len(rows); i++ {
		j := i
		for j > 0 && lessRecipeRank(rows[j], rows[j-1]) {
			rows[j], rows[j-1] = rows[j-1], rows[j]
			j--
		}
	}

	ids := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	return ids, nil
}

func lessRecipeRank(a, b struct {
	ID         string
	LastCooked interface{}
	AvgRating  float64
}) bool {
	aTime, aOK := parseLastCooked(a.LastCooked)
	bTime, bOK := parseLastCooked(b.LastCooked)
	if aOK != bOK {
		return !aOK
	}
	if aOK && bOK && !aTime.Equal(bTime) {
		return aTime.Before(bTime)
	}
	return a.AvgRating > b.AvgRating
}

// parseLastCooked normalizes recipePoolLeastRecentlyCooked's raw
// LastCooked column value - a native time.Time (Postgres), a formatted
// string, or []byte (some other database/sql drivers), or nil (never
// cooked) - into a time.Time, reporting false if it's nil/unparseable.
func parseLastCooked(v interface{}) (time.Time, bool) {
	switch t := v.(type) {
	case time.Time:
		return t, true
	case string:
		return parseFlexibleTime(t)
	case []byte:
		return parseFlexibleTime(string(t))
	default:
		return time.Time{}, false
	}
}

// flexibleTimeLayouts covers the datetime string formats this package
// might encounter from a non-Postgres database/sql driver (notably
// SQLite in tests), tried in order until one parses successfully.
var flexibleTimeLayouts = []string{
	time.RFC3339Nano,
	"2006-01-02 15:04:05.999999999-07:00",
	"2006-01-02 15:04:05.999999999",
	"2006-01-02 15:04:05",
	"2006-01-02",
}

func parseFlexibleTime(s string) (time.Time, bool) {
	for _, layout := range flexibleTimeLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// mostRecentlyCookedRecipeID returns the recipe ID most recently cooked
// (non-leftover) strictly before weekStart, if any - used as a fallback
// for a leftover day that precedes every cook day in the week being
// generated.
func mostRecentlyCookedRecipeID(db *gorm.DB, userID string, weekStart time.Time) (string, error) {
	var meal ScheduledMeal
	err := db.Where("user_id = ? AND is_leftover = false AND date < ?", userID, weekStart).
		Order("date desc").
		First(&meal).Error
	if err == gorm.ErrRecordNotFound {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return meal.RecipeID, nil
}
