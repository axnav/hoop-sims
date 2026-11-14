import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Check } from 'lucide-react'
import { api, type AssetRow } from './api'
import { useFlow } from './flow'
import { Card, ErrorBox, InjuryTag, TeamBadge, TierChip, money } from './ui'

function Assets({ title, rows }: { title: string; rows: AssetRow[] }) {
  return (
    <div className="min-w-0">
      <div className="h2 mb-1">{title}</div>
      <ul>
        {rows.map((r, i) => (
          <li key={i} className="flex items-center gap-1 border-b border-line/50 py-1 text-[13px]">
            <span className="min-w-0 flex-1 truncate font-medium">
              {r.kind === 'player' ? r.name : r.label}
              {r.kind === 'player' && <InjuryTag injury={r.injury} />}
              <TierChip tier={r.tier} />
            </span>
            {r.kind === 'player' && <span className="text-xs text-muted tabular-nums">{r.pos} · {r.age}y · {r.ovr}/{r.pot}</span>}
            {r.kind === 'player' && <span className="text-xs tabular-nums" title={`${r.years} years left`}>{money(r.salary)}</span>}
            <span className="w-10 text-right text-xs text-muted tabular-nums" title="Trade value">{r.value}</span>
          </li>
        ))}
      </ul>
    </div>
  )
}

export function OffersCard() {
  const { state } = useFlow()
  const qc = useQueryClient()
  const nav = useNavigate()
  const [accepted, setAccepted] = useState(false)
  const q = useQuery({ queryKey: ['offers', state?.day, state?.phase, state?.offers], queryFn: api.offers })
  const accept = useMutation({
    mutationFn: (id: number) => api.offerAccept(id),
    onSuccess: (r) => {
      setAccepted(true)
      qc.setQueryData(['state'], r.state)
      qc.invalidateQueries({ predicate: (query) => query.queryKey[0] !== 'state' })
    },
  })
  const reject = useMutation({
    mutationFn: (id: number) => api.offerReject(id),
    onSuccess: (r) => {
      qc.setQueryData(['offers', state?.day, state?.phase, state?.offers], { offers: r.offers })
      qc.invalidateQueries({ queryKey: ['state'] })
      qc.invalidateQueries({ queryKey: ['offers'] })
    },
  })
  const offers = q.data?.offers ?? []
  if (offers.length === 0 && !accepted) return null
  return (
    <div className="mb-4 space-y-3">
      {accepted && offers.length === 0 && (
        <div className="card flex items-center gap-2 border-win/50 p-3 font-semibold text-win"><Check size={16} />Trade complete</div>
      )}
      {offers.map((o) => (
        <Card key={o.id} title={<span className="flex items-center gap-2"><TeamBadge team={o.team} size={18} />Offer from {o.team.fullName}</span>}
          right={<span className="text-[11px] text-muted">expires in {o.daysLeft} {o.daysLeft === 1 ? 'day' : 'days'}</span>}>
          <div className="px-2 pb-2">
            <div className="mb-2 italic text-muted">&ldquo;{o.message}&rdquo;</div>
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <Assets title="You give" rows={o.give} />
              <Assets title="You get" rows={o.get} />
            </div>
            <div className="mt-3 flex flex-wrap gap-2">
              <button className="btn btn-accent" disabled={accept.isPending || reject.isPending} onClick={() => { setAccepted(false); accept.mutate(o.id) }}>Accept</button>
              <button className="btn" disabled={accept.isPending || reject.isPending} onClick={() => reject.mutate(o.id)}>Decline</button>
              <button className="btn" onClick={() => nav(`/trade?offer=${o.id}`)}>Change it</button>
            </div>
            {accept.isError && accept.variables === o.id && <div className="mt-2"><ErrorBox error={accept.error} /></div>}
          </div>
        </Card>
      ))}
    </div>
  )
}
