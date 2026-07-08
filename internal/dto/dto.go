// Package dto defines the JSON shapes the HTTP API emits and the mappers from the
// XML-free dsl AST. The AST itself carries no json tags (it stays presentation-
// free); these DTOs are the stable, hand-controlled contract that the SPA's
// types.ts mirrors, guarded by a golden test.
package dto

import (
	"github.com/mathieu-sabatier/twin-model/internal/dsl"
	"github.com/mathieu-sabatier/twin-model/internal/semdiff"
)

// Position is a source position mirroring dsl.Pos.
type Position struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Col  int    `json:"col"`
}

// TypeRef mirrors dsl.TypeRef (classification is done server-side by the emitter).
type TypeRef struct {
	Pos   Position `json:"pos"`
	Alias string   `json:"alias,omitempty"`
	Name  string   `json:"name"`
	Raw   string   `json:"raw"`
}

// Argument mirrors dsl.Argument.
type Argument struct {
	Pos  Position `json:"pos"`
	Name string   `json:"name"`
	Type TypeRef  `json:"type"`
}

// Member mirrors dsl.Member. Type is a pointer so a method (zero TypeRef) omits it.
type Member struct {
	Pos        Position   `json:"pos"`
	Name       string     `json:"name"`
	BrowseName string     `json:"browseName,omitempty"`
	Kind       string     `json:"kind"`
	Rule       string     `json:"rule"`
	Access     string     `json:"access,omitempty"`
	Type       *TypeRef   `json:"type,omitempty"`
	Unit       string     `json:"unit,omitempty"`
	Doc        string     `json:"doc,omitempty"`
	Children   []*Member  `json:"children,omitempty"`
	In         []Argument `json:"in,omitempty"`
	Out        []Argument `json:"out,omitempty"`
}

// ObjectType mirrors dsl.ObjectType.
type ObjectType struct {
	Pos      Position  `json:"pos"`
	Name     string    `json:"name"`
	Doc      string    `json:"doc,omitempty"`
	Base     *TypeRef  `json:"base,omitempty"`
	Abstract bool      `json:"abstract,omitempty"`
	Members  []*Member `json:"members,omitempty"`
}

// EnumValue mirrors dsl.EnumValue.
type EnumValue struct {
	Pos        Position `json:"pos"`
	Name       string   `json:"name"`
	Identifier int      `json:"identifier"`
	Explicit   bool     `json:"explicit,omitempty"`
}

// Enum mirrors dsl.Enum.
type Enum struct {
	Pos    Position    `json:"pos"`
	Name   string      `json:"name"`
	Doc    string      `json:"doc,omitempty"`
	Values []EnumValue `json:"values"`
}

// Import mirrors dsl.Import.
type Import struct {
	Pos   Position `json:"pos"`
	Alias string   `json:"alias"`
	URI   string   `json:"uri"`
}

// InstanceValue mirrors dsl.InstanceValue.
type InstanceValue struct {
	Pos    Position `json:"pos"`
	Member string   `json:"member"`
	Raw    string   `json:"raw"`
}

// InstanceChild mirrors dsl.InstanceChild.
type InstanceChild struct {
	Pos  Position `json:"pos"`
	Name string   `json:"name"`
	Of   TypeRef  `json:"of"`
}

// Instance mirrors dsl.Instance.
type Instance struct {
	Pos      Position        `json:"pos"`
	Name     string          `json:"name"`
	Type     TypeRef         `json:"type"`
	Under    TypeRef         `json:"under"`
	Level    string          `json:"level,omitempty"`
	Values   []InstanceValue `json:"values,omitempty"`
	Children []InstanceChild `json:"children,omitempty"`
}

// Model mirrors dsl.Model.
type Model struct {
	Pos             Position       `json:"pos"`
	Name            string         `json:"name"`
	Namespace       string         `json:"namespace"`
	Prefix          string         `json:"prefix,omitempty"`
	Version         string         `json:"version"`
	PublicationDate string         `json:"publicationDate"`
	Imports         []Import       `json:"imports,omitempty"`
	Enums           []*Enum        `json:"enums,omitempty"`
	ObjectTypes     []*ObjectType  `json:"objectTypes,omitempty"`
	Instances       []*Instance    `json:"instances,omitempty"`
	Hierarchy       *Hierarchy     `json:"hierarchy,omitempty"`
	Perspectives    []*Perspective `json:"perspectives,omitempty"`
}

