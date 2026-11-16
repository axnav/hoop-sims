import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, type OwnerJob } from './api'
import { useFlow } from './flow'
import { ownerKey } from './pages/Owner'
import { Chip, TeamBadge, money } from './ui'

function JobCard({ j, busy, onTake }: { j: OwnerJob; busy: boolean; onTake: () => void }) {
  return (
    <li className="flex flex-col gap-2 rounded-lg border border-line bg-panel2 p-3">
      <div className="flex items-center gap-2 font-bold"><TeamBadge team={j.team} size={28} /><span className="min-w-0">{j.team.fullName}</span></div>
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-[13px]">
        <span className="tabular-nums">Record {j.record}</span>
        <span className="tabular-nums">Payroll {money(j.payroll)}</span>
        <span>{j.marketSize.label} market</span>
        <Chip tone="violet">{j.direction.label}</Chip>
      </div>
      <div className="text-[13px]">
        <span className="font-semibold">{j.owner.name}</span> <Chip>{j.owner.personality.label}</Chip>
        <div className="mt-0.5 text-muted">{j.owner.personality.blurb}</div>
      </div>
      <div className="mt-auto"><button className="btn btn-accent" disabled={busy} onClick={onTake}>Take this job</button></div>
    </li>
  )
}

// Blocks every page while the user has been let go, until one of the offered jobs is taken.
export function FiredDialog() {
  const { state } = useFlow()
  const qc = useQueryClient()
  const nav = useNavigate()
  const fired = !!state?.owner?.fired
  const q = useQuery({ queryKey: ownerKey(state), queryFn: api.owner, enabled: fired })
  const [note, setNote] = useState('')
  const take = useMutation({
    mutationFn: (id: number) => api.ownerJob(id),
    onMutate: () => setNote(''),
    onSuccess: (r) => {
      qc.setQueryData(['state'], r.state)
      qc.setQueryData(ownerKey(r.state), r.owner)
      qc.invalidateQueries({ predicate: (x) => x.queryKey[0] !== 'state' && x.queryKey[0] !== 'owner' })
      nav('/')
    },
    onError: (e) => { setNote(e instanceof Error ? e.message : String(e)); qc.invalidateQueries({ queryKey: ['owner'] }) },
  })
  if (!fired) return null
  const d = q.data
  return (
    <div className="fixed inset-0 z-[80] flex items-start justify-center overflow-y-auto bg-black/70 p-3 min-[700px]:items-center">
      <div role="dialog" aria-modal="true" aria-labelledby="fired-title" className="card my-auto w-full max-w-4xl p-4 shadow-xl">
        <h2 id="fired-title" className="text-2xl font-extrabold tracking-tight">You were let go</h2>
        {q.isError && <p className="mt-2 text-loss">{q.error instanceof Error ? q.error.message : String(q.error)}</p>}
        {!d && !q.isError && <p className="mt-2 text-muted">Loading</p>}
        {d && (
          <>
            <p className="mt-2">{d.firedText}</p>
            <p className="mt-2 text-[13px] text-muted">Choose one of the teams that are hiring.</p>
            <ul className="mt-3 grid gap-3 min-[800px]:grid-cols-3">
              {d.jobs.map((j) => <JobCard key={j.team.id} j={j} busy={take.isPending} onTake={() => take.mutate(j.team.id)} />)}
            </ul>
          </>
        )}
        {note && <div className="mt-3 text-loss" role="alert">{note}</div>}
        <div className="mt-3 text-[13px]"><Link className="link text-accent" to="/owner">Open the Owner page</Link></div>
      </div>
    </div>
  )
}
