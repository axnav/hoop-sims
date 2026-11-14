import { Component, lazy, Suspense, useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { Link, useParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { AnimatePresence, motion } from 'motion/react'
import { Box as BoxIcon, FastForward, Pause, Play, Square } from 'lucide-react'
import { api, type GameEvents } from '../api'
import { Replay, clockLabel, periodLabel, type PlayerTotals } from '../replay'
import { AnimatedNumber, Card, ErrorBox, Page, PlayerLink, Q, Spinner, TeamBadge } from '../ui'
import type { CamCmd, CamName } from '../court3d/Court3D'

const Court3D = lazy(() => import('../court3d/Court3D'))

class Boundary extends Component<{ onError: () => void; children: ReactNode }, { failed: boolean }> {
  state = { failed: false }
  static getDerivedStateFromError() { return { failed: true } }
  componentDidCatch() { this.props.onError() }
  render() { return this.state.failed ? null : this.props.children }
}

const MODE_KEY = 'hoopsim.viewMode'
function webglOK() {
  try {
    const c = document.createElement('canvas')
    return !!(c.getContext('webgl2') || c.getContext('webgl'))
  } catch { return false }
}
const reduced = () => { try { return window.matchMedia('(prefers-reduced-motion: reduce)').matches } catch { return false } }
function initialMode(): '2d' | '3d' {
  if (!webglOK()) return '2d'
  try {
    const v = localStorage.getItem(MODE_KEY)
    if (v === '2d' || v === '3d') return v
  } catch { /* storage unavailable */ }
  return reduced() ? '2d' : '3d'
}

function MiniBox({ rows }: { rows: PlayerTotals[] }) {
  return (
    <table className="tbl">
      <thead><tr><th>Player</th><th className="r">PTS</th><th className="r">REB</th><th className="r">AST</th><th className="r">TOV</th><th className="r">FG</th><th className="r">3P</th><th className="r">FT</th></tr></thead>
      <tbody>
        {rows.map((p) => (
          <tr key={p.id}><td><PlayerLink id={p.id} name={p.name} /></td><td className="r font-bold">{p.pts}</td><td className="r">{p.reb}</td><td className="r">{p.ast}</td><td className="r">{p.tov}</td>
            <td className="r">{p.fgm}-{p.fga}</td><td className="r">{p.tpm}-{p.tpa}</td><td className="r">{p.ftm}-{p.fta}</td></tr>
        ))}
      </tbody>
    </table>
  )
}

function Viewer({ g }: { g: GameEvents }) {
  const replay = useMemo(
    () => new Replay(g.events, [
      { teamId: g.teams[0].team.id, lines: g.teams[0].lines },
      { teamId: g.teams[1].team.id, lines: g.teams[1].lines },
    ]),
    [g],
  )
  const [idx, setIdx] = useState(0)
  const [playing, setPlaying] = useState(true)
  const [speed, setSpeed] = useState(4)
  const acc = useRef(0)
  const finished = idx >= replay.length
  const [mode, setMode] = useState<'2d' | '3d'>(initialMode)
  const [failed, setFailed] = useState(false)
  const idxRef = useRef(0), speedRef = useRef(speed)
  idxRef.current = idx
  speedRef.current = speed
  const camRef = useRef<CamCmd>({ name: 'broadcast', n: 0 })
  const [cam, setCam] = useState<CamName>('broadcast')
  const pickMode = (m: '2d' | '3d') => {
    setMode(m); setFailed(false)
    try { localStorage.setItem(MODE_KEY, m) } catch { /* ignore */ }
  }
  const view3d = mode === '3d' && !failed
  useEffect(() => {
    if (!playing || finished) return
    const t = setInterval(() => {
      acc.current += (speed * 2) / 10
      const n = Math.floor(acc.current)
      if (n > 0) { acc.current -= n; setIdx((i) => Math.min(replay.length, i + n)) }
    }, 100)
    return () => clearInterval(t)
  }, [playing, speed, finished, replay.length])

  const st = replay.at(idx)
  const names = new Map(g.teams.flatMap((t) => t.lines.map((l) => [l.id, l.name] as const)))
  const nameOf = (id: number) => names.get(id) ?? '?'
  const feed = []
  for (let i = idx - 1; i >= Math.max(0, idx - 40); i--) feed.push({ i, e: g.events[i] })
  const [home, away] = [g.teams[0], g.teams[1]]
  const teamOf = (id: number) => (home.team.id === id ? home.team : away.team)

  const controls = (
    <>
        <button className="btn" onClick={() => setPlaying((p) => !p)} disabled={finished}>{playing && !finished ? <Pause size={15} /> : <Play size={15} />}{playing && !finished ? 'Pause' : 'Play'}</button>
        {[1, 4, 16].map((s) => <button key={s} className={`btn ${speed === s ? 'border-accent text-accent' : ''}`} onClick={() => { setSpeed(s); setPlaying(true) }}>{s}x</button>)}
        <button className="btn" onClick={() => setIdx(replay.length)} disabled={finished}><FastForward size={15} />Skip to end</button>
    </>
  )

  return (
    <Page title={g.label} right={<Link className="link text-muted" to="/playoffs">Back to bracket</Link>}>
      <div className="mb-3 flex items-center gap-2">
        <div className="flex overflow-hidden rounded-lg border border-line">
          {(['2d', '3d'] as const).map((m) => (
            <button key={m} className={`flex items-center gap-1.5 px-3 py-1.5 font-semibold ${mode === m ? 'bg-accent text-[#170a00]' : 'bg-panel2 text-muted hover:text-ink'}`} onClick={() => pickMode(m)}>
              {m === '2d' ? <Square size={13} /> : <BoxIcon size={13} />}{m.toUpperCase()}
            </button>
          ))}
        </div>
        {failed && <span className="text-xs text-muted">3D is not available here, showing 2D.</span>}
      </div>
      {view3d ? (
        <div className="relative mb-4 h-[min(66vh,640px)] min-h-[380px] overflow-hidden rounded-xl border border-line bg-[#06080d]">
          <Boundary onError={() => setFailed(true)}>
            <Suspense fallback={<Spinner label="Loading 3D court" />}>
              <Court3D
                replay={replay} names={names} idxRef={idxRef} speedRef={speedRef} camRef={camRef} reduceMotion={reduced()}
                home={{ id: home.team.id, primary: home.team.primary, secondary: home.team.secondary }}
                away={{ id: away.team.id, primary: away.team.primary, secondary: away.team.secondary }}
              />
            </Suspense>
          </Boundary>
          <div className="pointer-events-none absolute inset-x-0 top-0 flex justify-center p-3">
            <div className="flex items-center gap-4 rounded-xl bg-black/65 px-4 py-2 backdrop-blur">
              {[away, home].map((t, k) => (
                <div key={t.team.id} className={`flex items-center gap-2 ${k === 1 ? 'order-3 flex-row-reverse' : ''}`}>
                  <TeamBadge team={t.team} size={26} />
                  <span className="text-3xl font-black tabular-nums">{k === 0 ? st.away : st.home}</span>
                </div>
              ))}
              <div className="order-2 text-center leading-tight">
                <div className="font-extrabold">{finished ? 'Final' : periodLabel(st.period)}</div>
                <div className="tabular-nums text-muted">{finished ? (g.ot ? `${g.ot > 1 ? g.ot : ''}OT` : '') : clockLabel(st.clock)}</div>
              </div>
            </div>
          </div>
          <ul className="pointer-events-none absolute bottom-14 left-3 w-[min(340px,70%)] space-y-0.5 text-[12px]">
            {feed.slice(0, 5).map(({ i, e }) => (
              <li key={i} className={`truncate rounded bg-black/55 px-2 py-0.5 ${replay.isScore(e) ? 'text-ink' : 'text-muted'}`}>
                {periodLabel(e.p)} {clockLabel(e.c)} · {replay.describe(e, nameOf)}
              </li>
            ))}
          </ul>
          <div className="absolute inset-x-0 bottom-0 flex flex-wrap items-center gap-2 bg-gradient-to-t from-black/70 to-transparent p-3">
            {controls}
            <span className="ml-auto flex gap-1">
              {(['broadcast', 'baseline', 'overhead'] as const).map((c) => (
                <button key={c} className={`btn px-2 py-1 text-xs capitalize ${cam === c ? 'border-accent text-accent' : ''}`}
                  onClick={() => { camRef.current = { name: c, n: camRef.current.n + 1 }; setCam(c) }}>{c}</button>
              ))}
            </span>
          </div>
        </div>
      ) : (
        <>
      <div className="card mb-4 grid grid-cols-[1fr_auto_1fr] items-center gap-4 p-5">
        {[away, home].map((t, k) => (
          <div key={t.team.id} className={`flex items-center gap-4 ${k === 0 ? '' : 'order-3 flex-row-reverse'}`}>
            <TeamBadge team={t.team} size={44} />
            <div className={k === 0 ? '' : 'text-right'}>
              <div className="text-muted">{k === 0 ? 'Away' : 'Home'}</div>
              <div className="text-lg font-bold">{t.team.fullName}</div>
            </div>
            <div className={`text-6xl font-black tabular-nums ${k === 0 ? 'ml-auto' : 'mr-auto'}`}><AnimatedNumber value={k === 0 ? st.away : st.home} /></div>
          </div>
        ))}
        <div className="order-2 text-center">
          <div className="text-2xl font-extrabold">{finished ? 'Final' : periodLabel(st.period)}</div>
          <div className="text-xl tabular-nums text-muted">{finished ? (g.ot ? `${g.ot > 1 ? g.ot : ''}OT` : '') : clockLabel(st.clock)}</div>
        </div>
      </div>
      <div className="mb-4 flex flex-wrap items-center gap-2">
        {controls}
        <span className="ml-auto text-xs text-muted tabular-nums">{idx} / {replay.length} plays</span>
      </div>
        </>
      )}
      <div className="grid gap-4 xl:grid-cols-[320px_1fr]">
        <Card title="Play by play">
          <ul className="h-[460px] overflow-hidden">
            <AnimatePresence initial={false}>
              {feed.map(({ i, e }) => (
                <motion.li
                  key={i} initial={{ opacity: 0, height: 0 }} animate={{ opacity: 1, height: 'auto' }} transition={{ duration: 0.15 }}
                  className={`flex gap-2 overflow-hidden border-b border-line/50 px-2 py-1 text-[13px] ${replay.isScore(e) ? 'text-ink' : 'text-muted'}`}
                >
                  <TeamBadge team={teamOf(e.t)} size={14} />
                  <span className="w-14 shrink-0 tabular-nums text-muted">{periodLabel(e.p)} {clockLabel(e.c)}</span>
                  <span className="flex-1">{replay.describe(e, nameOf)}</span>
                  <span className="tabular-nums">{e.hs}-{e.as}</span>
                </motion.li>
              ))}
            </AnimatePresence>
          </ul>
        </Card>
        <div className="grid gap-4">
          {[away, home].map((t, k) => (
            <Card key={t.team.id} title={t.team.fullName}><MiniBox rows={st.boxes[k === 0 ? 1 : 0].filter((p) => p.pts + p.reb + p.ast + p.fga + p.tov + p.fta > 0 || idx === 0)} /></Card>
          ))}
        </div>
      </div>
      {finished && <div className="mt-4 text-muted">Game over. <Link className="link text-accent" to={`/box/${g.id}`}>Full box score</Link></div>}
    </Page>
  )
}

export default function GameViewer() {
  const id = useParams().id!
  const q = useQuery({ queryKey: ['events', id], queryFn: () => api.events(id), staleTime: Infinity })
  if (q.isError) return <ErrorBox error={q.error} />
  return <Q q={q} minH={400}>{(g) => <Viewer key={g.id} g={g} />}</Q>
}
