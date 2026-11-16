import { useState } from 'react'
import { Link } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, type GM, type GMArea, type GMAutoKey, type GMSuggestion } from '../api'
import { useFlow } from '../flow'
import { useOpenPlayer } from '../Inline'
import { ownerKey } from './Owner'
import { Card, Chip, Page, Q } from '../ui'

function Toggle({ label, checked, disabled, onChange }: { label: string; checked: boolean; disabled?: boolean; onChange: (v: boolean) => void }) {
  return (
    <button
      type="button" role="switch" aria-checked={checked} disabled={disabled} onClick={() => onChange(!checked)}
      className="flex items-center gap-2 text-left text-[13px] disabled:opacity-60"
    >
      <span className={`relative h-5 w-9 shrink-0 rounded-full transition-colors ${checked ? 'bg-accent' : 'bg-line'}`}>
        <span className={`absolute top-0.5 h-4 w-4 rounded-full bg-white transition-all ${checked ? 'left-[18px]' : 'left-0.5'}`} />
      </span>
      <span>{label}</span>
    </button>
  )
}

function SuggestionRow({ s, busy, onApply }: { s: GMSuggestion; busy: boolean; onApply: (s: GMSuggestion) => void }) {
  const open = useOpenPlayer()
  return (
    <li className="border-t border-line/50 px-3 py-3 first:border-t-0">
      <div className="font-bold">{s.title}</div>
      {s.reasons.length > 0 && (
        <ul className="mt-1 list-disc space-y-0.5 pl-5 text-[13px] text-muted">
          {s.reasons.map((r, i) => <li key={i}>{r}</li>)}
        </ul>
      )}
      <div className="mt-2 flex flex-wrap items-center gap-2">
        <button className="btn btn-accent" disabled={busy} onClick={() => onApply(s)}>{s.action}</button>
        {s.link && <Link className={`btn ${busy ? 'pointer-events-none opacity-60' : ''}`} to={s.link} aria-disabled={busy}>Look first</Link>}
        {s.playerId !== null && <button className="btn" disabled={busy} onClick={() => open(s.playerId!)}>Player</button>}
      </div>
    </li>
  )
}

function AreaCard({ area, gm, busy, saving, onApply, onAuto }: {
  area: GMArea; gm: GM; busy: boolean; saving: boolean
  onApply: (s: GMSuggestion) => void; onAuto: (k: GMAutoKey, v: boolean) => void
}) {
  const list = gm.suggestions.filter((s) => s.area === area.key)
  return (
    <Card
      title={<span className="flex items-center gap-2">{area.label}<Chip tone={list.length ? 'strong' : 'neutral'}>{list.length}</Chip></span>}
    >
      <div className="flex flex-wrap items-start justify-between gap-x-4 gap-y-2 px-2 pb-2">
        <div className="min-w-0 text-[13px] text-muted">{area.blurb}</div>
        <div className="flex flex-col gap-1">
          <Toggle label={area.autoLabel} checked={gm.auto[area.autoKey]} disabled={saving} onChange={(v) => onAuto(area.autoKey, v)} />
          {!area.canAuto && <div className="pl-11 text-xs text-muted">Trades always need your yes.</div>}
        </div>
      </div>
      {list.length === 0
        ? <div className="border-t border-line/50 px-3 py-3 text-muted">Nothing to suggest right now.</div>
        : <ul>{list.map((s) => <SuggestionRow key={s.id} s={s} busy={busy} onApply={onApply} />)}</ul>}
    </Card>
  )
}

function OwnerLine() {
  const { state } = useFlow()
  const q = useQuery({ queryKey: ownerKey(state), queryFn: api.owner, enabled: !!state?.owner && state.owner.label !== '' })
  const d = q.data
  if (!d || !d.active) return null
  return (
    <div className="card px-3 py-2 text-[13px]">
      <span className="text-muted">Owner's goals: </span>{d.goals.map((g) => g.title).join(' \u00b7 ')}
      {' '}<Link className="link text-accent" to="/owner">Owner page</Link>
    </div>
  )
}

