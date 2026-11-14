import { Link, useParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { Tv } from 'lucide-react'
import { api, type BoxLine, type Team } from '../api'
import { Card, Page, PlayerLink, Q, TeamBadge, TeamLink } from '../ui'

export function BoxTable({ team, score, lines }: { team: Team; score: number; lines: BoxLine[] }) {
  return (
    <Card title={<span className="flex items-center gap-2"><TeamBadge team={team} size={18} /><TeamLink team={team} badge={false} /> <span className="text-ink">{score}</span></span>}>
      <table className="tbl">
        <thead><tr><th>Player</th><th>Pos</th><th className="r">MIN</th><th className="r">PTS</th><th className="r">REB</th><th className="r">AST</th><th className="r">TOV</th><th className="r">FG</th><th className="r">3P</th><th className="r">FT</th></tr></thead>
        <tbody>
          {lines.map((l) => (
            <tr key={l.id}>
              <td><PlayerLink id={l.id} name={l.name} /></td><td className="text-muted">{l.pos}</td><td className="r">{l.min}</td>
              <td className="r font-bold">{l.pts}</td><td className="r">{l.reb}</td><td className="r">{l.ast}</td><td className="r">{l.tov}</td>
              <td className="r">{l.fgm}-{l.fga}</td><td className="r">{l.tpm}-{l.tpa}</td><td className="r">{l.ftm}-{l.fta}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </Card>
  )
}

export default function BoxScore() {
  const id = useParams().id!
  const q = useQuery({ queryKey: ['box', id], queryFn: () => api.box(id) })
  return (
    <Q q={q} minH={400}>
      {(b) => (
        <Page
          title={`${b.teams[1].team.abbr} @ ${b.teams[0].team.abbr}  ${b.teams[1].score}-${b.teams[0].score}${b.overtimes ? ` (${b.overtimes > 1 ? b.overtimes : ''}OT)` : ''}`}
          right={<span className="flex items-center gap-4 text-muted">{b.label}{b.hasReplay && b.id !== 'last' && <Link className="btn" to={`/game/${b.id}`}><Tv size={14} />Replay</Link>}</span>}
        >
          <div className="grid gap-4 xl:grid-cols-2">
            {[b.teams[1], b.teams[0]].map((t) => <BoxTable key={t.team.id} team={t.team} score={t.score} lines={t.lines} />)}
          </div>
        </Page>
      )}
    </Q>
  )
}
