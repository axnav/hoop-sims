import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api, type TeamRow, type TeamStatus } from '../api'
import { useFlow } from '../flow'
import { OfferList, useOffers } from '../OffersCard'
import { Card, DataTable, Page, PlayerLink, Q, StatusChip, TeamBadge, money, ovrColor, type Col } from '../ui'

type Filter = 'all' | TeamStatus['key'] | 'offer'
const filters: { id: Filter; label: string }[] = [
  { id: 'all', label: 'All' }, { id: 'contender', label: 'Contenders' }, { id: 'buyer', label: 'Buyers' },
  { id: 'seller', label: 'Sellers' }, { id: 'rebuilding', label: 'Rebuilding' }, { id: 'offer', label: 'Has an offer for you' },
]
const statusOrder: Record<TeamStatus['key'], number> = { contender: 0, buyer: 1, seller: 2, rebuilding: 3 }

export default function Teams() {
  const { state } = useFlow()
  const nav = useNavigate()
  const [filter, setFilter] = useState<Filter>('all')
  const q = useQuery({ queryKey: ['teams', state?.day, state?.phase, state?.offers], queryFn: api.teams })
  const { offers, onAccept, onReject, accept, reject } = useOffers()

  const cols: Col<TeamRow>[] = [
    {
      id: 'team', header: 'Team', accessorFn: (r) => r.team.fullName, sortDescFirst: false,
      cell: (i) => {
        const r = i.row.original
        return <Link to={`/team/${r.team.id}`} className="link inline-flex items-center gap-2"><TeamBadge team={r.team} size={18} /><span className={r.isUser ? 'font-bold' : ''}>{r.team.fullName}</span></Link>
      },
    },
    { id: 'conf', header: 'Conf', hideSmall: true, accessorFn: (r) => r.team.confName, sortDescFirst: false },
    { id: 'rec', header: 'W-L', accessorFn: (r) => r.w - r.l, cell: (i) => `${i.row.original.w}-${i.row.original.l}` },
    { accessorKey: 'confRank', header: 'Rank', num: true, sortDescFirst: false },
    {
      id: 'status', header: 'Status', accessorFn: (r) => statusOrder[r.status.key], sortDescFirst: false,
      cell: (i) => <StatusChip status={i.row.original.status} />,
    },
    { accessorKey: 'top8', header: 'Top 8', num: true, hideSmall: true },
    {
      id: 'star', header: 'Best player', accessorFn: (r) => r.star?.ovr ?? 0,
      cell: (i) => {
        const s = i.row.original.star
        return s ? <span onClick={(e) => e.stopPropagation()}><PlayerLink id={s.id} name={s.name} /> <span className="text-xs text-muted">{s.pos}</span> <b style={{ color: ovrColor(s.ovr) }}>{s.ovr}</b></span> : '-'
      },
    },
    { accessorKey: 'payroll', header: 'Payroll', num: true, cell: (i) => money(i.getValue()) },
    { accessorKey: 'capRoom', header: 'Cap room', num: true, hideSmall: true, cell: (i) => <span className={i.getValue() < 0 ? 'text-loss' : ''}>{money(i.getValue())}</span> },
    { accessorKey: 'rosterSize', header: 'Roster', num: true, hideSmall: true },
    {
      id: 'offer', header: 'Offer', accessorFn: (r) => r.offers,
      cell: (i) => (i.row.original.offers > 0 ? <span className="rounded bg-accent/25 px-1.5 py-px text-[10px] font-semibold uppercase tracking-wide text-accent ring-1 ring-accent/60">Offer</span> : null),
    },
  ]

  return (
    <Page title="Teams">
      <Q q={q} minH={500}>
        {(d) => {
          const rows = d.teams.filter((t) => filter === 'all' ? true : filter === 'offer' ? t.offers > 0 : t.status.key === filter)
          const legend = (Object.keys(statusOrder) as TeamStatus['key'][]).map((k) => d.teams.find((t) => t.status.key === k)?.status).filter((s): s is TeamStatus => !!s)
          const hasOffers = d.teams.some((t) => t.offers > 0)
          return (
            <>
              {hasOffers && offers.length > 0 && (
                <div className="mb-4">
                  <h2 className="h2 mb-2 px-1">Teams with offers for you</h2>
                  <OfferList offers={offers} onChangeIt={(id) => nav(`/trade?offer=${id}`)} onAccept={onAccept} onReject={onReject} accept={accept} reject={reject} />
                </div>
              )}
              <div className="mb-3 flex flex-wrap gap-2">
                {filters.map((f) => (
                  <button key={f.id} className={`btn ${filter === f.id ? 'btn-accent' : ''}`} onClick={() => setFilter(f.id)}>{f.label}</button>
                ))}
              </div>
              <Card>
                <DataTable
                  data={rows} columns={cols} rowClass={(r) => (r.isUser ? 'me' : '')}
                  initialSort={[{ id: 'status', desc: false }, { id: 'top8', desc: true }]} onRow={(r) => nav(`/team/${r.team.id}`)}
                />
              </Card>
              {legend.length > 0 && (
                <ul className="mt-3 space-y-1.5 text-xs text-muted">
                  {legend.map((s) => <li key={s.key} className="flex items-start gap-2"><StatusChip status={s} /><span>{s.wants}</span></li>)}
                </ul>
              )}
            </>
          )
        }}
      </Q>
    </Page>
  )
}
