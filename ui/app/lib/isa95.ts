// ISA-95 equipment-level metadata for the UI. Mirrors `isa95Levels` in
// internal/dsl/isa95.go (the validation/export source of truth). Kept as a plain
// table so the detail panel can show a level's rank in the org hierarchy and the
// tree can pick the org-vs-equipment icon from one place.

/** One ISA-95 level: its enum integer value and, for organizational levels, the
 *  tier (0-4) it occupies. Equipment-only levels carry no tier (org: false). */
export interface Isa95Level {
  value: number
  tier: number
  org: boolean
}

/** Mirrors ISA95EquipmentElementLevelEnum. tier is the organizational rung (0-4);
 *  the three leaf levels (EquipmentModule/ControlModule/Other) are not org rungs. */
export const ISA95_LEVELS: Record<string, Isa95Level> = {
  Enterprise: { value: 0, tier: 0, org: true },
  Site: { value: 1, tier: 1, org: true },
  Area: { value: 2, tier: 2, org: true },
  ProcessCell: { value: 3, tier: 3, org: true },
  Unit: { value: 4, tier: 4, org: true },
  ProductionLine: { value: 5, tier: 3, org: true },
  WorkCell: { value: 6, tier: 4, org: true },
  ProductionUnit: { value: 7, tier: 4, org: true },
  StorageZone: { value: 8, tier: 3, org: true },
  StorageUnit: { value: 9, tier: 4, org: true },
  WorkCenter: { value: 10, tier: 3, org: true },
  WorkUnit: { value: 11, tier: 4, org: true },
  EquipmentModule: { value: 12, tier: 0, org: false },
  ControlModule: { value: 13, tier: 0, org: false },
  Other: { value: 14, tier: 0, org: false },
}

/** Number of organizational rungs (Enterprise..tier-4). Used for "tier N of RUNGS". */
export const ISA95_ORG_RUNGS = 5

/** True when a level participates in the organizational tier ordering. */
export function isOrgLevel(level: string | undefined): boolean {
  return !!level && (ISA95_LEVELS[level]?.org ?? false)
}

/** 1-indexed organizational rank (Enterprise=1 .. WorkUnit-tier=5) for display,
 *  or null for equipment-only / unknown levels (which have no org rung). */
export function isa95Rank(level: string | undefined): number | null {
  if (!level) return null
  const l = ISA95_LEVELS[level]
  return l && l.org ? l.tier + 1 : null
}

/** Tree icon graded by ISA-95 tier so the spine reads as its layers rather than a
 *  wall of identical buildings: organizational at the top, factory segments below.
 *    tier 0 Enterprise → enterprise/corporate
 *    tier 1 Site       → facility/building
 *    tier 2 Area       → geographic area within the site
 *    tier 3 ProcessCell/ProductionLine/StorageZone/WorkCenter → production segment
 *    tier 4 Unit/WorkCell/ProductionUnit/StorageUnit/WorkUnit → work unit / machine
 *  Equipment (no level), leaf levels, and unknown levels stay the generic boxes. */
export function isa95Icon(level: string | undefined): string {
  const l = level ? ISA95_LEVELS[level] : undefined
  if (!l || !l.org) return 'i-lucide-boxes'
  switch (l.tier) {
    case 0: return 'i-lucide-building-2'
    case 1: return 'i-lucide-factory'
    case 2: return 'i-lucide-land-plot'
    case 3: return 'i-lucide-cog'
    case 4: return 'i-lucide-hammer'
    default: return 'i-lucide-cog'
  }
}