// Hierarchy mirrors dsl.Hierarchy's document-level options.
type Hierarchy struct {
	AllowLevelSkip bool `json:"allowLevelSkip"`
}

// Perspective mirrors dsl.Perspective: a named secondary hierarchy over the
// same instances.
type Perspective struct {
	Pos        Position           `json:"pos"`
	ID         string             `json:"id"`
	Label      string             `json:"label,omitempty"`
	Membership string             `json:"membership,omitempty"`
	Export     bool               `json:"export,omitempty"`
	Nodes      []*PerspectiveNode `json:"nodes,omitempty"`
}

// PerspectiveNode mirrors dsl.PerspectiveNode.
type PerspectiveNode struct {
	Pos      Position `json:"pos"`
	ID       string   `json:"id"`
	Label    string   `json:"label,omitempty"`
	Children []string `json:"children,omitempty"`
	Members  []string `json:"members,omitempty"`
}

// Diagnostic mirrors dsl.Diagnostic with a string severity.
type Diagnostic struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	Col      int    `json:"col"`
	Path     string `json:"path"`
	Message  string `json:"message"`
}

// ResolvedMember mirrors dsl.ResolvedMember (a flattened member + its declaring type).
type ResolvedMember struct {
	Member
	DeclaredIn string `json:"declaredIn"`
}

// ModelResponse is the envelope for the model-read endpoints. Parse errors are
// data, not transport errors: ParseError is set and Model is nil when the YAML is
// structurally unparseable, so the editor can still show the message.
type ModelResponse struct {
	File        string       `json:"file"`
	Model       *Model       `json:"model"`
	Diagnostics []Diagnostic `json:"diagnostics"`
	ParseError  string       `json:"parseError,omitempty"`
	// Files is the draft's full model-file list, included on draft model responses
	// so a client recovering a draft by id (page refresh / deep link) can restore
	// the file switcher without a second round-trip. Empty for committed-ref reads.
	Files []string `json:"files,omitempty"`
}

// ValidateResponse carries diagnostics without the AST.
type ValidateResponse struct {
	File        string       `json:"file"`
	Diagnostics []Diagnostic `json:"diagnostics"`
	ParseError  string       `json:"parseError,omitempty"`
}

// ResolvedResponse is the instance-form endpoint's payload.
type ResolvedResponse struct {
	Type    string           `json:"type"`
	Members []ResolvedMember `json:"members"`
}

// DraftResponse is a draft's metadata, letting a client re-hydrate a draft by id
// after a page refresh without holding any state locally.
type DraftResponse struct {
	ID        string   `json:"id"`
	BaseRef   string   `json:"baseRef"`
	Files     []string `json:"files"`
	UpdatedAt string   `json:"updatedAt"` // RFC3339
}

// CreateDraftResponse is returned by POST /api/drafts.
type CreateDraftResponse struct {
	ID      string   `json:"id"`
	BaseRef string   `json:"baseRef"`
	Files   []string `json:"files"`
}

// FilesResponse lists a draft's files after a write (PUT /api/drafts/{id}/files).
type FilesResponse struct {
	Files []string `json:"files"`
}

// DiffResponse is the semantic changelist plus its rendered human form.
type DiffResponse struct {
	Changes []semdiff.Change `json:"changes"`
	Text    string           `json:"text"`
}

// ProposeResponse carries the URL of the opened pull request.
type ProposeResponse struct {
	URL string `json:"url"`
}

// ProposeErrorResponse is the 502 body when a PR could not be opened: a friendly,
// actionable message plus the raw upstream detail for a "Details" disclosure.
type ProposeErrorResponse struct {
	Error  string `json:"error"`
	Detail string `json:"detail,omitempty"`
}

