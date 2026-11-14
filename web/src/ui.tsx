import { useEffect, useRef, useState, type ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { animate, motion, useReducedMotion } from 'motion/react'
import type { UseQueryResult } from '@tanstack/react-query'
import {
  flexRender, getCoreRowModel, getSortedRowModel, useReactTable, type ColumnDef, type SortingState,
} from '@tanstack/react-table'
import { ArrowDown, ArrowUp, Loader2 } from 'lucide-react'
import type { Injury, Team } from './api'

export const f1 = (v: number | null | undefined) => (v == null ? '-' : v.toFixed(1))
export const pct3 = (v: number) => (v === 1 ? '1.000' : v.toFixed(3).replace(/^0/, ''))

function lum(hex: string) {
  const n = parseInt(hex.slice(1), 16)
  return (0.299 * (n >> 16) + 0.587 * ((n >> 8) & 255) + 0.114 * (n & 255)) / 255
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

export type Col<T> = ColumnDef<T, any> & { num?: boolean }

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
                <th key={h.id} className={`cursor-pointer select-none ${num ? 'r' : ''}`} onClick={h.column.getToggleSortingHandler()}>
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
              <td key={c.id} className={(c.column.columnDef as Col<T>).num ? 'r' : ''}>
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

// Small label for an asset's tier. Top tiers get a stronger look.
export function TierChip({ tier }: { tier: string }) {
  const top = tier === 'Franchise' || tier === 'Lottery pick'
  return (
    <span className={`ml-1.5 shrink-0 rounded px-1.5 py-px align-middle text-[10px] font-semibold uppercase tracking-wide ${top ? 'bg-accent/25 text-accent ring-1 ring-accent/60' : 'bg-panel2 text-muted'}`}>{tier}</span>
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
