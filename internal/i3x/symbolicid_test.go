package i3x

import "testing"

func TestSymbolicID(t *testing.T) {
	tests := []struct {
		name  string
		parts []string
		want  string
	}{
		{"root only", []string{"FurnaceType"}, "FurnaceType"},
		{"one member", []string{"FurnaceType", "Zones"}, "FurnaceType_Zones"},
		{"placeholder base name", []string{"FurnaceType", "Zones", "Zone"}, "FurnaceType_Zones_Zone"},
		{"deep member", []string{"FurnaceType", "Zones", "Zone", "Temperature"}, "FurnaceType_Zones_Zone_Temperature"},
		{"instance composed", []string{"Furnace01", "Zones"}, "Furnace01_Zones"},
		{"inherited materialized", []string{"Furnace01", "State"}, "Furnace01_State"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := symbolicID(tt.parts...); got != tt.want {
				t.Errorf("symbolicID(%v) = %q, want %q", tt.parts, got, tt.want)
			}
		})
	}
}

func TestElementID(t *testing.T) {
	got := elementID("https://acme.example/UA/Equipment/", "FurnaceType_Zones_Zone_Temperature")
	want := "nsu=https://acme.example/UA/Equipment/;s=FurnaceType_Zones_Zone_Temperature"
	if got != want {
		t.Errorf("elementID = %q, want %q", got, want)
	}
}

func TestNS0ElementID(t *testing.T) {
	got := ns0("BaseObjectType")
	want := "nsu=http://opcfoundation.org/UA/;s=BaseObjectType"
	if got != want {
		t.Errorf("ns0 = %q, want %q", got, want)
	}
}
