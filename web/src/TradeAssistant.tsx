import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Search, Sparkles } from 'lucide-react'
import { api, type FindBody, type FindOffer, type TakeAsset, type TakePackage } from './api'
import { AdviceChip, ErrorBox, PlayerLink, Spinner, StatusChip, TeamBadge, money, ovrColor } from './ui'

// "What they want": direction, what the team values and what it lacks.
export function NeedsPanel({ teamId, bump }: { teamId: number; bump?: unknown }) {
  const q = useQuery({ queryKey: ['trade-needs', teamId, bump], queryFn: () => api.tradeNeeds(teamId) })
  if (q.isError) return <ErrorBox error={q.error} />
  if (!q.data) return <div className="card mb-3 min-h-[88px] p-3"><Spinner /></div>
  const n = q.data
  return (
    <section className="card mb-3 p-3" aria-label="What they want">
      <div className="mb-2 flex flex-wrap items-center gap-2">
        <h2 className="h2">What they want</h2>
        <TeamBadge team={n.team} size={16} />
        <StatusChip status={n.direction} />
      </div>
      <p className="mb-2 text-[13px]">{n.summary}</p>
      <div className="grid gap-3 sm:grid-cols-2">
        <div>
          <div className="mb-1 text-[10px] font-bold uppercase tracking-wide text-muted">They value</div>
          <ul className="flex flex-wrap gap-1.5">{n.values.map((v) => <li key={v} className="rounded-full bg-win/15 px-2 py-0.5 text-[12px] text-win">{v}</li>)}</ul>
        </div>
        <div>
          <div className="mb-1 text-[10px] font-bold uppercase tracking-wide text-muted">They lack</div>
          {n.lacks.length === 0
            ? <div className="text-[13px] text-muted">No glaring weak spots.</div>
            : <ul className="flex flex-wrap gap-1.5">{n.lacks.map((v) => <li key={v} className="rounded-full bg-loss/15 px-2 py-0.5 text-[12px] text-loss">{v}</li>)}</ul>}
        </div>
      </div>
    </section>
  )
}

const assetText = (a: TakeAsset) => (a.kind === 'player' ? a.name : a.label)

function AssetLine({ a }: { a: TakeAsset }) {
  return a.kind === 'player' ? (
    <li className="flex items-center gap-2 py-0.5 text-[13px]">
      <span className="w-7 text-xs text-muted">{a.pos}</span>
      <span className="min-w-0 flex-1 truncate font-medium"><PlayerLink id={a.id} name={a.name} /></span>
      <span className="text-xs text-muted tabular-nums">{a.age}y</span>
      <b className="w-7 text-right tabular-nums" style={{ color: ovrColor(a.ovr) }}>{a.ovr}</b>
      <span className="w-14 text-right text-xs tabular-nums">{money(a.salary)}</span>
    </li>
  ) : (
    <li className="flex items-center gap-2 py-0.5 text-[13px]">
      <span className="w-7 text-xs text-muted">Pick</span>
      <span className="min-w-0 flex-1 truncate font-medium">{a.label}</span>
    </li>
  )
}

// What would it take? The packages the team would accept for one player.
export function TakePanel({ playerId, bump, onLoad, onClose, disabled }: {
  playerId: number; bump?: unknown; disabled?: boolean; onClose: () => void
  onLoad: (teamId: number, pk: TakePackage, labels: string[]) => void
}) {
  const q = useQuery({ queryKey: ['what-would-it-take', playerId, bump], queryFn: () => api.whatWouldItTake(playerId), staleTime: 0 })
  return (
    <section className="card mb-3 border-accent/50 p-3" aria-label="What would it take?" id="take-panel">
      <div className="mb-2 flex items-center gap-2">
        <h2 className="h2 flex items-center gap-1.5"><Sparkles size={14} className="text-accent" />What would it take{q.data ? ` to get ${q.data.player.name}?` : '?'}</h2>
        <button className="btn ml-auto px-2 py-1 text-[12px]" onClick={onClose}>Close</button>
      </div>
      {q.isError && <ErrorBox error={q.error} />}
      {q.isLoading && <Spinner />}
      {q.data && !q.data.possible && <div className="text-[13px] font-semibold text-loss" role="status">{q.data.reason}</div>}
      {q.data && q.data.possible && (
        <div className="grid gap-3 md:grid-cols-2">
          {q.data.packages.map((pk, i) => (
            <div key={i} className="rounded-lg border border-line bg-panel2/40 p-2.5">
              <div className="mb-1 flex flex-wrap items-center gap-2">
                <span className="font-bold">{pk.label}</span>
                {pk.cheapest && <span className="rounded bg-accent/25 px-1.5 py-px text-[10px] font-semibold uppercase text-accent ring-1 ring-accent/60">Cheapest</span>}
              </div>
              <div className="text-[10px] font-bold uppercase tracking-wide text-muted">You send</div>
              <ul>{pk.give.map((a, j) => <AssetLine key={j} a={a} />)}</ul>
              <div className="mt-1 text-[10px] font-bold uppercase tracking-wide text-muted">You get</div>
              <ul>{pk.receive.map((a, j) => <AssetLine key={j} a={a} />)}</ul>
              {pk.note && <div className="mt-1.5 text-[12px] text-muted">{pk.note}</div>}
              <button className="btn btn-accent mt-2" disabled={disabled} onClick={() => onLoad(q.data!.team.id, pk, [...pk.give, ...pk.receive].map(assetText))}>Load this offer</button>
            </div>
          ))}
        </div>
      )}
    </section>
  )
}

