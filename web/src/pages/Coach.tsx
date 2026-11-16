import { useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, type CoachData, type CoachInfo } from '../api'
import { useFlow } from '../flow'
import { useOpenPlayer } from '../Inline'
import { Card, ConfirmDialog, FitGrade, Page, Q } from '../ui'

const errText = (e: unknown) => (e instanceof Error ? e.message : String(e))

function Bar({ label, v }: { label: string; v: number }) {
  return (
    <div>
      <div className="flex justify-between text-xs text-muted"><span>{label}</span><b className="tabular-nums text-ink">{v}</b></div>
      <div className="mt-1 h-2 overflow-hidden rounded-full bg-panel2"><div className="h-full rounded-full bg-accent" style={{ width: `${Math.max(0, Math.min(100, v))}%` }} /></div>
    </div>
  )
}

function Ratings({ c }: { c: CoachInfo }) {
  return (
    <div className="grid grid-cols-3 gap-x-4 gap-y-2 max-[500px]:grid-cols-1">
      <Bar label="Offense" v={c.offense} /><Bar label="Defense" v={c.defense} /><Bar label="Development" v={c.development} />
    </div>
  )
}

const shift = (label: string, v: string) => (
  <span className="rounded bg-panel2 px-1.5 py-0.5 text-[11px] text-muted">{label} <b className="text-ink">{v}</b></span>
)

function Market({ d, teamId }: { d: CoachData; teamId: number }) {
  const qc = useQueryClient()
  const q = useQuery({ queryKey: ['coach-pool'], queryFn: api.coachPool })
  const [pick, setPick] = useState<(CoachInfo & { id: number }) | null>(null)
  const hire = useMutation({
    mutationFn: (id: number) => api.coachHire(id),
    onSuccess: (r) => {
      qc.setQueryData(['coach', teamId], r)
      qc.invalidateQueries({ queryKey: ['coach-pool'] })
      qc.invalidateQueries({ queryKey: ['lineup'] })
      qc.invalidateQueries({ queryKey: ['team'] })
      setPick(null)
    },
    onError: () => setPick(null),
  })
  return (
    <Card title="Coaching market" className="mb-4" right={<span className="text-xs text-muted">Open in the offseason</span>}>
      <div className="px-4 pb-3">
        {hire.isError && <p className="mb-2 text-sm text-loss" role="alert">{errText(hire.error)}</p>}
        <Q q={q}>
          {(p) => (
            <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
              {p.candidates.map((c) => (
                <div key={c.id} className="rounded-lg border border-line bg-panel2/40 p-3">
                  <div className="mb-2 flex items-baseline justify-between gap-2">
                    <b>{c.name}</b><span className="text-xs text-muted">Age {c.age}</span>
                  </div>
                  <Ratings c={c} />
                  <div className="mt-2 flex items-center justify-between gap-2">
                    <span className="text-xs text-muted">Favourite: <b className="text-ink">{c.systemName}</b></span>
                    <button className="btn" disabled={hire.isPending} onClick={() => { hire.reset(); setPick(c) }}>Hire</button>
                  </div>
                </div>
              ))}
              {p.candidates.length === 0 && <p className="text-sm text-muted">No candidates right now.</p>}
            </div>
          )}
        </Q>
      </div>
      {pick && (
        <ConfirmDialog
          message={`Replace ${d.coach.name} with ${pick.name}? The team's system will switch to ${pick.systemName}.`}
          confirmLabel="Hire" onConfirm={() => hire.mutate(pick.id)} onCancel={() => setPick(null)}
        />
      )}
    </Card>
  )
}

export default function Coach() {
  const { state } = useFlow()
  const params = useParams()
  const userId = state!.user.team.id
  const teamId = params.teamId !== undefined ? Number(params.teamId) : userId
  const q = useQuery({ queryKey: ['coach', teamId], queryFn: () => api.coach(teamId) })
  return <Q q={q} minH={400}>{(d) => <CoachView d={d} teamId={teamId} />}</Q>
}

