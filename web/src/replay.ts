// Pure event-replay logic for the game viewer. No React, no DOM, so a 3D view can reuse it.
import type { GameEvent } from './api'

export type Side = { teamId: number; lines: { id: number; name: string; pos: string }[] }
export type PlayerTotals = {
  id: number; name: string; pos: string
  pts: number; reb: number; ast: number; fgm: number; fga: number; tpm: number; tpa: number; ftm: number; fta: number; tov: number
}
export type ReplayState = {
  index: number // number of events applied
  home: number; away: number; period: number; clock: number
  boxes: [PlayerTotals[], PlayerTotals[]]
  last: GameEvent | null
}

export const periodLabel = (p: number) => (p <= 4 ? `Q${p}` : p === 5 ? 'OT' : `${p - 4}OT`)
export const clockLabel = (c: number) => `${Math.floor(c / 60)}:${String(Math.floor(c % 60)).padStart(2, '0')}`

function blank(sides: [Side, Side]): [PlayerTotals[], PlayerTotals[]] {
  const mk = (s: Side) =>
    s.lines.map((l) => ({ id: l.id, name: l.name, pos: l.pos, pts: 0, reb: 0, ast: 0, fgm: 0, fga: 0, tpm: 0, tpa: 0, ftm: 0, fta: 0, tov: 0 }))
  return [mk(sides[0]), mk(sides[1])]
}

// Engine event types: 2, 3 (made), m2, m3 (missed), ft (N of 2 made), tov, oreb, dreb.
function applyEvent(e: GameEvent, st: ReplayState, sides: [Side, Side]) {
  const side = sides[0].teamId === e.t ? 0 : 1
  const find = (id: number) => st.boxes[side].find((p) => p.id === id)
  const me = find(e.pl)
  if (!me) return
  switch (e.y) {
    case '2':
    case '3':
      me.fgm++; me.fga++; me.pts += 2
      if (e.y === '3') { me.tpm++; me.tpa++; me.pts++ }
      if (e.a >= 0) { const a = find(e.a); if (a) a.ast++ }
      break
    case 'm2': me.fga++; break
    case 'm3': me.fga++; me.tpa++; break
    case 'ft': me.fta += 2; me.ftm += e.n ?? 0; me.pts += e.n ?? 0; break
    case 'tov': me.tov++; break
    case 'oreb': case 'dreb': me.reb++; break
  }
}

export class Replay {
  readonly events: GameEvent[]
  readonly sides: [Side, Side]
  readonly final: { home: number; away: number }
  constructor(events: GameEvent[], sides: [Side, Side]) {
    this.events = events
    this.sides = sides
    const l = events[events.length - 1]
    this.final = { home: l ? l.hs : 0, away: l ? l.as : 0 }
  }
  get length() { return this.events.length }
  // State after applying the first n events (recomputed from scratch; games are ~500 events).
  at(n: number): ReplayState {
    const st: ReplayState = { index: 0, home: 0, away: 0, period: 1, clock: 720, boxes: blank(this.sides), last: null }
    const k = Math.max(0, Math.min(n, this.events.length))
    for (let i = 0; i < k; i++) {
      const e = this.events[i]
      applyEvent(e, st, this.sides)
      st.home = e.hs
      st.away = e.as
      st.period = e.p
      st.clock = e.c
      st.last = e
    }
    st.index = k
    return st
  }
  // Seconds of game time elapsed before event i (used to pace playback by game clock).
  elapsed(i: number): number {
    const e = this.events[Math.min(i, this.events.length - 1)]
    if (!e) return 0
    const done = e.p <= 4 ? (e.p - 1) * 720 : 2880 + (e.p - 5) * 300
    const len = e.p <= 4 ? 720 : 300
    return done + (len - e.c)
  }
  // Human text for an event.
  describe(e: GameEvent, name: (id: number) => string): string {
    const who = name(e.pl)
    switch (e.y) {
      case '2': return `${who} makes a 2-pointer${e.a >= 0 ? ` (assist ${name(e.a)})` : ''}`
      case '3': return `${who} makes a 3-pointer${e.a >= 0 ? ` (assist ${name(e.a)})` : ''}`
      case 'm2': return `${who} misses a 2-pointer`
      case 'm3': return `${who} misses a 3-pointer`
      case 'ft': return `${who} makes ${e.n ?? 0} of 2 free throws`
      case 'tov': return `${who} turns it over`
      case 'oreb': return `${who} grabs an offensive rebound`
      case 'dreb': return `${who} grabs the rebound`
      default: return `${who} ${e.y}`
    }
  }
  isScore(e: GameEvent) { return e.y === '2' || e.y === '3' || (e.y === 'ft' && (e.n ?? 0) > 0) }
}
