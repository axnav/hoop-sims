import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { api } from '../api'
import { Card, Page, groupLabel, groupOrder, PlayerLink, Q, TeamLink, f1 } from '../ui'

export default function Leaders() {
  const [badge, setBadge] = useState('')
  const q = useQuery({ queryKey: ['leaders', badge], queryFn: () => api.leaders(badge) })
  const defs = useQuery({ queryKey: ['badges'], queryFn: api.badges, staleTime: Infinity })
  const select = (
    <label className="flex items-center gap-2 text-xs text-muted">Badge
      <select aria-label="Filter by badge" value={badge} onChange={(e) => setBadge(e.target.value)} className="max-w-[55vw] rounded border border-line bg-panel px-2 py-1 text-[13px] text-ink">
        <option value="">Any</option>
        {groupOrder.map((g) => {
          const list = (defs.data?.badges ?? []).filter((b) => b.group === g)
          return list.length === 0 ? null : <optgroup key={g} label={groupLabel[g]}>{list.map((b) => <option key={b.key} value={b.key}>{b.name}</option>)}</optgroup>
        })}
      </select>
    </label>
  )
  return (
    <Page title="League leaders" right={select}>
      <Q q={q} minH={400}>
        {(d) => (
          <>
            <div className="grid gap-4 lg:grid-cols-3">
              {([['Points', d.pts], ['Rebounds', d.reb], ['Assists', d.ast]] as const).map(([t, rows]) => (
                <Card key={t} title={`${t} per game`}>
                  <table className="tbl"><tbody>
                    {rows.length === 0 && <tr><td className="text-muted">Not enough games yet.</td></tr>}
                    {rows.map((r, i) => (
                      <tr key={r.playerId}>
                        <td className="w-6 text-muted">{i + 1}</td><td><PlayerLink id={r.playerId} name={r.name} /></td>
                        <td><TeamLink team={r.team} badge={false} short /></td><td className="r font-bold">{f1(r.value)}</td>
                      </tr>
                    ))}
                  </tbody></table>
                </Card>
              ))}
            </div>
            <p className="mt-3 text-xs text-muted">Minimum {d.minGP} games played.</p>
          </>
        )}
      </Q>
    </Page>
  )
}
