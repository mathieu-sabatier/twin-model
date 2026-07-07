// Package dsl parses and validates the twinmodel YAML DSL into a typed, XML-free
// AST. The AST is the stable seam a future HTTP API / UI can reuse: it carries
// modelling semantics (kinds, rules, units, type references) and source
// positions for diagnostics, but knows nothing about ModelDesign XML. Rendering
// the AST to XML is the job of internal/modeldesign.
package dsl

import "fmt"

// Pos is a source position: the file plus 1-based line and column of the node's
// defining token. Every AST node embeds a Pos, so node.File/node.Line/node.Col
// read through. It is the anchor the API/UI uses to attach diagnostics to fields.
type Pos struct {
	File string
	Line int
	Col  int
}

// String renders "file:line:col" (col omitted when zero).
func (p Pos) String() string {
	if p.Col == 0 {
		return fmt.Sprintf("%s:%d", p.File, p.Line)
	}
	return fmt.Sprintf("%s:%d:%d", p.File, p.Line, p.Col)
}

// Model is the whole parsed document.
type Model struct {
	Pos
	Name            string
	Namespace       string
	Prefix          string
	Version         string
	PublicationDate string // as written, e.g. "2026-07-02"

	Hierarchy Hierarchy

	HeadComment string

	Imports      []Import
	Enums        []*Enum
	ObjectTypes  []*ObjectType
	Instances    []*Instance
	Perspectives []*Perspective

	// Catalog resolves imported companion-spec references. nil = no specs loaded
	// (imported refs keep legacy trusted behavior). Attached post-Parse by callers.
	Catalog Catalog

	// TODO(v2 seam): custom structured DataTypes. v1 is enum-only; a future
	// `structs:` section would add a []*StructType here and a matching emitter
	// path (DataType with Fields of arbitrary DataType + Encodings). The AST is
	// the place that grows — the CLI and emitter dispatch off these slices.
}

// Hierarchy holds document-level hierarchy options. Set records whether the
// `hierarchy:` key was present, so the formatter round-trips it faithfully.
type Hierarchy struct {
	AllowLevelSkip bool
	Set            bool
}

// Import is one alias → namespace-URI entry. OpcUa is always present.
type Import struct {
	Pos
	Alias   string
	URI     string
	Version string // optional pinned companion-spec version; "" = use the bundled one
}

// Enum is an enumeration DataType.
type Enum struct {
	Pos
	Name   string
	Doc    string
	Values []EnumValue
}

// EnumValue is one enumeration field. Identifier is assigned 0..n in source
// order unless an explicit id was given.
type EnumValue struct {
	Pos
	Name       string
	Identifier int
	Explicit   bool // true if the id came from `Name: <id>` rather than position
}

// ObjectType is an ObjectType definition.
type ObjectType struct {
	Pos
	Name     string
	Doc      string
	Base     TypeRef
	Abstract bool
	Members  []*Member
}

// Kind is the node class of a member.
type Kind string

const (
	KindProperty Kind = "property"
	KindVariable Kind = "variable"
	KindObject   Kind = "object"
	KindMethod   Kind = "method"
)

// Rule is a ModellingRule.
type Rule string

const (
	RuleMandatory            Rule = "mandatory"
	RuleOptional             Rule = "optional"
	RuleOptionalPlaceholder  Rule = "optional_placeholder"
	RuleMandatoryPlaceholder Rule = "mandatory_placeholder"
)

// IsPlaceholder reports whether the rule is one of the placeholder rules.
func (r Rule) IsPlaceholder() bool {
	return r == RuleOptionalPlaceholder || r == RuleMandatoryPlaceholder
}

// Access is a variable/property access level.
type Access string

const (
	AccessRead      Access = "r"
	AccessReadWrite Access = "rw"
)

// Member is a child of an ObjectType (or of an object-kind member).
type Member struct {
	Pos
	// Name is the SymbolicName. For a placeholder member it is the base name
	// (e.g. "Zone"); BrowseName then holds the placeholder browse name
	// ("<ZoneNo>"). For a normal member BrowseName is "".
	Name       string
	BrowseName string

	Kind   Kind
	Rule   Rule
	Access Access
	Type   TypeRef // for property/variable/object; zero value for method
	Unit   string  // raw unit symbol, "" if none
	Doc    string

	Children []*Member  // for object-kind members (folders)
	In       []Argument // method input arguments
	Out      []Argument // method output arguments
}

// IsPlaceholder reports whether the member was written with `Name<Suffix>` syntax.
func (m *Member) IsPlaceholder() bool { return m.BrowseName != "" }

// Argument is a method input/output argument.
type Argument struct {
	Pos
	Name string
	Type TypeRef
}

// Instance is a declared object instance placed under some parent.
type Instance struct {
	Pos
	Name     string
	Type     TypeRef
	Under    TypeRef
	Level    string          // optional ISA-95 EquipmentLevel enum name; "" = not organizational
	LevelPos Pos             // position of the `level:` value, for diagnostics
	Values   []InstanceValue // override property/variable initial values
	Children []InstanceChild // instantiate placeholders
}

// InstanceValue overrides one value-bearing member's initial value. Raw is the
// value as written; its OPC UA type is resolved from the target member at
// validate/emit time.
type InstanceValue struct {
	Pos
	Member string
	Raw    string
}

// InstanceChild instantiates a placeholder member as a concrete child object.
// Of references the placeholder (e.g. "Zone<No>").
type InstanceChild struct {
	Pos
	Name string
	Of   TypeRef
}

// Perspective is a named secondary hierarchy over the same instances. Grouping
// nodes are local to the perspective and never affect the instance layer.
type Perspective struct {
	Pos
	ID         string
	Label      string
	Membership string // "" (default exclusive) | "exclusive" | "overlapping"
	Export     bool
	Nodes      []*PerspectiveNode
}

// PerspectiveNode is one grouping node. Children reference other node ids in the
// same perspective; Members reference instance ids from the instance layer.
type PerspectiveNode struct {
	Pos
	ID       string
	Label    string
	Children []string
	Members  []string
}

// TypeRef is an unresolved reference to a type, as written in the DSL. Alias is
// "" for a local or built-in name and e.g. "OpcUa" for "OpcUa:BaseObjectType".
// Classification (built-in vs local vs imported vs unknown) is done by Resolve,
// keeping the AST free of XML/QName concerns.
type TypeRef struct {
	Pos
	Alias string
	Name  string
	Raw   string // original text, for diagnostics
}

// IsZero reports whether the reference is empty (e.g. a method has no type).
func (t TypeRef) IsZero() bool { return t.Raw == "" }
