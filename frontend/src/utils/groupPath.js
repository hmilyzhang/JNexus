// Full-path label for a host group: "root / parent / child".
// Group names are unique per parent only, so flat pickers need the path
// to disambiguate same-named groups under different parents.
export function groupLabel(groups, g) {
  if (!g) return ''
  const byId = new Map((groups || []).map(x => [x.id, x]))
  const parts = [g.name]
  const seen = new Set([g.id])
  let cur = g
  while (cur.parent_id) {
    const p = byId.get(cur.parent_id)
    if (!p || seen.has(p.id)) break
    parts.unshift(p.name)
    seen.add(p.id)
    cur = p
  }
  return parts.join(' / ')
}
