import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Check } from 'lucide-react'
import { api, type AssetRow, type Offer } from './api'
import { useFlow } from './flow'
import { Card, Delta, ErrorBox, InjuryTag, TeamBadge, TierChip, money } from './ui'

function Assets({ title, rows }: { title: string; rows: AssetRow[] }) {
  return (
    <div className="min-w-0">
      <div className="h2 mb-1">{title}</div>
      <ul>
        {rows.map((r, i) => (
          <li key={i} className="flex flex-wrap items-center gap-x-1 border-b border-line/50 py-1 text-[13px]">
            <span className="min-w-[7rem] flex-1 truncate font-medium">
              {r.kind === 'player' ? r.name : r.label}
              {r.kind === 'player' && <InjuryTag injury={r.injury} />}
              <TierChip tier={r.tier} />
            </span>
            {r.kind === 'player' && <span className="text-xs text-muted tabular-nums">{r.pos} · {r.age}y · {r.ovr}<Delta value={r.ovrChange} />/{r.pot}<Delta value={r.potChange} /></span>}
            {r.kind === 'player' && <span className="text-xs tabular-nums" title={`${r.years} years left`}>{money(r.salary)}</span>}
            <span className="w-10 text-right text-xs text-muted tabular-nums" title="Trade value">{r.value}</span>
          </li>
        ))}
      </ul>
    </div>
  )
}

// Data and actions for the pending offers; shared by the Dashboard card and the sim window.
export function useOffers() {
  const { state } = useFlow()
  const qc = useQueryClient()
  const [accepted, setAccepted] = useState(false)
  const key = ['offers', state?.day, state?.phase, state?.offers]
  const q = useQuery({ queryKey: key, queryFn: api.offers })
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
      qc.setQueryData(key, { offers: r.offers })
      qc.invalidateQueries({ queryKey: ['state'] })
      qc.invalidateQueries({ queryKey: ['offers'] })
    },
  })
  const offers = q.data?.offers ?? []
  return {
    offers, accepted, accept, reject,
    onAccept: (id: number) => { setAccepted(false); accept.mutate(id) },
    onReject: (id: number) => reject.mutate(id),
  }
}

export function OfferList({ offers, compact = false, onChangeIt, onAccept, onReject, accept, reject }: {
  offers: Offer[]; compact?: boolean; onChangeIt: (id: number) => void
  onAccept: (id: number) => void; onReject: (id: number) => void
  accept: { isPending: boolean; isError: boolean; error: unknown; variables?: number }
  reject: { isPending: boolean }
}) {
  const busy = accept.isPending || reject.isPending
  const body = (o: Offer) => (
    <div className={compact ? 'p-2' : 'px-2 pb-2'}>
      <div className="mb-2 italic text-muted">&ldquo;{o.message}&rdquo;</div>
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <Assets title="You give" rows={o.give} />
        <Assets title="You get" rows={o.get} />
      </div>
      <div className="mt-3 flex flex-wrap gap-2">
        <button className="btn btn-accent" disabled={busy} onClick={() => onAccept(o.id)}>Accept</button>
        <button className="btn" disabled={busy} onClick={() => onReject(o.id)}>Decline</button>
        <button className="btn" onClick={() => onChangeIt(o.id)}>Change it</button>
      </div>
      {accept.isError && accept.variables === o.id && <div className="mt-2"><ErrorBox error={accept.error} /></div>}
    </div>
  )
  const title = (o: Offer) => <span className="flex items-center gap-2"><TeamBadge team={o.team} size={18} />Offer from {o.team.fullName}</span>
  const expires = (o: Offer) => <span className="text-[11px] text-muted">expires in {o.daysLeft} {o.daysLeft === 1 ? 'day' : 'days'}</span>
  return (
    <div className="space-y-3">
      {offers.map((o) => compact ? (
        <div key={o.id} className="rounded-lg border border-line bg-panel2/40">
          <div className="flex items-center justify-between gap-2 border-b border-line/60 px-2 py-1.5 font-semibold">{title(o)}{expires(o)}</div>
          {body(o)}
        </div>
      ) : (
        <Card key={o.id} title={title(o)} right={expires(o)}>{body(o)}</Card>
      ))}
    </div>
  )
}

export function OffersCard() {
  const nav = useNavigate()
  const { offers, accepted, accept, reject, onAccept, onReject } = useOffers()
  if (offers.length === 0 && !accepted) return null
  return (
    <div className="mb-4 space-y-3">
      {accepted && offers.length === 0 && (
        <div className="card flex items-center gap-2 border-win/50 p-3 font-semibold text-win"><Check size={16} />Trade complete</div>
      )}
      <OfferList offers={offers} onChangeIt={(id) => nav(`/trade?offer=${id}`)} onAccept={onAccept} onReject={onReject} accept={accept} reject={reject} />
    </div>
  )
}
