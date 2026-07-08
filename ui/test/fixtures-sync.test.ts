// Guards ui/app/mocks/model.golden.json and twinmodel.schema.json (committed copies
// that keep the runtime bundle self-contained) against the REAL sources in the Go
// tree. If a source changes and the copy is not re-synced, this FAILS — preserving
// the "single source of truth" the fixtures rely on. Runs under vitest only (the
// Vite fs.allow lets tests read above ui/); it is never part of the SSG bundle.
import { describe, it, expect } from 'vitest'
import { equipmentModel, schemaJson } from '~/mocks/fixtures'
import goldenReal from '../../internal/dto/testdata/model.golden.json'
import schemaReal from '../../schema/twinmodel.schema.json'

describe('mock fixtures stay in sync with the real Go sources', () => {
  it('equipmentModel equals the Go golden AST', () => {
    expect(equipmentModel).toEqual(goldenReal)
  })
  it('schemaJson equals the real DSL JSON Schema', () => {
    expect(schemaJson).toEqual(schemaReal)
  })
})
