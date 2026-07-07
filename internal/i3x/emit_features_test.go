package i3x

import (
	"encoding/json"
	"strings"
	"testing"
)

// emitDocs is a decoded view of a full Emit run, for semantic assertions that
// are independent of exact field order (the golden test locks the bytes).
type emitDocs struct {
	objectTypes []map[string]any
	objects     []map[string]any
}

func emitExample(t *testing.T) emitDocs {
	t.Helper()
	m := loadExampleModel(t)
	b, err := Emit(m)
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	var d emitDocs
	mustUnmarshal(t, b.File("objecttypes.json"), &d.objectTypes)
	mustUnmarshal(t, b.File("objects.json"), &d.objects)
	return d
}

func mustUnmarshal(t *testing.T, data []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
}

// objectType returns the decoded objecttype with the given displayName.
func (d emitDocs) objectType(name string) map[string]any {
	for _, ot := range d.objectTypes {
		if ot["displayName"] == name {
			return ot
		}
	}
	return nil
}

// schemaOf returns a type's schema and its properties map.
func (d emitDocs) schemaOf(t *testing.T, name string) (schema, props map[string]any) {
	t.Helper()
	ot := d.objectType(name)
	if ot == nil {
		t.Fatalf("objecttype %q not found", name)
	}
	schema = ot["schema"].(map[string]any)
	if p, ok := schema["properties"].(map[string]any); ok {
		props = p
	}
	return schema, props
}

func TestSourceTypeIDChain(t *testing.T) {
	d := emitExample(t)
	cases := map[string]string{
		"EquipmentType":   "nsu=http://opcfoundation.org/UA/;s=BaseObjectType",
		"HeatingZoneType": "nsu=http://opcfoundation.org/UA/;s=BaseObjectType",
		"FurnaceType":     "nsu=https://acme.example/UA/Equipment/;s=EquipmentType",
		"PressType":       "nsu=https://acme.example/UA/Equipment/;s=EquipmentType",
	}
	for name, want := range cases {
		if got := d.objectType(name)["sourceTypeId"]; got != want {
			t.Errorf("%s sourceTypeId = %v, want %v", name, got, want)
		}
	}
}

func TestOwnMembersOnly(t *testing.T) {
	d := emitExample(t)
	_, props := d.schemaOf(t, "FurnaceType")
	// Inherited members (Manufacturer, State) must NOT be re-listed on the type;
	// they come via sourceTypeId.
	for _, inherited := range []string{"Manufacturer", "SerialNumber", "State", "CycleCount"} {
		if _, ok := props[inherited]; ok {
			t.Errorf("FurnaceType should not re-list inherited member %q", inherited)
		}
	}
	if _, ok := props["DoorClosed"]; !ok {
		t.Error("FurnaceType missing own member DoorClosed")
	}
}

func TestEnumDefAndRef(t *testing.T) {
	d := emitExample(t)
	schema, props := d.schemaOf(t, "EquipmentType")

	state := props["State"].(map[string]any)
	if state["$ref"] != "#/$defs/EquipmentState" {
		t.Errorf("State $ref = %v", state["$ref"])
	}

	defs, ok := schema["$defs"].(map[string]any)
	if !ok {
		t.Fatal("EquipmentType schema missing $defs")
	}
	es := defs["EquipmentState"].(map[string]any)
	if es["type"] != "integer" {
		t.Errorf("enum type = %v", es["type"])
	}
	enum := es["enum"].([]any)
	if len(enum) != 5 || enum[0].(float64) != 0 || enum[4].(float64) != 4 {
		t.Errorf("enum ids = %v", enum)
	}
	names := es["x-enumNames"].([]any)
	if names[0] != "Idle" || names[3] != "Fault" {
		t.Errorf("x-enumNames = %v", names)
	}

	// $defs is emitted only where an own member uses the enum: FurnaceType inherits
	// State (own-members-only) so must NOT carry $defs.
	fs, _ := d.schemaOf(t, "FurnaceType")
	if _, ok := fs["$defs"]; ok {
		t.Error("FurnaceType should not carry $defs for an inherited enum member")
	}
}

