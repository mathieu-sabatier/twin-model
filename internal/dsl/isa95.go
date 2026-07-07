package dsl

// ISA-95 equipment-level model. Single source of truth for both validation
// (internal/dsl) and export (internal/modeldesign). Values mirror the
// ISA95EquipmentElementLevelEnum in Opc.ISA95.NodeSet2.xml.

const (
	ISA95NamespaceURI         = "http://www.OPCFoundation.org/UA/2013/01/ISA95"
	ISA95EquipmentLevelEnum   = "ISA95EquipmentElementLevelEnum"
	ISA95EquipmentLevelMember = "EquipmentLevel"
)

// isa95Level carries both facts about one enum name: its integer enum value and,
// for organizational levels, the tier (0-4) it occupies in the hierarchy. One row
// per level keeps value and tier from drifting apart (single source of truth).
type isa95Level struct {
	value int
	tier  int
	org   bool // participates in org-tier ordering (Enterprise..WorkUnit)
}

// isa95Levels mirrors ISA95EquipmentElementLevelEnum in Opc.ISA95.NodeSet2.xml.
var isa95Levels = map[string]isa95Level{
	"Enterprise":      {value: 0, tier: 0, org: true},
	"Site":            {value: 1, tier: 1, org: true},
	"Area":            {value: 2, tier: 2, org: true},
	"ProcessCell":     {value: 3, tier: 3, org: true},
	"Unit":            {value: 4, tier: 4, org: true},
	"ProductionLine":  {value: 5, tier: 3, org: true},
	"WorkCell":        {value: 6, tier: 4, org: true},
	"ProductionUnit":  {value: 7, tier: 4, org: true},
	"StorageZone":     {value: 8, tier: 3, org: true},
	"StorageUnit":     {value: 9, tier: 4, org: true},
	"WorkCenter":      {value: 10, tier: 3, org: true},
	"WorkUnit":        {value: 11, tier: 4, org: true},
	"EquipmentModule": {value: 12}, // tier-less leaf equipment level
	"ControlModule":   {value: 13},
	"Other":           {value: 14},
}

// ISA95LevelValue returns the enum integer value for a level name.
func ISA95LevelValue(name string) (int, bool) {
	l, ok := isa95Levels[name]
	return l.value, ok
}

// ISA95LevelTier returns the organizational tier (0-4) for a level name, or
// ok=false for tier-less (leaf/equipment) or unknown names.
func ISA95LevelTier(name string) (int, bool) {
	l, ok := isa95Levels[name]
	if !ok || !l.org {
		return 0, false
	}
	return l.tier, true
}
