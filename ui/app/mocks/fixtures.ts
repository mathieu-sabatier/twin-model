// Fixtures for the MSW mock of the (not-yet-running) Go API. Shared by the dev
// browser worker and the Vitest node server. Derived from the REAL golden + the
// REAL schema wherever possible so the mock stays honest against the contract.
//
// NOTE: goldenModel and schema are committed copies of the Go sources (kept in sync
// by ui/test/fixtures-sync.test.ts), making the bundle fully self-contained and
// Docker-buildable without access to the parent Go tree. See RT2.5.
import goldenModel from './model.golden.json'
import schema from './twinmodel.schema.json'
import type {
  BranchList,
  Change,
  Diagnostic,
  DiffResponse,
  Model,
  ModelResponse,
  Perspective,
  PullRequest,
  RepoInfo,
  ResolvedResponse,
  ValidateResponse,
} from '~/types'

/** The real Go golden, retyped as our Model. Single source — never hand-retyped. */
export const equipmentModel: Model = goldenModel as unknown as Model

/** The real DSL JSON Schema served at GET /api/schema. */
export const schemaJson: Record<string, unknown> = schema as Record<string, unknown>

/** The canonical file the seeded draft edits. */
export const seedFile = 'equipment.yaml'

/** A small canonical YAML string consistent with equipmentModel (for the YAML pane). */
export const equipmentYaml = `model: AcmeEquipment
namespace: https://acme.example/UA/Equipment/
version: "1.0.0"
objectTypes:
  - name: EquipmentType
    abstract: true
    members:
      - name: Manufacturer
        kind: variable
        dataType: String
  - name: FurnaceType
    base: EquipmentType
`

/**
 * A representative diagnostics sample. Includes at least one Path-anchored error
 * so a later task can exercise the field registry (mapping `path` -> a form cell).
 * Kept small; the mock returns these verbatim from /validate.
 */
export const diagnostics: Diagnostic[] = [
  {
    code: 'unit-on-non-numeric',
    severity: 'error',
    file: seedFile,
    line: 34,
    col: 46,
    path: 'object_types/FurnaceType/members/Efficiency/unit',
    message: 'unit "%" is only valid on numeric variables',
  },
  {
    code: 'optional',
    severity: 'warning',
    file: seedFile,
    line: 37,
    col: 7,
    path: 'object_types/EquipmentType/members/CycleCount',
    message: 'optional member has no default; consumers must handle absence',
  },
]

/** A clean (no-diagnostics) validate result, for the happy path. */
export const validateClean: ValidateResponse = {
  file: seedFile,
  diagnostics: [],
}

/** A validate result carrying the diagnostics sample above. */
export const validateWithDiagnostics: ValidateResponse = {
  file: seedFile,
  diagnostics,
}

/**
 * A small named perspective (secondary hierarchy) consistent with the golden's
 * instances — Furnace01 assigned to a single node. Merged into every mock
 * model response (see draftModelResponse in handlers.ts) so perspective-view
 * tests have a real perspective to select without per-test seeding.
 */
export const mockPerspectives: Perspective[] = [
  {
    pos: { file: 'equipment.yaml', line: 1, col: 1 }, id: 'spatial_zones', label: 'Spatial / fire zones', membership: 'exclusive', export: false,
    nodes: [
      { pos: { file: 'equipment.yaml', line: 1, col: 1 }, id: 'hall_b', label: 'Hall B', members: ['Furnace01'] },
    ],
  },
]

/** A full model-read envelope built from the golden. */
export const modelResponse: ModelResponse = {
  file: seedFile,
  model: equipmentModel,
  diagnostics: [],
}

/**
 * FurnaceType's flattened members incl. inherited EquipmentType members, each
 * tagged with the type that declared it. Mirrors what dsl.ResolveMembers + the
 * ResolvedResponse DTO would return. Reuses the golden's own member objects.
 */
