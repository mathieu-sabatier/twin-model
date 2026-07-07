package dsl

import "testing"

func TestISA95LevelValue(t *testing.T) {
	cases := map[string]int{"Enterprise": 0, "Site": 1, "Area": 2, "WorkCenter": 10, "WorkUnit": 11, "EquipmentModule": 12}
	for name, want := range cases {
		if got, ok := ISA95LevelValue(name); !ok || got != want {
			t.Errorf("ISA95LevelValue(%q) = %d,%v; want %d,true", name, got, ok, want)
		}
	}
	if _, ok := ISA95LevelValue("Nope"); ok {
		t.Errorf("ISA95LevelValue(Nope) should be !ok")
	}
}

func TestISA95LevelTier(t *testing.T) {
	orgTiers := map[string]int{"Enterprise": 0, "Site": 1, "Area": 2, "WorkCenter": 3, "ProductionLine": 3, "WorkUnit": 4, "ProductionUnit": 4}
	for name, want := range orgTiers {
		if got, ok := ISA95LevelTier(name); !ok || got != want {
			t.Errorf("ISA95LevelTier(%q) = %d,%v; want %d,true", name, got, ok, want)
		}
	}
	for _, leaf := range []string{"EquipmentModule", "ControlModule", "Other"} {
		if _, ok := ISA95LevelTier(leaf); ok {
			t.Errorf("ISA95LevelTier(%q) should be tier-less (!ok)", leaf)
		}
	}
}
