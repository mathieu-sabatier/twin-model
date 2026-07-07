package dsl

import "sort"

// Units returns the known engineering units sorted by symbol, for the UI's unit
// picker. The underlying map is the single source of truth (see units.go).
func Units() []Unit {
	out := make([]Unit, 0, len(units))
	for _, u := range units {
		out = append(out, u)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Symbol < out[j].Symbol })
	return out
}
