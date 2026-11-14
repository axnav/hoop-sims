import { useEffect, useRef, useState } from 'react'
import { AnimatePresence, motion } from 'motion/react'
import { useQueryClient } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { api, type DayResult, type DevLine, type GameEvents, type GameState, type InjuryLine, type SimPause, type Team } from './api'
import { AnimatedNumber, Delta, TeamBadge, WL } from './ui'
import { LiveGame } from './LiveGame'
import { OfferList, useOffers } from './OffersCard'

const WATCH_KEY = 'hoopsim.watchGames'
function readWatch() {
  try { return localStorage.getItem(WATCH_KEY) === '1' } catch { return false }
}

// Pending offers shown inside the sim window (only mounted while paused for an offer).
function OfferPause({ onChangeIt }: { onChangeIt: (id: number) => void }) {
  const { offers, accepted, accept, reject, onAccept, onReject } = useOffers()
  return (
    <div className="mt-3 space-y-3">
      {accepted && offers.length === 0 && (
        <div className="rounded-lg border border-win/50 p-2 font-semibold text-win">Trade complete</div>
      )}
      <OfferList compact offers={offers} onChangeIt={onChangeIt} onAccept={onAccept} onReject={onReject} accept={accept} reject={reject} />
    </div>
  )
}

// watch: play the user's game live for this run even if the saved "Watch my games" switch is off.
export type SimPlan = { days: number; toEnd: boolean; watch?: boolean }
type Strip = { rank: number; team: Team; w: number; l: number }[]

