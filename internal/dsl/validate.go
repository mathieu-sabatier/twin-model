package dsl

import (
	"fmt"
	"regexp"
	"strings"
)

// Stable diagnostic codes. These are part of the tool's contract (CI greps them,
// docs list them) — keep the string values stable across releases.
const (
	CodeMissingName        = "missing-name"
	CodeMissingNamespace   = "missing-namespace"
	CodeNamespaceSlash     = "namespace-trailing-slash"
	CodeMissingVersion     = "missing-version"
	CodeVersionSemver      = "version-semver"
	CodeMissingPubDate     = "missing-publication-date"
	CodePubDateFormat      = "publication-date-format"
	CodeDuplicateType      = "duplicate-type"
	CodeUnknownBase        = "unknown-base"
	CodeInheritanceCycle   = "inheritance-cycle"
	CodeDuplicateMember    = "duplicate-member"
	CodeInvalidKind        = "invalid-kind"
	CodeInvalidRule        = "invalid-rule"
	CodeInvalidAccess      = "invalid-access"
	CodePlaceholderNoRule  = "placeholder-without-rule"
	CodeRuleNoPlaceholder  = "rule-without-placeholder"
	CodeUnitOnProperty     = "unit-on-property"
	CodeUnknownUnit        = "unknown-unit"
	CodeUnitOnNonNumeric   = "unit-on-non-numeric"
	CodeUnknownType        = "unknown-type"
	CodeMissingType        = "missing-type"
	CodeUnknownImportAlias = "unknown-import-alias"
	CodeAbstractInstance   = "abstract-instance"
	CodeDuplicateInstance  = "duplicate-instance"

	CodeUnitRequiresVariable = "unit-requires-variable"
	CodeEmptyEnum            = "empty-enum"
	CodeDuplicateEnumValue   = "duplicate-enum-value"
	CodeDuplicateEnumID      = "duplicate-enum-id"
	CodeNegativeEnumID       = "negative-enum-id"

	CodeUnknownValueMember   = "unknown-value-member"
	CodeValueNotValueBearing = "value-not-value-bearing"
	CodeUnknownPlaceholder   = "unknown-placeholder"
	CodeInstanceCycle        = "instance-cycle"
	CodeUnknownUnder         = "unknown-under"

	CodeUnknownImportType = "unknown-import-type"
	CodeImportNotBundled  = "import-not-bundled"

	CodeUnknownLevel           = "unknown-level"
	CodeLevelOnUnsupportedType = "level-on-unsupported-type"
	CodeHierarchyLevelOrder    = "hierarchy-level-order"
	CodeHierarchyLevelSkip     = "hierarchy-level-skip"
	CodeEquipmentParent        = "equipment-parent"
	CodeMachineUnderMachine    = "machine-under-machine"

	CodeDanglingMember             = "dangling-member"
	CodeExclusiveMembership        = "exclusive-membership"
	CodeUnknownMembershipMode      = "unknown-membership-mode"
	CodeUnknownPerspectiveNode     = "unknown-perspective-node"
	CodePerspectiveNodeCycle       = "perspective-node-cycle"
	CodePerspectiveIDNotExportable = "perspective-id-not-exportable"
)

// Severity distinguishes hard errors (fail CI) from warnings.
type Severity int

const (
	SeverityError Severity = iota
	SeverityWarning
)

func (s Severity) String() string {
	if s == SeverityWarning {
		return "warning"
	}
	return "error"
}

// Diagnostic is one lint finding with a stable code, a source position, and a
// structured Path (e.g. object_types/FurnaceType/members/Setpoint/access) the
// API/UI uses to anchor the error to a field.
type Diagnostic struct {
	Code     string
	Severity Severity
	File     string
	Line     int
	Col      int
	Path     string
	Message  string
}

func (d Diagnostic) String() string {
	return fmt.Sprintf("%s:%d: %s: [%s] %s", d.File, d.Line, d.Severity, d.Code, d.Message)
}

var (
	semverRe = regexp.MustCompile(`^\d+\.\d+\.\d+$`)
	dateRe   = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
)

