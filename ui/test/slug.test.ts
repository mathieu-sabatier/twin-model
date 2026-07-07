import { describe, expect, it } from 'vitest'
import { slugBranch } from '~/lib/slug'

describe('slugBranch', () => {
  it('kebab-cases a normal title with extras', () => {
    expect(slugBranch('Add Furnace02 + Zones')).toBe('model/add-furnace02-zones')
  })

  it('returns model/change for an empty string', () => {
    expect(slugBranch('')).toBe('model/change')
  })

  it('returns model/change for a string of only non-alphanumeric chars', () => {
    expect(slugBranch('  !!!  ')).toBe('model/change')
  })

  it('strips leading and trailing dashes', () => {
    expect(slugBranch('---hello---')).toBe('model/hello')
  })

  it('collapses multiple separators into a single dash', () => {
    expect(slugBranch('fix   the  bug!!!')).toBe('model/fix-the-bug')
  })
})