// ConflictResponse is the 409 body: the diagnostics blocking a propose.
type ConflictResponse struct {
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// PullRequest is an open pull-request summary from the git host.
type PullRequest struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	URL    string `json:"url"`
	Branch string `json:"branch"` // head ref
	State  string `json:"state"`
}

// PRListResponse is the GET /api/prs payload.
type PRListResponse struct {
	PRs []PullRequest `json:"prs"`
}

// RepoInfo is the GET /api/repo payload: which repo the server edits, the identity
// commits are authored as, and whether proposing (opening a PR) is possible. All
// fields derive from server config; the browser never sees the token.
type RepoInfo struct {
	Host           string `json:"host"`
	Owner          string `json:"owner"`
	Repo           string `json:"repo"`
	URL            string `json:"url"`
	DefaultBranch  string `json:"defaultBranch"`
	CommitName     string `json:"commitName"`
	CommitEmail    string `json:"commitEmail"`
	ProposeEnabled bool   `json:"proposeEnabled"`
	ProposeReason  string `json:"proposeReason"`
}

// BranchList is the GET /api/branches payload: the repo's head branches (default
// first, then alphabetical) and its resolved default branch. Produced by a
// go-git ls-remote, so it reflects the real branches without a full clone.
type BranchList struct {
	Branches      []string `json:"branches"`
	DefaultBranch string   `json:"defaultBranch"`
}

// UnitInfo is one engineering unit for the UI's unit picker.
type UnitInfo struct {
	Symbol      string `json:"symbol"`
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
}

// UnitsResponse is the GET /api/units payload.
type UnitsResponse struct {
	Units []UnitInfo `json:"units"`
}

// FromUnits maps dsl units to their DTOs.
func FromUnits(us []dsl.Unit) []UnitInfo {
	out := make([]UnitInfo, len(us))
	for i, u := range us {
		out[i] = UnitInfo{Symbol: u.Symbol, DisplayName: u.DisplayName, Description: u.Description}
	}
	return out
}

func pos(p dsl.Pos) Position { return Position{File: p.File, Line: p.Line, Col: p.Col} }

func typeRefPtr(t dsl.TypeRef) *TypeRef {
	if t.IsZero() {
		return nil
	}
	tr := fromTypeRef(t)
	return &tr
}

func fromTypeRef(t dsl.TypeRef) TypeRef {
	return TypeRef{Pos: pos(t.Pos), Alias: t.Alias, Name: t.Name, Raw: t.Raw}
}

func fromMember(m *dsl.Member) *Member {
	out := &Member{
		Pos:        pos(m.Pos),
		Name:       m.Name,
		BrowseName: m.BrowseName,
		Kind:       string(m.Kind),
		Rule:       string(m.Rule),
		Access:     string(m.Access),
		Type:       typeRefPtr(m.Type),
		Unit:       m.Unit,
		Doc:        m.Doc,
	}
	for _, c := range m.Children {
		out.Children = append(out.Children, fromMember(c))
	}
	out.In = fromArgs(m.In)
	out.Out = fromArgs(m.Out)
	return out
}

// fromArgs maps method arguments to their DTOs, returning nil (an omitted JSON
// field) for an empty list.
func fromArgs(args []dsl.Argument) []Argument {
	if len(args) == 0 {
		return nil
	}
	out := make([]Argument, len(args))
	for i, a := range args {
		out[i] = Argument{Pos: pos(a.Pos), Name: a.Name, Type: fromTypeRef(a.Type)}
	}
	return out
}