// numericTypes are the built-in scalars a `unit:` may be attached to.
var numericTypes = map[string]bool{
	"SByte": true, "Byte": true, "Int16": true, "UInt16": true,
	"Int32": true, "UInt32": true, "Int64": true, "UInt64": true,
	"Float": true, "Double": true, "Number": true, "Integer": true, "UInteger": true,
}

// Validate runs every semantic lint rule and returns all diagnostics (errors and
// warnings), in a deterministic order. The CLI decides exit status from whether
// any error-severity diagnostic is present.
func Validate(m *Model) []Diagnostic {
	v := &validator{m: m}
	v.checkHeader()
	v.checkImports()
	v.checkDuplicateTypes()
	v.checkEnums()
	v.checkInheritance()
	for _, ot := range m.ObjectTypes {
		v.checkMembers(ot.Name, "object_types/"+ot.Name+"/members", ot.Members)
	}
	v.checkInstances()
	v.checkPerspectives()
	return v.diags
}

type validator struct {
	m     *Model
	diags []Diagnostic
}

func (v *validator) err(code, path string, pos Pos, format string, args ...any) {
	v.diags = append(v.diags, Diagnostic{
		Code:     code,
		Severity: SeverityError,
		File:     pos.File,
		Line:     pos.Line,
		Col:      pos.Col,
		Path:     path,
		Message:  fmt.Sprintf(format, args...),
	})
}

func (v *validator) checkHeader() {
	m := v.m
	if m.Name == "" {
		v.err(CodeMissingName, "model/name", m.Pos, "model.name is required")
	}
	switch {
	case m.Namespace == "":
		v.err(CodeMissingNamespace, "model/namespace", m.Pos, "model.namespace is required")
	case !strings.HasSuffix(m.Namespace, "/"):
		v.err(CodeNamespaceSlash, "model/namespace", m.Pos, "model.namespace %q must end with '/'", m.Namespace)
	}
	switch {
	case m.Version == "":
		v.err(CodeMissingVersion, "model/version", m.Pos, "model.version is required")
	case !semverRe.MatchString(m.Version):
		v.err(CodeVersionSemver, "model/version", m.Pos, "model.version %q must be semver MAJOR.MINOR.PATCH", m.Version)
	}
	switch {
	case m.PublicationDate == "":
		v.err(CodeMissingPubDate, "model/publication_date", m.Pos, "model.publication_date is required")
	case !dateRe.MatchString(m.PublicationDate):
		v.err(CodePubDateFormat, "model/publication_date", m.Pos, "model.publication_date %q must be YYYY-MM-DD", m.PublicationDate)
	}
}

// checkImports flags imported namespaces that are not bundled (so cannot be
// resolved or supplied to the ModelCompiler). Only runs when a catalog is
// attached; without one, imports keep their legacy trusted behavior.
func (v *validator) checkImports() {
	if v.m.Catalog == nil {
		return
	}
	for _, im := range v.m.Imports {
		if im.URI == OpcUaNamespaceURI || im.Alias == "OpcUa" {
			continue
		}
		if _, ok := v.m.Catalog.Namespace(im.URI); !ok {
			v.err(CodeImportNotBundled, "imports/"+im.Alias, im.Pos,
				"import %q (%s) has no bundled NodeSet2 — it cannot be resolved or compiled", im.Alias, im.URI)
		}
	}
}

// nsBundled reports whether an import's namespace is loaded in the catalog, used
// to suppress unknown-import-type noise when the whole spec is missing.
func (v *validator) nsBundled(uri string) bool {
	if v.m.Catalog == nil {
		return false
	}
	_, ok := v.m.Catalog.Namespace(uri)
	return ok
}

func (v *validator) checkDuplicateTypes() {
	seen := map[string]bool{}
	check := func(name, kind string, pos Pos) {
		if seen[name] {
			v.err(CodeDuplicateType, kind+"/"+name, pos, "duplicate type name %q", name)
			return
		}
		seen[name] = true
	}
	for _, e := range v.m.Enums {
		check(e.Name, "enums", e.Pos)
	}
	for _, ot := range v.m.ObjectTypes {
		check(ot.Name, "object_types", ot.Pos)
	}
}

