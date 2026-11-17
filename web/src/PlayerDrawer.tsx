import { useEffect, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AnimatePresence, motion } from 'motion/react'
import { Search, Sparkles, X } from 'lucide-react'
import { api, type ExtensionAsk, type PlayerPage, type ReleasePreview, type Training, type StatLine, type Badge } from './api'
import { useFlow } from './flow'
import { AdviceChip, BadgeChip, Chip, groupLabel, groupOrder, tierLabel, tierOrder, Avatar, Delta, ErrorBox, InjuryTag, Q, RangeBar, Ring, MoraleChip, moraleColor, StanceChip, TeamBadge, injuryText, levelClass, motivationRows, ovrColor, f1, money, rangeText } from './ui'
import { useScout } from './scout'

function Stats({ title, s }: { title: string; s: StatLine }) {
  if (!s) return null
  const cells: [string, string][] = [
    ['GP', String(s.gp)], ['MPG', f1(s.mpg)], ['PPG', f1(s.ppg)], ['RPG', f1(s.rpg)], ['APG', f1(s.apg)],
    ['TOV', f1(s.tpg)], ['FG%', f1(s.fg)], ['3P%', f1(s.tp)], ['FT%', f1(s.ft)],
  ]
  return (
    <div className="mt-4">
      <div className="h2 mb-1">{title}</div>
      <div className="grid grid-cols-9 gap-1 text-center">
        {cells.map(([k, v]) => (
          <div key={k} className="rounded bg-panel2 py-1.5">
            <div className="text-[10px] text-muted">{k}</div>
            <div className="font-bold tabular-nums">{v}</div>
          </div>
        ))}
      </div>
    </div>
  )
}

// Awards grouped by title: "MVP 2027, 2029". Hidden when empty or missing.
function AwardsSection({ awards }: { awards?: { year: number; title: string }[] | null }) {
  if (!awards || awards.length === 0) return null
  const groups = new Map<string, number[]>()
  for (const a of awards) groups.set(a.title, [...(groups.get(a.title) ?? []), a.year])
  return (
    <div className="mt-3">
      <div className="h2 mb-1">Awards</div>
      <div className="flex flex-wrap gap-1.5">
        {[...groups].map(([t, ys]) => <Chip key={t} tone="good" className="whitespace-normal normal-case">{t} {[...ys].sort((a, b) => b - a).join(', ')}</Chip>)}
      </div>
    </div>
  )
}