// FromModel maps a parsed dsl.Model to its JSON DTO.
func FromModel(m *dsl.Model) *Model {
	if m == nil {
		return nil
	}
	out := &Model{
		Pos:             pos(m.Pos),
		Name:            m.Name,
		Namespace:       m.Namespace,
		Prefix:          m.Prefix,
		Version:         m.Version,
		PublicationDate: m.PublicationDate,
	}
	for _, im := range m.Imports {
		out.Imports = append(out.Imports, Import{Pos: pos(im.Pos), Alias: im.Alias, URI: im.URI})
	}
	for _, e := range m.Enums {
		ed := &Enum{Pos: pos(e.Pos), Name: e.Name, Doc: e.Doc}
		for _, v := range e.Values {
			ed.Values = append(ed.Values, EnumValue{Pos: pos(v.Pos), Name: v.Name, Identifier: v.Identifier, Explicit: v.Explicit})
		}
		out.Enums = append(out.Enums, ed)
	}
	for _, ot := range m.ObjectTypes {
		otd := &ObjectType{Pos: pos(ot.Pos), Name: ot.Name, Doc: ot.Doc, Base: typeRefPtr(ot.Base), Abstract: ot.Abstract}
		for _, mem := range ot.Members {
			otd.Members = append(otd.Members, fromMember(mem))
		}
		out.ObjectTypes = append(out.ObjectTypes, otd)
	}
	for _, inst := range m.Instances {
		id := &Instance{Pos: pos(inst.Pos), Name: inst.Name, Type: fromTypeRef(inst.Type), Under: fromTypeRef(inst.Under), Level: inst.Level}
		for _, v := range inst.Values {
			id.Values = append(id.Values, InstanceValue{Pos: pos(v.Pos), Member: v.Member, Raw: v.Raw})
		}
		for _, c := range inst.Children {
			id.Children = append(id.Children, InstanceChild{Pos: pos(c.Pos), Name: c.Name, Of: fromTypeRef(c.Of)})
		}
		out.Instances = append(out.Instances, id)
	}
	if m.Hierarchy.Set {
		out.Hierarchy = &Hierarchy{AllowLevelSkip: m.Hierarchy.AllowLevelSkip}
	}
	for _, p := range m.Perspectives {
		pd := &Perspective{Pos: pos(p.Pos), ID: p.ID, Label: p.Label, Membership: p.Membership, Export: p.Export}
		for _, nd := range p.Nodes {
			pd.Nodes = append(pd.Nodes, &PerspectiveNode{
				Pos: pos(nd.Pos), ID: nd.ID, Label: nd.Label, Children: nd.Children, Members: nd.Members,
			})
		}
		out.Perspectives = append(out.Perspectives, pd)
	}
	return out
}

// FromDiagnostic maps one dsl.Diagnostic to its DTO.
func FromDiagnostic(d dsl.Diagnostic) Diagnostic {
	sev := "error"
	if d.Severity == dsl.SeverityWarning {
		sev = "warning"
	}
	return Diagnostic{
		Code: d.Code, Severity: sev, File: d.File, Line: d.Line,
		Col: d.Col, Path: d.Path, Message: d.Message,
	}
}

// FromDiagnostics maps a slice of diagnostics.
func FromDiagnostics(ds []dsl.Diagnostic) []Diagnostic {
	out := make([]Diagnostic, 0, len(ds))
	for _, d := range ds {
		out = append(out, FromDiagnostic(d))
	}
	return out
}

// FromResolvedMembers maps ResolveMembers output to DTOs.
func FromResolvedMembers(rms []dsl.ResolvedMember) []ResolvedMember {
	out := make([]ResolvedMember, 0, len(rms))
	for _, rm := range rms {
		out = append(out, ResolvedMember{Member: *fromMember(rm.Member), DeclaredIn: rm.DeclaredIn})
	}
	return out
}

// ── Catalog DTOs (companion-spec discovery). Shallow by design: members carry
// only name/kind/placeholder (see spec §Design decisions). ──────────────────

// CatalogSpec is one bundled companion spec in the discovery list.
type CatalogSpec struct {
	Alias           string   `json:"alias"`
	URI             string   `json:"uri"`
	Version         string   `json:"version"`
	PublicationDate string   `json:"publicationDate"`
	Dependencies    []string `json:"dependencies"` // transitive dep aliases, deps-first
}

// CatalogTypeSummary is one type in a spec's type list.
type CatalogTypeSummary struct {
	Name      string `json:"name"`
	NodeClass string `json:"nodeClass"`
	Abstract  bool   `json:"abstract"`
}

// CatalogTypeRef names a companion type by alias + browse name (+ its URI).
type CatalogTypeRef struct {
	Alias string `json:"alias"`
	Name  string `json:"name"`
	URI   string `json:"uri"`
}