func (v *validator) checkEnums() {
	for _, e := range v.m.Enums {
		base := "enums/" + e.Name
		if len(e.Values) == 0 {
			v.err(CodeEmptyEnum, base, e.Pos, "enum %q has no values", e.Name)
			continue
		}
		names := map[string]bool{}
		ids := map[int]bool{}
		for _, val := range e.Values {
			vp := base + "/values/" + val.Name
			if names[val.Name] {
				v.err(CodeDuplicateEnumValue, vp, val.Pos, "duplicate value name %q in enum %q", val.Name, e.Name)
			}
			names[val.Name] = true
			if val.Identifier < 0 {
				v.err(CodeNegativeEnumID, vp, val.Pos, "enum %q value %q has a negative id %d", e.Name, val.Name, val.Identifier)
			}
			if ids[val.Identifier] {
				v.err(CodeDuplicateEnumID, vp, val.Pos, "duplicate identifier %d in enum %q", val.Identifier, e.Name)
			}
			ids[val.Identifier] = true
		}
	}
}

func (v *validator) checkInheritance() {
	for _, ot := range v.m.ObjectTypes {
		seen := map[string]bool{ot.Name: true}
		cur := ot
		for !cur.Base.IsZero() {
			r := v.m.ResolveType(cur.Base)
			// A non-RefLocal base ends the walk (see the post-switch break);
			// each case only records diagnostics before that happens.
			switch r.Kind {
			case RefImport:
				// no catalog: trusted external base (legacy)
			case RefImportResolved:
				if ct, ok := v.m.CatalogType(r); ok && ct.NodeClass != "ObjectType" {
					v.err(CodeUnknownBase, "object_types/"+cur.Name+"/base", cur.Base.Pos,
						"base %q of %q is a %s, not an ObjectType", cur.Base.Raw, cur.Name, ct.NodeClass)
				}
			case RefImportUnknown:
				if v.nsBundled(r.URI) {
					v.err(CodeUnknownImportType, "object_types/"+cur.Name+"/base", cur.Base.Pos,
						"base %q of %q is not a type in %s", cur.Base.Raw, cur.Name, r.URI)
				}
			case RefUnknownAlias:
				v.err(CodeUnknownImportAlias, "object_types/"+cur.Name+"/base", cur.Base.Pos, "unknown import alias %q in base %q", cur.Base.Alias, cur.Base.Raw)
			case RefBuiltin, RefUnknownName:
				v.err(CodeUnknownBase, "object_types/"+cur.Name+"/base", cur.Base.Pos, "base type %q of %q is not a defined object type", cur.Base.Raw, cur.Name)
			}
			if r.Kind != RefLocal {
				break
			}
			next, ok := v.m.localObjectType(r.Name)
			if !ok {
				v.err(CodeUnknownBase, "object_types/"+cur.Name+"/base", cur.Base.Pos, "base type %q of %q is not a defined object type", cur.Base.Raw, cur.Name)
				break
			}
			if seen[next.Name] {
				v.err(CodeInheritanceCycle, "object_types/"+cur.Name+"/base", cur.Base.Pos, "inheritance cycle: %q ultimately extends itself via %q", ot.Name, next.Name)
				break
			}
			seen[next.Name] = true
			cur = next
		}
	}
}

func (v *validator) checkMembers(owner, membersPath string, members []*Member) {
	seen := map[string]bool{}
	for _, mem := range members {
		memPath := membersPath + "/" + mem.Name
		if seen[mem.Name] {
			v.err(CodeDuplicateMember, memPath, mem.Pos, "duplicate member %q in %q", mem.Name, owner)
		}
		seen[mem.Name] = true
		v.checkMember(owner, memPath, mem)
	}
}

