package recipes

import (
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"
)

func titles(list []Recipe) []string {
	out := make([]string, len(list))
	for i, r := range list {
		out[i] = r.Title
	}
	return out
}

func TestArchiveAndRestoreRecipe(t *testing.T) {
	db := newTestDB(t)
	const userID = "alice"

	keep, err := CreateRecipe(db, userID, RecipeInput{Title: "Tacos", BodyMarkdown: "x"})
	if err != nil {
		t.Fatal(err)
	}
	gone, err := CreateRecipe(db, userID, RecipeInput{
		Title: "Old Soup", BodyMarkdown: "x",
		Ingredients: []IngredientInput{{Name: "leeks", Quantity: 2}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := UpsertRating(db, userID, gone.ID, 4, "fine"); err != nil {
		t.Fatal(err)
	}

	archived, err := SetRecipeArchived(db, userID, gone.ID, true)
	if err != nil {
		t.Fatalf("archive: %s", err)
	}
	if archived.ArchivedAt == nil {
		t.Fatal("ArchivedAt not set")
	}

	active, _ := ListRecipes(db, userID)
	if got := titles(active); len(got) != 1 || got[0] != keep.Title {
		t.Fatalf("active list = %v, want only %q", got, keep.Title)
	}
	arch, _ := ListArchivedRecipes(db, userID)
	if got := titles(arch); len(got) != 1 || got[0] != gone.Title {
		t.Fatalf("archived list = %v, want only %q", got, gone.Title)
	}

	// Still readable directly (detail page shows an "archived" banner),
	// with ingredients and rating intact.
	got, err := GetRecipe(db, userID, gone.ID)
	if err != nil || got.ArchivedAt == nil || len(got.Ingredients) != 1 || got.RatingCount != 1 {
		t.Fatalf("archived recipe lost data: %+v err=%v", got, err)
	}

	// Another user can't archive or restore it.
	if _, err := SetRecipeArchived(db, "mallory", gone.ID, false); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-user restore: got %v, want ErrRecordNotFound", err)
	}

	restored, err := SetRecipeArchived(db, userID, gone.ID, false)
	if err != nil || restored.ArchivedAt != nil {
		t.Fatalf("restore: %+v err=%v", restored, err)
	}
	active, _ = ListRecipes(db, userID)
	if len(active) != 2 {
		t.Fatalf("restored recipe not back in library: %v", titles(active))
	}
}

func TestGenerateWeekSkipsArchivedRecipes(t *testing.T) {
	db := newTestDB(t)
	const userID = "alice"

	live, _ := CreateRecipe(db, userID, RecipeInput{Title: "Tacos", BodyMarkdown: "x"})
	old, _ := CreateRecipe(db, userID, RecipeInput{Title: "Old Soup", BodyMarkdown: "x"})
	if _, err := SetRecipeArchived(db, userID, old.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveScheduleConfig(db, userID, 7, 4, 1); err != nil {
		t.Fatal(err)
	}
	meals, err := GenerateWeek(db, userID, time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range meals {
		if m.RecipeID != live.ID {
			t.Fatalf("archived recipe %s was scheduled", m.RecipeID)
		}
	}

	// With every recipe archived there's nothing to plan.
	if _, err := SetRecipeArchived(db, userID, live.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := GenerateWeek(db, userID, time.Date(2026, 1, 12, 0, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("expected an error when all recipes are archived")
	}
}

func TestDeleteRecipeRemovesScheduledMeals(t *testing.T) {
	db := newTestDB(t)
	const userID = "alice"
	r, _ := CreateRecipe(db, userID, RecipeInput{Title: "Tacos", BodyMarkdown: "x"})
	if _, err := SaveScheduleConfig(db, userID, 3, 1, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := GenerateWeek(db, userID, time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if err := DeleteRecipe(db, userID, r.ID); err != nil {
		t.Fatal(err)
	}
	var n int64
	db.Model(&ScheduledMeal{}).Where("recipe_id = ?", r.ID).Count(&n)
	if n != 0 {
		t.Fatalf("%d scheduled meals still reference the deleted recipe", n)
	}
}
