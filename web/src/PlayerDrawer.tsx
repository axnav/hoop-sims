import { useEffect, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AnimatePresence, motion } from 'motion/react'
import { X } from 'lucide-react'
import { api, type PlayerPage, type StatLine } from './api'
import { useFlow } from './flow'
import { Delta, ErrorBox, InjuryTag, Q, RangeBar, Ring, TeamBadge, injuryText, ovrColor, f1, money, rangeText } from './ui'
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

function Body({ id, close }: { id: number; close: () => void }) {
  const nav = useNavigate()
  const q = useQuery({ queryKey: ['player', id], queryFn: () => api.player(id) })
  const { state } = useFlow()
  const [editing, setEditing] = useState(false)
  const ts = useQuery({ queryKey: ['trade-status', state?.phase, state?.day], queryFn: api.tradeStatus })
  return (
    <Q q={q} minH={300}>
      {(p) => (
        <div className="p-5">
          <div className="flex items-start justify-between">
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
            <button className="btn mt-3" onClick={() => { close(); nav(`/trade?team=${p.team!.id}&get=${p.id}`) }}>Trade for this player</button>
          )}
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
          <div className="mt-3 text-muted">
            {p.experience} seasons played{p.draftPick > 0 && <> · Drafted {p.draftYear > 0 ? p.draftYear : ''} (pick {p.draftPick})</>}
          </div>
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