func (v *validator) checkMember(owner, memPath string, mem *Member) {
	switch mem.Kind {
	case KindProperty, KindVariable, KindObject, KindMethod:
	default:
		v.err(CodeInvalidKind, memPath+"/kind", mem.Pos, "invalid kind %q (want property|variable|object|method)", mem.Kind)
	}
	switch mem.Rule {
	case RuleMandatory, RuleOptional, RuleOptionalPlaceholder, RuleMandatoryPlaceholder:
	default:
		v.err(CodeInvalidRule, memPath+"/rule", mem.Pos, "invalid rule %q", mem.Rule)
	}
	if mem.Kind == KindVariable || mem.Kind == KindProperty {
		switch mem.Access {
		case AccessRead, AccessReadWrite:
		default:
			v.err(CodeInvalidAccess, memPath+"/access", mem.Pos, "invalid access %q (want r|rw)", mem.Access)
		}
	}
	if mem.IsPlaceholder() && !mem.Rule.IsPlaceholder() {
		v.err(CodePlaceholderNoRule, memPath, mem.Pos, "member %q uses placeholder name syntax but rule %q is not a placeholder rule", mem.BrowseName, mem.Rule)
	}
	if !mem.IsPlaceholder() && mem.Rule.IsPlaceholder() {
		v.err(CodeRuleNoPlaceholder, memPath, mem.Pos, "member %q has placeholder rule %q but its name is not `Name<Suffix>`", mem.Name, mem.Rule)
	}
	if mem.Kind != KindMethod {
		if mem.Type.IsZero() {
			v.err(CodeMissingType, memPath+"/type", mem.Pos, "member %q has no type", mem.Name)
		} else {
			v.checkTypeRef(memPath+"/type", mem.Type)
		}
	}
	if mem.Unit != "" {
		switch mem.Kind {
		case KindVariable:
			if _, ok := LookupUnit(mem.Unit); !ok {
				v.err(CodeUnknownUnit, memPath+"/unit", mem.Pos, "unknown unit symbol %q", mem.Unit)
			}
			if !isNumericType(mem.Type) {
				v.err(CodeUnitOnNonNumeric, memPath+"/unit", mem.Pos, "unit %q on non-numeric type %q", mem.Unit, mem.Type.Raw)
			}
		case KindProperty:
			v.err(CodeUnitOnProperty, memPath+"/unit", mem.Pos, "unit %q on property %q: engineering units require kind: variable (AnalogUnitType is a VariableType)", mem.Unit, mem.Name)
		default:
			v.err(CodeUnitRequiresVariable, memPath+"/unit", mem.Pos, "unit %q on %s %q: engineering units require kind: variable", mem.Unit, mem.Kind, mem.Name)
		}
	}
	for i, a := range mem.In {
		v.checkTypeRef(fmt.Sprintf("%s/in/%d/type", memPath, i), a.Type)
	}
	for i, a := range mem.Out {
		v.checkTypeRef(fmt.Sprintf("%s/out/%d/type", memPath, i), a.Type)
	}
	if mem.Kind == KindObject {
		v.checkMembers(owner+"."+mem.Name, memPath+"/children", mem.Children)
	}
}

func (v *validator) checkInstances() {
	abstract := map[string]bool{}
	for _, ot := range v.m.ObjectTypes {
		if ot.Abstract {
			abstract[ot.Name] = true
		}
	}
	instByName := map[string]*Instance{}
	for _, inst := range v.m.Instances {
		instByName[inst.Name] = inst
	}
	seen := map[string]bool{}
	for _, inst := range v.m.Instances {
		base := "instances/" + inst.Name
		if seen[inst.Name] {
			v.err(CodeDuplicateInstance, base, inst.Pos, "duplicate instance name %q", inst.Name)
		}
		seen[inst.Name] = true

		v.checkTypeRef(base+"/type", inst.Type)
		if r := v.m.ResolveType(inst.Type); r.Kind == RefLocal && abstract[r.Name] {
			v.err(CodeAbstractInstance, base+"/type", inst.Pos, "instance %q uses abstract type %q", inst.Name, inst.Type.Raw)
		}
		if r := v.m.ResolveType(inst.Type); r.Kind == RefImportResolved {
			if ct, ok := v.m.CatalogType(r); ok && ct.Abstract {
				v.err(CodeAbstractInstance, base+"/type", inst.Pos, "instance %q uses abstract type %q", inst.Name, inst.Type.Raw)
			}
		}

		v.checkUnder(base, inst, instByName)
		v.checkInstanceMembers(base, inst)
		v.checkHierarchy(base, inst, instByName)
	}
	v.checkInstanceCycles(instByName)
}

