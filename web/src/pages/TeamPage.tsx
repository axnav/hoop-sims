import { Link, useParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { Star } from 'lucide-react'
import { api, type RosterRow } from '../api'
import { useOpenPlayer } from '../Inline'
import { Card, DataTable, Page, InjuryTag, PlayerLink, Q, TeamBadge, f1, ovrColor, type Col } from '../ui'

export default function TeamPage() {
  const id = Number(useParams().id)
  const q = useQuery({ queryKey: ['team', id], queryFn: () => api.team(id) })
  const open = useOpenPlayer()
  const cols: Col<RosterRow>[] = [
    { accessorKey: 'starter', header: '', cell: (i) => (i.getValue() ? <Star size={12} className="fill-accent text-accent" /> : null), sortingFn: 'basic' },
    { accessorKey: 'name', header: 'Player', cell: (i) => <><PlayerLink id={i.row.original.id} name={i.getValue()} /><InjuryTag injury={i.row.original.injury} /></>, sortDescFirst: false },
    { accessorKey: 'pos', header: 'Pos', sortDescFirst: false },
    { accessorKey: 'age', header: 'Age', num: true },
    { accessorKey: 'ovr', header: 'OVR', num: true, cell: (i) => <b style={{ color: ovrColor(i.getValue()) }}>{i.getValue()}</b> },
    { accessorKey: 'pot', header: 'POT', num: true },
    { accessorKey: 'gp', header: 'GP', num: true },
    { accessorKey: 'mpg', header: 'MPG', num: true, cell: (i) => f1(i.getValue()) },
    { accessorKey: 'ppg', header: 'PPG', num: true, cell: (i) => f1(i.getValue()) },
    { accessorKey: 'rpg', header: 'RPG', num: true, cell: (i) => f1(i.getValue()) },
    { accessorKey: 'apg', header: 'APG', num: true, cell: (i) => f1(i.getValue()) },
  ]
  return (
    <Q q={q} minH={400}>
      {(d) => (
        <Page
          title={<span className="flex items-center gap-3"><TeamBadge team={d.team} size={34} />{d.team.fullName}</span>}
          right={<div className="flex items-center gap-4 text-muted"><span className="font-bold tabular-nums text-ink">{d.w}-{d.l}</span><span>#{d.confRank} {d.team.confName}</span><Link className="link" to={`/schedule/${d.team.id}`}>Schedule</Link></div>}
        >
          <Card title="Roster" right={<span className="text-xs text-muted">Click a column to sort · starters marked</span>}>
            <DataTable data={d.roster} columns={cols} initialSort={[{ id: 'ovr', desc: true }]} onRow={(r) => open(r.id)} />
          </Card>
        </Page>
      )}
    </Q>
  )
}
