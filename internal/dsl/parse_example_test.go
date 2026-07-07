package dsl

import (
	"os"
	"testing"
)

func TestParseEnums(t *testing.T) {
	src := `
model: { name: M, namespace: https://x/, version: 1.0.0, publication_date: 2026-07-02 }
enums:
  EquipmentState:
    doc: Operational state common to all equipment
    values: [Idle, Running, Paused, Fault, Maintenance]
  Mode:
    values:
      - Off
      - Manual: 5
      - Auto
`
	m, err := Parse("f.yaml", []byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(m.Enums) != 2 {
		t.Fatalf("got %d enums, want 2", len(m.Enums))
	}
	es := m.Enums[0]
	if es.Name != "EquipmentState" || es.Doc != "Operational state common to all equipment" {
		t.Errorf("enum0 = %+v", es)
	}
	wantVals := []struct {
		name string
		id   int
	}{{"Idle", 0}, {"Running", 1}, {"Paused", 2}, {"Fault", 3}, {"Maintenance", 4}}
	if len(es.Values) != len(wantVals) {
		t.Fatalf("EquipmentState has %d values, want %d", len(es.Values), len(wantVals))
	}
	for i, w := range wantVals {
		if es.Values[i].Name != w.name || es.Values[i].Identifier != w.id {
			t.Errorf("value[%d] = %+v, want %s=%d", i, es.Values[i], w.name, w.id)
		}
	}
	// explicit id form: positional index unless overridden
	mode := m.Enums[1]
	got := map[string]int{}
	for _, v := range mode.Values {
		got[v.Name] = v.Identifier
	}
	if got["Off"] != 0 || got["Manual"] != 5 || got["Auto"] != 2 {
		t.Errorf("Mode ids = %v, want Off=0 Manual=5 Auto=2", got)
	}
}

// TestParseEquipmentExample parses the canonical example end to end and checks
// the AST structure the emitter will consume.
func TestParseEquipmentExample(t *testing.T) {
	data, err := os.ReadFile("../../examples/equipment.yaml")
	if err != nil {
		t.Fatalf("read example: %v", err)
	}
	m, err := Parse("examples/equipment.yaml", data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if m.Name != "AcmeEquipment" || m.Namespace != "https://acme.example/UA/Equipment/" {
		t.Errorf("header = %+v", m)
	}
	if len(m.Enums) != 1 || len(m.ObjectTypes) != 4 || len(m.Instances) != 3 {
		t.Fatalf("counts: enums=%d types=%d instances=%d", len(m.Enums), len(m.ObjectTypes), len(m.Instances))
	}

	byName := map[string]*ObjectType{}
	for _, ot := range m.ObjectTypes {
		byName[ot.Name] = ot
	}

	eq := byName["EquipmentType"]
	if eq == nil || !eq.Abstract || eq.Base.Alias != "OpcUa" || eq.Base.Name != "BaseObjectType" {
		t.Fatalf("EquipmentType = %+v", eq)
	}
	if len(eq.Members) != 4 {
		t.Fatalf("EquipmentType members = %d, want 4", len(eq.Members))
	}
	// defaults: first member is a property (explicit), State defaults to variable/mandatory/r
	state := memberByName(eq, "State")
	if state == nil || state.Kind != KindVariable || state.Rule != RuleMandatory || state.Access != AccessRead {
		t.Errorf("State = %+v", state)
	}
	if cc := memberByName(eq, "CycleCount"); cc == nil || cc.Rule != RuleOptional {
		t.Errorf("CycleCount = %+v", cc)
	}
	if mf := memberByName(eq, "Manufacturer"); mf == nil || mf.Kind != KindProperty {
		t.Errorf("Manufacturer = %+v", mf)
	}

	// units
	zone := byName["HeatingZoneType"]
	temp := memberByName(zone, "Temperature")
	if temp == nil || temp.Unit != "°C" {
		t.Errorf("Temperature = %+v", temp)
	}
	if sp := memberByName(zone, "Setpoint"); sp == nil || sp.Access != AccessReadWrite {
		t.Errorf("Setpoint = %+v", sp)
	}

	// object + placeholder + method
	furnace := byName["FurnaceType"]
	if furnace.Base.Name != "EquipmentType" || furnace.Base.Alias != "" {
		t.Errorf("FurnaceType base = %+v", furnace.Base)
	}
	zones := memberByName(furnace, "Zones")
	if zones == nil || zones.Kind != KindObject || zones.Type.Alias != "OpcUa" || zones.Type.Name != "FolderType" {
		t.Fatalf("Zones = %+v", zones)
	}
	if len(zones.Children) != 1 {
		t.Fatalf("Zones children = %d, want 1", len(zones.Children))
	}
	ph := zones.Children[0]
	if ph.Name != "Zone" || ph.BrowseName != "<ZoneNo>" || ph.Rule != RuleOptionalPlaceholder || !ph.IsPlaceholder() {
		t.Errorf("placeholder = %+v", ph)
	}
	sp := memberByName(furnace, "StartProgram")
	if sp == nil || sp.Kind != KindMethod || len(sp.In) != 1 || len(sp.Out) != 1 {
		t.Fatalf("StartProgram = %+v", sp)
	}
	if sp.In[0].Name != "ProgramId" || sp.In[0].Type.Name != "String" {
		t.Errorf("StartProgram in = %+v", sp.In[0])
	}
	if sp.Out[0].Name != "Accepted" || sp.Out[0].Type.Name != "Boolean" {
		t.Errorf("StartProgram out = %+v", sp.Out[0])
	}

	// MaxForce is now a variable (option B), with a unit
	press := byName["PressType"]
	mfv := memberByName(press, "MaxForce")
	if mfv == nil || mfv.Kind != KindVariable || mfv.Unit != "kN" {
		t.Errorf("MaxForce = %+v", mfv)
	}
	if es := memberByName(press, "EmergencyStop"); es == nil || es.Kind != KindMethod {
		t.Errorf("EmergencyStop = %+v", es)
	}

	// instances
	f01 := m.Instances[0]
	if f01.Name != "Furnace01" || f01.Type.Name != "FurnaceType" || f01.Under.Alias != "OpcUa" || f01.Under.Name != "ObjectsFolder" {
		t.Errorf("Furnace01 = %+v", f01)
	}
	// Furnace02 exercises instance overrides: values (property + variable, incl. enum)
	// and two instantiated Zone placeholders.
	f02 := m.Instances[2]
	if f02.Name != "Furnace02" || len(f02.Values) != 5 || len(f02.Children) != 2 {
		t.Errorf("Furnace02 = %q values=%d children=%d", f02.Name, len(f02.Values), len(f02.Children))
	}
	if f02.Children[0].Name != "Zone1" || f02.Children[0].Of.Raw != "Zone<No>" {
		t.Errorf("Furnace02 child0 = %+v", f02.Children[0])
	}
}

func memberByName(ot *ObjectType, name string) *Member {
	for _, m := range ot.Members {
		if m.Name == name {
			return m
		}
	}
	return nil
}
