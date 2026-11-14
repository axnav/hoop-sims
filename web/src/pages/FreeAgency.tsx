import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Loader2, Play, SkipForward } from 'lucide-react'
import { api, type FAPlayer, type FASheet, type FASigning, type FAVia, type FreeAgencyData } from '../api'
import { useFlow } from '../flow'
import { Card, Delta, ErrorBox, InjuryTag, Page, PayrollBar, PlayerLink, Q, TierChip, money, ovrColor } from '../ui'

const viaText: Record<FAVia, string> = { room: 'cap room', exception: 'mid-level exception', minimum: 'minimum', own: 'your own player' }
const yrs = (n: number) => `${n} ${n === 1 ? 'yr' : 'yrs'}`

function OfferPanel({ p, d, onDone }: { p: FAPlayer; d: FreeAgencyData; onDone: (r: FreeAgencyData) => void }) {
  const [sal, setSal] = useState((p.myOffer?.salary ?? p.askSalary) / 1000)
  const [years, setYears] = useState(Math.max(p.minYears ?? 1, p.myOffer?.years ?? p.askYears))
  const m = useMutation({ mutationFn: api.faOffer, onSuccess: onDone })
  const send = (salary: number, y: number) => m.mutate({ playerId: p.id, salary: Math.round(salary), years: y })
  const minM = d.minSalary / 1000
  const bad = !Number.isFinite(sal) || sal * 1000 < d.minSalary
  return (
    <div className="flex flex-wrap items-end gap-x-3 gap-y-2 bg-panel2/60 px-3 py-2 text-[13px]">
      <label className="flex flex-col gap-0.5 text-[11px] text-muted">Salary ($M)
        <input type="number" step="0.1" min={minM} value={Number.isFinite(sal) ? sal : ''} onChange={(e) => setSal(e.target.valueAsNumber)}
          className="w-24 rounded border border-line bg-panel px-2 py-1 text-[13px] text-ink" />
      </label>
      <label className="flex flex-col gap-0.5 text-[11px] text-muted">Years
        <select value={years} onChange={(e) => setYears(Number(e.target.value))} className="rounded border border-line bg-panel px-2 py-1 text-[13px] text-ink">
          {Array.from({ length: Math.max(0, d.maxYears - (p.minYears ?? 1) + 1) }, (_, i) => i + (p.minYears ?? 1)).map((y) => <option key={y} value={y}>{y}</option>)}
        </select>
      </label>
      <button className="btn" onClick={() => { setSal(p.askSalary / 1000); setYears(Math.max(p.minYears ?? 1, p.askYears)) }}>Match the ask</button>
      <button className="btn" onClick={() => { setSal(minM); setYears(p.minYears ?? 1) }}>Offer the minimum</button>
      {p.restricted && !p.mine && p.rightsTeam && <div className="basis-full text-[12px] text-muted">An offer sheet must be at least 2 years. {p.rightsTeam.fullName} can match it.</div>}
      <button className="btn btn-accent" disabled={m.isPending || bad} onClick={() => send(sal * 1000, years)}>
        {m.isPending && <Loader2 size={14} className="animate-spin" />}Send offer
      </button>
      {m.isError && <div className="basis-full text-[12px] text-loss">{m.error instanceof Error ? m.error.message : String(m.error)}</div>}
    </div>
  )
}

