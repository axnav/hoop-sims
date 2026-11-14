import { Link, useParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '../api'
import { Card, Page, Q, TeamBadge, TeamLink, WL } from '../ui'

export default function Schedule() {
  const p = useParams().id
  const id = p === undefined ? undefined : Number(p)
  const q = useQuery({ queryKey: ['schedule', id ?? 'user'], queryFn: () => api.schedule(id) })
  return (
    <Q q={q} minH={500}>
      {(d) => {
        const next = d.games.find((g) => !g.played)
        return (
          <Page title={<span className="flex items-center gap-3"><TeamBadge team={d.team} size={30} />{d.team.fullName} schedule</span>}>
            <div className="grid gap-4 lg:grid-cols-2">
              <Card title="Regular season">
                <table className="tbl">
                  <thead><tr><th>Day</th><th></th><th>Opponent</th><th></th><th className="r">Score</th></tr></thead>
                  <tbody>
                    {d.games.map((g) => (
                      <tr key={g.day} className={g === next ? 'me' : g.played ? '' : 'opacity-60'}>
                        <td className="text-muted">{g.day}</td><td className="text-muted">{g.home ? 'vs' : '@'}</td>
                        <td><TeamLink team={g.opp} /></td>
                        <td>{g.played && <WL win={!!g.win} />}</td>
                        <td className="r">{g.played ? `${g.me}-${g.them}` : g === next ? 'Next' : ''}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </Card>
              {d.playoffGames.length > 0 && (
                <Card title="Playoffs">
                  <table className="tbl"><tbody>
                    {d.playoffGames.map((g) => (
                      <tr key={g.box}>
                        <td className="text-muted">{g.label}</td><td className="text-muted">{g.home ? 'vs' : '@'}</td><td><TeamLink team={g.opp} /></td>
                        <td><WL win={g.win} /></td><td className="r"><Link className="link" to={`/box/${g.box}`}>{g.me}-{g.them}</Link></td>
                      </tr>
                    ))}
                  </tbody></table>
                </Card>
              )}
            </div>
          </Page>
        )
      }}
    </Q>
  )
}
