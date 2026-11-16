import { useEffect, useRef, useState, type ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { animate, motion, useReducedMotion } from 'motion/react'
import type { UseQueryResult } from '@tanstack/react-query'
import {
  flexRender, getCoreRowModel, getSortedRowModel, useReactTable, type ColumnDef, type SortingState,
} from '@tanstack/react-table'
import { ArrowDown, ArrowUp, Loader2 } from 'lucide-react'
import type { MoraleBrief, MoraleLabel, Fit, Badge, Injury, Level, Motivation, Payroll, Stance, Team, TeamStatus } from './api'

export const f1 = (v: number | null | undefined) => (v == null ? '-' : v.toFixed(1))
export const pct3 = (v: number) => (v === 1 ? '1.000' : v.toFixed(3).replace(/^0/, ''))

function lum(hex: string) {
  const n = parseInt(hex.slice(1), 16)
  return (0.299 * (n >> 16) + 0.587 * ((n >> 8) & 255) + 0.114 * (n & 255)) / 255
}

// Player picture: the photo when the server says there is one, else initials
// on a circle in the team colour (also used when the image fails to load).
export function Avatar({ id, name, photo, color, size = 32 }: { id: number; name: string; photo?: boolean; color?: string; size?: number }) {
  const [failed, setFailed] = useState(false)
  const base = { width: size, height: size, minWidth: size } as const
  if (photo && !failed) {
    return (
      <img src={'/api/photo/' + id} alt={name} loading="lazy" onError={() => setFailed(true)}
        className="inline-block shrink-0 rounded-full bg-panel2 align-middle"
        style={{ ...base, objectFit: 'cover', objectPosition: 'top' }} />
    )
  }
  const bg = color && /^#[0-9a-fA-F]{6}$/.test(color) ? color : '#64748b'
  const words = name.trim().split(/\s+/).filter(Boolean)
  const initials = (words.length > 1 ? words[0][0] + words[words.length - 1][0] : (words[0] ?? '?').slice(0, 1)).toUpperCase()
  return (
    <span role="img" aria-label={name} className="inline-flex shrink-0 select-none items-center justify-center rounded-full align-middle font-bold"
      style={{ ...base, background: bg, color: lum(bg) > 0.6 ? '#10131a' : '#fff', fontSize: Math.max(8, Math.round(size * 0.38)) }}>
      {initials}
    </span>
  )
}

export function TeamBadge({ team, size = 28 }: { team: Team; size?: number }) {
  return (
    <span
      className="inline-flex shrink-0 items-center justify-center rounded-md font-extrabold tracking-tight"
      style={{
        width: size * 1.35, height: size, fontSize: size * 0.4, background: team.primary,
        color: lum(team.primary) > 0.6 ? '#10131a' : '#fff', boxShadow: `inset 0 -3px 0 ${team.secondary}`,
      }}
    >
      {team.abbr}
    </span>
  )
}

export function TeamLink({ team, badge = true, short = false }: { team: Team; badge?: boolean; short?: boolean }) {
  return (
    <Link to={`/team/${team.id}`} className="link inline-flex items-center gap-2">
      {badge && <TeamBadge team={team} size={20} />}
      <span>{short ? team.name : team.fullName}</span>
    </Link>
  )
}

export function PlayerLink({ id, name }: { id: number; name: string }) {
  return <Link to={`?player=${id}`} className="link font-medium">{name}</Link>
}

export function AnimatedNumber({ value, decimals = 0 }: { value: number; decimals?: number }) {
  const ref = useRef<HTMLSpanElement>(null)
  const prev = useRef(value)
  const reduce = useReducedMotion()
  useEffect(() => {
    const el = ref.current
    if (!el) return
    if (reduce || prev.current === value) {
      el.textContent = value.toFixed(decimals)
      prev.current = value
      return
    }
    const c = animate(prev.current, value, { duration: 0.5, onUpdate: (v) => (el.textContent = v.toFixed(decimals)) })
    prev.current = value
    return () => c.stop()
  }, [value, decimals, reduce])
  return <span ref={ref} className="tabular-nums">{value.toFixed(decimals)}</span>
}

export function Ring({ value, label, color, size = 84 }: { value: number; label: string; color: string; size?: number }) {
  const r = size / 2 - 7
  return (
    <div className="relative" style={{ width: size, height: size }}>
      <svg width={size} height={size} className="-rotate-90">
        <circle cx={size / 2} cy={size / 2} r={r} fill="none" stroke="var(--color-line)" strokeWidth={6} />
        <motion.circle
          cx={size / 2} cy={size / 2} r={r} fill="none" stroke={color} strokeWidth={6} strokeLinecap="round"
          initial={{ pathLength: 0 }} animate={{ pathLength: Math.min(1, value / 99) }} transition={{ duration: 0.9, ease: 'easeOut' }}
        />
      </svg>
      <div className="absolute inset-0 flex flex-col items-center justify-center">
        <span className="text-xl font-extrabold leading-none tabular-nums">{value}</span>
        <span className="mt-0.5 text-[10px] uppercase tracking-wider text-muted">{label}</span>
      </div>
    </div>
  )
}

export function Spinner({ label = 'Loading' }: { label?: string }) {
  return (
    <div className="flex items-center gap-2 p-6 text-muted"><Loader2 className="animate-spin" size={16} />{label}</div>
  )
}

export function ErrorBox({ error }: { error: unknown }) {
  return <div className="card m-2 border-loss/50 p-4 text-loss">{error instanceof Error ? error.message : String(error)}</div>
}

// Renders loading / error / data for a query. Keeps old data visible during refetch (no layout jump).
export function Q<T>({ q, children, minH = 120 }: { q: UseQueryResult<T>; children: (d: T) => ReactNode; minH?: number }) {
  if (q.data !== undefined) return <>{children(q.data)}</>
  if (q.isError) return <ErrorBox error={q.error} />
  return <div style={{ minHeight: minH }}><Spinner /></div>
}

export function Card({ title, right, children, className = '' }: { title?: ReactNode; right?: ReactNode; children: ReactNode; className?: string }) {
  return (
    <section className={`card ${className}`}>
      {(title || right) && (
        <header className="flex items-center justify-between px-4 pt-3">
          <h2 className="h2">{title}</h2>
          {right}
        </header>
      )}
      <div className="p-2 pt-1">{children}</div>
    </section>
  )
}

export function Page({ title, right, children }: { title: ReactNode; right?: ReactNode; children: ReactNode }) {
  return (
    <div className="mx-auto max-w-[1400px] px-3 py-5 min-[1000px]:px-5">
      <div className="mb-4 flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
        <h1 className="text-2xl font-extrabold tracking-tight">{title}</h1>
        {right}
      </div>
      {children}
    </div>
  )
}

export type Col<T> = ColumnDef<T, any> & { num?: boolean; hideSmall?: boolean }

export function DataTable<T>({
  data, columns, initialSort, onRow, rowClass,
}: {
  data: T[]; columns: Col<T>[]; initialSort?: SortingState; onRow?: (r: T) => void; rowClass?: (r: T) => string
}) {
  const [sorting, setSorting] = useState<SortingState>(initialSort ?? [])
  const table = useReactTable({
    data, columns, state: { sorting }, onSortingChange: setSorting,
    getCoreRowModel: getCoreRowModel(), getSortedRowModel: getSortedRowModel(), sortDescFirst: true,
  })
  return (
    <table className="tbl">
      <thead>
        {table.getHeaderGroups().map((hg) => (
          <tr key={hg.id}>
            {hg.headers.map((h) => {
              const num = (h.column.columnDef as Col<T>).num
              const s = h.column.getIsSorted()
              return (
                <th key={h.id} className={`cursor-pointer select-none ${num ? 'r' : ''} ${(h.column.columnDef as Col<T>).hideSmall ? 'max-[699px]:hidden' : ''}`} onClick={h.column.getToggleSortingHandler()}>
                  <span className="inline-flex items-center gap-1">
                    {flexRender(h.column.columnDef.header, h.getContext())}
                    {s === 'asc' ? <ArrowUp size={11} /> : s === 'desc' ? <ArrowDown size={11} /> : <span className="w-[11px]" />}
                  </span>
                </th>
              )
            })}
          </tr>
        ))}
      </thead>
      <tbody>
        {table.getRowModel().rows.map((row) => (
          <tr key={row.id} className={`${onRow ? 'cursor-pointer' : ''} ${rowClass?.(row.original) ?? ''}`} onClick={() => onRow?.(row.original)}>
            {row.getVisibleCells().map((c) => (
              <td key={c.id} className={`${(c.column.columnDef as Col<T>).num ? 'r' : ''} ${(c.column.columnDef as Col<T>).hideSmall ? 'max-[699px]:hidden' : ''}`}>
                {flexRender(c.column.columnDef.cell, c.getContext())}
              </td>
            ))}
          </tr>
        ))}
      </tbody>
    </table>
  )
}

export const WL = ({ win }: { win: boolean }) => (
  <span className={`inline-flex h-5 w-5 items-center justify-center rounded text-[11px] font-extrabold ${win ? 'bg-win/20 text-win' : 'bg-loss/20 text-loss'}`}>
    {win ? 'W' : 'L'}
  </span>
)

export const ovrColor = (v: number) => (v >= 80 ? '#3ddc84' : v >= 70 ? '#a3e635' : v >= 60 ? '#facc15' : v >= 50 ? '#fb923c' : '#ff5a67')

// Thin bar on the common 0-99 rating scale with a span from lo to hi.
export function RangeBar({ lo, hi, color, className = '' }: { lo: number; hi: number; color?: string; className?: string }) {
  const a = Math.max(0, Math.min(99, lo)), b = Math.max(a, Math.min(99, hi))
  return (
    <div className={`relative h-2 rounded bg-panel2 ${className}`} aria-hidden>
      <div className="absolute top-0 h-full rounded" style={{ left: `${(a / 99) * 100}%`, width: `${Math.max(1.5, ((b - a) / 99) * 100)}%`, background: color ?? ovrColor((a + b) / 2) }} />
    </div>
  )
}

export const rangeText = (lo: number, hi: number) => `${lo}\u2013${hi}`

// Small label for an asset's tier. Top tiers get a stronger look.
export function TierChip({ tier }: { tier: string }) {
  const top = tier === 'Franchise' || tier === 'Lottery pick'
  return (
    <span className={`ml-1.5 shrink-0 rounded px-1.5 py-px align-middle text-[10px] font-semibold uppercase tracking-wide ${top ? 'bg-accent/25 text-accent ring-1 ring-accent/60' : 'bg-panel2 text-muted'}`}>{tier}</span>
  )
}

// Small green/red change marker for a progression delta. Nothing when unknown or zero.
export function Delta({ value, scope }: { value?: number; scope?: 'season' | 'year' }) {
  if (!value) return null
  const title = scope === 'season' ? 'Change this season' : 'Change over the last year'
  return (
    <span title={title} className={`ml-1 text-[10px] font-semibold tabular-nums ${value > 0 ? 'text-win' : 'text-loss'}`}>
      {value > 0 ? '\u25B2' : '\u25BC'}{Math.abs(value)}
    </span>
  )
}

export const injuryText = (i: Injury) => `${i.name}, out ${i.gamesLeft} ${i.gamesLeft === 1 ? 'game' : 'games'}${i.serious ? ' (serious)' : ''}`

// Red "INJ" tag shown next to an injured player's name.
export function InjuryTag({ injury }: { injury?: Injury }) {
  if (!injury) return null
  return (
    <span title={injuryText(injury)} aria-label={`Injured: ${injuryText(injury)}`}
      className="ml-1.5 shrink-0 rounded bg-loss/20 px-1.5 py-px align-middle text-[10px] font-extrabold tracking-wide text-loss ring-1 ring-loss/50">INJ</span>
  )
}

// Money values from the API are integers in thousands of dollars.
export const money = (k: number) => {
  const neg = k < 0
  const a = Math.abs(k)
  const t = a >= 1000 ? `$${(a / 1000).toFixed(1)}M` : `$${Math.round(a)}K`
  return neg ? `-${t}` : t
}

// Bar scaled to the hard ceiling, with marks at the cap, tax line and ceiling.
export function PayrollBar({ p, compact = false }: { p: Payroll; compact?: boolean }) {
  const max = Math.max(p.ceiling, p.payroll, 1)
  const at = (v: number) => `${Math.min(100, (100 * v) / max)}%`
  const fill = p.payroll > p.taxLine ? 'bg-loss' : p.payroll > p.cap ? 'bg-amber-400' : 'bg-win'
  const marks: [string, number][] = [['Cap', p.cap], ['Tax', p.taxLine], ['Max', p.ceiling]]
  return (
    <div className={compact ? 'px-2 pb-1' : 'px-2 pb-2'}>
      <div className={`relative ${compact ? 'h-7' : 'h-9'}`}>
        <div className="absolute inset-x-0 top-0 h-2.5 overflow-hidden rounded-full bg-panel2">
          <div className={`h-full rounded-full ${fill}`} style={{ width: at(p.payroll) }} />
        </div>
        {marks.map(([l, v], i) => (
          <div key={l} className="absolute top-0" style={{ left: at(v), transform: i === marks.length - 1 ? 'translateX(-100%)' : 'translateX(-50%)' }}>
            <div className="mx-auto h-3.5 w-px bg-ink/60" style={i === marks.length - 1 ? { marginRight: 0 } : undefined} />
            <div className="whitespace-nowrap text-[10px] leading-tight text-muted">{l} {money(v)}</div>
          </div>
        ))}
      </div>
      <div className="mt-1 text-[12px] text-muted">
        <span>Payroll <b className="text-ink tabular-nums">{money(p.payroll)}</b></span>
        {' \u00b7 '}
        <span className={p.room < 0 ? 'text-loss' : ''}>{p.room < 0 ? `Over the cap by ${money(-p.room)}` : `Cap room ${money(p.room)}`}</span>
        {p.taxBill > 0 && <>{' \u00b7 '}<span className="font-semibold text-loss">Luxury tax {money(p.taxBill)}</span></>}
      </div>
    </div>
  )
}

export const ExpiringChip = () => (
  <span className="ml-1.5 shrink-0 rounded bg-amber-400/20 px-1.5 py-px align-middle text-[10px] font-semibold uppercase tracking-wide text-amber-400 ring-1 ring-amber-400/50">Expiring</span>
)

// Small accessible confirm dialog (replaces window.confirm). Escape or the backdrop cancels.
export function ConfirmDialog({ message, confirmLabel = 'Continue', onConfirm, onCancel }: { message: string; confirmLabel?: string; onConfirm: () => void; onCancel: () => void }) {
  const ok = useRef<HTMLButtonElement>(null)
  useEffect(() => {
    ok.current?.focus()
    const h = (e: KeyboardEvent) => { if (e.key === 'Escape') onCancel() }
    window.addEventListener('keydown', h)
    return () => window.removeEventListener('keydown', h)
  }, [onCancel])
  return (
    <div className="fixed inset-0 z-[70] flex items-center justify-center bg-black/60 p-4" onMouseDown={(e) => { if (e.target === e.currentTarget) onCancel() }}>
      <div role="alertdialog" aria-modal="true" aria-label="Confirm" className="card w-full max-w-sm p-4 shadow-xl">
        <p className="mb-4">{message}</p>
        <div className="flex justify-end gap-2">
          <button className="btn" onClick={onCancel}>Cancel</button>
          <button ref={ok} className="btn btn-accent" onClick={onConfirm}>{confirmLabel}</button>
        </div>
      </div>
    </div>
  )
}

const statusStyle: Record<TeamStatus['key'], string> = {
  contender: 'bg-accent/25 text-accent ring-accent/60',
  buyer: 'bg-win/20 text-win ring-win/50',
  seller: 'bg-amber-400/20 text-amber-400 ring-amber-400/50',
  rebuilding: 'bg-sky-400/15 text-sky-300 ring-sky-400/40',
}

// Small pill for a team's trade status. Nothing when unknown.
export function StatusChip({ status }: { status?: TeamStatus }) {
  if (!status) return null
  return (
    <span title={status.wants} className={`inline-block shrink-0 rounded px-1.5 py-px align-middle text-[10px] font-semibold uppercase tracking-wide ring-1 ${statusStyle[status.key]}`}>{status.label}</span>
  )
}

const stanceStyle: Record<Stance['key'], string> = {
  resign: 'bg-win/20 text-win ring-win/50',
  open: 'bg-amber-400/20 text-amber-400 ring-amber-400/50',
  test: 'bg-loss/20 text-loss ring-loss/50',
}
const stanceShort: Record<Stance['key'], string> = { resign: 'Re-sign', open: 'Open', test: 'Test FA' }

// Small pill for a player's re-sign stance (short text; full label as tooltip) and an "Ext" badge when extended.
export function StanceChip({ stance, extended }: { stance?: Stance; extended?: boolean }) {
  if (!stance && !extended) return null
  return (
    <>
      {stance && <span title={stance.label} className={`ml-1.5 inline-block shrink-0 rounded px-1.5 py-px align-middle text-[10px] font-semibold uppercase tracking-wide ring-1 ${stanceStyle[stance.key]}`}>{stanceShort[stance.key]}</span>}
      {extended && <span title="Contract extended: new deal starts next season" className="ml-1.5 inline-block shrink-0 rounded bg-sky-400/15 px-1.5 py-px align-middle text-[10px] font-semibold uppercase tracking-wide text-sky-300 ring-1 ring-sky-400/40">Ext</span>}
    </>
  )
}

export const levelClass = (l: Level) => (l === 'High' ? 'bg-accent/25 text-accent ring-1 ring-accent/60' : l === 'Medium' ? 'bg-panel2 text-ink' : 'bg-panel2 text-muted')

export const motivationRows: [keyof Motivation, string][] = [['money', 'Money'], ['winning', 'Winning'], ['role', 'Role'], ['loyalty', 'Loyalty']]
export const motivationTitle = (m: Motivation) => motivationRows.map(([k, l]) => `${l}: ${m[k]}`).join(', ')

// Compact list of the traits a player rates High, with a tooltip listing all four. Nothing when none is High.
export function ValuesChips({ m }: { m?: Motivation }) {
  if (!m) return null
  const high = motivationRows.filter(([k]) => m[k] === 'High')
  return (
    <span title={motivationTitle(m)} className="ml-1.5 inline-flex flex-wrap items-center gap-1 align-middle">
      {high.length > 0 && <span className="text-[10px] text-muted">Values</span>}
      {high.map(([k, l]) => <span key={k} className={`rounded px-1.5 py-px text-[10px] font-semibold ${levelClass('High')}`}>{l}</span>)}
    </span>
  )
}

export const focusLabels: Record<string, string> = { shooting: 'Shooting', inside: 'Inside', playmaking: 'Playmaking', defense: 'Defense', rebounding: 'Rebounding', body: 'Body' }

// Small muted training-focus label for roster rows; nothing for balanced or unknown.
export function FocusLabel({ focus }: { focus?: string }) {
  if (!focus || !focusLabels[focus]) return null
  return <span title="Training focus" className="ml-1.5 inline-block shrink-0 align-middle text-[10px] text-muted">{focusLabels[focus]}</span>
}

export const tierLabel: Record<string, string> = { bronze: 'Bronze', silver: 'Silver', gold: 'Gold', hof: 'Hall of Fame', legend: 'Legend' }
export const tierShort: Record<string, string> = { bronze: 'B', silver: 'S', gold: 'G', hof: 'HOF', legend: 'L' }
export const tierOrder = ['legend', 'hof', 'gold', 'silver', 'bronze']
export const groupLabel: Record<string, string> = { finishing: 'Finishing', shooting: 'Shooting', playmaking: 'Playmaking', defense: 'Defense', rebounding: 'Rebounding', general: 'General', personality: 'Personality' }
export const groupOrder = ['finishing', 'shooting', 'playmaking', 'defense', 'rebounding', 'general', 'personality']
const tierStyle: Record<string, string> = {
  bronze: 'bg-amber-700/25 text-amber-300 ring-amber-600/60',
  silver: 'bg-slate-300/15 text-slate-200 ring-slate-300/50',
  gold: 'bg-yellow-400/20 text-yellow-300 ring-yellow-400/60',
  hof: 'bg-purple-500/25 text-purple-200 ring-purple-400/60',
  legend: 'bg-gradient-to-r from-red-600/50 to-orange-500/50 text-orange-100 ring-orange-400/80',
  '': 'bg-sky-400/15 text-sky-200 ring-sky-400/50',
}

export function BadgeChip({ b, active, onClick }: { b: Badge; active?: boolean; onClick?: () => void }) {
  const tip = b.tier ? `${b.name} (${tierLabel[b.tier]})` : b.name
  return (
    <button type="button" title={tip} aria-expanded={active} onClick={onClick}
      className={`inline-flex max-w-full items-center gap-1 rounded-full px-2 py-0.5 text-xs font-semibold ring-1 ${tierStyle[b.tier] ?? tierStyle['']} ${active ? 'outline outline-2 outline-ink/70' : ''}`}>
      <span className="truncate">{b.name}</span>
      {b.tier && <span className="shrink-0 text-[10px] font-extrabold opacity-80">{tierShort[b.tier]}</span>}
      {b.kind === 'performance' && <span className="shrink-0" title="Raised by last season's stats">↑</span>}
    </button>
  )
}

export function BadgeCount({ n }: { n?: number }) {
  if (!n) return null
  return <span title={`${n} badges`} className="ml-1.5 inline-block shrink-0 whitespace-nowrap rounded-full bg-panel2 px-1.5 py-px align-middle text-[10px] font-semibold tabular-nums text-muted ring-1 ring-line">{n} ⬢</span>
}

const gradeTone: Record<string, string> = {
  A: 'bg-win/20 text-win ring-win/50',
  B: 'bg-teal-400/20 text-teal-300 ring-teal-400/50',
  C: 'bg-amber-400/20 text-amber-400 ring-amber-400/50',
  D: 'bg-loss/20 text-loss ring-loss/50',
}

// Roster fit grade (A to D) from the coach and system.
export function FitGrade({ fit, size = 'sm', title }: { fit?: Fit; size?: 'sm' | 'lg'; title?: string }) {
  if (!fit) return null
  const g = fit.grade.charAt(0).toUpperCase()
  const tone = gradeTone[g] ?? 'bg-panel2 text-muted ring-line'
  const cls = size === 'lg' ? 'min-w-14 px-4 py-2 text-4xl' : 'min-w-6 px-1.5 py-px text-[11px]'
  return (
    <span className={`inline-block rounded text-center font-extrabold tabular-nums ring-1 ${tone} ${cls}`} title={title ?? `Fit ${fit.score}`} aria-label={`Fit grade ${fit.grade}`}>
      {fit.grade}
    </span>
  )
}

export const moraleColor: Record<MoraleLabel, string> = {
  Furious: '#ef4444', Unhappy: '#fb923c', Content: '#94a3b8', Happy: '#4ade80', Thrilled: '#22ff88',
}

// Small pill: coloured dot plus the mood word. With dotOnly (narrow screens) the word is hidden.
export function MoraleChip({ morale, dotOnly = false }: { morale?: MoraleBrief; dotOnly?: boolean }) {
  if (!morale) return null
  return (
    <span title={`${morale.label} (${morale.score}/100)`} className="inline-flex shrink-0 items-center gap-1.5 whitespace-nowrap rounded-full bg-panel2 px-2 py-px align-middle text-[11px] font-semibold">
      <span className="inline-block h-2 w-2 rounded-full" style={{ background: moraleColor[morale.label] }} />
      <span className={dotOnly ? 'max-[699px]:sr-only' : ''}>{morale.label}</span>
    </span>
  )
}

export const TradeRequestTag = () => (
  <span title="He has asked to be traded" className="ml-1.5 inline-block shrink-0 rounded bg-loss/20 px-1.5 py-px align-middle text-[10px] font-semibold uppercase tracking-wide text-loss ring-1 ring-loss/50">Wants out</span>
)