export function SimOverlay({ plan, onClose }: { plan: SimPlan; onClose: () => void }) {
  const qc = useQueryClient()
  const start = qc.getQueryData<GameState>(['state'])!
  const [results, setResults] = useState<DayResult[]>([])
  const [strip, setStrip] = useState<Strip>([])
  const [done, setDone] = useState(false)
  const [error, setError] = useState('')
  const stop = useRef(false)
  const started = useRef(false)
  const nav = useNavigate()
  const remaining = useRef(0)
  const [hurt, setHurt] = useState<InjuryLine[]>([])
  const [dev, setDev] = useState<DevLine[]>([])
  const [pause, setPause] = useState<SimPause | null>(null)
  const [noStop, setNoStop] = useState(false)
  const closeBtn = useRef<HTMLButtonElement>(null)
  const [watch, setWatch] = useState(() => !!plan.watch || readWatch())
  const watchRef = useRef(watch)
  const [live, setLive] = useState<GameEvents | null>(null)
  const gameRes = useRef<(() => void) | null>(null)

  const toggleWatch = (v: boolean) => {
    setWatch(v); watchRef.current = v
    try { localStorage.setItem(WATCH_KEY, v ? '1' : '0') } catch { /* storage unavailable */ }
  }
  // Ends the game view and releases the sim loop that is waiting on it.
  const finishGame = () => {
    setLive(null)
    const r = gameRes.current
    gameRes.current = null
    r?.()
  }
  const playGame = (g: GameEvents) => new Promise<void>((res) => { gameRes.current = res; setLive(g) })

  // Sims one day at a time; `remaining` survives a pause so "Keep simming" resumes where it stopped.
  const loop = async () => {
    setPause(null); setDone(false)
    try {
      while (remaining.current > 0 && !stop.current) {
        const r = await api.sim({ days: 1, auto: true })
        if (watchRef.current && !stop.current && r.days.some((d) => d.hasReplay)) {
          let g: GameEvents | null = null
          try { g = await api.events('last') } catch { /* skip silently */ }
          if (g && g.events?.length && !stop.current) await playGame(g)
        }
        setResults((p) => [...p, ...r.days])
        if (r.injuries?.length) setHurt((p) => [...p, ...r.injuries])
        if (r.development?.length) {
          setDev((p) => {
            const m = new Map(p.map((d) => [d.playerId, d]))
            for (const d of r.development!) { const o = m.get(d.playerId); m.set(d.playerId, o ? { ...d, change: o.change + d.change } : d) }
            return [...m.values()]
          })
        }
        if (r.strip) setStrip(r.strip)
        qc.setQueryData(['state'], r.state)
        if (r.days.length > 0) remaining.current--
        if (r.pause) { setPause(r.pause); return }
        if (r.state.phase !== 'regular') break
        await new Promise((res) => setTimeout(res, 110))
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
    setDone(true)
  }

  useEffect(() => {
    if (started.current) return
    started.current = true
    remaining.current = plan.toEnd ? start.totalDays - start.day : plan.days
    loop()
  }, [plan, qc, start]) // eslint-disable-line react-hooks/exhaustive-deps

  const answer = (go: boolean) => {
    if (!pause) return
    if (noStop) api.setOptions(pause.reason === 'offer' ? { noOfferStop: true } : { noDeadlineStop: true }).catch(() => {})
    setNoStop(false)
    if (go) { remaining.current = 0; setPause(null); onClose(); nav('/trade') }
    else loop()
  }
  const changeOffer = (id: number) => {
    remaining.current = 0; setPause(null); onClose(); nav(`/trade?offer=${id}`)
  }
  const closeNow = () => { remaining.current = 0; setPause(null); onClose() }
  const stopSim = () => { stop.current = true; finishGame() }

  useEffect(() => { if (done) closeBtn.current?.focus() }, [done])

  const last = results[results.length - 1]
  const w = last?.w ?? start.user.w
  const l = last?.l ?? start.user.l
  const rank = last?.rank ?? start.user.confRank
  const span = results.filter((r) => r.opp)
  const sw = span.filter((r) => r.win).length
  const sl = span.length - sw
  const day = last?.day ?? start.day

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4 backdrop-blur-sm">
      <motion.div
        initial={{ opacity: 0, y: 20, scale: 0.97 }} animate={{ opacity: 1, y: 0, scale: 1 }}
        className="card max-h-[calc(100dvh-2rem)] w-full max-w-3xl overflow-y-auto shadow-2xl"
      >
        <div className="h-1.5 bg-line"><motion.div className="h-full bg-accent" animate={{ width: `${(day / start.totalDays) * 100}%` }} /></div>
        <div className="grid gap-4 p-5 md:grid-cols-[1fr_260px]">
          <div>
            <div className="flex items-baseline justify-between">
              <h2 className="text-lg font-extrabold">{done ? (stop.current ? 'Stopped' : 'Done') : plan.watch ? 'Game day' : 'Simulating season'}</h2>
              <label className="flex items-center gap-1.5 text-[13px] text-muted">
                <input type="checkbox" checked={watch} onChange={(e) => toggleWatch(e.target.checked)} />Watch my games
              </label>
              <span className="text-muted">Day <AnimatedNumber value={day} /> / {start.totalDays}</span>
            </div>
            <div className="mt-3 flex items-center gap-4">
              <TeamBadge team={start.user.team} size={36} />
              <div className="text-3xl font-extrabold tabular-nums"><AnimatedNumber value={w} />-<AnimatedNumber value={l} /></div>
              <div className="text-muted">#<AnimatedNumber value={rank} /> in {start.user.confName}</div>
            </div>
            {live && <LiveGame game={live} onDone={finishGame} />}
            <ul className="mt-4 h-[210px] overflow-hidden">
              <AnimatePresence initial={false}>
                {results.filter((r) => r.opp).slice(-6).reverse().map((r) => (
                  <motion.li
                    key={r.day} layout initial={{ opacity: 0, x: -30 }} animate={{ opacity: 1, x: 0 }} exit={{ opacity: 0 }}
                    className="flex items-center gap-3 border-b border-line/60 py-1.5"
                  >
                    <span className="w-14 text-muted">Day {r.day}</span>
                    <WL win={!!r.win} />
                    <span className="w-20 font-bold tabular-nums">{r.me}-{r.them}</span>
                    <span className="text-muted">{r.home ? 'vs' : '@'}</span>
                    <TeamBadge team={r.opp!} size={20} />
                    <span>{r.opp!.name}</span>
                  </motion.li>
                ))}
              </AnimatePresence>
            </ul>
            {error && <div className="mt-2 text-loss">{error}</div>}
            {hurt.length > 0 && (done || pause) && (
              <div className="mt-2 rounded-lg border border-loss/50 bg-panel2 p-3 text-[13px]">
                <div className="h2 mb-1 text-loss">Injured</div>
                <ul className="max-h-24 space-y-0.5 overflow-y-auto">
                  {hurt.map((h, i) => <li key={i}>{h.name}, {h.injury}, out {h.gamesLeft} {h.gamesLeft === 1 ? 'game' : 'games'}</li>)}
                </ul>
              </div>
            )}
            {dev.length > 0 && (done || pause) && (
              <div className="mt-2 rounded-lg border border-accent/50 bg-panel2 p-3 text-[13px]">
                <div className="h2 mb-1 text-accent">Development</div>
                <ul className="max-h-24 space-y-0.5 overflow-y-auto">
                  {dev.map((d) => <li key={d.playerId}>{d.name} {d.ovr}<Delta value={d.change} scope="season" /></li>)}
                </ul>
              </div>
            )}
            {pause && (
              <div className="mt-2 rounded-lg border border-accent/60 bg-panel2 p-3" role="alertdialog" aria-label="Sim paused">
                <div className="font-semibold">{pause.text}</div>
                <label className="mt-2 flex items-center gap-2 text-[13px] text-muted">
                  <input type="checkbox" checked={noStop} onChange={(e) => setNoStop(e.target.checked)} />
                  {pause.reason === 'offer' ? "Don't stop for offers" : "Don't stop before the deadline"}
                </label>
                {pause.reason === 'offer' && <OfferPause onChangeIt={changeOffer} />}
                <div className="mt-2 flex flex-wrap gap-2">
                  {pause.reason !== 'offer' && <button className="btn btn-accent" onClick={() => answer(true)}>Go to trades</button>}
                  <button className={`btn ${pause.reason === 'offer' ? 'btn-accent' : ''}`} onClick={() => answer(false)}>Keep simming</button>
                  {pause.reason === 'offer' && <button className="btn" onClick={closeNow}>Close</button>}
                </div>
              </div>
            )}
            <div className="mt-3 flex items-center justify-between">
              {done ? (
                <>
                  <span className="text-muted">
                    {span.length} games: <b className="text-ink">{sw}-{sl}</b>, rank #{results.length ? rank : start.user.confRank}
                    {results.length > 0 && start.user.confRank !== rank && (
                      <span className={rank < start.user.confRank ? 'text-win' : 'text-loss'}> (from #{start.user.confRank})</span>
                    )}
                  </span>
                  <button ref={closeBtn} className="btn btn-accent" onClick={onClose}>Close</button>
                </>
              ) : pause ? <span /> : (
                <button className="btn" onClick={stopSim}>Stop</button>
              )}
            </div>
          </div>
          <div>
            <div className="h2 mb-1">{start.user.confName}</div>
            <ul>
              {strip.map((s) => (
                <motion.li
                  key={s.team.id} layout transition={{ type: 'spring', stiffness: 400, damping: 36 }}
                  className={`flex items-center gap-2 px-2 py-0.5 text-[13px] ${s.team.id === start.user.team.id ? 'rounded bg-white/10' : ''} ${s.rank === 8 ? 'border-b border-dashed border-muted/60' : ''}`}
                >
                  <span className="w-4 text-muted tabular-nums">{s.rank}</span>
                  <TeamBadge team={s.team} size={16} />
                  <span className="flex-1">{s.team.name}</span>
                  <span className="tabular-nums">{s.w}-{s.l}</span>
                </motion.li>
              ))}
            </ul>
          </div>
        </div>
      </motion.div>
    </div>
  )
}