// checkUnder accepts under: an import target (OpcUa:ObjectsFolder) or an
// unprefixed name matching a declared instance (nesting). A scalar DataType
// (RefBuiltin) or a type name (RefLocal) is not a placeable parent.
func (v *validator) checkUnder(base string, inst *Instance, instByName map[string]*Instance) {
	ref := inst.Under
	if ref.IsZero() {
		return
	}
	r := v.m.ResolveType(ref)
	switch r.Kind {
	case RefImport:
		return // import target, e.g. OpcUa:ObjectsFolder
	case RefImportResolved:
		return // a resolved imported object/folder is a valid parent
	case RefImportUnknown:
		// Known gap: under: targets are organizational/instance nodes (folders,
		// object instances) that LookupType cannot validate for ANY namespace —
		// they aren't types. A genuinely bogus ref (e.g. DI:NonexistentNode) is
		// silently unchecked here. Accepted trade-off; do not promote to an error.
		return
	case RefUnknownAlias:
		v.err(CodeUnknownImportAlias, base+"/under", ref.Pos, "unknown import alias %q in under %q", ref.Alias, ref.Raw)
	case RefUnknownName:
		if _, ok := instByName[ref.Name]; ok {
			return // nesting under a declared instance
		}
		v.err(CodeUnknownUnder, base+"/under", ref.Pos, "under %q is neither an import target nor a declared instance", ref.Raw)
	default: // RefBuiltin / RefLocal — a scalar DataType or a type name is not a placeable parent
		v.err(CodeUnknownUnder, base+"/under", ref.Pos, "under %q is neither an import target nor a declared instance", ref.Raw)
	}
}

// checkInstanceMembers validates values/children against the resolved type.
func (v *validator) checkInstanceMembers(base string, inst *Instance) {
	r := v.m.ResolveType(inst.Type)
	byName := map[string]*Member{}
	switch r.Kind {
	case RefLocal:
		resolved, err := v.m.ResolveMembers(r.Name)
		if err != nil {
			return
		}
		for _, rm := range resolved {
			byName[rm.Name] = rm.Member
		}
	case RefImportResolved:
		ct, ok := v.m.CatalogType(r)
		if !ok {
			return
		}
		for _, cm := range ct.Members {
			byName[cm.Name] = synthMember(cm)
		}
	default:
		return // opaque/unknown type
	}
	for _, val := range inst.Values {
		mem, ok := byName[val.Member]
		if !ok {
			v.err(CodeUnknownValueMember, base+"/values/"+val.Member, val.Pos, "value %q is not a member of type %q", val.Member, inst.Type.Raw)
			continue
		}
		if mem.Kind != KindProperty && mem.Kind != KindVariable {
			v.err(CodeValueNotValueBearing, base+"/values/"+val.Member, val.Pos, "member %q is a %s and cannot carry a value", val.Member, mem.Kind)
		}
	}
	for _, ch := range inst.Children {
		if !placeholderMatches(byName, ch.Of) {
			v.err(CodeUnknownPlaceholder, base+"/children/"+ch.Name, ch.Pos, "child %q references %q, which is not a placeholder of type %q", ch.Name, ch.Of.Raw, inst.Type.Raw)
		}
	}
}

// placeholderMatches reports whether `of` names a placeholder member (matched by
// its base name, e.g. of: "Zone<No>" -> member base name "Zone"). The placeholder
// may live on a nested object member, so search recursively.
func placeholderMatches(top map[string]*Member, of TypeRef) bool {
	base, _, ok := splitPlaceholder(of.Raw)
	if !ok {
		base = of.Raw
	}
	var search func(members map[string]*Member) bool
	search = func(members map[string]*Member) bool {
		for _, mem := range members {
			if mem.IsPlaceholder() && mem.Name == base {
				return true
			}
			if len(mem.Children) > 0 {
				child := map[string]*Member{}
				for _, c := range mem.Children {
					child[c.Name] = c
				}
				if search(child) {
					return true
				}
			}
		}
		return false
	}
	return search(top)
}

// orgTier returns an instance's organizational tier and whether it is an
// organizational node at all (carries a level mapped to an org tier 0-4).
// Equipment = no level, or a tier-less leaf level.
func (v *validator) orgTier(inst *Instance) (tier int, isOrg bool) {
	if inst == nil || inst.Level == "" {
		return 0, false
	}
	return ISA95LevelTier(inst.Level)
}