function Players({ d, onData }: { d: FreeAgencyData; onData: (r: FreeAgencyData) => void }) {
  const [name, setName] = useState('')
  const [pos, setPos] = useState('')
  const [openId, setOpenId] = useState<number | null>(null)
  const [onlyMine, setOnlyMine] = useState(false)
  const full = d.rosterCount >= d.rosterSize
  const positions = useMemo(() => [...new Set(d.players.map((p) => p.pos))].sort(), [d.players])
  const rows = useMemo(
    () => d.players
      .filter((p) => (!pos || p.pos === pos) && (!onlyMine || p.mine) && p.name.toLowerCase().includes(name.trim().toLowerCase()))
      .sort((a, b) => b.ovr - a.ovr || b.pot - a.pot),
    [d.players, name, pos, onlyMine],
  )
  return (
    <Card
      title={`Available players (${rows.length})`}
      right={
        <div className="flex flex-wrap items-center gap-2">
          <label className="flex items-center gap-1 text-[12px] text-muted"><input type="checkbox" checked={onlyMine} onChange={(e) => setOnlyMine(e.target.checked)} />My restricted players</label>
          <input aria-label="Filter by name" placeholder="Name" value={name} onChange={(e) => setName(e.target.value)} className="w-28 rounded border border-line bg-panel px-2 py-1 text-[13px]" />
          <select aria-label="Filter by position" value={pos} onChange={(e) => setPos(e.target.value)} className="rounded border border-line bg-panel px-2 py-1 text-[13px]">
            <option value="">All</option>
            {positions.map((x) => <option key={x} value={x}>{x}</option>)}
          </select>
        </div>
      }
    >
      {full && <div className="px-2 pb-1 text-[13px] text-loss">Your roster is full; release a player first.</div>}
      <div className="max-h-[560px] overflow-auto">
        <table className="tbl">
          <thead>
            <tr><th>Player</th><th>Pos</th><th>Age</th><th className="r">Ovr/Pot</th><th>Ask</th><th>Interest</th><th>Your offer</th><th /></tr>
          </thead>
          <tbody>
            {rows.length === 0 && <tr><td colSpan={8} className="text-muted">No players match.</td></tr>}
            {rows.flatMap((p) => {
              const o = openId === p.id
              return [
                <tr key={p.id} className={p.mine ? 'bg-accent/10' : ''}>
                  <td><PlayerLink id={p.id} name={p.name} />{p.restricted && <span className="ml-1.5 rounded bg-accent/20 px-1.5 py-0.5 text-[10px] font-bold text-accent" title={p.mine ? 'Restricted: you can match any offer' : `Restricted: ${p.rightsTeam?.fullName ?? 'his team'} can match any offer`}>RFA</span>}<TierChip tier={p.tier} /><InjuryTag injury={p.injury} /></td>
                  <td>{p.pos}</td>
                  <td>{p.age}</td>
                  <td className="r"><b style={{ color: ovrColor(p.ovr) }}>{p.ovr}</b><Delta value={p.ovrChange} scope={p.changeScope} /><span className="text-muted">/{p.pot}</span><Delta value={p.potChange} scope={p.changeScope} /></td>
                  <td>{money(p.askSalary)} for {yrs(p.askYears)}</td>
                  <td>{p.interest > 0 ? `${p.interest} ${p.interest === 1 ? 'team' : 'teams'}` : '-'}</td>
                  <td>{p.myOffer ? <span className="font-semibold text-accent">{money(p.myOffer.salary)}, {yrs(p.myOffer.years)} <span className="font-normal text-muted">(day {p.myOffer.day})</span></span> : '-'}</td>
                  <td className="r"><button className="btn" disabled={full || !d.open} aria-expanded={o} onClick={() => setOpenId(o ? null : p.id)}>{p.myOffer ? 'Change' : 'Offer'}</button></td>
                </tr>,
                o && !full && (
                  <tr key={`${p.id}-o`}><td colSpan={8} className="!p-0 !whitespace-normal">
                    <OfferPanel p={p} d={d} onDone={(r) => { setOpenId(null); onData(r) }} />
                  </td></tr>
                ),
              ]
            })}
          </tbody>
        </table>
      </div>
    </Card>
  )
}

function Offers({ d, onData }: { d: FreeAgencyData; onData: (r: FreeAgencyData) => void }) {
  const m = useMutation({ mutationFn: api.faWithdraw, onSuccess: onData })
  return (
    <Card title={`Your offers (${d.offers.length})`}>
      {d.offers.length === 0 ? <div className="px-2 pb-2 text-[13px] text-muted">No offers yet.</div> : (
        <ul className="px-2 pb-1">
          {d.offers.map((o) => (
            <li key={o.id} className="flex flex-wrap items-center gap-x-3 gap-y-1 border-b border-line/50 py-1.5 text-[13px]">
              <span className="min-w-0 flex-1 basis-32"><PlayerLink id={o.playerId} name={o.name} /></span>
              <span className="tabular-nums">{money(o.salary)}, {yrs(o.years)}</span>
              <span className="text-xs text-muted">made on day {o.day}{o.via && `, signed with ${viaText[o.via]}`}</span>
              <button className="btn" disabled={m.isPending} onClick={() => m.mutate(o.id)}>Withdraw</button>
            </li>
          ))}
        </ul>
      )}
      {m.isError && <ErrorBox error={m.error} />}
    </Card>
  )
}

function SheetCard({ sheets, onData }: { sheets: FASheet[]; onData: (r: FreeAgencyData) => void }) {
  const qc = useQueryClient()
  const m = useMutation({
    mutationFn: (v: { id: number; match: boolean }) => api.faSheet(v.id, v.match),
    onSuccess: (r) => { onData(r.freeAgency); qc.setQueryData(['state'], r.state); qc.invalidateQueries({ queryKey: ['team'] }) },
  })
  return (
    <Card title="Offer sheet: decide before the next day" className="mb-4 !border-accent">
      <div className="px-4 pb-1 text-[13px] text-muted">He is a restricted free agent. Matching keeps him on your team on these exact terms, even over the salary cap.</div>
      <ul className="px-2 pb-2">
        {sheets.map((s) => (
          <li key={s.playerId} className="border-b border-line/50 py-2 text-[13px] last:border-0">
            <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
              <span className="min-w-0 flex-1 basis-40"><PlayerLink id={s.playerId} name={s.name} /> <span className="text-xs text-muted tabular-nums">{s.pos} &middot; {s.age}y &middot; {s.ovr}/{s.pot}</span></span>
              <span className="tabular-nums">{s.team.fullName} offered {money(s.salary)} for {yrs(s.years)}</span>
              <span className="flex gap-2">
                <button className="btn btn-accent" disabled={m.isPending || !s.canMatch} onClick={() => m.mutate({ id: s.playerId, match: true })}>Match</button>
                <button className="btn" disabled={m.isPending} onClick={() => m.mutate({ id: s.playerId, match: false })}>Let him go</button>
              </span>
            </div>
            {!s.canMatch && s.problem && <div className="mt-1 text-[12px] text-loss">{s.problem}</div>}
          </li>
        ))}
      </ul>
      {m.isError && <ErrorBox error={m.error} />}
    </Card>
  )
}

