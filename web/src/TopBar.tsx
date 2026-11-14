import { useEffect, useRef, useState } from 'react'
import { NavLink } from 'react-router-dom'
import { useMutation } from '@tanstack/react-query'
import { motion } from 'motion/react'
import { Check, ChevronDown, Loader2, Play, Save } from 'lucide-react'
import { api, type GameState } from './api'
import { useFlow } from './flow'
import { TeamBadge } from './ui'

const steps = ['Regular season', 'Playoffs', 'Draft', 'Free agency', 'New season']
const stepOf = (s: GameState) =>
  ({ regular: 0, regular_done: 0, playoffs: 1, playoffs_done: 1, offseason: 2, draft: 2, draft_done: 2, free_agency: 3 })[s.phase]

function Stepper({ s }: { s: GameState }) {
  const cur = stepOf(s)
  return (
    <div className="flex items-center gap-1">
      {steps.map((n, i) => (
        <div key={n} className="w-28">
          <div className={`mb-1 truncate text-[11px] font-bold uppercase tracking-wider ${i === cur ? 'text-accent' : i < cur ? 'text-ink/70' : 'text-muted/60'}`}>{n}</div>
          <div className="h-1 overflow-hidden rounded bg-line">
            <motion.div
              className="h-full" style={{ background: i === cur ? 'var(--color-accent)' : 'var(--color-muted)' }}
              animate={{ width: i < cur ? '100%' : i === cur ? (i === 0 ? `${(s.day / s.totalDays) * 100}%` : '50%') : '0%' }}
            />
          </div>
          {i === 0 && cur === 0 && <div className="mt-0.5 text-[10px] text-muted tabular-nums">Day {s.day}/{s.totalDays}</div>}
        </div>
      ))}
    </div>
  )
}

// Narrow layouts: just the current phase name and the day progress.
function PhaseLine({ s }: { s: GameState }) {
  const cur = stepOf(s)
  return (
    <div className="truncate text-[11px] font-semibold text-muted">
      <span className="font-bold uppercase tracking-wider text-accent">{steps[cur]}</span>
      {cur === 0 && <span className="tabular-nums"> · Day {s.day}/{s.totalDays}</span>}
    </div>
  )
}

function ContinueMenu() {
  const { state, run, busy } = useFlow()
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)
  useEffect(() => {
    const h = (e: MouseEvent) => { if (!ref.current?.contains(e.target as Node)) setOpen(false) }
    window.addEventListener('mousedown', h)
    return () => window.removeEventListener('mousedown', h)
  }, [])
  if (!state) return null
  const items = state.actions ?? []
  return (
    <div ref={ref} className="relative flex">
      <button className="btn btn-accent rounded-r-none" disabled={busy} onClick={() => run(state.nextAction.id)} title="Space or Enter">
        {busy ? <Loader2 size={16} className="animate-spin" /> : <Play size={16} />}
        <span className="max-w-[7.5rem] truncate min-[1000px]:max-w-none">{state.nextAction.label}</span>
      </button>
      {items.length > 0 && (
        <button className="btn btn-accent rounded-l-none border-l border-black/20 px-2" disabled={busy} onClick={() => setOpen((o) => !o)} aria-label="More actions">
          <ChevronDown size={16} />
        </button>
      )}
      {open && (
        <div className="absolute right-0 top-full z-50 mt-1 min-w-52 overflow-hidden rounded-lg border border-line bg-panel shadow-xl">
          {items.map((a) => (
            <button key={a.id} className="block w-full px-3 py-2 text-left hover:bg-panel2" onClick={() => { setOpen(false); run(a.id) }}>
              {a.label}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}

function SaveButton() {
  const m = useMutation({ mutationFn: api.save })
  return (
    <button className="btn" onClick={() => m.mutate()} disabled={m.isPending} title={m.isError ? String(m.error) : 'Save game'} aria-label="Save game">
      {m.isPending ? <Loader2 size={15} className="animate-spin" /> : m.isSuccess ? <Check size={15} className="text-win" /> : <Save size={15} />}
      <span className="max-[999px]:hidden">Save</span>
    </button>
  )
}

export function TopBar() {
  const { state: s } = useFlow()
  if (!s) return null
  const u = s.user
  const offseason = s.phase === 'offseason' || s.phase === 'draft' || s.phase === 'draft_done'
  const link = (to: string, label: string, badge = 0, red = false, expiring = 0) => (
    <NavLink to={to} end={to === '/'} className={({ isActive }) => `rounded-md px-3 py-1.5 font-semibold ${isActive ? 'bg-white/10 text-ink' : 'text-muted hover:text-ink'}`}>{label}{badge > 0 && <span className={`ml-1.5 rounded-full ${red ? 'bg-loss text-white' : 'bg-accent text-black'} px-1.5 py-px text-[10px] font-bold tabular-nums`} aria-label={`${badge} ${red ? 'injured' : 'offers'}`}>{badge}</span>}{expiring > 0 && <span className="ml-1.5 rounded-full bg-amber-400 px-1.5 py-px text-[10px] font-bold tabular-nums text-black" aria-label={`${expiring} contracts ending`} title="Contracts ending">{expiring}</span>}</NavLink>
  )
  return (
    <header
      className="sticky top-0 z-20 border-b backdrop-blur"
      style={{ background: `linear-gradient(90deg, color-mix(in srgb, ${u.team.primary} 38%, #090c12), #090c12 70%)`, borderColor: u.team.primary }}
    >
      <div className="mx-auto flex max-w-[1400px] items-center justify-between gap-2 px-3 pt-2.5 min-[1000px]:gap-4 min-[1000px]:px-5">
        <div className="flex min-w-0 items-center gap-6">
          <div className="min-w-0">
            <div className="whitespace-nowrap text-lg font-black tracking-tight">HOOP<span className="text-accent">SIM</span> <span className="ml-1 text-sm font-semibold text-muted tabular-nums">{s.year}</span></div>
            <div className="min-[1000px]:hidden"><PhaseLine s={s} /></div>
          </div>
          <div className="max-[999px]:hidden"><Stepper s={s} /></div>
        </div>
        <div className="flex shrink-0 items-center gap-2 min-[1000px]:gap-3">
          <div className="flex items-center gap-2">
            <TeamBadge team={u.team} size={26} />
            <div className="leading-tight">
              <div className="font-bold tabular-nums">{u.w}-{u.l}</div>
              <div className="text-[11px] text-muted">#{u.confRank} {u.confName}</div>
            </div>
          </div>
          <SaveButton />
          <ContinueMenu />
        </div>
      </div>
      <nav className="mx-auto flex max-w-[1400px] items-center gap-1 overflow-x-auto px-2 py-1.5 whitespace-nowrap min-[1000px]:px-4 [&>a]:shrink-0">
        {link('/', 'Dashboard')}
        {link(`/team/${u.team.id}`, 'Roster', s.injured, true, s.expiring)}
        {link('/lineup', 'Lineup')}
        {link('/standings', 'Standings')}
        {link('/leaders', 'Leaders')}
        {link('/schedule', 'Schedule')}
        {link('/playoffs', 'Playoffs')}
        {offseason && link('/offseason', 'Offseason')}
        {offseason && link('/draft', 'Draft')}
        {s.phase === 'free_agency' && link('/free-agency', 'Free agency')}
        {link('/history', 'History')}
        {link('/trade', 'Trades', s.offers)}
        {link('/settings', 'Settings')}
      </nav>
    </header>
  )
}
