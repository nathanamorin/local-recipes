package recipes

import (
	"math"
	"testing"
	"time"

	"gorm.io/gorm"
)

func findItem(list *ShoppingList, name string) *ShoppingItem {
	for i := range list.Items {
		if list.Items[i].Name == name {
			return &list.Items[i]
		}
	}
	return nil
}

func approx(a, b float64) bool { return math.Abs(a-b) < 0.01 }

func ingredientID(t *testing.T, db *gorm.DB, userID, name string) string {
	t.Helper()
	ing, err := FindOrCreateIngredient(db, userID, name, "", "")
	if err != nil {
		t.Fatal(err)
	}
	return ing.ID
}

func TestBuildShoppingList(t *testing.T) {
	db := newTestDB(t)
	const user = "alice"
	monday := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)

	pancakes, _ := CreateRecipe(db, user, RecipeInput{Title: "Pancakes", BodyMarkdown: "x", Servings: 4, Ingredients: []IngredientInput{
		{Name: "flour", Quantity: 2, Unit: "cup"},
		{Name: "eggs", Quantity: 2, Unit: "count"},
		{Name: "milk", Quantity: 500, Unit: "ml"},
		{Name: "salt", Quantity: 0, Unit: ""}, // to taste
		{Name: "blueberries", Quantity: 1, Unit: "cup", Optional: true},
	}})
	omelet, _ := CreateRecipe(db, user, RecipeInput{Title: "Omelet", BodyMarkdown: "x", Ingredients: []IngredientInput{
		{Name: "eggs", Quantity: 3},
		{Name: "butter", Quantity: 1, Unit: "tbsp"},
	}})
	// Pantry: eggs tracked as a count with a threshold, milk tracked in
	// litres, flour tracked in grams (cups can't convert), butter plenty.
	SetStock(db, user, ingredientID(t, db, user, "eggs"), 4, "count")
	SetLowStockThreshold(db, user, ingredientID(t, db, user, "eggs"), 6)
	SetStock(db, user, ingredientID(t, db, user, "milk"), 0.2, "l")
	SetStock(db, user, ingredientID(t, db, user, "flour"), 1000, "g")
	SetStock(db, user, ingredientID(t, db, user, "butter"), 20, "tbsp")
	// An out-of-stock item nothing needs, with no threshold.
	SetStock(db, user, ingredientID(t, db, user, "coffee"), 0, "bag")
	// A low item nothing needs.
	SetStock(db, user, ingredientID(t, db, user, "rice"), 1, "cup")
	SetLowStockThreshold(db, user, ingredientID(t, db, user, "rice"), 2)

	plan := []ScheduledMeal{
		{Date: monday, RecipeID: pancakes.ID},
		{Date: monday.AddDate(0, 0, 1), RecipeID: pancakes.ID, IsLeftover: true}, // leftovers: no new ingredients
		{Date: monday.AddDate(0, 0, 2), RecipeID: omelet.ID},
		{Date: monday.AddDate(0, 0, 9), RecipeID: omelet.ID}, // next week: out of range
	}
	for i := range plan {
		plan[i].ID = string(rune('a' + i))
		plan[i].UserID = user
		plan[i].WeekStart = monday
		if err := db.Create(&plan[i]).Error; err != nil {
			t.Fatal(err)
		}
	}

	list, err := BuildShoppingList(db, user, monday, monday.AddDate(0, 0, 6))
	if err != nil {
		t.Fatal(err)
	}
	if list.PlannedMeals != 2 {
		t.Fatalf("planned meals = %d, want 2 (leftovers and next week excluded)", list.PlannedMeals)
	}

	// eggs: need 2 + 3 = 5, threshold 6, have 4 -> buy 7.
	if eggs := findItem(list, "eggs"); eggs == nil || !approx(eggs.Quantity, 7) || !approx(eggs.Planned, 5) {
		t.Fatalf("eggs = %+v, want buy 7 (planned 5)", eggs)
	}
	// milk: need 500 ml = 0.5 l, have 0.2 l -> buy 0.3 l.
	if milk := findItem(list, "milk"); milk == nil || milk.Unit != "l" || !approx(milk.Quantity, 0.3) {
		t.Fatalf("milk = %+v, want buy 0.3 l", milk)
	}
	// flour: cups vs grams can't be compared -> listed with the need unconverted.
	if flour := findItem(list, "flour"); flour == nil || len(flour.Unconverted) != 1 || flour.Quantity != 0 {
		t.Fatalf("flour = %+v, want unconverted cup need", flour)
	}
	// Reasons must serialize as [] (never null) - the UI iterates it.
	for _, it := range list.Items {
		if it.Reasons == nil {
			t.Fatalf("%s has nil Reasons", it.Name)
		}
	}
	// butter: 1 tbsp needed, 20 on hand -> not listed.
	if findItem(list, "butter") != nil {
		t.Fatal("butter is well stocked and shouldn't be listed")
	}
	// optional ingredients are ignored.
	if findItem(list, "blueberries") != nil {
		t.Fatal("optional blueberries shouldn't be listed")
	}
	// untracked salt "to taste": listed as untracked so the user checks.
	if salt := findItem(list, "salt"); salt == nil || salt.Tracked {
		t.Fatalf("salt = %+v, want untracked entry", salt)
	}
	// coffee is out with no threshold: listed, amount unknown.
	if coffee := findItem(list, "coffee"); coffee == nil || coffee.Quantity != 0 || coffee.Reasons[0].Kind != "restock" {
		t.Fatalf("coffee = %+v, want restock with unknown amount", coffee)
	}
	// rice: low (1 <= 2) and nothing planned -> buy 1 to get back to the threshold.
	if rice := findItem(list, "rice"); rice == nil || !approx(rice.Quantity, 1) {
		t.Fatalf("rice = %+v, want buy 1", rice)
	}
	// tracked items sort before untracked ones.
	if last := list.Items[len(list.Items)-1]; last.Tracked {
		t.Fatalf("untracked items should sort last, got %+v", last)
	}
}

