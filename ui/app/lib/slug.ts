/** Build a git branch name `model/<kebab>` from a human title. */
export function slugBranch(title: string): string {
  const kebab = title.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '')
  return kebab ? `model/${kebab}` : 'model/change'
}
