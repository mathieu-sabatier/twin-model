package dsl

import "testing"

func TestLookupUnit(t *testing.T) {
	cases := []struct {
		symbol      string
		code        string
		unitID      int32
		displayName string
		description string
	}{
		{"°C", "CEL", 4408652, "°C", "degree Celsius"},
		{"%", "P1", 20529, "%", "percent"},
		{"kN", "B47", 4338743, "kN", "kilonewton"},
		{"mm", "MMT", 5066068, "mm", "millimetre"},
		{"ml", "MLT", 5065812, "ml", "millilitre"},
		{"l", "LTR", 5002322, "l", "litre"},
		{"kWh", "KWH", 4937544, "kW·h", "kilowatt hour"}, // ergonomic alias for "kW·h"
		// CSV-native symbols the old hand table never had — guard the embedded
		// UNECE_to_OPCUA.csv is actually loaded (Regression: /qa 2026-07-05).
		{"g", "GRM", 4674125, "g", "gram"},
		{"Hz", "HTZ", 4740186, "Hz", "hertz"},
		{"V", "VLT", 5655636, "V", "volt"},
	}
	for _, c := range cases {
		u, ok := LookupUnit(c.symbol)
		if !ok {
			t.Fatalf("LookupUnit(%q): want found, got not found", c.symbol)
		}
		if u.Code != c.code || u.UnitID != c.unitID || u.DisplayName != c.displayName || u.Description != c.description {
			t.Errorf("LookupUnit(%q) = %+v; want code=%s id=%d display=%s desc=%s",
				c.symbol, u, c.code, c.unitID, c.displayName, c.description)
		}
	}
}

func TestLookupUnitUnknown(t *testing.T) {
	if u, ok := LookupUnit("furlong"); ok {
		t.Errorf("LookupUnit(furlong): want not found, got %+v", u)
	}
}