// Trade finder: what each computer team would give for the players and picks you offer.
export function FinderPanel({ give, bump, onLoad, onClose, disabled }: {
  give: Omit<FindBody, 'want'>; bump?: unknown; disabled?: boolean; onClose: () => void
  onLoad: (teamId: number, o: Pick<FindOffer, 'deal'>, labels: string[]) => void
}) {
  const [want, setWant] = useState('any')
  const empty = give.players.length + give.picks.length === 0
  const body = { ...give, want }
  const q = useQuery({ queryKey: ['trade-find', JSON.stringify(body), bump], queryFn: () => api.tradeFind(body), enabled: !empty, staleTime: 0 })
  const d = q.data
  return (
    <section className="card mb-3 border-accent/50 p-3" aria-label="Find offers" id="finder-panel">
      <div className="mb-2 flex items-center gap-2">
        <h2 className="h2 flex items-center gap-1.5"><Search size={14} className="text-accent" />Find offers</h2>
        <button className="btn ml-auto px-2 py-1 text-[12px]" onClick={onClose}>Close</button>
      </div>
      {empty && <div className="text-[13px] text-muted">Tick players or picks on your side (up to 3 players and 4 picks) and the best offer from each team shows here.</div>}
      {!empty && q.isError && <ErrorBox error={q.error} />}
      {!empty && q.isLoading && <div style={{ minHeight: 120 }}><Spinner /></div>}
      {!empty && d && (
        <>
          <div className="mb-2">
            <div className="text-[10px] font-bold uppercase tracking-wide text-muted">On the table (worth {d.giveValue} to you)</div>
            <ul className="grid gap-x-4 sm:grid-cols-2">{d.give.map((a, i) => <AssetLine key={i} a={a} />)}</ul>
          </div>
          <div className="mb-3 flex flex-wrap items-center gap-1.5" role="group" aria-label="What you want back">
            <span className="mr-1 text-xs text-muted">You want back</span>
            {d.wants.map((w) => (
              <button key={w.key} aria-pressed={want === w.key} onClick={() => setWant(w.key)}
                className={`rounded-md border px-2 py-0.5 text-[12px] font-semibold ${want === w.key ? 'border-accent bg-accent/25 text-accent' : 'border-line text-muted hover:text-ink'}`}>{w.label}</button>
            ))}
          </div>
          {!d.possible && <div className="text-[13px] font-semibold text-loss" role="status">{d.reason}</div>}
          {d.possible && (
            <div className="grid gap-3 md:grid-cols-2">
              {d.offers.map((o) => (
                <div key={o.team.id} className="rounded-lg border border-line bg-panel2/40 p-2.5">
                  <div className="mb-1 flex flex-wrap items-center gap-2">
                    <TeamBadge team={o.team} size={18} />
                    <span className="font-bold">{o.team.fullName}</span>
                    <StatusChip status={o.direction} />
                  </div>
                  <div className="text-[10px] font-bold uppercase tracking-wide text-muted">You get (worth {o.value} to you)</div>
                  <ul>{o.receive.map((a, j) => <AssetLine key={j} a={a} />)}</ul>
                  <div className="mt-1 text-[12px] text-muted">You take in {money(o.salaryIn)} and send out {money(o.salaryOut)}.</div>
                  {o.note && <div className="text-[12px] text-muted">{o.note}</div>}
                  <button className="btn btn-accent mt-2" disabled={disabled} onClick={() => onLoad(o.team.id, o, [...d.give, ...o.receive].map(assetText))}>Load this offer</button>
                </div>
              ))}
            </div>
          )}
          {d.passed.length > 0 && (
            <details className="mt-3 text-[13px]">
              <summary className="cursor-pointer text-muted">{d.passed.length} {d.passed.length === 1 ? 'team' : 'teams'} passed</summary>
              <ul className="mt-1 space-y-1">
                {d.passed.map((p) => (
                  <li key={p.team.id} className="flex flex-wrap items-center gap-2"><TeamBadge team={p.team} size={16} /><span className="font-medium">{p.team.abbr}</span><span className="text-muted">{p.reason}</span></li>
                ))}
              </ul>
            </details>
          )}
        </>
      )}
    </section>
  )
}

// Keep / Shop / Sell for your roster.
export function AdvisorList({ onPick }: { onPick: (id: number) => void }) {
  const q = useQuery({ queryKey: ['advisor'], queryFn: api.tradeAdvisor, staleTime: 0 })
  if (q.isError) return <ErrorBox error={q.error} />
  if (!q.data) return <div style={{ minHeight: 200 }}><Spinner /></div>
  return (
    <div>
      <div className="mb-2 flex flex-wrap items-center gap-2 px-2 text-[13px]"><StatusChip status={q.data.direction} /><span>{q.data.summary}</span></div>
      <ul>
        {q.data.players.map((p) => (
          <li key={p.id} className="cursor-pointer border-b border-line/50 px-2 py-1.5 hover:bg-panel2" onClick={() => onPick(p.id)} title="Add to the deal">
            <div className="flex items-center gap-2">
              <span className="w-7 text-xs text-muted">{p.pos}</span>
              <span className="min-w-0 flex-1 truncate font-medium"><PlayerLink id={p.id} name={p.name} /></span>
              <span className="text-xs text-muted tabular-nums">{p.age}y</span>
              <b className="w-7 text-right tabular-nums" style={{ color: ovrColor(p.ovr) }}>{p.ovr}</b>
              <AdviceChip advice={p.advice} />
            </div>
            <ul className="mt-0.5 list-disc pl-12 text-[12px] text-muted">{p.reasons.map((r, i) => <li key={i}>{r}</li>)}</ul>
          </li>
        ))}
      </ul>
    </div>
  )
}