// CatalogEnum carries the allowed values of an enumeration-typed catalog member
// so the read-only detail view can render `value · name` pairs.
type CatalogEnum struct {
	Members []CatalogEnumMember `json:"members"`
}

// CatalogEnumMember is one allowed value of an enumeration.
type CatalogEnumMember struct {
	Name  string `json:"name"`
	Value int64  `json:"value"`
}

// CatalogMember is one flattened member of a companion type. Type is set only
// when the member's type is a bundled companion type (so the UI can link to it);
// it is omitted for primitives and ns0 base types.
type CatalogMember struct {
	Name        string          `json:"name"`
	Kind        string          `json:"kind"`
	Placeholder bool            `json:"placeholder"`
	Type        *CatalogTypeRef `json:"type,omitempty"`
	Enum        *CatalogEnum    `json:"enum,omitempty"`
}

// CatalogTypeDetail is the base chain + resolved members of one companion type.
type CatalogTypeDetail struct {
	Alias     string           `json:"alias"`
	URI       string           `json:"uri"`
	Name      string           `json:"name"`
	NodeClass string           `json:"nodeClass"`
	Abstract  bool             `json:"abstract"`
	BaseChain []CatalogTypeRef `json:"baseChain"`
	Members   []CatalogMember  `json:"members"`
}

// CatalogSearchHit is one type matched by a catalog search.
type CatalogSearchHit struct {
	Alias     string `json:"alias"`
	Name      string `json:"name"`
	NodeClass string `json:"nodeClass"`
	Abstract  bool   `json:"abstract"`
}

// CatalogListResponse wraps the spec list (GET /api/catalog).
type CatalogListResponse struct {
	Specs []CatalogSpec `json:"specs"`
}

// CatalogTypesResponse wraps a spec's type list.
type CatalogTypesResponse struct {
	Types []CatalogTypeSummary `json:"types"`
}

// CatalogSearchResponse wraps search hits.
type CatalogSearchResponse struct {
	Hits []CatalogSearchHit `json:"hits"`
}

// FromCatalogMembers maps flattened catalog members to DTOs (never nil, for
// stable JSON). aliasFor resolves a namespace URI to its registry alias ("" if not
// bundled); a member whose type resolves to a bundled spec gets a linkable Type.
func FromCatalogMembers(ms []dsl.CatalogMember, aliasFor func(uri string) string) []CatalogMember {
	out := make([]CatalogMember, 0, len(ms))
	for _, m := range ms {
		cm := CatalogMember{Name: m.Name, Kind: string(m.Kind), Placeholder: m.Placeholder}
		if m.TypeName != "" {
			if a := aliasFor(m.TypeURI); a != "" {
				cm.Type = &CatalogTypeRef{Alias: a, Name: m.TypeName, URI: m.TypeURI}
			}
		}
		if len(m.Enum) > 0 {
			em := make([]CatalogEnumMember, len(m.Enum))
			for i, e := range m.Enum {
				em[i] = CatalogEnumMember{Name: e.Name, Value: e.Value}
			}
			cm.Enum = &CatalogEnum{Members: em}
		}
		out = append(out, cm)
	}
	return out
}

// FromCatalogType assembles a type-detail DTO. The caller supplies alias, the
// resolved base chain, and an alias resolver (all need the registry, which dto
// must not import).
func FromCatalogType(alias, uri string, t dsl.CatalogType, baseChain []CatalogTypeRef, aliasFor func(uri string) string) CatalogTypeDetail {
	if baseChain == nil {
		baseChain = []CatalogTypeRef{}
	}
	return CatalogTypeDetail{
		Alias: alias, URI: uri, Name: t.Name, NodeClass: t.NodeClass,
		Abstract: t.Abstract, BaseChain: baseChain, Members: FromCatalogMembers(t.Members, aliasFor),
	}
}

// DraftWriteResponse is returned by the additive patch operations: the draft's
// file list plus per-file validation diagnostics, so the caller sees lint state
// without a separate validate call.
type DraftWriteResponse struct {
	Files       []string                `json:"files"`
	Diagnostics map[string][]Diagnostic `json:"diagnostics"`
}
