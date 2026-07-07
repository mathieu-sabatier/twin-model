/**
 * Highlight the selected type's node inside a rendered mermaid classDiagram SVG
 * by appending a scoped <style>. Mermaid ids each class group `classId-<Name>-<n>`,
 * so we target that id prefix. Returns the SVG unchanged when there is no type
 * selection (or no SVG). QA finding L1.
 */
export function focusDiagram(svg: string, typeName: string | null): string {
  if (!typeName || !svg) return svg
  const safe = typeName.replace(/[^A-Za-z0-9_]/g, '')
  const style =
    `<style>[id^="classId-${safe}-"] rect,` +
    `[id^="classId-${safe}-"] path{stroke:var(--ui-primary,#2563eb) !important;stroke-width:2.5px !important}</style>`
  const i = svg.lastIndexOf('</svg>')
  return i === -1 ? svg + style : svg.slice(0, i) + style + svg.slice(i)
}