func TestCookRecipe(t *testing.T) {
	db := newTestDB(t)
	const user = "alice"
	r, _ := CreateRecipe(db, user, RecipeInput{Title: "Pancakes", BodyMarkdown: "x", Servings: 4, Ingredients: []IngredientInput{
		{Name: "flour", Quantity: 2, Unit: "cup"},
		{Name: "eggs", Quantity: 2},
		{Name: "butter", Quantity: 4, Unit: "tbsp"},
		{Name: "vanilla", Quantity: 1, Unit: "tsp"},
		{Name: "berries", Quantity: 1, Unit: "cup", Optional: true},
	}})
	SetStock(db, user, ingredientID(t, db, user, "flour"), 1000, "g")  // can't convert
	SetStock(db, user, ingredientID(t, db, user, "eggs"), 3, "count")  // short when doubled
	SetStock(db, user, ingredientID(t, db, user, "butter"), 1, "cup")  // 16 tbsp
	SetStock(db, user, ingredientID(t, db, user, "berries"), 2, "cup") // optional

	// Dry run at 8 servings (x2) changes nothing.
	preview, err := CookRecipe(db, user, r.ID, 8, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Changes) != 2 || len(preview.Skipped) != 3 {
		t.Fatalf("preview = %+v", preview)
	}
	ings, _ := ListIngredients(db, user)
	for _, i := range ings {
		if i.Name == "eggs" && i.Stock.QuantityOnHand != 3 {
			t.Fatal("dry run modified stock")
		}
	}

	res, err := CookRecipe(db, user, r.ID, 8, false)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]CookChange{}
	for _, c := range res.Changes {
		got[c.Name] = c
	}
	// eggs: need 4, have 3 -> clamps at 0, removes 3, flagged short.
	if e := got["eggs"]; e.Quantity != 3 || e.After != 0 || !e.Short {
		t.Fatalf("eggs change = %+v", e)
	}
	// butter: 8 tbsp = 0.5 cup off 1 cup, unit kept as cup.
	if b := got["butter"]; !approx(b.Quantity, 0.5) || b.Unit != "cup" || !approx(b.After, 0.5) {
		t.Fatalf("butter change = %+v", b)
	}
	ings, _ = ListIngredients(db, user)
	for _, i := range ings {
		switch i.Name {
		case "butter":
			if !approx(i.Stock.QuantityOnHand, 0.5) || i.Stock.Unit != "cup" {
				t.Fatalf("butter stock = %+v", i.Stock)
			}
		case "flour":
			if i.Stock.QuantityOnHand != 1000 || i.Stock.Unit != "g" {
				t.Fatalf("flour stock should be untouched, got %+v", i.Stock)
			}
		case "berries":
			if i.Stock.QuantityOnHand != 2 {
				t.Fatal("optional berries were deducted")
			}
		}
	}
}
