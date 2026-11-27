package recipes

import "testing"

func TestUpsertRatingOverwritesPreviousRating(t *testing.T) {
	db := newTestDB(t)
	const userID = "alice"
	recipe, err := CreateRecipe(db, userID, RecipeInput{Title: "Lasagna", BodyMarkdown: "x"})
	if err != nil {
		t.Fatalf("CreateRecipe: %s", err)
	}

	if _, err := UpsertRating(db, userID, recipe.ID, 3, "decent"); err != nil {
		t.Fatalf("UpsertRating: %s", err)
	}
	if _, err := UpsertRating(db, userID, recipe.ID, 5, "actually great"); err != nil {
		t.Fatalf("UpsertRating: %s", err)
	}

	rating, err := GetRating(db, userID, recipe.ID)
	if err != nil {
		t.Fatalf("GetRating: %s", err)
	}
	if rating.Score != 5 || rating.Notes != "actually great" {
		t.Fatalf("expected the second rating to overwrite the first, got %+v", rating)
	}

	list, err := ListRecipes(db, userID)
	if err != nil {
		t.Fatalf("ListRecipes: %s", err)
	}
	if len(list) != 1 || list[0].AverageRating != 5 || list[0].RatingCount != 1 {
		t.Fatalf("expected a single rating of 5 to be reflected in the recipe summary, got %+v", list)
	}
}

func TestUpsertRatingRejectsOutOfRangeScore(t *testing.T) {
	db := newTestDB(t)
	const userID = "alice"
	recipe, err := CreateRecipe(db, userID, RecipeInput{Title: "Lasagna", BodyMarkdown: "x"})
	if err != nil {
		t.Fatalf("CreateRecipe: %s", err)
	}
	if _, err := UpsertRating(db, userID, recipe.ID, 0, ""); err == nil {
		t.Fatal("expected an error for a score of 0")
	}
	if _, err := UpsertRating(db, userID, recipe.ID, 6, ""); err == nil {
		t.Fatal("expected an error for a score of 6")
	}
}
