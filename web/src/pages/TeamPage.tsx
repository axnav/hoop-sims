import { Link, useParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { Star } from 'lucide-react'
import { api, type RosterRow } from '../api'
import { useOpenPlayer } from '../Inline'
import { Avatar, Card, FitGrade, DataTable, Delta, ExpiringChip, Page, InjuryTag, PayrollBar, PlayerLink, Q, FocusLabel, BadgeCount, MoraleChip, TradeRequestTag, StanceChip, StatusChip, TeamBadge, f1, money, ovrColor, type Col } from '../ui'

export default function TeamPage() {
  const id = Number(useParams().id)
  const q = useQuery({ queryKey: ['team', id], queryFn: () => api.team(id) })
  const open = useOpenPlayer()
  const cols: Col<RosterRow>[] = [
    { accessorKey: 'starter', header: '', cell: (i) => (i.getValue() ? <Star size={12} className="fill-accent text-accent" /> : null), sortingFn: 'basic' },
    { accessorKey: 'name', header: 'Player', cell: (i) => <><Avatar id={i.row.original.id} name={i.getValue()} photo={i.row.original.photo} color={q.data?.team.primary} size={28} /> <PlayerLink id={i.row.original.id} name={i.getValue()} /><InjuryTag injury={i.row.original.injury} />{i.row.original.expiring && <ExpiringChip />}<StanceChip stance={i.row.original.stance} extended={i.row.original.extended} /><FocusLabel focus={i.row.original.focus} /><BadgeCount n={i.row.original.badgeCount} />{i.row.original.tradeRequest && <TradeRequestTag />}<span className="ml-1.5 min-[700px]:hidden"><MoraleChip morale={i.row.original.morale} dotOnly /></span></>, sortDescFirst: false },
    { id: 'morale', accessorFn: (r) => r.morale?.score ?? 0, header: 'Mood', hideSmall: true, cell: (i) => <MoraleChip morale={i.row.original.morale} /> },
    { accessorKey: 'pos', header: 'Pos', sortDescFirst: false },
    { accessorKey: 'age', header: 'Age', num: true },
    { accessorKey: 'ovr', header: 'OVR', num: true, cell: (i) => <><b style={{ color: ovrColor(i.getValue()) }}>{i.getValue()}</b><Delta value={i.row.original.ovrChange} scope={i.row.original.changeScope} /></> },
    { accessorKey: 'pot', header: 'POT', num: true, cell: (i) => <>{i.getValue()}<Delta value={i.row.original.potChange} scope={i.row.original.changeScope} /></> },
    { accessorKey: 'salary', header: 'Salary', num: true, cell: (i) => money(i.getValue()) },
    { accessorKey: 'years', header: 'Yrs', num: true },
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
          title={<span className="flex items-center gap-3"><TeamBadge team={d.team} size={34} />{d.team.fullName}<StatusChip status={d.status} /></span>}
          right={<div className="flex items-center gap-4 text-muted"><span className="font-bold tabular-nums text-ink">{d.w}-{d.l}</span><span>#{d.confRank} {d.team.confName}</span><Link className="link" to={`/schedule/${d.team.id}`}>Schedule</Link></div>}
        >
          {d.status && <p className="-mt-2 mb-4 text-sm text-muted">{d.status.wants}</p>}
          {d.coach && (
            <p className="-mt-2 mb-4 flex flex-wrap items-center gap-x-2 gap-y-1 text-sm text-muted">
              Coach <b className="text-ink">{d.coach.name}</b> · {d.coach.systemName} system · Fit <FitGrade fit={d.teamFit} />
              <Link className="link" to={d.isUser ? '/coach' : `/coach/${d.team.id}`}>Coach page</Link>
            </p>
          )}
          {d.teamMorale && (
            <p className="-mt-2 mb-4 flex flex-wrap items-center gap-x-2 gap-y-1 text-sm text-muted">Team morale <MoraleChip morale={d.teamMorale} /><span className="tabular-nums">{d.teamMorale.score}/100</span></p>
          )}
          <Card title="Payroll" className="mb-4">
            <PayrollBar p={d.payroll} />
            {d.deadMoney && d.deadMoney.length > 0 && (
              <div className="px-2 pb-2 text-[12px]">
                <div className="mb-0.5 font-bold uppercase tracking-wide text-muted">Dead money (released, still paid)</div>
                <ul>{d.deadMoney.map((x, i) => <li key={i} className="flex justify-between gap-3 border-b border-line/40 py-0.5"><span className="min-w-0 truncate">{x.name}</span><span className="shrink-0 tabular-nums text-muted">{money(x.salary)} · {x.years} {x.years === 1 ? 'year' : 'years'} left</span></li>)}</ul>
              </div>
            )}
          </Card>
          <Card title="Roster" right={<span className="flex items-center gap-3 text-xs text-muted">{d.isUser && <Link className="btn !px-2.5 !py-1 text-xs" to="/lineup">Set lineup</Link>}<span>Click a column to sort · starters marked</span></span>}>
            <DataTable data={d.roster} columns={cols} initialSort={[{ id: 'ovr', desc: true }]} onRow={(r) => open(r.id)} />
          </Card>
        </Page>
      )}
    </Q>
  )
}
