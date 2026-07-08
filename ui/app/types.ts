// Hand-kept mirror of internal/dto/dto.go. Guarded by types.drift.test.ts.
//
// These interfaces are the SPA's copy of the Go JSON DTOs (camelCase, matching
// the `json:"..."` tags). Fields the Go DTO marks `omitempty` are optional (`?:`)
// here. The drift guard imports the REAL Go golden and asserts (both at
// compile-time and at runtime) that it is assignable to `Model`, so a renamed or
// dropped field breaks the build/test rather than silently diverging.
//
// Do NOT add fields the server does not emit, and do NOT relax optionality: the
// server owns the canonical form (see API-CONTRACT.md "Determinism contract").

// --- schema enums (small string-literal unions, from schema/twinmodel.schema.json) ---

/** member.kind (default `variable`). */
export type MemberKind = 'property' | 'variable' | 'object' | 'method'

/** member.rule (default `mandatory`). */
export type MemberRule =
  | 'mandatory'
  | 'optional'
  | 'optional_placeholder'
  | 'mandatory_placeholder'

/** member.access (default `r`); only meaningful for property/variable. */
export type MemberAccess = 'r' | 'rw'

/** Diagnostic severity. */
export type Severity = 'error' | 'warning'

/** semdiff Change.kind — the full set from internal/semdiff/semdiff.go. */
export type ChangeKind =
  | 'TypeAdded'
  | 'TypeRemoved'
  | 'MemberAdded'
  | 'MemberRemoved'
  | 'MemberChanged'
  | 'EnumAdded'
  | 'EnumRemoved'
  | 'EnumValueChanged'
  | 'InstanceAdded'
  | 'InstanceRemoved'
  | 'InstanceChanged'
  | 'ValueChanged'
  | 'ChildAdded'
  | 'ChildRemoved'

// --- AST shapes (mirror dto.go) ---

/** Source position, mirrors dto.Position. */
export interface Position {
  file: string
  line: number
  col: number
}

/** Namespace import, mirrors dto.Import. */
export interface Import {
  pos: Position
  alias: string
  uri: string
}

/** Enum value, mirrors dto.EnumValue. `explicit` is `omitempty`. */
export interface EnumValue {
  pos: Position
  name: string
  identifier: number
  explicit?: boolean
}

/** Enum declaration, mirrors dto.Enum. */
export interface Enum {
  pos: Position
  name: string
  doc?: string
  values: EnumValue[]
}

/** Type reference, mirrors dto.TypeRef. `alias` is `omitempty`. */
export interface TypeRef {
  pos: Position
  alias?: string
  name: string
  raw: string
}

/** Method argument, mirrors dto.Argument. */
export interface Argument {
  pos: Position
  name: string
  type: TypeRef
}

/**
 * Object-type member, mirrors dto.Member. `kind`/`rule` are always present;
 * everything else past `name` is `omitempty`. `type` is a pointer in Go (a
 * method omits it), so it is optional here.
 */
export interface Member {
  pos: Position
  name: string
  browseName?: string
  kind: MemberKind
  rule: MemberRule
  access?: MemberAccess
  type?: TypeRef
  unit?: string
  doc?: string
  children?: Member[]
  in?: Argument[]
  out?: Argument[]
}

/** Object type, mirrors dto.ObjectType. */
export interface ObjectType {
  pos: Position
  name: string
  doc?: string
  base?: TypeRef
  abstract?: boolean
  members?: Member[]
}

/** Instance value assignment, mirrors dto.InstanceValue. */
export interface InstanceValue {
  pos: Position
  member: string
  raw: string
}

/** Instance placeholder child, mirrors dto.InstanceChild. */
export interface InstanceChild {
  pos: Position
  name: string
  of: TypeRef
}

/** Instance declaration, mirrors dto.Instance. `level` is `omitempty`. */
export interface Instance {
  pos: Position
  name: string
  type: TypeRef
  under: TypeRef
  level?: string
  values?: InstanceValue[]
  children?: InstanceChild[]
}

/** Document-level hierarchy options, mirrors dto.Hierarchy. */
export interface Hierarchy {
  allowLevelSkip: boolean
}

/**
 * Named secondary hierarchy over the same instances, mirrors dto.Perspective.
 * Everything past `id` is `omitempty`.
 */
export interface Perspective {
  pos: Position
  id: string
  label?: string
  membership?: string
  export?: boolean
  nodes?: PerspectiveNode[]
}

/** One node in a perspective's tree, mirrors dto.PerspectiveNode. */
export interface PerspectiveNode {
  pos: Position
  id: string
  label?: string
  children?: string[]
  members?: string[]
}

