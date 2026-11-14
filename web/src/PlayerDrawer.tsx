import { useEffect } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { AnimatePresence, motion } from 'motion/react'
import { X } from 'lucide-react'
import { api, type StatLine } from './api'
import { useFlow } from './flow'
import { Q, Ring, TeamBadge, ovrColor, f1 } from './ui'

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

function Body({ id, close }: { id: number; close: () => void }) {
  const nav = useNavigate()
  const q = useQuery({ queryKey: ['player', id], queryFn: () => api.player(id) })
  const { state } = useFlow()
  const ts = useQuery({ queryKey: ['trade-status', state?.phase, state?.day], queryFn: api.tradeStatus })
  return (
    <Q q={q} minH={300}>
      {(p) => (
        <div className="p-5">
          <div className="flex items-start justify-between">
            <div>
              <h2 className="text-2xl font-extrabold">{p.name}</h2>
              <div className="mt-1 flex items-center gap-2 text-muted">
                <span>{p.pos}</span><span>Age {p.age}</span><span>{p.status}</span>
              </div>
              <div className="mt-1 text-sm text-accent">{p.role} · shot tendency {p.usage}/90</div>
              {p.team && (
                <button className="mt-2 inline-flex items-center gap-2 hover:text-accent" onClick={() => { close(); nav(`/team/${p.team!.id}`) }}>
                  <TeamBadge team={p.team} size={20} />{p.team.fullName}
                </button>
              )}
            </div>
            <div className="flex gap-2">
              <Ring value={p.ovr} label="OVR" color={ovrColor(p.ovr)} />
              <Ring value={p.pot} label="POT" color="var(--color-accent)" />
            </div>
          </div>
          {p.team && p.team.id !== state?.user.team.id && ts.data?.open && (
            <button className="btn mt-3" onClick={() => { close(); nav(`/trade?team=${p.team!.id}&get=${p.id}`) }}>Trade for this player</button>
          )}
          <div className="mt-4 space-y-1.5">
            {p.ratings.map((r) => (
              <div key={r.key} className="flex items-center gap-3">
                <span className="w-24 text-muted">{r.label}</span>
                <div className="h-2 flex-1 overflow-hidden rounded bg-panel2">
                  <motion.div className="h-full rounded" style={{ background: ovrColor(r.value) }} initial={{ width: 0 }} animate={{ width: `${r.value}%` }} transition={{ duration: 0.7, ease: 'easeOut' }} />
                </div>
                <span className="w-7 text-right font-bold tabular-nums">{r.value}</span>
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
