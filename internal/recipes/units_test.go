package recipes

import (
	"math"
	"testing"
)

func TestConvertQuantity(t *testing.T) {
	cases := []struct {
		q        float64
		from, to string
		want     float64
		ok       bool
	}{
		{1, "cup", "tbsp", 16, true},
		{3, "tsp", "tbsp", 1, true},
		{2, "cups", "cup", 2, true},
		{1, "lb", "oz", 16, true},
		{1, "kg", "g", 1000, true},
		{500, "g", "lb", 1.1023, true},
		{1, "T", "t", 3, true}, // tablespoon -> teaspoon
		{2, "", "count", 2, true},
		{3, "cloves", "cloves", 3, true}, // unknown but identical
		{1, "Clove", "clove", 1, true},
		{2, "cup", "g", 0, false}, // volume vs weight
		{1, "clove", "g", 0, false},
		{1, "can", "cup", 0, false},
	}
	for _, c := range cases {
		got, ok := convertQuantity(c.q, c.from, c.to)
		if ok != c.ok || (ok && math.Abs(got-c.want) > 0.001) {
			t.Errorf("convert(%v %q -> %q) = %v, %v; want %v, %v", c.q, c.from, c.to, got, ok, c.want, c.ok)
		}
	}
}