function Signings({ d, fresh }: { d: FreeAgencyData; fresh: FASigning[] }) {
  const freshIds = new Set(fresh.map((s) => s.playerId))
  const days = [...new Set(d.log.map((s) => s.day))].sort((a, b) => b - a)
  return (
    <Card title="Signings">
      {days.length === 0 ? <div className="px-2 pb-2 text-[13px] text-muted">No signings yet.</div> : (
        <div className="max-h-[420px] overflow-auto px-2 pb-1">
          {days.map((day) => (
            <div key={day} className="mb-2">
              <div className="text-[11px] font-bold uppercase tracking-wider text-muted">Day {day}</div>
              <ul>
                {d.log.filter((s) => s.day === day).map((s) => (
                  <li key={s.playerId} className={`rounded px-2 py-1 text-[13px] ${s.mine ? 'bg-accent/15 font-semibold' : ''} ${freshIds.has(s.playerId) ? 'ring-1 ring-accent' : ''}`}>
                    {s.matched && s.sheetFrom ? (
                      <>Day {s.day}: {s.mine ? 'You' : s.team.fullName} matched the offer sheet from {s.sheetFrom.fullName} and kept <PlayerLink id={s.playerId} name={s.name} /> ({s.pos}, {s.ovr}) for {money(s.salary)}, {yrs(s.years)}</>
                    ) : (
                      <>Day {s.day}: <PlayerLink id={s.playerId} name={s.name} /> ({s.pos}, {s.ovr}) signed with {s.mine ? 'you' : s.team.fullName} for {money(s.salary)}, {yrs(s.years)}</>
                    )}
                    {s.via === 'exception' && <span className="font-normal text-muted"> (exception)</span>}
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>
      )}
    </Card>
  )
}

export default function FreeAgency() {
  const { state, run, busy, lastSignings } = useFlow()
  const qc = useQueryClient()
  const q = useQuery({ queryKey: ['freeagents', state?.phase, state?.day, state?.fa?.day], queryFn: api.freeAgents })
  const onData = (r: FreeAgencyData) => { qc.setQueryData(['freeagents', state?.phase, state?.day, state?.fa?.day], r); qc.setQueryData(['freeagents'], r) }
  return (
    <Page title="Free agency">
      <Q q={q}>
        {(d) => !d.open ? (
          <>
            <div className="card mb-4 p-4 text-muted">Free agency is not open right now. It starts after the draft.</div>
            {d.log.length > 0 && <Signings d={d} fresh={[]} />}
          </>
        ) : (
          <>
            {(d.sheets?.length ?? 0) > 0 && <SheetCard sheets={d.sheets!} onData={onData} />}
            <div className="card mb-4 p-3">
              <div className="mb-2 flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
                <div className="flex flex-wrap items-baseline gap-x-4 gap-y-1">
                  <span className="text-lg font-bold tabular-nums">Day {d.day} of {d.days}</span>
                  <span className="text-muted tabular-nums">{d.rosterCount} of {d.rosterSize} players</span>
                </div>
                <div className="flex flex-wrap gap-2">
                  <button className="btn btn-accent" disabled={busy} onClick={() => run('fa_day')}>
                    {busy ? <Loader2 size={16} className="animate-spin" /> : <Play size={16} />}Next day
                  </button>
                  <button className="btn" disabled={busy} onClick={() => run('start_season')}><SkipForward size={16} />Skip to the new season</button>
                </div>
              </div>
              <PayrollBar p={d.payroll} compact />
              {d.exception && d.exception.kind !== 'none' && (
                <div className="mx-2 mt-1 rounded border border-line bg-panel2/60 px-3 py-1.5 text-[13px]">
                  {d.exception.used
                    ? `${d.exception.label} already used this offseason.`
                    : `${d.exception.label}: you may sign one player for up to ${money(d.exception.maxSalary)} a year, up to ${d.exception.maxYears} years, without cap room.`}
                </div>
              )}
              {d.exception && d.exception.kind === 'none' && d.exception.reason && <p className="px-2 pt-1 text-[13px] text-muted">{d.exception.reason}</p>}
              <p className="px-2 pt-1 text-[13px] text-muted">
                An offer must stand for one day before the player can accept it, and asking prices fall as days pass. Offers above {money(d.minSalary)} need cap room; an offer at {money(d.minSalary)} is always allowed while you have a roster spot.
              </p>
            </div>
            <div className="grid min-w-0 gap-4 min-[1100px]:grid-cols-[minmax(0,2fr)_minmax(0,1fr)]">
              <Players d={d} onData={onData} />
              <div className="flex min-w-0 flex-col gap-4">
                <Offers d={d} onData={onData} />
                <Signings d={d} fresh={lastSignings} />
              </div>
            </div>
          </>
        )}
      </Q>
    </Page>
  )
}
