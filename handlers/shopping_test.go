package handlers

import (
	"testing"
	"time"
)

func TestShoppingRange(t *testing.T) {
	thu := time.Date(2026, 11, 26, 15, 0, 0, 0, time.UTC) // a Thursday
	from, to, err := shoppingRange("", "", thu)
	if err != nil || from.Format("2006-01-02") != "2026-11-26" || to.Format("2006-01-02") != "2026-11-29" {
		t.Fatalf("default = %s..%s %v, want today..Sunday", from, to, err)
	}
	sun := time.Date(2026, 11, 29, 9, 0, 0, 0, time.UTC)
	if _, to, _ := shoppingRange("", "", sun); to.Format("2006-01-02") != "2026-11-29" {
		t.Fatalf("on Sunday the range should end the same day, got %s", to)
	}
	if _, _, err := shoppingRange("2026-11-30", "2026-11-29", thu); err == nil {
		t.Fatal("expected error for to < from")
	}
	if _, _, err := shoppingRange("nope", "", thu); err == nil {
		t.Fatal("expected error for bad date")
	}
}
