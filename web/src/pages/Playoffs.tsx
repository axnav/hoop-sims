import { Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { motion } from 'motion/react'
import { Trophy } from 'lucide-react'
import { api, type Series, type Team } from '../api'
import { useFlow } from '../flow'
import { InlineContinue } from '../Inline'
import { Page, Q, TeamBadge, TeamLink } from '../ui'

function Pips({ n, color }: { n: number; color: string }) {
  return (
    <span className="flex gap-0.5">
      {[0, 1, 2, 3].map((i) => (
        <motion.span
          key={i} className="h-2 w-2 rounded-full" style={{ background: i < n ? color : 'var(--color-line)' }}
          animate={{ scale: i < n ? [1, 1.5, 1] : 1 }} transition={{ duration: 0.4 }}
        />
      ))}
    </span>
  )
}

function Row({ team, seed, wins, won, lost, user }: { team: Team; seed: number; wins: number; won: boolean; lost: boolean; user: number }) {
  return (
    <div className={`flex items-center gap-2 px-2 py-1 ${lost ? 'opacity-45' : ''} ${team.id === user ? 'bg-white/10' : ''}`}>
      <span className="w-4 text-[11px] text-muted tabular-nums">{seed}</span>
      <TeamBadge team={team} size={18} />
      <Link to={`/team/${team.id}`} className={`link flex-1 truncate ${won ? 'font-extrabold' : ''}`}>{team.name}</Link>
      <Pips n={wins} color={team.primary === '#000000' ? team.secondary : team.primary} />
      <b className="w-3 text-right tabular-nums">{wins}</b>
    </div>
  )
}

function SeriesCard({ s, live, user }: { s?: Series; live: boolean; user: number }) {
  if (!s) return <div className="card flex h-[74px] items-center justify-center text-xs text-muted">TBD</div>
  return (
    <div className={`card overflow-hidden ${s.mine ? 'ring-1 ring-accent' : ''}`}>
      <Row team={s.high} seed={s.highSeed} wins={s.highWins} won={s.done && s.winner === s.high.id} lost={s.done && s.winner !== s.high.id} user={user} />
      <Row team={s.low} seed={s.lowSeed} wins={s.lowWins} won={s.done && s.winner === s.low.id} lost={s.done && s.winner !== s.low.id} user={user} />
      {live && s.games.length > 0 && (
        <div className="flex flex-wrap gap-1 border-t border-line/60 px-2 py-1">
          {s.games.map((g) => (
            <Link key={g.box} to={`/box/${g.box}`} className="rounded bg-panel2 px-1.5 text-[10px] text-muted tabular-nums hover:text-accent" title={`Game ${g.n}`}>
              G{g.n} {g.homeScore}-{g.awayScore}
            </Link>
          ))}
        </div>
      )}
    </div>
  )
}

export default function Playoffs() {
  const { state: gs } = useFlow()
  const q = useQuery({ queryKey: ['playoffs'], queryFn: api.playoffs })
  const controls = gs ? [gs.nextAction, ...(gs.actions ?? [])] : []
  const showControls = gs && (gs.phase === 'regular_done' || gs.phase === 'playoffs' || gs.phase === 'playoffs_done')
  return (
    <Page
      title={q.data ? `${q.data.year} playoffs` : 'Playoffs'}
      right={showControls ? <PlayoffControls ids={controls.map((c) => c.id)} /> : null}
    >
      <Q q={q} minH={500}>
        {(d) => {
          if (d.series.length === 0)
            return <div className="card p-8 text-center text-muted">The playoffs have not started. Finish the regular season first.{d.canStart && <div className="mt-3"><InlineContinue /></div>}</div>
          const find = (round: number, conf: number, slot: number) => d.series.find((s) => s.round === round && s.conf === conf && s.slot === slot)
          const user = gs?.user.team.id ?? -1
          const col = (title: string, round: number, conf: number, n: number) => (
            <div className="flex min-w-0 flex-col" key={title}>
              <div className="h2 mb-2 truncate text-center">{title}</div>
              <div className="flex flex-1 flex-col justify-around gap-3">
                {Array.from({ length: n }, (_, i) => <SeriesCard key={i} s={find(round, conf, i)} live={d.live} user={user} />)}
              </div>
            </div>
          )
          const [w, e] = [d.conferences[0], d.conferences[1]]
          return (
            <>
              {d.champion && (
                <div className="card mb-4 flex items-center gap-3 border-[#d9a521] p-4">
                  <Trophy className="text-[#f5c542]" /> <span className="h2">Champions</span> <TeamLink team={d.champion} />
                </div>
              )}
              <div className="grid grid-cols-[repeat(7,minmax(0,1fr))] gap-2.5 overflow-x-auto" style={{ minWidth: 980 }}>
                {col(`${w} R1`, 1, 0, 4)}{col(`${w} Semis`, 2, 0, 2)}{col(`${w} Finals`, 3, 0, 1)}
                {col('Finals', 4, -1, 1)}
                {col(`${e} Finals`, 3, 1, 1)}{col(`${e} Semis`, 2, 1, 2)}{col(`${e} R1`, 1, 1, 4)}
              </div>
            </>
          )
        }}
      </Q>
    </Page>
  )
}

function PlayoffControls({ ids }: { ids: string[] }) {
  const { state, run, busy } = useFlow()
  const all = [state!.nextAction, ...(state!.actions ?? [])].filter((a) => ids.includes(a.id))
  return (
    <div className="flex flex-wrap gap-2">
      {all.map((a, i) => (
        <button key={a.id} className={`btn ${i === 0 ? 'btn-accent' : ''}`} disabled={busy} onClick={() => run(a.id)}>{a.label}</button>
      ))}
    </div>
  )
}