func TestUnitEngineeringUnits(t *testing.T) {
	d := emitExample(t)
	_, props := d.schemaOf(t, "HeatingZoneType")
	temp := props["Temperature"].(map[string]any)
	eu, ok := temp["engineeringUnits"].(map[string]any)
	if !ok {
		t.Fatal("Temperature missing engineeringUnits")
	}
	if eu["unitId"].(float64) != 4408652 {
		t.Errorf("unitId = %v", eu["unitId"])
	}
	if eu["displayName"] != "°C" {
		t.Errorf("displayName = %v", eu["displayName"])
	}
	if eu["namespaceUri"] != "http://www.opcfoundation.org/UA/units/un/cefact" {
		t.Errorf("namespaceUri = %v", eu["namespaceUri"])
	}
}

func TestOptionalNullableNotRequired(t *testing.T) {
	d := emitExample(t)
	schema, props := d.schemaOf(t, "EquipmentType")
	cc := props["CycleCount"].(map[string]any)
	typ := cc["type"].([]any)
	if len(typ) != 2 || typ[0] != "integer" || typ[1] != "null" {
		t.Errorf("CycleCount type = %v, want [integer null]", typ)
	}
	for _, r := range schema["required"].([]any) {
		if r == "CycleCount" {
			t.Error("optional CycleCount must not be in required")
		}
	}
}

func TestAccessReadOnly(t *testing.T) {
	d := emitExample(t)
	_, props := d.schemaOf(t, "HeatingZoneType")
	if props["Temperature"].(map[string]any)["readOnly"] != true {
		t.Error("Temperature (access r) should be readOnly:true")
	}
	if props["Setpoint"].(map[string]any)["readOnly"] != false {
		t.Error("Setpoint (access rw) should be readOnly:false")
	}
}

func TestCompositionRef(t *testing.T) {
	d := emitExample(t)
	_, props := d.schemaOf(t, "FurnaceType")
	zones := props["Zones"].(map[string]any)

	xo := zones["x-opcua"].(map[string]any)
	if xo["nodeClass"] != "Object" || xo["composition"] != true {
		t.Errorf("Zones x-opcua = %v", xo)
	}
	if xo["typeDefinition"] != "nsu=http://opcfoundation.org/UA/;s=FolderType" {
		t.Errorf("Zones typeDefinition = %v", xo["typeDefinition"])
	}
	ap := zones["additionalProperties"].(map[string]any)
	if ap["$ref"] != "nsu=https://acme.example/UA/Equipment/;s=HeatingZoneType" {
		t.Errorf("Zones additionalProperties $ref = %v", ap["$ref"])
	}
}

func TestPlaceholderAnnotation(t *testing.T) {
	d := emitExample(t)
	_, props := d.schemaOf(t, "FurnaceType")
	ph := props["Zones"].(map[string]any)["x-opcua-placeholder"].(map[string]any)
	if ph["symbolicName"] != "Zone" {
		t.Errorf("symbolicName = %v", ph["symbolicName"])
	}
	if ph["browseNamePattern"] != "<ZoneNo>" {
		t.Errorf("browseNamePattern = %v", ph["browseNamePattern"])
	}
	if ph["rule"] != "OptionalPlaceholder" {
		t.Errorf("rule = %v", ph["rule"])
	}
}

