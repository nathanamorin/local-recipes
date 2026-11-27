package recipes

import (
	"reflect"
	"testing"
	"time"
)

func TestSpreadIndices(t *testing.T) {
	cases := []struct {
		total, count int
		want         []int
	}{
		{7, 3, []int{0, 2, 4}},
		{7, 5, []int{0, 1, 2, 4, 5}},
		{7, 7, []int{0, 1, 2, 3, 4, 5, 6}},
		{7, 0, nil},
	}
	for _, c := range cases {
		got := spreadIndices(c.total, c.count)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("spreadIndices(%d, %d) = %v, want %v", c.total, c.count, got, c.want)
		}
	}
}

func TestGenerateWeekCooksAndFillsLeftovers(t *testing.T) {
	db := newTestDB(t)
	const userID = "alice"

	for _, title := range []string{"Tacos", "Stir Fry", "Soup"} {
		if _, err := CreateRecipe(db, userID, RecipeInput{Title: title, BodyMarkdown: "x"}); err != nil {
			t.Fatalf("CreateRecipe(%s): %s", title, err)
		}
	}

	if _, err := SaveScheduleConfig(db, userID, 6, 3, 1); err != nil {
		t.Fatalf("SaveScheduleConfig: %s", err)
	}

	weekStart := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC) // a Monday
	meals, err := GenerateWeek(db, userID, weekStart)
	if err != nil {
		t.Fatalf("GenerateWeek: %s", err)
	}

	if len(meals) != 6 {
		t.Fatalf("expected 6 scheduled meals (meals_per_week), got %d", len(meals))
	}

	var cookCount, leftoverCount int
	for _, m := range meals {
		if m.IsLeftover {
			leftoverCount++
			if m.CookedFromID == nil {
				t.Errorf("leftover meal on %s has no CookedFromID", m.Date)
			}
		} else {
			cookCount++
		}
		if m.RecipeID == "" {
			t.Errorf("meal on %s has no recipe assigned", m.Date)
		}
	}
	if cookCount != 3 {
		t.Errorf("expected 3 cook days (meals_to_cook), got %d", cookCount)
	}
	if leftoverCount != 3 {
		t.Errorf("expected 3 leftover days (meals_per_week - meals_to_cook), got %d", leftoverCount)
	}
}

func TestGenerateWeekIsIdempotentlyRegenerable(t *testing.T) {
	db := newTestDB(t)
	const userID = "alice"
	if _, err := CreateRecipe(db, userID, RecipeInput{Title: "Only Recipe", BodyMarkdown: "x"}); err != nil {
		t.Fatalf("CreateRecipe: %s", err)
	}
	if _, err := SaveScheduleConfig(db, userID, 3, 2, 1); err != nil {
		t.Fatalf("SaveScheduleConfig: %s", err)
	}

	weekStart := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	if _, err := GenerateWeek(db, userID, weekStart); err != nil {
		t.Fatalf("first GenerateWeek: %s", err)
	}
	second, err := GenerateWeek(db, userID, weekStart)
	if err != nil {
		t.Fatalf("second GenerateWeek: %s", err)
	}

	all, err := ListScheduledMeals(db, userID, weekStart, weekStart.AddDate(0, 0, 6))
	if err != nil {
		t.Fatalf("ListScheduledMeals: %s", err)
	}
	if len(all) != len(second) {
		t.Fatalf("regenerating the same week should replace (not append to) existing meals: have %d stored, expected %d", len(all), len(second))
	}
}

func TestGenerateWeekErrorsWithNoRecipes(t *testing.T) {
	db := newTestDB(t)
	if _, err := GenerateWeek(db, "alice", time.Now()); err == nil {
		t.Fatal("expected an error when no recipes exist to schedule")
	}
}
