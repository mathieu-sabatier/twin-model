import { describe, expect, it } from 'vitest'
import { focusDiagram } from '~/lib/focusDiagram'

const svg = '<svg><g id="classId-FurnaceType-3"><rect/></g><g id="classId-PressType-4"><rect/></g></svg>'

describe('focusDiagram', () => {
  it('injects a highlight style targeting the selected type node', () => {
    const out = focusDiagram(svg, 'FurnaceType')
    expect(out).toContain('[id^="classId-FurnaceType-"]')
    expect(out).toContain('</svg>')
    expect(out.indexOf('<style>')).toBeGreaterThan(-1)
  })

  it('returns the svg unchanged when nothing (or no type) is selected', () => {
    expect(focusDiagram(svg, null)).toBe(svg)
    expect(focusDiagram('', 'FurnaceType')).toBe('')
  })
})