func TestMethodPreserved(t *testing.T) {
	d := emitExample(t)
	schema, _ := d.schemaOf(t, "FurnaceType")
	methods := schema["x-opcuaMethods"].([]any)
	sp := methods[0].(map[string]any)
	if sp["name"] != "StartProgram" || sp["description"] != "Start a named heating program" {
		t.Errorf("StartProgram = %v", sp)
	}
	in := sp["in"].([]any)[0].(map[string]any)
	if in["name"] != "ProgramId" || in["type"] != "String" {
		t.Errorf("StartProgram in = %v", in)
	}
	out := sp["out"].([]any)[0].(map[string]any)
	if out["name"] != "Accepted" || out["type"] != "Boolean" {
		t.Errorf("StartProgram out = %v", out)
	}

	// A no-argument method still appears (nothing silently dropped).
	ps, _ := d.schemaOf(t, "PressType")
	em := ps["x-opcuaMethods"].([]any)[0].(map[string]any)
	if em["name"] != "EmergencyStop" {
		t.Errorf("EmergencyStop = %v", em)
	}
	if _, ok := em["in"]; ok {
		t.Error("EmergencyStop should not carry an empty in")
	}
}

func TestInstanceTopology(t *testing.T) {
	d := emitExample(t)
	find := func(elementID string) map[string]any {
		for _, o := range d.objects {
			if o["elementId"] == elementID {
				return o
			}
		}
		return nil
	}

	f01 := find("nsu=https://acme.example/UA/Equipment/;s=Furnace01")
	if f01["typeElementId"] != "nsu=https://acme.example/UA/Equipment/;s=FurnaceType" {
		t.Errorf("Furnace01 typeElementId = %v", f01["typeElementId"])
	}
	if f01["parentId"] != "nsu=http://opcfoundation.org/UA/;s=ObjectsFolder" {
		t.Errorf("Furnace01 parentId = %v", f01["parentId"])
	}
	if f01["isComposition"] != false {
		t.Errorf("Furnace01 isComposition = %v", f01["isComposition"])
	}

	zones := find("nsu=https://acme.example/UA/Equipment/;s=Furnace01_Zones")
	if zones["isComposition"] != true {
		t.Errorf("Furnace01_Zones isComposition = %v", zones["isComposition"])
	}
	if zones["parentId"] != "nsu=https://acme.example/UA/Equipment/;s=Furnace01" {
		t.Errorf("Furnace01_Zones parentId = %v", zones["parentId"])
	}
	if zones["typeElementId"] != "nsu=http://opcfoundation.org/UA/;s=FolderType" {
		t.Errorf("Furnace01_Zones typeElementId = %v", zones["typeElementId"])
	}

	// Instantiated placeholder child materializes as a composed object.
	z1 := find("nsu=https://acme.example/UA/Equipment/;s=Furnace02_Zone1")
	if z1 == nil || z1["typeElementId"] != "nsu=https://acme.example/UA/Equipment/;s=HeatingZoneType" {
		t.Errorf("Furnace02_Zone1 = %v", z1)
	}
}

// TestSchemasAreValidJSONWithResolvableRefs asserts each schema parses and that
// every $ref points at an emitted objecttype elementId or a local #/$defs entry.
func TestSchemasAreValidJSONWithResolvableRefs(t *testing.T) {
	d := emitExample(t)

	emittedIDs := map[string]bool{}
	for _, ot := range d.objectTypes {
		emittedIDs[ot["elementId"].(string)] = true
	}

	for _, ot := range d.objectTypes {
		schema := ot["schema"].(map[string]any)
		defs, _ := schema["$defs"].(map[string]any)
		walkRefs(schema, func(ref string) {
			switch {
			case strings.HasPrefix(ref, "#/$defs/"):
				name := strings.TrimPrefix(ref, "#/$defs/")
				if _, ok := defs[name]; !ok {
					t.Errorf("%s: local $ref %q has no $defs entry", ot["displayName"], ref)
				}
			default:
				if !emittedIDs[ref] {
					t.Errorf("%s: $ref %q does not resolve to an emitted objecttype", ot["displayName"], ref)
				}
			}
		})
	}
}

// walkRefs invokes fn for every "$ref" string value anywhere in v.
func walkRefs(v any, fn func(string)) {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			if k == "$ref" {
				if s, ok := val.(string); ok {
					fn(s)
				}
				continue
			}
			walkRefs(val, fn)
		}
	case []any:
		for _, e := range t {
			walkRefs(e, fn)
		}
	}
}
