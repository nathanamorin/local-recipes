package recipes

import "strings"

// Unit handling for comparing recipe quantities against pantry stock.
// Only conversions that are exact are supported: volume<->volume,
// weight<->weight and count<->count. Volume<->weight (cups of flour vs
// grams) depends on the ingredient's density, so those are reported as
// not convertible rather than guessed.

type dimension int

const (
	dimUnknown dimension = iota
	dimCount
	dimVolume // base: millilitres
	dimWeight // base: grams
)

type unitInfo struct {
	dim    dimension
	toBase float64
	name   string // canonical display name
}

var units = map[string]unitInfo{}

func registerUnit(info unitInfo, aliases ...string) {
	for _, a := range aliases {
		units[a] = info
	}
}

func init() {
	registerUnit(unitInfo{dimCount, 1, ""}, "", "count", "counts", "whole", "each", "ea", "piece", "pieces", "pc", "pcs", "item", "items")
	registerUnit(unitInfo{dimVolume, 4.92892, "tsp"}, "tsp", "tsps", "teaspoon", "teaspoons", "t")
	registerUnit(unitInfo{dimVolume, 14.7868, "tbsp"}, "tbsp", "tbsps", "tbs", "tablespoon", "tablespoons", "T")
	registerUnit(unitInfo{dimVolume, 29.5735, "fl oz"}, "fl oz", "floz", "fluid ounce", "fluid ounces")
	registerUnit(unitInfo{dimVolume, 236.588, "cup"}, "cup", "cups", "c")
	registerUnit(unitInfo{dimVolume, 473.176, "pint"}, "pint", "pints", "pt")
	registerUnit(unitInfo{dimVolume, 946.353, "quart"}, "quart", "quarts", "qt")
	registerUnit(unitInfo{dimVolume, 3785.41, "gallon"}, "gallon", "gallons", "gal")
	registerUnit(unitInfo{dimVolume, 1, "ml"}, "ml", "mls", "milliliter", "milliliters", "millilitre", "millilitres")
	registerUnit(unitInfo{dimVolume, 1000, "l"}, "l", "liter", "liters", "litre", "litres")
	registerUnit(unitInfo{dimWeight, 1, "g"}, "g", "gram", "grams", "gr")
	registerUnit(unitInfo{dimWeight, 1000, "kg"}, "kg", "kgs", "kilogram", "kilograms")
	registerUnit(unitInfo{dimWeight, 28.3495, "oz"}, "oz", "ounce", "ounces")
	registerUnit(unitInfo{dimWeight, 453.592, "lb"}, "lb", "lbs", "pound", "pounds")
}

func lookupUnit(u string) (unitInfo, bool) {
	trimmed := strings.TrimSuffix(strings.TrimSpace(u), ".")
	// "T" (tablespoon) vs "t" (teaspoon) is the one case-sensitive alias.
	if info, ok := units[trimmed]; ok && (trimmed == "T" || trimmed == "t") {
		return info, true
	}
	info, ok := units[strings.ToLower(trimmed)]
	return info, ok
}

// sameUnit reports whether a and b name the same unit ("cups" == "cup").
// Unknown units (e.g. "clove", "can") compare by normalized spelling.
func sameUnit(a, b string) bool {
	ia, oka := lookupUnit(a)
	ib, okb := lookupUnit(b)
	if oka && okb {
		return ia == ib
	}
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

// convertQuantity converts q from unit `from` to unit `to`. ok is false
// when the units measure different things or either is unknown (unless
// they're spelled the same, in which case q is returned unchanged).
func convertQuantity(q float64, from, to string) (float64, bool) {
	if sameUnit(from, to) {
		return q, true
	}
	fi, okf := lookupUnit(from)
	ti, okt := lookupUnit(to)
	if !okf || !okt || fi.dim != ti.dim || fi.dim == dimUnknown {
		return 0, false
	}
	return q * fi.toBase / ti.toBase, true
}