/** Top-level model, mirrors dto.Model. `prefix` is `omitempty`. */
export interface Model {
  pos: Position
  name: string
  namespace: string
  prefix?: string
  version: string
  publicationDate: string
  imports?: Import[]
  enums?: Enum[]
  objectTypes?: ObjectType[]
  instances?: Instance[]
  hierarchy?: Hierarchy
  perspectives?: Perspective[]
}

/** Validation diagnostic, mirrors dto.Diagnostic. `path` anchors it to a field. */
export interface Diagnostic {
  code: string
  severity: Severity
  file: string
  line: number
  col: number
  path: string
  message: string
}

/**
 * A flattened member plus its declaring type, mirrors dto.ResolvedMember
 * (which embeds dto.Member and adds `declaredIn`).
 */
export interface ResolvedMember extends Member {
  declaredIn: string
}

/** semdiff Change (bare array element), mirrors semdiff.Change. */
export interface Change {
  kind: ChangeKind
  type?: string
  member?: string
  instance?: string
  under?: string
  child?: string
  enum?: string
  field?: string
  old?: string
  new?: string
  text: string
}

/** One engineering unit for the unit picker (GET /api/units). */
export interface UnitInfo { symbol: string; displayName: string; description: string }

/** Semantic diff response envelope, mirrors dto.DiffResponse. */
export interface DiffResponse {
  changes: Change[]
  text: string
}

// --- response envelopes (mirror dto.go) ---

/** Envelope for model-read endpoints, mirrors dto.ModelResponse. */
export interface ModelResponse {
  file: string
  model: Model | null
  diagnostics: Diagnostic[]
  parseError?: string
  /** Draft's full model-file list, present on draft model responses so a client
   *  recovering a draft by id can restore the file switcher. Absent for ref reads. */
  files?: string[]
}

/** Validation payload (diagnostics without the AST), mirrors dto.ValidateResponse. */
export interface ValidateResponse {
  file: string
  diagnostics: Diagnostic[]
  parseError?: string
}

/** Instance-form payload, mirrors dto.ResolvedResponse. */
export interface ResolvedResponse {
  type: string
  members: ResolvedMember[]
}

// --- catalog shapes (mirror the Catalog* DTOs in dto.go) ---

/** One bundled companion spec, mirrors dto.CatalogSpec. */
export interface CatalogSpec {
  alias: string
  uri: string
  version: string
  publicationDate: string
  dependencies: string[]
}

/** One type in a spec's type list, mirrors dto.CatalogTypeSummary. */
export interface CatalogTypeSummary {
  name: string
  nodeClass: string
  abstract: boolean
}

/** A companion type reference (base-chain entry), mirrors dto.CatalogTypeRef. */
export interface CatalogTypeRef {
  alias: string
  name: string
  uri: string
}

/** One member of an enumeration DataType, mirrors dto.CatalogEnumMember. */
export interface CatalogEnumMember {
  name: string
  value: number
}

/** Enum values carried by an enum-typed member, mirrors dto.CatalogEnum. */
export interface CatalogEnum {
  members: CatalogEnumMember[]
}

/** A flattened companion member, mirrors dto.CatalogMember. */
export interface CatalogMember {
  name: string
  kind: MemberKind
  placeholder: boolean
  /** Set only when the member's type is a bundled companion type (linkable). */
  type?: CatalogTypeRef
  /** Set only when the member's DataType is an enumeration (read-only display). */
  enum?: CatalogEnum
}

/** Base chain + resolved members of a companion type, mirrors dto.CatalogTypeDetail. */
export interface CatalogTypeDetail {
  alias: string
  uri: string
  name: string
  nodeClass: string
  abstract: boolean
  baseChain: CatalogTypeRef[]
  members: CatalogMember[]
}

/** One search hit, mirrors dto.CatalogSearchHit. */
export interface CatalogSearchHit {
  alias: string
  name: string
  nodeClass: string
  abstract: boolean
}

/** GET /api/catalog envelope, mirrors dto.CatalogListResponse. */
export interface CatalogListResponse { specs: CatalogSpec[] }
/** Spec type-list envelope, mirrors dto.CatalogTypesResponse. */
export interface CatalogTypesResponse { types: CatalogTypeSummary[] }
/** Search envelope, mirrors dto.CatalogSearchResponse. */
export interface CatalogSearchResponse { hits: CatalogSearchHit[] }

/** Repo context + commit identity + propose availability, mirrors dto.RepoInfo (GET /api/repo). */
export interface RepoInfo {
  host: string
  owner: string
  repo: string
  url: string
  defaultBranch: string
  commitName: string
  commitEmail: string
  proposeEnabled: boolean
  proposeReason: string
}

/** One open pull request (GET /api/prs), whose head branch seeds the branch picker. */
export interface PullRequest {
  number: number
  title: string
  url: string
  branch: string
  state: string
}

/** GET /api/branches response: all branches (default first) + the resolved default. */
export interface BranchList {
  branches: string[]
  defaultBranch: string
}
