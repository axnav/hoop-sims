import { useQuery } from '@tanstack/react-query'
import { api, type Award } from '../api'
import { Card, Page, PlayerLink, Q, TeamLink, f1 } from '../ui'

const A = ({ a }: { a: Award }) => (a ? <span><PlayerLink id={a.playerId} name={a.name} /> <span className="text-muted">{a.team.abbr}</span></span> : <span className="text-muted">-</span>)

export default function History() {
  const q = useQuery({ queryKey: ['history'], queryFn: api.history })
  return (
    <Page title="League history">
      <Q q={q} minH={300}>
        {(d) => d.seasons.length === 0 ? (
          <div className="card p-8 text-center text-muted">No finished seasons yet.</div>
        ) : (
          <Card>
            <table className="tbl">
              <thead><tr><th>Year</th><th>Champion</th><th>Finals</th><th>MVP</th><th>Finals MVP</th><th>Scoring leader</th></tr></thead>
              <tbody>
                {d.seasons.map((r) => (
                  <tr key={r.year}>
                    <td className="font-bold">{r.year}</td><td><TeamLink team={r.champion} /></td>
                    <td className="text-muted tabular-nums">{r.finals ? `over ${r.finals.loser.name}, ${r.finals.winnerWins}-${r.finals.loserWins}` : '-'}</td>
                    <td><A a={r.mvp} /></td><td><A a={r.finalsMvp} /></td>
                    <td><A a={r.scoringLeader} />{r.scoringLeader && <span className="text-muted tabular-nums"> {f1(r.scoringLeader.ppg)}</span>}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </Card>
        )}
      </Q>
    </Page>
  )
}
