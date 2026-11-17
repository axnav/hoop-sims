import { Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { motion } from 'motion/react'
import { ArrowLeftRight, Trophy } from 'lucide-react'
import { api, type Award } from '../api'
import { useFlow } from '../flow'
import { InlineContinue, useOpenPlayer } from '../Inline'
import { OffersCard } from '../OffersCard'
import { InjuriesCard } from '../InjuriesCard'
import { ExpiringCard } from '../ExpiringCard'
import { TradeRequestCard } from '../TradeRequestCard'
import { NewsStory } from '../NewsCard'
import { ownerKey } from './Owner'
import { Card, Chip, GoalStatusChip, Page, PlayerLink, Q, TeamBadge, TeamLink, WL, f1, trustTone } from '../ui'

function Hero({ children, tone = 'accent' }: { children: React.ReactNode; tone?: 'accent' | 'gold' }) {
  return (
    <motion.div
      initial={{ opacity: 0, y: 12 }} animate={{ opacity: 1, y: 0 }}
      className="card mb-4 flex flex-wrap items-center justify-between gap-4 p-6"
      style={{ borderColor: tone === 'gold' ? '#d9a521' : 'var(--team)', background: 'linear-gradient(120deg, color-mix(in srgb, var(--team) 22%, var(--color-panel)), var(--color-panel))' }}
    >
      {children}
    </motion.div>
  )
}

function AwardLine({ label, a }: { label: string; a: Award }) {
  if (!a) return null
  return (
    <div className="text-muted">{label}: <PlayerLink id={a.playerId} name={a.name} /> <span className="tabular-nums">({f1(a.ppg)} / {f1(a.rpg)} / {f1(a.apg)})</span></div>
  )
}

function AssistantCard() {
  const { state: s } = useFlow()
  const q = useQuery({ queryKey: ['gm', s?.year, s?.phase, s?.day], queryFn: api.gm, enabled: (s?.gm?.count ?? 0) > 0 })
  const g = q.data
  if (!g || g.suggestions.length === 0) return null
  const label = (k: string) => g.areas.find((a) => a.key === k)?.label ?? k
  return (
    <Card title="Assistant" className="mb-4">
      <ul className="space-y-1 px-2 py-1">
        {g.suggestions.slice(0, 3).map((x) => (
          <li key={x.id} className="flex flex-wrap items-baseline gap-x-2"><Chip>{label(x.area)}</Chip><span className="min-w-0">{x.title}</span></li>
        ))}
      </ul>
      <div className="px-2 pb-1 pt-1"><Link className="link text-accent" to="/assistant">Open the assistant ({g.suggestions.length})</Link></div>
    </Card>
  )
}

function OwnerCard() {
  const { state: s } = useFlow()
  const q = useQuery({ queryKey: ownerKey(s), queryFn: api.owner, enabled: !!s?.owner && s.owner.label !== '' })
  const d = q.data
  if (!d || !d.active) return null
  const arrow = d.trust.trend === 'up' ? '\u2191' : d.trust.trend === 'down' ? '\u2193' : '\u2192'
  return (
    <Card title="Owner" className="mb-4">
      <div className="space-y-1 px-2 py-1">
        <div className="flex flex-wrap items-center gap-x-2">
          <span className="text-xl font-black tabular-nums">{d.trust.score}</span><span className="text-muted">trust</span>
          <Chip tone={trustTone(d.trust.label)}>{d.trust.label}</Chip>
          <span className="text-[13px] text-muted tabular-nums">{arrow} {d.trust.lastChange > 0 ? '+' : ''}{d.trust.lastChange}</span>
        </div>
        <ul className="space-y-1">
          {d.goals.map((g) => (
            <li key={g.key} className="flex flex-wrap items-baseline gap-x-2"><GoalStatusChip status={g.status} /><span className="min-w-0">{g.title}</span></li>
          ))}
        </ul>
        {s?.owner?.alert && <div className="rounded border border-warn/50 bg-warn/10 px-2 py-1 text-[13px] text-warn" role="status">{s.owner.alert}</div>}
      </div>
      <div className="px-2 pb-1 pt-1"><Link className="link text-accent" to="/owner">Open the Owner page</Link></div>
    </Card>
  )
}

function NewsCard() {
  const q = useQuery({ queryKey: ['news', 'all', 5], queryFn: () => api.news('all', 5) })
  const items = q.data?.items ?? []
  if (items.length === 0) return null
  return (
    <Card title="League news" className="mt-4">
      <div>{items.map((n) => <NewsStory key={n.id} n={n} compact />)}</div>
      <div className="px-2 pb-1 pt-1"><Link className="link text-accent" to="/news">All news</Link></div>
    </Card>
  )
}

function Interlude() {
  const { state: s } = useFlow()
  const draft = useQuery({ queryKey: ['draft'], queryFn: api.draft, enabled: s?.phase === 'draft_done' })
  if (!s) return null
  if (s.phase === 'regular_done' && s.summary) {
    const m = s.summary
    return (
      <Hero>
        <div>
          <div className="h2">Regular season complete</div>
          <div className="mt-1 text-4xl font-black tabular-nums">{m.w}-{m.l}</div>
          <div className="mt-1 text-lg">{m.rank === 1 ? 'Top seed' : `#${m.rank} seed`} in the {s.user.confName}. {m.madePlayoffs ? 'You are in the playoffs.' : 'You missed the playoffs.'}</div>
          <div className="mt-2 space-y-0.5"><AwardLine label="MVP" a={m.mvp} /><AwardLine label="Scoring leader" a={m.scoringLeader} /></div>
        </div>
        <InlineContinue big />
      </Hero>
    )
  }
  if (s.phase === 'playoffs_done' && s.playoffs?.champion) {
    const p = s.playoffs
    const c = p.champion!
    return (
      <Hero tone="gold">
        <div className="flex items-center gap-5">
          <motion.div initial={{ scale: 0, rotate: -40 }} animate={{ scale: 1, rotate: 0 }} transition={{ type: 'spring', stiffness: 200, damping: 12 }}>
            <Trophy size={72} className="text-[#f5c542]" />
          </motion.div>
          <div>
            <div className="h2">{s.year} champions</div>
            <div className="mt-1 flex items-center gap-3 text-3xl font-black"><TeamBadge team={c} size={34} /><TeamLink team={c} badge={false} /></div>
            {p.finals && (
              <div className="mt-1 text-muted tabular-nums">
                Finals: {c.name} won {Math.max(p.finals.highWins, p.finals.lowWins)}-{Math.min(p.finals.highWins, p.finals.lowWins)}
              </div>
            )}
            <div className="mt-1"><AwardLine label="Finals MVP" a={p.finalsMvp ?? null} /></div>
            <div className="mt-1 font-semibold">Your season: {p.userResult}</div>
          </div>
        </div>
        <InlineContinue big />
      </Hero>
    )
  }
  if (s.phase === 'draft_done') {
    const mine = draft.data?.active ? draft.data.picks.filter((k) => k.made && k.team.id === s.user.team.id) : []
    return (
      <Hero>
        <div>
          <div className="h2">Draft complete</div>
          <div className="mt-1 text-3xl font-black">Ready for the {s.year + 1} season</div>
          <div className="mt-2 text-muted">
            {mine.length > 0 ? <>Your picks: {mine.map((k, i) => <span key={k.pick}>{i > 0 && ', '}#{k.pick} <PlayerLink id={k.player!.id} name={k.player!.name} /></span>)}</> : 'You had no picks.'}
          </div>
        </div>
        <InlineContinue big />
      </Hero>
    )
  }
  return null
}

// Computer-to-computer trades are made when the offseason finishes, so show them at the start of the new season.
function OffseasonTrades() {
  const { state: s } = useFlow()
  const show = !!s && s.phase === 'regular' && s.day <= 5
  const q = useQuery({ queryKey: ['transactions'], queryFn: api.transactions, enabled: show })
  if (!s || !show || !q.data) return null
  const rows = q.data.transactions.filter((t) => t.year === s.year && t.day === 0 && t.teamA.id !== s.user.team.id && t.teamB.id !== s.user.team.id)
  if (rows.length === 0) return null
  return (
    <motion.div initial={{ opacity: 0, y: 12 }} animate={{ opacity: 1, y: 0 }} className="card mb-4 p-4">
      <div className="mb-2 flex items-center justify-between gap-2">
        <div className="h2 flex items-center gap-1.5"><ArrowLeftRight size={13} />Offseason trades around the league</div>
        <Link className="link text-accent" to="/trade#transactions">All transactions</Link>
      </div>
      <ul className="space-y-1.5">
        {rows.map((t, i) => {
          const got = (id: number) => t.moved.filter((m) => m.to.id === id)
          return (
            <li key={i} className="flex flex-wrap items-center gap-x-3 gap-y-1">
              {[t.teamA, t.teamB].map((tm, k) => (
                <span key={tm.id} className="flex items-center gap-1.5">
                  {k === 1 && <span className="text-muted">and</span>}
                  <TeamBadge team={tm} size={16} /><span className="text-muted">get</span>
                  {got(tm.id).map((m, j) => <span key={m.playerId}>{j > 0 && ', '}<PlayerLink id={m.playerId} name={m.name} /></span>)}
                </span>
              ))}
            </li>
          )
        })}
      </ul>
    </motion.div>
  )
}

export default function Dashboard() {
  const { state: s } = useFlow()
  const stand = useQuery({ queryKey: ['standings'], queryFn: api.standings })
  const sched = useQuery({ queryKey: ['schedule', 'user'], queryFn: () => api.schedule() })
  const team = useQuery({ queryKey: ['team', s?.user.team.id], queryFn: () => api.team(s!.user.team.id), enabled: !!s })
  const open = useOpenPlayer()
  if (!s) return null
  const roster = (team.data?.roster ?? []).filter((r) => r.gp > 0)
  const top = (k: 'ppg' | 'rpg' | 'apg') => [...roster].sort((a, b) => b[k] - a[k]).slice(0, 3)
  const recent = (sched.data?.games ?? []).filter((g) => g.played).slice(-12)
  return (
    <Page title={`${s.user.team.fullName}`}>
      {(s.pendingSheets ?? 0) > 0 && (
        <Link to="/free-agency" className="card mb-4 block border-accent p-4 font-semibold text-accent">An offer sheet is waiting for your answer.</Link>
      )}
      <Interlude />
      <OffseasonTrades />
      <OwnerCard />
      <AssistantCard />
      <OffersCard />
      <ExpiringCard />
      <InjuriesCard />
      <TradeRequestCard />
      <div className="grid gap-4 lg:grid-cols-3">
        <Card title="Next game" className="min-h-[150px]">
          {s.next ? (
            <div className="p-3">
              <div className="text-muted">{s.next.label}{s.next.kind === 'playoff' && s.next.seriesMe !== undefined && ` · series ${s.next.seriesMe}-${s.next.seriesThem}`}</div>
              <div className="mt-2 flex items-center gap-3 text-xl font-bold">
                <span className="text-muted">{s.next.home ? 'vs' : '@'}</span><TeamBadge team={s.next.opp} size={32} /><TeamLink team={s.next.opp} badge={false} />
              </div>
              <div className="mt-1 text-muted tabular-nums">{s.next.kind === 'regular' ? `${s.next.oppW}-${s.next.oppL}` : ''}</div>
            </div>
          ) : <div className="p-3 text-muted">{s.phase === 'regular_done' ? 'Season complete.' : s.phase === 'playoffs' && !s.playoffs?.userAlive ? 'Your team is not playing.' : 'No game scheduled.'}</div>}
        </Card>
        <Card title="Last game" className="min-h-[150px]">
          {s.last ? (
            <div className="p-3">
              <div className="text-muted">{s.last.label}</div>
              <div className="mt-2 flex items-center gap-3 text-xl font-bold">
                <WL win={s.last.win} /><span className="tabular-nums">{s.last.me}-{s.last.them}</span>
                <span className="text-muted">{s.last.home ? 'vs' : '@'}</span><TeamLink team={s.last.opp} short />
              </div>
              <Link className="link mt-2 inline-block text-accent" to={`/box/${s.last.boxId}`}>Box score</Link>
            </div>
          ) : <div className="p-3 text-muted">No games played yet.</div>}
        </Card>
        <Card title="Recent results" className="min-h-[150px]">
          <Q q={sched} minH={100}>{() => (
            <div className="flex flex-wrap gap-1.5 p-3">
              {recent.length === 0 && <span className="text-muted">Nothing yet.</span>}
              {recent.map((g) => (
                <span key={g.day} title={`Day ${g.day}: ${g.me}-${g.them} ${g.home ? 'vs' : '@'} ${g.opp.name}`}
                  className={`rounded px-2 py-1 text-xs font-bold tabular-nums ${g.win ? 'bg-win/15 text-win' : 'bg-loss/15 text-loss'}`}>
                  {g.win ? 'W' : 'L'} {g.me}-{g.them}
                </span>
              ))}
            </div>
          )}</Q>
        </Card>
        <Card title="Team leaders" className="lg:col-span-1">
          <Q q={team} minH={160}>{() => (
            <div className="grid grid-cols-3 gap-2 p-2">
              {([['ppg', 'PTS'], ['rpg', 'REB'], ['apg', 'AST']] as const).map(([k, label]) => (
                <div key={k}>
                  <div className="h2 mb-1">{label}</div>
                  {top(k).map((r) => (
                    <button key={r.id} onClick={() => open(r.id)} className="flex w-full justify-between py-0.5 text-left hover:text-accent">
                      <span className="truncate pr-1">{r.name.split(' ').slice(-1)[0]}</span><b className="tabular-nums">{f1(r[k])}</b>
                    </button>
                  ))}
                </div>
              ))}
            </div>
          )}</Q>
        </Card>
        {[0, 1].map((c) => (
          <Card key={c} title={`${s.conferences[c]} top 5`}>
            <Q q={stand} minH={160}>{(d) => (
              <table className="tbl"><tbody>
                {d.conferences[c].rows.slice(0, 5).map((r) => (
                  <tr key={r.team.id} className={r.team.id === s.user.team.id ? 'me' : ''}>
                    <td className="w-6 text-muted">{r.rank}</td><td><TeamLink team={r.team} short /></td><td className="r">{r.w}-{r.l}</td>
                  </tr>
                ))}
              </tbody></table>
            )}</Q>
          </Card>
        ))}
      </div>
      <NewsCard />
    </Page>
  )
}
