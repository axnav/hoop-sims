import { useState } from 'react'
import type { LineupPlayer } from './api'
import { InjuryTag, ovrColor } from './ui'

type Sel = { kind: 'slot'; i: number } | { kind: 'player'; id: number } | null

// Spot centres as percentages of the court picture (baseline at the bottom).
const SPOTS: Record<string, [number, number]> = {
  PG: [50, 36], SG: [15, 56], SF: [85, 56], PF: [25, 77], C: [50, 72],
}
const FALLBACK: [number, number] = [50, 50]

const lastName = (n: string) => {
  const parts = n.trim().split(/\s+/)
  return parts.length > 1 ? parts.slice(1).join(' ') : parts[0]
}
// Display only: the API sent the positions this player can play.
const outOfPos = (slot: string, pos: string) => !pos.split('/').map((s) => s.trim()).includes(slot)

function Court() {
  const l = 'var(--color-line)'
  return (
    <svg viewBox="0 0 500 470" className="absolute inset-0 h-full w-full" fill="none" stroke={l} strokeWidth={2.5} aria-hidden="true">
      <rect x="2" y="2" width="496" height="466" rx="6" />
      <rect x="170" y="280" width="160" height="188" />
      <rect x="205" y="280" width="90" height="188" strokeWidth={1.5} opacity={0.6} />
      <path d="M190 280 A60 60 0 0 1 310 280" />
      <path d="M190 280 A60 60 0 0 0 310 280" strokeDasharray="7 7" />
      <path d="M40 468 V325 A235 235 0 0 1 460 325 V468" />
      <path d="M205 408 A45 45 0 0 1 295 408" strokeWidth={1.5} />
      <line x1="220" y1="448" x2="280" y2="448" strokeWidth={4} />
      <circle cx="250" cy="432" r="10" />
    </svg>
  )
}

