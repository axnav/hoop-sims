import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { AnimatePresence, motion } from 'motion/react'
import { Pause, Play } from 'lucide-react'
import { api, type LotteryRow, type Pick } from '../api'
import { useScout } from '../scout'
import { useFlow } from '../flow'
import { InlineContinue } from '../Inline'
import { OffersCard } from '../OffersCard'
import { Card, ErrorBox, Page, PlayerLink, Q, RangeBar, TeamBadge, rangeText } from '../ui'

type SortKey = 'pot' | 'ovr' | 'age' | 'name'
const mid = (lo: number, hi: number) => (lo + hi) / 2

function RangeCell({ lo, hi, color }: { lo: number; hi: number; color?: string }) {
  return (
    <div className="ml-auto w-24">
      <div className="text-right font-bold tabular-nums">{rangeText(lo, hi)}</div>
      <RangeBar lo={lo} hi={hi} color={color} className="mt-0.5" />
    </div>
  )
}

function Dots({ level, max }: { level: number; max: number }) {
  return (
    <span title={`Scouting level ${level}/${max}`} aria-label={`Scouting level ${level} of ${max}`} className="inline-flex items-center gap-1">
      {Array.from({ length: max }, (_, i) => <span key={i} className={`h-2 w-2 rounded-full ${i < level ? 'bg-info' : 'bg-panel2 ring-1 ring-line'}`} />)}
      <span className="ml-1 text-[11px] text-muted tabular-nums">{level}/{max}</span>
    </span>
  )
}