export default function Assistant() {
  const qc = useQueryClient()
  const { state } = useFlow()
  const key = ['gm', state?.year, state?.phase, state?.day]
  const q = useQuery({ queryKey: key, queryFn: api.gm })
  const [note, setNote] = useState<{ text: string; bad: boolean } | null>(null)
  const [shown, setShown] = useState(30)

  const settings = useMutation({
    mutationFn: api.gmSettings,
    onSuccess: (g) => qc.setQueryData(key, g),
    onError: (e) => { setNote({ text: e instanceof Error ? e.message : String(e), bad: true }); qc.invalidateQueries({ queryKey: ['gm'] }) },
  })
  const apply = useMutation({
    mutationFn: (s: GMSuggestion) => api.gmApply(s.id),
    onMutate: () => setNote(null),
    onSuccess: (r) => {
      setNote({ text: r.message, bad: false })
      qc.setQueryData(['state'], r.state)
      qc.setQueryData(['gm', r.state.year, r.state.phase, r.state.day], r.gm)
      qc.invalidateQueries({ predicate: (x) => x.queryKey[0] !== 'state' && x.queryKey[0] !== 'gm' })
    },
    onError: (e) => { setNote({ text: e instanceof Error ? e.message : String(e), bad: true }); qc.invalidateQueries({ queryKey: ['gm'] }) },
  })

  return (
    <Page title="Assistant">
      <div className="space-y-4">
        <OwnerLine />
        <Q q={q} minH={240}>{(gm) => (
          <>
            <Card title="Plan">
              <div role="radiogroup" aria-label="Plan" className="grid gap-2 p-2 min-[700px]:grid-cols-3">
                {gm.plans.map((p) => {
                  const on = gm.plan === p.key
                  return (
                    <button
                      key={p.key} type="button" role="radio" aria-checked={on} disabled={settings.isPending}
                      onClick={() => !on && settings.mutate({ plan: p.key })}
                      className={`rounded-lg border p-3 text-left disabled:opacity-60 ${on ? 'border-accent bg-accent/10' : 'border-line bg-panel2 hover:border-muted'}`}
                    >
                      <div className={`font-bold ${on ? 'text-accent' : ''}`}>{p.label}</div>
                      <div className="mt-0.5 text-[13px] text-muted">{p.blurb}</div>
                    </button>
                  )
                })}
              </div>
            </Card>
            {note && <div className={`card p-3 ${note.bad ? 'border-loss/50 text-loss' : 'border-win/50 text-win'}`} role="status">{note.text}</div>}
            {gm.areas.map((a) => (
              <AreaCard
                key={a.key} area={a} gm={gm} busy={apply.isPending} saving={settings.isPending}
                onApply={(s) => apply.mutate(s)} onAuto={(k, v) => settings.mutate({ auto: { [k]: v } })}
              />
            ))}
            <Card title="What the assistant did">
              {gm.log.length === 0
                ? <div className="px-3 py-3 text-muted">The assistant has not done anything yet.</div>
                : (
                  <ul>
                    {gm.log.slice(0, shown).map((e, i) => (
                      <li key={i} className="flex flex-wrap items-baseline gap-x-2 border-t border-line/50 px-3 py-1.5 text-[13px] first:border-t-0">
                        <span className="text-muted tabular-nums">{e.year} · Day {e.day} · {gm.areas.find((a) => a.key === e.area)?.label ?? e.area}</span>
                        {e.auto && <Chip>auto</Chip>}
                        <span className="min-w-0">{e.text}</span>
                      </li>
                    ))}
                  </ul>
                )}
              {gm.log.length > shown && <div className="p-2"><button className="btn" onClick={() => setShown((n) => n + 30)}>Show more</button></div>}
            </Card>
          </>
        )}</Q>
      </div>
    </Page>
  )
}
