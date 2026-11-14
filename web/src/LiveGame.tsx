import { useEffect, useMemo, useRef, useState } from 'react'
import type { GameEvents } from './api'
import { Replay, clockLabel, periodLabel } from './replay'
import { AnimatedNumber, TeamBadge } from './ui'

const SPEEDS = [1, 4, 16, 64]

// Plays one recorded game inside the sim window. Calls onDone once, about a second after the final event
// (or at once after "Skip to final"). Pure replay logic comes from replay.ts.
export function LiveGame({ game, onDone }: { game: GameEvents; onDone: () => void }) {
  const replay = useMemo(
    () => new Replay(game.events, [
      { teamId: game.teams[0].team.id, lines: game.teams[0].lines },
      { teamId: game.teams[1].team.id, lines: game.teams[1].lines },
    ]),
    [game],
  )
  const [idx, setIdx] = useState(0)
  const [speed, setSpeed] = useState(4)
  const acc = useRef(0)
  const doneRef = useRef(onDone)
  doneRef.current = onDone
  const finished = idx >= replay.length

  useEffect(() => {
    if (finished) return
    const t = setInterval(() => {
      acc.current += (speed * 2) / 10
      const n = Math.floor(acc.current)
      if (n > 0) { acc.current -= n; setIdx((i) => Math.min(replay.length, i + n)) }
    }, 100)
    return () => clearInterval(t)
  }, [speed, finished, replay.length])

  useEffect(() => {
    if (!finished) return
    const t = setTimeout(() => doneRef.current(), 1000)
    return () => clearTimeout(t)
  }, [finished])

  const st = replay.at(idx)
  const names = useMemo(() => new Map(game.teams.flatMap((t) => t.lines.map((l) => [l.id, l.name] as const))), [game])
  const nameOf = (id: number) => names.get(id) ?? '?'
  const feed: { i: number; text: string; score: boolean }[] = []
  for (let i = idx - 1; i >= Math.max(0, idx - 6); i--) {
    const e = game.events[i]
    feed.push({ i, text: replay.describe(e, nameOf), score: replay.isScore(e) })
  }
  const [home, away] = [game.teams[0], game.teams[1]]
  const side = (t: typeof home, score: number) => (
    <div className="flex min-w-0 flex-1 flex-col items-center gap-1">
      <TeamBadge team={t.team} size={32} />
      <span className="max-w-full truncate text-xs text-muted">{t.team.name}</span>
      <span className="text-4xl font-black tabular-nums"><AnimatedNumber value={score} /></span>
    </div>
  )
  return (
    <div className="mt-2 rounded-lg border border-accent/60 bg-panel2 p-3" aria-label="Live game">
      <div className="flex items-center gap-2">
        {side(away, st.away)}
        <div className="w-20 text-center">
          <div className="font-bold">{finished ? 'Final' : periodLabel(st.period)}</div>
          {!finished && <div className="text-lg tabular-nums text-muted">{clockLabel(st.clock)}</div>}
        </div>
        {side(home, st.home)}
      </div>
      <ul className="mt-2 h-[120px] overflow-hidden text-[13px]">
        {feed.map((f, k) => (
          <li key={f.i} className={`truncate border-b border-line/40 py-0.5 ${f.score ? 'font-semibold' : 'text-muted'}`} style={{ opacity: 1 - k * 0.13 }}>{f.text}</li>
        ))}
      </ul>
      <div className="mt-2 flex flex-wrap items-center gap-2">
        {SPEEDS.map((s) => (
          <button key={s} className={`btn ${speed === s ? 'btn-accent' : ''}`} onClick={() => setSpeed(s)}>{s}x</button>
        ))}
        <button className="btn ml-auto" disabled={finished} onClick={() => setIdx(replay.length)}>Skip to final</button>
      </div>
    </div>
  )
}