function Lottery({ rows, me, collapsible }: { rows: LotteryRow[]; me: number; collapsible: boolean }) {
  const [open, setOpen] = useState(true)
  const show = !collapsible || open
  return (
    <Card className="mb-4" title="Draft lottery" right={collapsible ? <button className="btn py-0.5" onClick={() => setOpen((o) => !o)}>{open ? 'Hide' : 'Show'}</button> : undefined}>
      {show && (
        <div className="overflow-x-auto">
          <table className="tbl">
            <thead><tr><th className="r">Slot</th><th>Team</th><th className="r">Odds for #1</th><th>Move</th></tr></thead>
            <tbody>
              {rows.slice(0, 14).map((r) => {
                const mine = r.owner.id === me || r.team.id === me
                return (
                  <tr key={r.slot} className={`${mine ? 'bg-white/10 font-semibold' : ''} ${r.change > 0 ? 'text-win' : ''}`}>
                    <td className="r tabular-nums">{r.slot}</td>
                    <td>
                      <span className="inline-flex items-center gap-2"><TeamBadge team={r.team} size={16} />{r.team.fullName}</span>
                      {r.owner.id !== r.team.id && <span className="ml-2 text-[12px] text-muted">owned by {r.owner.fullName}</span>}
                    </td>
                    <td className="r tabular-nums">{r.odds}%</td>
                    <td className="whitespace-nowrap">{r.change > 0 ? `up ${r.change}` : r.change < 0 ? `down ${-r.change}` : '-'}</td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}
    </Card>
  )
}

function PickCell({ p, me, current }: { p: Pick; me: boolean; current: boolean }) {
  return (
    <div className={`flex items-center gap-2 rounded px-2 py-1 text-[13px] ${current ? 'bg-accent/20 ring-1 ring-accent' : me ? 'bg-white/10' : ''}`}>
      <span className="w-6 text-muted tabular-nums">{p.pick}</span>
      <TeamBadge team={p.team} size={16} />
      {p.via && <span className="shrink-0 truncate text-[11px] text-muted" title={`${p.team.fullName} (via ${p.via.fullName})`}>{p.team.abbr} (via {p.via.abbr})</span>}
      <AnimatePresence mode="wait" initial={false}>
        {p.made ? (
          <motion.span key="m" initial={{ opacity: 0, x: 12 }} animate={{ opacity: 1, x: 0 }} className="flex-1 truncate">
            <PlayerLink id={p.player!.id} name={p.player!.name} /> <span className="text-muted tabular-nums">{p.player!.ovr}/{p.player!.pot}</span>
          </motion.span>
        ) : (
          <motion.span key="u" className="flex-1 text-muted">{current ? 'On the clock' : '-'}</motion.span>
        )}
      </AnimatePresence>
    </div>
  )
}

export default function Draft() {
  const { state, run, busy } = useFlow()
  const qc = useQueryClient()
  const q = useQuery({ queryKey: ['draft'], queryFn: api.draft })
  // The draft waits for the user to start it, so there is time to scout first.
  const [running, setRunning] = useState(false)
  const [err, setErr] = useState('')
  const [sort, setSort] = useState<SortKey>('pot')
  const [posF, setPosF] = useState('All')
  const scout = useScout(setErr)
  const d = q.data && q.data.active ? q.data : null
  const made = d?.made

  useEffect(() => {
    if (!d || !d.started || d.done || d.userOnClock || !running) return
    const t = setTimeout(async () => {
      try {
        const r = await api.draftStep(1)
        qc.setQueryData(['state'], r.state)
        await qc.invalidateQueries({ queryKey: ['draft'] })
      } catch (e) {
        setErr(e instanceof Error ? e.message : String(e))
        setRunning(false)
      }
    }, 450)
    return () => clearTimeout(t)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [made, d?.userOnClock, d?.started, running])

  const pick = async (id: number) => {
    setErr('')
    try {
      const r = await api.draftPick(id)
      qc.setQueryData(['state'], r.state)
      await qc.invalidateQueries({ queryKey: ['draft'] })
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e))
    }
  }

  return (
    <Page title="Draft" right={d && !d.done ? (
      <div className="flex flex-wrap items-center gap-2">
        <Link to="/trade" className="btn">Trade</Link>
        {d.started && !d.userOnClock && <button className="btn" onClick={() => setRunning((r) => !r)}>{running ? <Pause size={15} /> : <Play size={15} />}{running ? 'Pause' : d.made === 0 ? 'Start the draft' : 'Resume'}</button>}
        {d.started && <button className="btn" disabled={busy} onClick={() => run('draft_auto')}>Auto-draft the rest</button>}
      </div>
    ) : null}>
      {err && <ErrorBox error={err} />}
      <Q q={q} minH={500}>
        {(data) => {
          if (!data.active) return <div className="card p-8 text-center text-muted">There is no draft right now.</div>
          const cur = data.picks.find((p) => !p.made)?.pick
          const me = data.userTeam
          const sc = data.scouting
          const positions = [...new Set(data.prospects.map((p) => p.pos))].sort()
          const shown = data.prospects.filter((p) => posF === 'All' || p.pos === posF).sort((a, b) => {
            if (sort === 'name') return a.name.localeCompare(b.name)
            if (sort === 'age') return a.age - b.age
            if (sort === 'ovr') return mid(b.ovrLo, b.ovrHi) - mid(a.ovrLo, a.ovrHi)
            return mid(b.potLo, b.potHi) - mid(a.potLo, a.potHi)
          })
          return (
            <>
              {!data.started && <div className="card mb-4 p-4">Look at the <Link to="/offseason" className="link text-accent">offseason report</Link> first, then go to the draft.</div>}
              {data.started && data.userOnClock && (
                <motion.div initial={{ opacity: 0, y: -8 }} animate={{ opacity: 1, y: 0 }} className="card mb-4 border-accent p-4 text-lg font-bold">
                  You are on the clock: pick #{cur}. Choose a prospect below.
                </motion.div>
              )}
              {!data.done && <OffersCard />}
              {data.done && (
                <div className="card mb-4 flex items-center justify-between p-4">
                  <span className="text-lg font-bold">Draft complete.</span>
                  {state?.phase === 'draft_done' && <InlineContinue big />}
                </div>
              )}
              {data.lottery.length > 0 && <Lottery rows={data.lottery} me={me} collapsible={data.started} />}
              <div className="grid gap-4 xl:grid-cols-[360px_1fr]">
                <Card title={`Picks (${data.made}/${data.total})`}>
                  <div className="max-h-[640px] overflow-y-auto pr-1">
                    {data.picks.map((p) => <PickCell key={p.pick} p={p} me={p.team.id === me} current={p.pick === cur && !data.done} />)}
                  </div>
                </Card>
                <Card title={`Available prospects (${shown.length}${shown.length !== data.prospects.length ? ` of ${data.prospects.length}` : ''})`}>
                  <div className="px-2 pb-2 text-[13px]">
                    <div className="font-semibold">Scouting points: {sc.points} of {sc.perDraft} left</div>
                    <div className="text-muted">Ranges always contain the true rating. Each point narrows one prospect's ranges.</div>
                    <div className="mt-2 flex flex-wrap items-center gap-3">
                      <label className="flex items-center gap-1.5">Sort by
                        <select className="rounded border border-line bg-panel2 px-2 py-1" value={sort} onChange={(e) => setSort(e.target.value as SortKey)}>
                          <option value="pot">Potential</option><option value="ovr">Overall</option><option value="age">Age</option><option value="name">Name</option>
                        </select>
                      </label>
                      <label className="flex items-center gap-1.5">Position
                        <select className="rounded border border-line bg-panel2 px-2 py-1" value={posF} onChange={(e) => setPosF(e.target.value)}>
                          {['All', ...positions].map((p) => <option key={p}>{p}</option>)}
                        </select>
                      </label>
                    </div>
                  </div>
                  <div className="max-h-[640px] overflow-auto">
                    <table className="tbl">
                      <thead><tr><th>Prospect</th><th>Pos</th><th className="r">Age</th><th className="r">OVR</th><th className="r">POT</th><th>Scouting</th><th></th></tr></thead>
                      <tbody>
                        {shown.map((p) => {
                          const full = p.scoutLevel >= sc.maxLevel
                          const none = sc.points <= 0
                          return (
                            <tr key={p.id}>
                              <td><PlayerLink id={p.id} name={p.name} /></td>
                              <td>{p.pos}</td>
                              <td className="r">{p.age}</td>
                              <td className="r"><RangeCell lo={p.ovrLo} hi={p.ovrHi} /></td>
                              <td className="r"><RangeCell lo={p.potLo} hi={p.potHi} color="var(--color-info)" /></td>
                              <td><Dots level={p.scoutLevel} max={sc.maxLevel} /></td>
                              <td className="whitespace-nowrap">
                                <button className="btn py-0.5" disabled={full || none || scout.isPending}
                                  title={full ? 'Fully scouted' : none ? 'No scouting points left' : 'Spend one scouting point'}
                                  onClick={() => scout.mutate(p.id)}>Scout</button>
                                {data.userOnClock && <button className="btn btn-accent ml-1 py-0.5" onClick={() => pick(p.id)}>Pick</button>}
                              </td>
                            </tr>
                          )
                        })}
                      </tbody>
                    </table>
                  </div>
                </Card>
              </div>
            </>
          )
        }}
      </Q>
    </Page>
  )
}