func (v *validator) checkHierarchy(base string, inst *Instance, instByName map[string]*Instance) {
	// 1. level value + supporting type.
	if inst.Level != "" {
		if _, ok := ISA95LevelValue(inst.Level); !ok {
			v.err(CodeUnknownLevel, base+"/level", inst.LevelPos, "unknown ISA-95 level %q", inst.Level)
		} else if !v.m.TypeHasMember(inst.Type, ISA95EquipmentLevelMember) {
			v.err(CodeLevelOnUnsupportedType, base+"/level", inst.LevelPos,
				"level %q set but type %q has no %s member", inst.Level, inst.Type.Raw, ISA95EquipmentLevelMember)
		}
	}

	// Ordering rules only apply when the parent is a declared instance.
	parent := instByName[inst.Under.Name]
	if parent == nil || inst.Under.Alias != "" {
		return // root (under an import target) or dangling (reported by checkUnder)
	}

	childTier, childIsOrg := v.orgTier(inst)
	parentTier, parentIsOrg := v.orgTier(parent)

	switch {
	case childIsOrg && parentIsOrg:
		if childTier <= parentTier {
			v.err(CodeHierarchyLevelOrder, base+"/under", inst.Pos,
				"level %q (tier %d) cannot nest under %q's level %q (tier %d)",
				inst.Level, childTier, parent.Name, parent.Level, parentTier)
		} else if childTier > parentTier+1 && !v.m.Hierarchy.AllowLevelSkip {
			v.err(CodeHierarchyLevelSkip, base+"/under", inst.Pos,
				"level %q skips a tier under %q's level %q; set hierarchy.allowLevelSkip to permit",
				inst.Level, parent.Name, parent.Level)
		}
	case !childIsOrg && parentIsOrg:
		// Equipment leaf under an org node: allowed only under WorkCenter-tier (3) or WorkUnit-tier (4).
		if parentTier < 3 {
			v.err(CodeEquipmentParent, base+"/under", inst.Pos,
				"equipment %q may only be parented under WorkCenter or WorkUnit, not %q (level %q)",
				inst.Name, parent.Name, parent.Level)
		}
	case !childIsOrg && !parentIsOrg:
		// Two non-org instances: only flag as machine-under-machine when the ISA-95
		// catalog is present (i.e. we are operating in an ISA-95 hierarchy context).
		if v.m.Catalog == nil {
			break
		}
		if _, ok := v.m.Catalog.Namespace(ISA95NamespaceURI); !ok {
			break
		}
		// Both are non-org equipment: flag the nesting.
		v.err(CodeMachineUnderMachine, base+"/under", inst.Pos,
			"equipment %q must not be parented under equipment %q; use the type's HasComponent structure",
			inst.Name, parent.Name)
	}
}

func (v *validator) checkInstanceCycles(instByName map[string]*Instance) {
	for _, inst := range v.m.Instances {
		seen := map[string]bool{inst.Name: true}
		cur := inst
		for {
			parent, ok := instByName[cur.Under.Name]
			if !ok || cur.Under.Alias != "" {
				break // parent is an import target or unknown — not a nesting cycle
			}
			if seen[parent.Name] {
				v.err(CodeInstanceCycle, "instances/"+inst.Name+"/under", inst.Pos, "instance nesting cycle: %q ultimately nests under itself", inst.Name)
				break
			}
			seen[parent.Name] = true
			cur = parent
		}
	}
}

func (v *validator) checkTypeRef(path string, ref TypeRef) {
	if ref.IsZero() {
		return
	}
	switch r := v.m.ResolveType(ref); r.Kind {
	case RefUnknownAlias:
		v.err(CodeUnknownImportAlias, path, ref.Pos, "unknown import alias %q in type %q", ref.Alias, ref.Raw)
	case RefUnknownName:
		v.err(CodeUnknownType, path, ref.Pos, "unknown type %q", ref.Raw)
	case RefImportUnknown:
		if v.nsBundled(r.URI) {
			v.err(CodeUnknownImportType, path, ref.Pos, "type %q is not defined in %s", ref.Raw, r.URI)
		}
	}
}

func isNumericType(ref TypeRef) bool {
	return ref.Alias == "" && numericTypes[ref.Name]
}