export default function CourtLineup({ slots, starters, players, minutes, onSet }: {
  slots: string[]
  starters: number[]
  players: LineupPlayer[]
  minutes: Record<number, number>
  onSet: (slot: number, id: number) => void // same as the page's starter dropdowns
}) {
  const [sel, setSel] = useState<Sel>(null)
  const [drag, setDrag] = useState<number | null>(null) // player id being dragged
  const [over, setOver] = useState<number | 'bench' | null>(null)
  const byId = new Map(players.map((p) => [p.id, p]))
  const bench = players.filter((p) => !starters.includes(p.id))

  const place = (slot: number, id: number) => { onSet(slot, id); setSel(null) }

  const clickSpot = (i: number) => {
    if (sel?.kind === 'player') return place(i, sel.id)
    if (sel?.kind === 'slot') {
      if (sel.i === i) return setSel(null)
      const a = starters[sel.i], b = starters[i]
      if (a >= 0) return place(i, a) // fills or swaps (the page's setStarter moves the old one across)
      if (b >= 0) return place(sel.i, b)
    }
    setSel({ kind: 'slot', i })
  }
  const clickBench = (id: number) => {
    if (sel?.kind === 'slot') return place(sel.i, id)
    setSel(sel?.kind === 'player' && sel.id === id ? null : { kind: 'player', id })
  }
  const startDrag = (e: React.DragEvent, id: number) => {
    e.dataTransfer.setData('text/plain', String(id))
    e.dataTransfer.effectAllowed = 'move'
    setDrag(id); setSel(null)
  }
  const endDrag = () => { setDrag(null); setOver(null) }
  const dropSpot = (e: React.DragEvent, i: number) => {
    e.preventDefault()
    if (drag != null) place(i, drag)
    endDrag()
  }
  const dropBench = (e: React.DragEvent) => {
    e.preventDefault()
    const j = drag != null ? starters.indexOf(drag) : -1
    if (j >= 0) place(j, -1)
    endDrag()
  }

  return (
    <div className="flex flex-col gap-4 px-2 pb-2 lg:flex-row lg:items-start">
      <div className="relative w-full max-w-[520px] shrink-0 overflow-hidden rounded-lg bg-panel2" style={{ aspectRatio: '500 / 470' }}>
        <Court />
        {slots.map((slot, i) => {
          const id = starters[i]
          const p = id >= 0 ? byId.get(id) : undefined
          const [x, y] = SPOTS[slot] ?? FALLBACK
          const picked = sel?.kind === 'slot' && sel.i === i
          const off = p ? outOfPos(slot, p.pos) : false
          const ring = picked ? 'ring-2 ring-accent bg-accent/20' : over === i ? 'ring-2 ring-accent' : off ? 'ring-2 ring-amber-400' : p ? 'ring-1 ring-line' : 'ring-1 ring-dashed ring-line'
          return (
            <div key={slot} className="absolute -translate-x-1/2 -translate-y-1/2" style={{ left: `${x}%`, top: `${y}%`, width: '19%', minWidth: 52 }}>
              <div className="relative">
                <button type="button" aria-pressed={picked} aria-label={`${slot} spot: ${p ? p.name : 'empty'}`}
                  draggable={!!p} onDragStart={(e) => p && startDrag(e, p.id)} onDragEnd={endDrag}
                  onDragOver={(e) => { e.preventDefault(); setOver(i) }} onDragLeave={() => setOver((o) => (o === i ? null : o))}
                  onDrop={(e) => dropSpot(e, i)} onClick={() => clickSpot(i)}
                  className={`flex aspect-square w-full flex-col items-center justify-center rounded-full bg-panel p-0.5 text-center leading-tight ${ring}`}>
                  {p ? (
                    <>
                      <span className="max-w-full truncate text-[10px] font-semibold sm:text-[11px]">{lastName(p.name)}</span>
                      <b className="text-sm tabular-nums sm:text-base" style={{ color: ovrColor(p.ovr) }}>{p.ovr}</b>
                      <span className="text-[9px] text-muted">{p.pos}</span>
                    </>
                  ) : (
                    <span className="text-xs font-bold text-muted">{slot}</span>
                  )}
                </button>
                <span className="pointer-events-none absolute -top-1 left-0 rounded bg-panel px-1 text-[9px] font-bold uppercase tracking-wider text-muted ring-1 ring-line">{slot}</span>
                {p?.injury && <span className="pointer-events-none absolute -bottom-1 left-1/2 -translate-x-1/2"><InjuryTag injury={p.injury} /></span>}
                {p && (
                  <button type="button" aria-label={`Remove ${p.name} from ${slot} spot`} onClick={() => place(i, -1)}
                    className="absolute -top-1 -right-1 flex h-6 w-6 items-center justify-center rounded-full bg-panel text-xs text-muted ring-1 ring-line hover:text-loss">&times;</button>
                )}
              </div>
              {off && <div className="mt-0.5 text-center text-[9px] font-semibold text-amber-400">out of position</div>}
            </div>
          )
        })}
      </div>

      <div className={`min-w-0 flex-1 rounded-lg p-2 ${over === 'bench' ? 'ring-2 ring-accent' : 'ring-1 ring-line'}`}
        onDragOver={(e) => { e.preventDefault(); setOver('bench') }} onDragLeave={() => setOver((o) => (o === 'bench' ? null : o))} onDrop={dropBench}>
        <div className="mb-2 text-[11px] font-bold uppercase tracking-wider text-muted">Bench</div>
        <p className="mb-2 text-xs text-muted">Tap a spot, then a bench player (or the other way round). Drag works too. Tap a filled spot, then another, to swap.</p>
        <div className="grid grid-cols-1 gap-1.5 sm:grid-cols-2 lg:grid-cols-1">
          {bench.map((p) => {
            const picked = sel?.kind === 'player' && sel.id === p.id
            return (
              <button key={p.id} type="button" aria-pressed={picked} aria-label={`Bench: ${p.name}`}
                draggable onDragStart={(e) => startDrag(e, p.id)} onDragEnd={endDrag} onClick={() => clickBench(p.id)}
                className={`flex min-h-11 items-center gap-2 rounded-md bg-panel2 px-2 py-1 text-left text-sm ${picked ? 'ring-2 ring-accent' : 'ring-1 ring-line'}`}>
                <span className="min-w-0 flex-1 truncate font-medium">{p.name}<InjuryTag injury={p.injury} /></span>
                <span className="text-xs text-muted">{p.pos}</span>
                <b className="tabular-nums" style={{ color: ovrColor(p.ovr) }}>{p.ovr}</b>
                <span className="w-10 text-right text-xs tabular-nums text-muted">{minutes[p.id] ?? 0}m</span>
              </button>
            )
          })}
          {bench.length === 0 && <div className="text-sm text-muted">No one on the bench.</div>}
        </div>
      </div>
    </div>
  )
}
