import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, type Expiring } from './api'
import { useFlow } from './flow'
import { Card, Delta, ErrorBox, PayrollBar, NameCell, PlayerLink, RfaChip, StanceChip, TierChip, money } from './ui'

export function ExpiringCard() {
  const { state } = useFlow()
  const qc = useQueryClient()
  const key = ['contracts', 'expiring', state?.day, state?.phase, state?.expiring]
  const q = useQuery({ queryKey: key, queryFn: api.contractsExpiring })
  const done = (r: { expiring: Expiring; state: import('./api').GameState }) => {
    qc.setQueryData(key, r.expiring)
    qc.setQueryData(['state'], r.state)
    qc.invalidateQueries({ queryKey: ['team'] })
    qc.invalidateQueries({ queryKey: ['trade-block'] })
  }
  const resign = useMutation({ mutationFn: (id: number) => api.contractResign(id), onSuccess: done })
  const release = useMutation({ mutationFn: (id: number) => api.contractRelease(id), onSuccess: done })
  const d = q.data
  if (!d || d.players.length === 0) return null
  const busy = resign.isPending || release.isPending
  return (
    <Card title={`Contracts ending (${d.players.length})`} className="mb-4">
      <div className="px-2 pb-1 text-[13px] text-muted">{d.open ? 'Players you do not re-sign become free agents when free agency begins.' : 'Free agency has begun, so these contracts can no longer be renewed here. Players not re-signed are in the free-agent pool.'}</div>
      {d.players.some((p) => p.restricted) && <div className="px-2 pb-1 text-[13px] text-muted">Restricted free agents (RFA): if you do not re-sign him now, you can still match any offer he gets in free agency.</div>}
      <PayrollBar p={d.payroll} />
      <ul className="px-2 pb-1">
        {d.players.map((p) => (
          <li key={p.id} className="flex flex-wrap items-center gap-x-3 gap-y-1 border-b border-line/50 py-1.5 text-[13px]">
            <span className="min-w-0 flex-1 basis-40"><NameCell name={<PlayerLink id={p.id} name={p.name} />} chips={<>{p.restricted && <RfaChip title="Restricted free agent: if you do not re-sign him now, you can still match any offer he gets in free agency." />}<StanceChip stance={p.stance} extended={p.extended} /><TierChip tier={p.tier} /></>} /></span>
            <span className="text-xs text-muted tabular-nums">{p.pos} &middot; {p.age}y &middot; {p.ovr}<Delta value={p.ovrChange} scope={p.changeScope} />/{p.pot}<Delta value={p.potChange} scope={p.changeScope} /></span>
            <span className="text-xs tabular-nums">Asks {money(p.askSalary)} for {p.askYears} {p.askYears === 1 ? 'year' : 'years'}</span>
            {d.open && (
              <span className="flex flex-wrap items-center gap-2">
                {!p.canReSign && p.problem && <span className="text-[11px] text-loss">{p.problem}</span>}
                <button className="btn btn-accent" disabled={busy || !p.canReSign} title={p.canReSign ? undefined : p.problem} onClick={() => resign.mutate(p.id)}>Re-sign</button>
                <button className="btn" disabled={busy} onClick={() => release.mutate(p.id)}>Let go</button>
              </span>
            )}
          </li>
        ))}
      </ul>
      {resign.isError && <ErrorBox error={resign.error} />}
      {release.isError && <ErrorBox error={release.error} />}
    </Card>
  )
}
