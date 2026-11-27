package recipes

import (
	"testing"
)

func TestCreateRecipeTagsAndDedupesIngredients(t *testing.T) {
	db := newTestDB(t)
	const userID = "alice"

	recipe, err := CreateRecipe(db, userID, RecipeInput{
		Title:        "Pancakes",
		BodyMarkdown: "# Pancakes\n\nMix and cook.",
		Servings:     4,
		Ingredients: []IngredientInput{
			{Name: "Flour", Quantity: 2, Unit: "cup"},
			{Name: "  flour ", Quantity: 1, Unit: "cup"}, // should dedupe with "Flour"
			{Name: "Eggs", Quantity: 2, Unit: "count"},
		},
	})
	if err != nil {
		t.Fatalf("CreateRecipe: %s", err)
	}
	if len(recipe.Ingredients) != 3 {
		t.Fatalf("expected 3 recipe_ingredient rows, got %d", len(recipe.Ingredients))
	}

	ingredients, err := ListIngredients(db, userID)
	if err != nil {
		t.Fatalf("ListIngredients: %s", err)
	}
	if len(ingredients) != 2 {
		t.Fatalf("expected flour+eggs to dedupe to 2 ingredients, got %d", len(ingredients))
	}
}

func TestCreateRecipeDisambiguatesDuplicateSlugs(t *testing.T) {
	db := newTestDB(t)
	const userID = "alice"

	r1, err := CreateRecipe(db, userID, RecipeInput{Title: "Chili", BodyMarkdown: "v1"})
	if err != nil {
		t.Fatalf("CreateRecipe: %s", err)
	}
	r2, err := CreateRecipe(db, userID, RecipeInput{Title: "Chili", BodyMarkdown: "v2"})
	if err != nil {
		t.Fatalf("CreateRecipe: %s", err)
	}
	if r1.Slug == r2.Slug {
		t.Fatalf("expected distinct slugs, got %q for both", r1.Slug)
	}
}

func TestRecipesScopedPerUser(t *testing.T) {
	db := newTestDB(t)
	if _, err := CreateRecipe(db, "alice", RecipeInput{Title: "Alice's Soup", BodyMarkdown: "x"}); err != nil {
		t.Fatalf("CreateRecipe: %s", err)
	}
	if _, err := CreateRecipe(db, "bob", RecipeInput{Title: "Bob's Soup", BodyMarkdown: "x"}); err != nil {
		t.Fatalf("CreateRecipe: %s", err)
	}

	aliceRecipes, err := ListRecipes(db, "alice")
	if err != nil {
		t.Fatalf("ListRecipes: %s", err)
	}
	if len(aliceRecipes) != 1 || aliceRecipes[0].Title != "Alice's Soup" {
		t.Fatalf("expected alice to see only her own recipe, got %+v", aliceRecipes)
	}
}
