import { useQuery } from '@tanstack/react-query'
import { api } from '../api'
import { Card, Page, PlayerLink, Q, TeamLink, f1 } from '../ui'

export default function Leaders() {
  const q = useQuery({ queryKey: ['leaders'], queryFn: api.leaders })
  return (
    <Page title="League leaders">
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