function CoachView({ d, teamId }: { d: CoachData; teamId: number }) {
  const qc = useQueryClient()
  const open = useOpenPlayer()
  const set = useMutation({
    mutationFn: (key: string) => api.coachSystem(key),
    onSuccess: (r) => {
      qc.setQueryData(['coach', teamId], r)
      qc.invalidateQueries({ queryKey: ['lineup'] })
      qc.invalidateQueries({ queryKey: ['team'] })
    },
  })
  const c = d.coach
  const mine = d.editable
  return (
    <Page title={<span className="flex flex-wrap items-center gap-3">Coach<span className="text-base font-semibold text-muted">{c.name}</span></span>}
      right={mine ? <Link className="link" to="/lineup">Lineup</Link> : <Link className="link" to={`/team/${teamId}`}>Team page</Link>}>
      <div className="mb-4 grid gap-4 md:grid-cols-[1fr_auto]">
        <Card title="Head coach">
          <div className="px-4 pb-3">
            <div className="mb-3 text-sm text-muted"><b className="text-base text-ink">{c.name}</b> · Age {c.age} · Favourite system <b className="text-ink">{c.systemName}</b></div>
            <Ratings c={c} />
            <p className="mt-3 text-sm">Playing the <b>{d.systemName}</b> system{d.favourite ? ' (the coach\'s favourite)' : ''}.</p>
          </div>
        </Card>
        <Card title="Team fit">
          <div className="flex items-center gap-4 px-4 pb-4">
            <FitGrade fit={d.teamFit} size="lg" />
            <div className="text-sm text-muted">Score <b className="tabular-nums text-ink">{d.teamFit.score}</b><br />Roster vs coach and system</div>
          </div>
        </Card>
      </div>

      {d.canHire && mine && <Market d={d} teamId={teamId} />}

      <Card title="Systems" className="mb-4">
        <div className="px-4 pb-3">
          <p className="mb-3 text-sm text-muted">The system sets how the team plays. Any system other than the coach's favourite lowers fit. The grade on each card is what this roster would get under it.</p>
          {set.isError && <p className="mb-2 text-sm text-loss" role="alert">{errText(set.error)}</p>}
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
            {d.systems.map((s) => {
              const inUse = s.key === d.system
              const fav = s.key === c.system
              const can = d.canChangeSystem && !inUse && !set.isPending
              return (
                <button key={s.key} type="button" disabled={!can} onClick={() => set.mutate(s.key)} aria-pressed={inUse}
                  className={`flex flex-col justify-start rounded-lg border p-3 text-left ${inUse ? 'border-accent bg-accent/10' : 'border-line bg-panel2/40'} ${can ? 'hover:border-accent/70' : 'cursor-default'}`}>
                  <div className="mb-1 flex items-start justify-between gap-2">
                    <div>
                      <b>{s.name}</b>
                      <div className="mt-0.5 flex flex-wrap gap-1">
                        {inUse && <span className="rounded bg-accent px-1.5 py-px text-[10px] font-bold uppercase text-black">In use</span>}
                        {fav && <span className="rounded bg-panel2 px-1.5 py-px text-[10px] font-bold uppercase text-ink ring-1 ring-line">Coach's favourite</span>}
                      </div>
                    </div>
                    <FitGrade fit={s.teamFit} size="lg" />
                  </div>
                  <p className="my-2 text-sm text-muted">{s.description}</p>
                  <div className="mb-2 flex flex-wrap gap-1">{shift('Pace', s.pace)}{shift('Threes', s.threes)}{shift('Stars', s.starUsage)}{shift('Defense', s.defense)}</div>
                  <div className="text-[11px] font-bold uppercase tracking-wider text-muted">Wants</div>
                  {s.wants.length ? <ul className="list-disc pl-4 text-sm">{s.wants.map((w) => <li key={w}>{w}</li>)}</ul> : <p className="text-sm text-muted">Nothing in particular.</p>}
                </button>
              )
            })}
          </div>
        </div>
      </Card>

      <Card title="Player fit">
        <div className="overflow-x-auto">
          <table className="tbl">
            <thead><tr><th>Player</th><th>Pos</th><th className="r">Min</th><th>Fit</th><th>Why</th></tr></thead>
            <tbody>
              {d.players.map((p) => (
                <tr key={p.id} className="cursor-pointer" onClick={() => open(p.id)}>
                  <td><button className="link font-medium text-left" onClick={(e) => { e.stopPropagation(); open(p.id) }}>{p.name}</button></td>
                  <td>{p.pos}</td>
                  <td className="r tabular-nums">{p.minutes}</td>
                  <td><FitGrade fit={p.fit} /></td>
                  <td className="min-w-[14rem] text-sm text-muted">{p.fit.reasons.join(' · ')}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Card>
    </Page>
  )
}
