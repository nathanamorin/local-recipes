package recipes

import "testing"

func TestAdjustStockClampsAtZero(t *testing.T) {
	db := newTestDB(t)
	const userID = "alice"

	ing, err := FindOrCreateIngredient(db, userID, "Eggs", "", "count")
	if err != nil {
		t.Fatalf("FindOrCreateIngredient: %s", err)
	}

	if _, err := AdjustStock(db, userID, ing.ID, 12, "count"); err != nil {
		t.Fatalf("AdjustStock +12: %s", err)
	}
	stock, err := AdjustStock(db, userID, ing.ID, -20, "count")
	if err != nil {
		t.Fatalf("AdjustStock -20: %s", err)
	}
	if stock.QuantityOnHand != 0 {
		t.Fatalf("expected quantity to clamp at 0, got %v", stock.QuantityOnHand)
	}
}

func TestSetStockOverwritesAbsolute(t *testing.T) {
	db := newTestDB(t)
	const userID = "alice"

	ing, err := FindOrCreateIngredient(db, userID, "Milk", "", "cup")
	if err != nil {
		t.Fatalf("FindOrCreateIngredient: %s", err)
	}
	if _, err := AdjustStock(db, userID, ing.ID, 5, "cup"); err != nil {
		t.Fatalf("AdjustStock: %s", err)
	}
	stock, err := SetStock(db, userID, ing.ID, 1.5, "cup")
	if err != nil {
		t.Fatalf("SetStock: %s", err)
	}
	if stock.QuantityOnHand != 1.5 {
		t.Fatalf("expected absolute quantity 1.5, got %v", stock.QuantityOnHand)
	}
}

func TestListLowStockOnlyReturnsBelowThreshold(t *testing.T) {
	db := newTestDB(t)
	const userID = "alice"

	low, err := FindOrCreateIngredient(db, userID, "Butter", "", "stick")
	if err != nil {
		t.Fatalf("FindOrCreateIngredient: %s", err)
	}
	high, err := FindOrCreateIngredient(db, userID, "Rice", "", "cup")
	if err != nil {
		t.Fatalf("FindOrCreateIngredient: %s", err)
	}

	if _, err := SetStock(db, userID, low.ID, 1, "stick"); err != nil {
		t.Fatalf("SetStock low: %s", err)
	}
	if err := SetLowStockThreshold(db, userID, low.ID, 2); err != nil {
		t.Fatalf("SetLowStockThreshold low: %s", err)
	}
	if _, err := SetStock(db, userID, high.ID, 10, "cup"); err != nil {
		t.Fatalf("SetStock high: %s", err)
	}
	if err := SetLowStockThreshold(db, userID, high.ID, 2); err != nil {
		t.Fatalf("SetLowStockThreshold high: %s", err)
	}

	lowStock, err := ListLowStock(db, userID)
	if err != nil {
		t.Fatalf("ListLowStock: %s", err)
	}
	if len(lowStock) != 1 || lowStock[0].ID != low.ID {
		t.Fatalf("expected only %q to be low stock, got %+v", low.Name, lowStock)
	}
}