function BadgesSection({ badges }: { badges: Badge[] }) {
  const [all, setAll] = useState(false)
  const [sel, setSel] = useState('')
  if (badges.length === 0) return null
  const pers = badges.filter((b) => b.kind === 'personality')
  const skills = badges.filter((b) => b.kind !== 'personality')
  const counts = tierOrder.map((t) => [t, skills.filter((b) => b.tier === t).length] as const).filter(([, n]) => n > 0)
  const shown = all ? skills : skills.slice(0, 10)
  const chosen = badges.find((b) => b.key === sel)
  const toggle = (k: string) => setSel(sel === k ? '' : k)
  const detail = chosen && (
    <div className="mt-2 rounded-lg border border-line bg-panel2 p-2.5 text-xs">
      <div className="font-bold">{chosen.name}{chosen.tier && <span className="text-muted"> · {tierLabel[chosen.tier]}</span>}</div>
      <div className="mt-1"><span className="text-muted">Rule: </span>{chosen.rule}</div>
      <div className="mt-0.5"><span className="text-muted">Effect: </span>{chosen.effect}</div>
      {chosen.kind === 'performance' && <div className="mt-0.5 text-muted">Raised one tier by last season's stats.</div>}
      {chosen.next && <div className="mt-0.5"><span className="text-muted">Next: </span>{chosen.next}</div>}
    </div>
  )
  return (
    <div className="mt-5">
      <div className="h2 mb-1">Badges</div>
      <div className="mb-2 text-xs text-muted">
        {badges.length} badges{counts.map(([t, n]) => ` · ${n} ${tierLabel[t]}`).join('')}
      </div>
      {pers.length > 0 && <div className="mb-2 flex flex-wrap gap-1.5">{pers.map((b) => <BadgeChip key={b.key} b={b} active={sel === b.key} onClick={() => toggle(b.key)} />)}</div>}
      {groupOrder.map((g) => {
        const list = shown.filter((b) => b.group === g)
        if (list.length === 0) return null
        return (
          <div key={g} className="mb-2">
            <div className="mb-1 text-[10px] font-bold uppercase tracking-wide text-muted">{groupLabel[g]}</div>
            <div className="flex flex-wrap gap-1.5">{list.map((b) => <BadgeChip key={b.key} b={b} active={sel === b.key} onClick={() => toggle(b.key)} />)}</div>
          </div>
        )
      })}
      {detail}
      {skills.length > 10 && (
        <button type="button" className="mt-1 text-xs font-semibold text-accent hover:underline" onClick={() => setAll(!all)}>
          {all ? 'Show fewer' : `Show all ${skills.length}`}
        </button>
      )}
    </div>
  )
}

function Line({ data }: { data: { year: number; ovr: number; pot: number }[] }) {
  if (data.length < 2) return null
  const w = 400, h = 90, pad = 8
  const vals = data.flatMap((d) => [d.ovr, d.pot])
  const lo = Math.min(...vals) - 3, hi = Math.max(...vals) + 3
  const x = (i: number) => pad + (i * (w - 2 * pad)) / (data.length - 1)
  const y = (v: number) => h - pad - ((v - lo) / (hi - lo)) * (h - 2 * pad)
  const path = (k: 'ovr' | 'pot') => data.map((d, i) => `${i ? 'L' : 'M'}${x(i)},${y(d[k])}`).join(' ')
  return (
    <svg viewBox={`0 0 ${w} ${h}`} className="mt-2 w-full">
      <motion.path d={path('pot')} fill="none" stroke="var(--color-muted)" strokeDasharray="3 4" strokeWidth={1.5} initial={{ pathLength: 0 }} animate={{ pathLength: 1 }} />
      <motion.path d={path('ovr')} fill="none" stroke="var(--color-accent)" strokeWidth={2.5} initial={{ pathLength: 0 }} animate={{ pathLength: 1 }} transition={{ duration: 0.8 }} />
      {data.map((d, i) => <circle key={d.year} cx={x(i)} cy={y(d.ovr)} r={2.5} fill="var(--color-accent)" />)}
    </svg>
  )
}

function EditForm({ p, onDone }: { p: PlayerPage; onDone: () => void }) {
  const qc = useQueryClient()
  const L = p.limits
  const [ratings, setRatings] = useState<Record<string, string>>(() => Object.fromEntries(p.ratings.map((r) => [r.key, String(r.value)])))
  const [age, setAge] = useState(String(p.age))
  const [pot, setPot] = useState(String(p.pot))
  const [heal, setHeal] = useState(false)
  const save = useMutation({
    mutationFn: (b: Parameters<typeof api.adminEditPlayer>[1]) => api.adminEditPlayer(p.id, b),
    onSuccess: (np) => {
      qc.setQueryData(['player', p.id], np)
      for (const k of ['team', 'trade-block', 'leaders', 'injuries', 'state']) qc.invalidateQueries({ queryKey: [k] })
      onDone()
    },
  })
  const submit = () => {
    const body: Parameters<typeof api.adminEditPlayer>[1] = {}
    const changed: Record<string, number> = {}
    for (const r of p.ratings) {
      const v = Number(ratings[r.key])
      if (ratings[r.key] !== '' && v !== r.value) changed[r.key] = v
    }
    if (Object.keys(changed).length) body.ratings = changed
    if (age !== '' && Number(age) !== p.age) body.age = Number(age)
    if (pot !== '' && Number(pot) !== p.pot) body.potential = Number(pot)
    if (heal) body.heal = true
    save.mutate(body)
  }
  const num = (v: string, set: (s: string) => void, min: number, max: number, label: string) => (
    <input type="number" inputMode="numeric" min={min} max={max} value={v} aria-label={label} onChange={(e) => set(e.target.value)}
      className="w-16 rounded border border-line bg-panel2 px-2 py-1 text-right tabular-nums" />
  )
  return (
    <div className="mt-4 rounded-lg border border-accent/50 bg-panel2/50 p-3">
      <div className="h2 mb-2">Edit player</div>
      <div className="grid grid-cols-1 gap-x-6 gap-y-1.5 sm:grid-cols-2">
        {p.ratings.map((r) => (
          <label key={r.key} className="flex items-center justify-between gap-2">
            <span className="text-muted">{r.label}</span>
            {num(ratings[r.key], (s) => setRatings((m) => ({ ...m, [r.key]: s })), L.minRating, L.maxRating, r.label)}
          </label>
        ))}
        <label className="flex items-center justify-between gap-2"><span className="text-muted">Age</span>{num(age, setAge, L.minAge, L.maxAge, 'Age')}</label>
        <label className="flex items-center justify-between gap-2"><span className="text-muted">Potential</span>{num(pot, setPot, L.minRating, L.maxRating, 'Potential')}</label>
      </div>
      {p.injury && (
        <label className="mt-3 flex items-center gap-2 text-[13px]">
          <input type="checkbox" checked={heal} onChange={(e) => setHeal(e.target.checked)} />Heal injury
        </label>
      )}
      <div className="mt-3 flex flex-wrap gap-2">
        <button className="btn btn-accent" disabled={save.isPending} onClick={submit}>Save</button>
        <button className="btn" disabled={save.isPending} onClick={onDone}>Cancel</button>
      </div>
      {save.isError && <ErrorBox error={save.error} />}
    </div>
  )
}

function RangeRing({ lo, hi, label, color }: { lo: number; hi: number; label: string; color: string }) {
  return (
    <div className="flex h-[84px] w-[84px] flex-col items-center justify-center rounded-full border-[6px] border-line text-center" style={{ borderColor: 'var(--color-line)' }} title={`${label} is somewhere between ${lo} and ${hi}`}>
      <span className="text-[15px] font-extrabold leading-none tabular-nums" style={{ color }}>{rangeText(lo, hi)}</span>
      <span className="mt-0.5 text-[10px] uppercase tracking-wider text-muted">{label}</span>
    </div>
  )
}

function ScoutPanel({ id, level, maxLevel }: { id: number; level: number; maxLevel: number }) {
  const [err, setErr] = useState('')
  const scout = useScout(setErr)
  const draft = useQuery({ queryKey: ['draft'], queryFn: api.draft })
  const sc = draft.data && draft.data.active ? draft.data.scouting : null
  const max = sc ? sc.maxLevel : maxLevel
  const full = level >= max
  const none = sc ? sc.points <= 0 : false
  return (
    <div className="mt-3 flex flex-wrap items-center gap-3 rounded-lg border border-line bg-panel2/50 p-3">
      <span className="font-semibold">Scouting level {level}/{max}</span>
      {sc && <span className="text-muted">{sc.points} {sc.points === 1 ? 'point' : 'points'} left</span>}
      <button className="btn btn-accent ml-auto" disabled={full || none || scout.isPending} title={full ? 'Fully scouted' : none ? 'No scouting points left' : 'Spend one scouting point'} onClick={() => scout.mutate(id)}>Scout</button>
      <div className="w-full text-[12px] text-muted">Ranges always contain the true rating.</div>
      {err && <div className="w-full text-loss">{err}</div>}
    </div>
  )
}

function ExtensionForm({ id, ask, onDone }: { id: number; ask: Extract<ExtensionAsk, { ok: true }>; onDone: () => void }) {
  const qc = useQueryClient()
  const [salary, setSalary] = useState(String(ask.salary))
  const [years, setYears] = useState(ask.maxYears)
  const [msg, setMsg] = useState('')
  const go = useMutation({
    mutationFn: () => api.contractExtend(id, Math.round(Number(salary)), years),
    onSuccess: (r) => {
      qc.setQueryData(['state'], r.state)
      for (const k of ['player', 'team', 'contracts', 'trade-block']) qc.invalidateQueries({ queryKey: [k] })
      setMsg(r.message)
      onDone()
    },
  })
  const n = Number(salary)
  const bad = !Number.isFinite(n) || n < ask.salary || n > ask.maxSalary
  return (
    <div className="mt-2 space-y-2 text-[13px]">
      <div className="flex flex-wrap items-center gap-2">
        <label className="flex items-center gap-1.5">Salary (thousands)
          <input type="number" inputMode="numeric" min={ask.salary} max={ask.maxSalary} step={10} value={salary} onChange={(e) => setSalary(e.target.value)}
            className="w-24 rounded border border-line bg-panel px-2 py-1 text-right tabular-nums" />
        </label>
        <label className="flex items-center gap-1.5">Years
          <select value={years} onChange={(e) => setYears(Number(e.target.value))} className="rounded border border-line bg-panel px-2 py-1">
            {Array.from({ length: ask.maxYears }, (_, i) => i + 1).map((y) => <option key={y} value={y}>{y}</option>)}
          </select>
        </label>
        <button className="btn btn-accent" disabled={go.isPending || bad} onClick={() => go.mutate()}>Confirm</button>
      </div>
      <div className="text-muted">Ask {money(ask.salary)}, maximum {money(ask.maxSalary)} (now {money(ask.current)}). Starts next season.</div>
      {go.isError && <div className="text-loss">{go.error instanceof Error ? go.error.message : String(go.error)}</div>}
      {msg && <div className="font-semibold text-win">{msg}</div>}
    </div>
  )
}

const trendText = { up: '\u2191 improving', down: '\u2193 falling', steady: '\u2192 steady' } as const

function MoodBlock({ p }: { p: PlayerPage }) {
  const m = p.morale
  if (!m) return null
  return (
    <div className="mt-4 rounded-lg border border-line bg-panel2/50 p-3">
      <div className="h2 mb-2">Morale</div>
      {m.tradeRequest && (
        <div role="alert" className="mb-3 rounded-md border border-loss/60 bg-loss/15 p-2.5 text-[13px] font-semibold text-loss">
          He has asked to be traded. Other teams know, so his trade value is lower. Keeping him hurts his mood and, a little, his teammates'.
        </div>
      )}
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
        <MoraleChip morale={m} />
        <span className="text-[13px] text-muted">{trendText[m.trend]}</span>
        <span className="ml-auto text-xs text-muted tabular-nums">{m.score}/100</span>
      </div>
      <div className="mt-2 h-1.5 overflow-hidden rounded bg-panel2" role="meter" aria-valuemin={0} aria-valuemax={100} aria-valuenow={m.score} aria-label="Morale">
        <div className="h-full rounded" style={{ width: `${Math.max(2, m.score)}%`, background: moraleColor[m.label] }} />
      </div>
      {m.reasons?.length > 0 && <ul className="mt-2 list-disc pl-5 text-[13px] text-muted">{m.reasons.map((r, i) => <li key={i}>{r}</li>)}</ul>}
    </div>
  )
}

function Wants({ p }: { p: PlayerPage }) {
  const [ask, setAsk] = useState<ExtensionAsk | null>(null)
  const [done, setDone] = useState(false)
  const fetchAsk = useMutation({ mutationFn: () => api.extensionAsk(p.id), onSuccess: setAsk })
  if (!p.motivation && !p.stance) return null
  return (
    <div className="mt-4 rounded-lg border border-line bg-panel2/50 p-3">
      <div className="h2 mb-2">What he wants</div>
      {p.motivation && (
        <div className="grid grid-cols-2 gap-x-6 gap-y-1.5">
          {motivationRows.map(([k, l]) => (
            <div key={k} className="flex items-center justify-between gap-2">
              <span className="text-muted">{l}</span>
              <span className={`rounded px-1.5 py-px text-[11px] font-semibold ${levelClass(p.motivation![k])}`}>{p.motivation![k]}</span>
            </div>
          ))}
        </div>
      )}
      {p.stance && (
        <div className="mt-3">
          <div className="flex flex-wrap items-center gap-1"><StanceChip stance={p.stance} /><span className="ml-1 text-[13px]">{p.stance.label}</span></div>
          {p.stance.reasons?.length > 0 && <ul className="mt-1 list-disc pl-5 text-[13px] text-muted">{p.stance.reasons.map((r, i) => <li key={i}>{r}</li>)}</ul>}
          {p.extension ? (
            <div className="mt-2 text-sm font-semibold">Extended: {money(p.extension.salary)} for {p.extension.years} {p.extension.years === 1 ? 'year' : 'years'} <span className="font-normal text-muted">(starts next season)</span></div>
          ) : done ? null : ask === null ? (
            <button className="btn mt-2" disabled={fetchAsk.isPending} onClick={() => fetchAsk.mutate()}>Offer extension</button>
          ) : ask.ok ? (
            <ExtensionForm id={p.id} ask={ask} onDone={() => setDone(true)} />
          ) : (
            <div className="mt-2 text-[13px] text-loss">{ask.reason}</div>
          )}
          {fetchAsk.isError && <ErrorBox error={fetchAsk.error} />}
        </div>
      )}
    </div>
  )
}

function TrainingPanel({ id, ratings, t }: { id: number; ratings: PlayerPage['ratings']; t: Training }) {
  const qc = useQueryClient()
  const [msg, setMsg] = useState('')
  const label = (k: string) => ratings.find((r) => r.key === k)?.label ?? k
  const set = useMutation({
    mutationFn: (focus: string) => api.setTraining(id, focus),
    onSuccess: (r) => {
      qc.setQueryData<PlayerPage>(['player', id], (old) => (old ? { ...old, training: r.training } : old))
      qc.invalidateQueries({ queryKey: ['team'] })
      setMsg(r.message)
      setTimeout(() => setMsg(''), 3000)
    },
  })
  const cur = t.options.find((o) => o.key === t.focus)?.label ?? t.focus
  const speedCls = { Fast: 'bg-win/20 text-win', Normal: 'bg-panel2 text-ink', Slow: 'bg-panel2 text-muted' }
  return (
    <div className="mt-4 rounded-lg border border-line bg-panel2/50 p-3">
      <div className="h2 mb-2">Training</div>
      <div className="text-[13px]"><span className="text-muted">Focus:</span> <span className="font-semibold">{cur}</span></div>
      {t.editable && (
        <div className="mt-2 flex flex-wrap gap-1.5">
          {t.options.map((o) => (
            <button key={o.key} disabled={set.isPending} onClick={() => o.key !== t.focus && set.mutate(o.key)}
              className={`rounded-full px-2.5 py-1 text-[12px] font-semibold disabled:opacity-50 ${o.key === t.focus ? 'bg-accent/25 text-accent ring-1 ring-accent/60' : 'bg-panel2 text-muted hover:text-ink'}`}>{o.label}</button>
          ))}
        </div>
      )}
      {set.isError && <ErrorBox error={set.error} />}
      {msg && <div className="mt-1 text-[13px] text-win">{msg}</div>}
      {t.skills.length > 0 && (
        <div className="mt-3 space-y-1">
          {t.skills.map((s) => (
            <div key={s.rating} className="flex items-center gap-2 text-[13px]">
              <span className="w-24 shrink-0 text-muted">{label(s.rating)}</span>
              <span className={`w-16 shrink-0 rounded px-1.5 py-px text-center text-[11px] font-semibold ${speedCls[s.speed]}`}>{s.speed}</span>
              {s.open ? <span className="min-w-0">{s.stepsFor2} <span className="text-[11px] text-muted">steps for +2</span></span> : <span className="text-muted">Window closed</span>}
            </div>
          ))}
        </div>
      )}
      <div className="mt-2 text-[12px] text-muted">Focus moves where growth goes; it does not add growth.</div>
    </div>
  )
}

function ReleaseBox({ p, close }: { p: PlayerPage; close: () => void }) {
  const qc = useQueryClient()
  const [open, setOpen] = useState(false)
  const pv = useQuery({ queryKey: ['release-preview', p.id], queryFn: () => api.releasePreview(p.id), enabled: open, gcTime: 0 })
  const go = useMutation({
    mutationFn: () => api.contractRelease(p.id),
    onSuccess: (r) => {
      qc.setQueryData(['state'], r.state)
      for (const k of ['team', 'teams', 'player', 'state', 'lineup', 'transactions', 'trade-block', 'contracts', 'injuries', 'freeagents', 'leaders']) qc.invalidateQueries({ queryKey: [k] })
      close()
    },
  })
  if (!open) return <button className="btn mt-3 ml-2" onClick={() => setOpen(true)}>Release</button>
  const v: ReleasePreview | undefined = pv.data
  const years = (n: number) => `${n} more ${n === 1 ? 'year' : 'years'}`
  return (
    <div className="mt-3 rounded-lg border border-line bg-panel2/50 p-3 text-[13px]">
      {pv.isLoading && <div className="text-muted">Checking...</div>}
      {pv.isError && <ErrorBox error={pv.error} />}
      {v && !v.allowed && <div className="text-loss">{v.reason}</div>}
      {v && v.allowed && (
        <div>
          <div className="font-semibold">Release {p.name}?</div>
          <div className="mt-1">
            {v.deadYears > 0 ? <>You still pay {money(v.deadSalary)} a year for {years(v.deadYears)} (dead money).</> : <>His contract is over, so you owe nothing more.</>}
            {v.refill && <> The best free agent willing to take the minimum ({v.refillName}) will be signed to keep 13 players.</>}
            {v.short && <> No free agent is available, so the roster will drop below 13.</>}
          </div>
        </div>
      )}
      {go.isError && <ErrorBox error={go.error} />}
      <div className="mt-2 flex gap-2">
        {v && v.allowed && <button className="btn btn-accent" disabled={go.isPending} onClick={() => go.mutate()}>Confirm</button>}
        <button className="btn" disabled={go.isPending} onClick={() => setOpen(false)}>Cancel</button>
      </div>
    </div>
  )
}

function AdviceBlock({ id }: { id: number }) {
  const q = useQuery({ queryKey: ['advisor'], queryFn: api.tradeAdvisor, staleTime: 0 })
  const row = q.data?.players.find((r) => r.id === id)
  if (!row) return null
  return (
    <div className="mt-3 rounded-lg border border-line bg-panel2/50 p-3" aria-label="Trade advice">
      <div className="flex items-center gap-2"><span className="h2">Trade advice</span><AdviceChip advice={row.advice} /></div>
      <ul className="mt-1 list-disc pl-5 text-[13px] text-muted">{row.reasons.map((r, i) => <li key={i}>{r}</li>)}</ul>
    </div>
  )
}

function Body({ id, close }: { id: number; close: () => void }) {
  const nav = useNavigate()
  const q = useQuery({ queryKey: ['player', id], queryFn: () => api.player(id) })
  const { state } = useFlow()
  const [editing, setEditing] = useState(false)
  const credit = useQuery({ queryKey: ['photo-credit', id], queryFn: () => api.photoCredit(id), enabled: !!q.data?.photo, retry: false, staleTime: Infinity })
  const ts = useQuery({ queryKey: ['trade-status', state?.phase, state?.day], queryFn: api.tradeStatus })
  return (
    <Q q={q} minH={300}>
      {(p) => (
        <div className="p-5">
          <div className="flex items-start justify-between">
            <div className="flex gap-3">
              <div className="shrink-0">
                <Avatar id={p.id} name={p.name} photo={p.photo} color={p.team?.primary} size={64} />
              </div>
              <div>
              <h2 className="text-2xl font-extrabold">{p.name}<InjuryTag injury={p.injury} /></h2>
              {p.injury && <div className="mt-1 text-sm font-semibold text-loss">Injured: {injuryText(p.injury)}</div>}
              <div className="mt-1 flex items-center gap-2 text-muted">
                <span>{p.pos}</span><span>Age {p.age}</span><span>{p.status}</span>
              </div>
              {p.salary > 0 && (
                <div className="mt-1 text-sm font-semibold">
                  {money(p.salary)} per season <span className="text-muted">&middot; {p.years === 0 && p.team ? 'Contract expiring' : `${p.years} ${p.years === 1 ? 'year' : 'years'} left`}</span>
                </div>
              )}
              <div className="mt-1 text-sm text-accent">{p.role} · shot tendency {p.usage}/90</div>
              {p.team && (
                <button className="mt-2 inline-flex items-center gap-2 hover:text-accent" onClick={() => { close(); nav(`/team/${p.team!.id}`) }}>
                  <TeamBadge team={p.team} size={20} />{p.team.fullName}
                </button>
              )}
              </div>
            </div>
            <div className="flex gap-2">
              {p.prospect && p.scout ? (
                <>
                  <RangeRing lo={p.scout.ovrLo} hi={p.scout.ovrHi} label="OVR" color={ovrColor((p.scout.ovrLo + p.scout.ovrHi) / 2)} />
                  <RangeRing lo={p.scout.potLo} hi={p.scout.potHi} label="POT" color="var(--color-accent)" />
                </>
              ) : (
                <>
                  <Ring value={p.ovr} label="OVR" color={ovrColor(p.ovr)} />
                  <Ring value={p.pot} label="POT" color="var(--color-accent)" />
                </>
              )}
            </div>
          </div>
          {!p.prospect && (!!p.ovrChange || !!p.potChange) && (
            <div className="mt-2 text-sm text-muted">{p.changeScope === 'season' ? 'This season' : 'Last year'}: OVR<Delta value={p.ovrChange} scope={p.changeScope} /> POT<Delta value={p.potChange} scope={p.changeScope} /></div>
          )}
          {p.team && p.team.id !== state?.user.team.id && ts.data?.open && (
            <div className="mt-3 flex flex-wrap gap-2">
              <button className="btn" onClick={() => { close(); nav(`/trade?team=${p.team!.id}&get=${p.id}`) }}>Trade for this player</button>
              {!p.prospect && <button className="btn" onClick={() => { close(); nav(`/trade?team=${p.team!.id}&want=${p.id}`) }}><Sparkles size={14} className="text-accent" />What would it take?</button>}
            </div>
          )}
          {!p.prospect && p.team && p.team.id === state?.user.team.id && ts.data?.open && (
            <div className="mt-3 flex flex-wrap gap-2">
              <button className="btn" onClick={() => { close(); nav(`/trade?find=${p.id}`) }}><Search size={14} className="text-accent" />See offers for him</button>
            </div>
          )}
          {!p.prospect && p.team && p.team.id === state?.user.team.id && <AdviceBlock id={p.id} />}
          {!p.prospect && p.team && p.team.id === state?.user.team.id && <ReleaseBox p={p} close={close} />}
          <MoodBlock p={p} />
          <Wants p={p} />
          {p.prospect && p.scout && <ScoutPanel id={p.id} level={p.scout.level} maxLevel={3} />}
          {p.editable && !editing && <button className="btn mt-3 ml-2" onClick={() => setEditing(true)}>Edit</button>}
          {p.editable && editing && <EditForm p={p} onDone={() => setEditing(false)} />}
          <div className="mt-4 space-y-1.5">
            {p.ratings.map((r) => p.prospect && r.lo != null && r.hi != null ? (
              <div key={r.key} className="flex items-center gap-3">
                <span className="w-24 text-muted">{r.label}</span>
                <RangeBar lo={r.lo} hi={r.hi} className="flex-1" />
                <span className="w-14 text-right font-bold tabular-nums">{rangeText(r.lo, r.hi)}</span>
              </div>
            ) : (
              <div key={r.key} className="flex items-center gap-3">
                <span className="w-24 text-muted">{r.label}</span>
                <div className="h-2 flex-1 overflow-hidden rounded bg-panel2">
                  <motion.div className="h-full rounded" style={{ background: ovrColor(r.value) }} initial={{ width: 0 }} animate={{ width: `${r.value}%` }} transition={{ duration: 0.7, ease: 'easeOut' }} />
                </div>
                <span className="w-7 text-right font-bold tabular-nums">{r.value}</span>
                <span className="w-8"><Delta value={p.ratingChanges?.[r.key]} scope={p.changeScope} /></span>
              </div>
            ))}
          </div>
          {!p.prospect && <AwardsSection awards={p.awards} />}
          {p.badges && !p.prospect && <BadgesSection badges={p.badges} />}
          {p.training && <TrainingPanel id={p.id} ratings={p.ratings} t={p.training} />}
          <div className="mt-3 text-muted">
            {p.experience} seasons played{p.draftPick > 0 && <> · Drafted {p.draftYear > 0 ? p.draftYear : ''} (pick {p.draftPick})</>}
          </div>
          {p.photo && credit.data && (
            <a href={credit.data.source} target="_blank" rel="noreferrer" className="mt-1 block text-xs text-muted hover:underline">
              Photo: {credit.data.artist} ({credit.data.licence}), Wikimedia Commons
            </a>
          )}
          <Stats title="Season" s={p.season} />
          <Stats title="Playoffs" s={p.playoffs} />
          {p.career.length > 0 && (
            <div className="mt-5">
              <div className="h2">Career</div>
              <Line data={p.career} />
              <table className="tbl mt-1">
                <thead><tr><th>Year</th><th>Team</th><th className="r">Age</th><th className="r">OVR</th><th className="r">POT</th><th className="r">GP</th><th className="r">PPG</th><th className="r">RPG</th><th className="r">APG</th></tr></thead>
                <tbody>
                  {[...p.career].reverse().map((c) => (
                    <tr key={c.year}>
                      <td>{c.year}</td><td>{c.team.abbr}</td><td className="r">{c.age}</td><td className="r">{c.ovr}</td><td className="r">{c.pot}</td>
                      <td className="r">{c.gp}</td><td className="r">{f1(c.ppg)}</td><td className="r">{f1(c.rpg)}</td><td className="r">{f1(c.apg)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}
    </Q>
  )
}

export function PlayerDrawer() {
  const [sp, setSp] = useSearchParams()
  const raw = sp.get('player')
  const id = raw ? Number(raw) : null
  const close = () => setSp((p) => { p.delete('player'); return p }, { replace: true })
  useEffect(() => {
    if (id === null) return
    const h = (e: KeyboardEvent) => e.key === 'Escape' && close()
    window.addEventListener('keydown', h)
    return () => window.removeEventListener('keydown', h)
  })
  return (
    <AnimatePresence>
      {id !== null && !Number.isNaN(id) && (
        <>
          <motion.div key="bg" className="fixed inset-0 z-30 bg-black/50" initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} onClick={close} />
          <motion.aside
            key="panel" className="fixed inset-y-0 right-0 z-40 w-full max-w-[560px] overflow-y-auto border-l border-line bg-panel shadow-2xl"
            initial={{ x: '100%' }} animate={{ x: 0 }} exit={{ x: '100%' }} transition={{ type: 'spring', stiffness: 380, damping: 38 }}
          >
            <button className="absolute right-3 top-3 z-10 rounded p-1 text-muted hover:text-ink" onClick={close} aria-label="Close"><X size={18} /></button>
            <Body id={id} close={close} />
          </motion.aside>
        </>
      )}
    </AnimatePresence>
  )
}