export const resolvedFurnace: ResolvedResponse = (() => {
  const equipmentType = equipmentModel.objectTypes!.find((t) => t.name === 'EquipmentType')!
  const furnaceType = equipmentModel.objectTypes!.find((t) => t.name === 'FurnaceType')!
  return {
    type: 'FurnaceType',
    members: [
      // Inherited from the base first (declaration order), then own members.
      ...equipmentType.members!.map((m) => ({ ...m, declaredIn: 'EquipmentType' })),
      ...furnaceType.members!.map((m) => ({ ...m, declaredIn: 'FurnaceType' })),
    ],
  }
})()

/** A small, representative semdiff changelist in the server envelope shape. */
export const diffSample: DiffResponse = {
  changes: [
    {
      kind: 'MemberAdded',
      type: 'FurnaceType',
      member: 'DoorClosed',
      text: 'FurnaceType: added member DoorClosed',
    },
    {
      kind: 'MemberChanged',
      type: 'HeatingZoneType',
      member: 'Setpoint',
      field: 'access',
      old: 'r',
      new: 'rw',
      text: 'HeatingZoneType.Setpoint: access r -> rw',
    },
    {
      kind: 'ValueChanged',
      instance: 'Furnace02',
      member: 'CycleCount',
      old: '0',
      new: '42',
      text: 'Furnace02.CycleCount: 0 -> 42',
    },
  ],
  text: 'PressType.Setpoint: access r → rw\nAdded instance Furnace02 (FurnaceType) under ObjectsFolder',
}

/** A small sample of engineering units for the unit picker. */
export const unitsSample = [{ symbol: '°C', displayName: '°C', description: 'degree Celsius' }, { symbol: 'bar', displayName: 'bar', description: 'bar' }]

/** The PR URL a successful propose returns. */
export const proposeUrl = 'https://github.com/mathieu-sabatier/twin-model/pull/42'

/** The 409 (lint-red) propose error body. */
export const proposeConflict: { error: string; diagnostics: Diagnostic[] } = {
  error: 'draft has lint errors',
  diagnostics,
}

/** A small, valid Mermaid classDiagram source (preview/diagram returns text). */
export const diagramMermaid = `classDiagram
  class EquipmentType {
    <<abstract>>
    +String Manufacturer
    +String SerialNumber
    +EquipmentState State
    +UInt32 CycleCount
  }
  class FurnaceType
  EquipmentType <|-- FurnaceType
`

/** A small ModelDesign XML preview (preview/modeldesign returns text). */
export const modelDesignXml = `<?xml version="1.0" encoding="utf-8"?>
<ModelDesign xmlns="http://opcfoundation.org/UA/ModelDesign.xsd" TargetNamespace="https://acme.example/UA/Equipment/">
  <ObjectType SymbolicName="FurnaceType" BaseType="EquipmentType" />
</ModelDesign>
`

/** GET /api/repo payload for dev/tests (proposing enabled against the real repo). */
export const repoInfoSample: RepoInfo = {
  host: 'github',
  owner: 'mathieu-sabatier',
  repo: 'twin-model',
  url: 'https://github.com/mathieu-sabatier/twin-model',
  defaultBranch: 'main',
  commitName: 'twinmodel-bot',
  commitEmail: 'bot@twinmodel',
  proposeEnabled: true,
  proposeReason: '',
}

/** GET /api/prs sample: two open PRs whose head branches seed the branch picker. */
export const prsSample: PullRequest[] = [
  { number: 42, title: 'Add furnace zones', url: 'https://github.com/mathieu-sabatier/twin-model/pull/42', branch: 'model/furnace-zones', state: 'open' },
  { number: 43, title: 'Tune press curve', url: 'https://github.com/mathieu-sabatier/twin-model/pull/43', branch: 'model/press-curve', state: 'open' },
]

/** GET /api/branches sample: default first, then two more branches. */
export const branchListSample: BranchList = {
  branches: ['main', 'model/furnace-zones', 'model/press-curve'],
  defaultBranch: 'main',
}
