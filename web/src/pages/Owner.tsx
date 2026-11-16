import { useState } from 'react'
import { Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { ArrowDown, ArrowRight, ArrowUp } from 'lucide-react'
import { api, type Owner as OwnerData, type OwnerGoal, type OwnerTone } from '../api'
import { useFlow } from '../flow'
import { Card, Chip, GoalStatusChip, Page, Q, money, trustTone, type Tone } from '../ui'

export const ownerKey = (s?: { year: number; phase: string; day: number }) => ['owner', s?.year, s?.phase, s?.day]

const kindLabel: Record<OwnerGoal['kind'], string> = { results: 'Results', money: 'Money', building: 'Building' }
const dot: Record<OwnerTone, string> = { good: 'bg-good', warn: 'bg-warn', bad: 'bg-bad', info: 'bg-info' }
const barColor: Record<Tone, string> = { good: 'bg-good', warn: 'bg-warn', bad: 'bg-bad', info: 'bg-info', violet: 'bg-violet', neutral: 'bg-muted', strong: 'bg-muted' }
const signed = (n: number) => (n > 0 ? `+${n}` : String(n))

function Line({ label, value, bold, tone }: { label: string; value: string; bold?: boolean; tone?: string }) {
  return (
    <div className={`flex justify-between gap-3 py-0.5 ${bold ? 'border-t border-line/50 pt-1 font-bold' : ''}`}>
      <span className={bold ? '' : 'text-muted'}>{label}</span><span className={`tabular-nums ${tone ?? ''}`}>{value}</span>
    </div>
  )
}

function TrustCard({ d }: { d: OwnerData }) {
  const t = d.trust
  const tone = trustTone(t.label)
  const Arrow = t.trend === 'up' ? ArrowUp : t.trend === 'down' ? ArrowDown : ArrowRight
  return (
    <Card title="Trust">
      <div className="px-2 pb-2">
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
          <span className="text-3xl font-black tabular-nums">{t.score}</span>
          <span className="text-muted">/ 100</span>
          <Chip tone={tone}>{t.label}</Chip>
          <span className={`flex items-center gap-1 text-[13px] tabular-nums ${t.trend === 'up' ? 'text-good' : t.trend === 'down' ? 'text-bad' : 'text-muted'}`} title="Last change">
            <Arrow size={14} aria-hidden />{signed(t.lastChange)}
          </span>
        </div>
        <div className="mt-2 h-2.5 overflow-hidden rounded-full bg-panel2" role="meter" aria-valuemin={0} aria-valuemax={100} aria-valuenow={t.score} aria-label="Owner trust">
          <div className={`h-full rounded-full ${barColor[tone]}`} style={{ width: `${Math.max(0, Math.min(100, t.score))}%` }} />
        </div>
        <p className="mt-2 text-[13px] text-muted">{d.firing ? 'At zero you are let go at the end of the season.' : 'Firing is switched off in Settings.'}</p>
      </div>
    </Card>
  )
}

function GoalsCard({ goals }: { goals: OwnerGoal[] }) {
  return (
    <Card title="This season's goals">
      <ul>
        {goals.map((g) => (
          <li key={g.key} className="border-t border-line/50 px-2 py-2.5 first:border-t-0">
            <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
              <span className="text-[11px] font-bold uppercase tracking-wide text-muted">{kindLabel[g.kind]}</span>
              <span className="min-w-0 font-bold">{g.title}</span>
              <Chip tone={g.weight === 'High' ? 'strong' : 'neutral'} title="How much this goal matters to the owner">{g.weight}</Chip>
              <GoalStatusChip status={g.status} />
            </div>
            <div className="mt-0.5 text-[13px] text-muted">{g.detail}</div>
            <div className="mt-0.5 text-[13px]">{g.progress}</div>
          </li>
        ))}
      </ul>
    </Card>
  )
}

function MoneyCard({ m }: { m: OwnerData['money'] }) {
  return (
    <Card title="Money">
      <div className="px-2 pb-2">
        <div className="text-[13px]">
          <span className="text-muted">Budget </span><b className="tabular-nums">{money(m.budget)}</b>
          <span className="text-muted"> · Payroll </span><b className="tabular-nums">{money(m.payroll)}</b>
        </div>
        {m.overBudget > 0 && <div className="mt-0.5 text-[13px] font-semibold text-loss">Over budget by {money(m.overBudget)}</div>}
        <div className="mt-2 grid gap-x-8 gap-y-3 min-[600px]:grid-cols-2">
          <div>
            <div className="h2 mb-1">Revenue</div>
            <Line label="Market" value={money(m.revenue.market)} />
            <Line label="Home wins" value={money(m.revenue.wins)} />
            <Line label="Playoffs" value={money(m.revenue.playoffs)} />
            <Line label="Total" value={money(m.revenue.total)} bold />
          </div>
          <div>
            <div className="h2 mb-1">Costs</div>
            <Line label="Salaries" value={money(m.costs.salaries)} />
            <Line label="Dead money" value={money(m.costs.deadMoney)} />
            <Line label="Luxury tax" value={money(m.costs.tax)} />
            <Line label="Total" value={money(m.costs.total)} bold />
          </div>
        </div>
        <div className={`mt-3 flex items-baseline justify-between gap-3 border-t border-line/50 pt-2 font-bold ${m.profit < 0 ? 'text-loss' : 'text-win'}`}>
          <span>{m.profit < 0 ? 'Loss' : 'Profit'}</span><span className="tabular-nums">{money(m.profit)}</span>
        </div>
        <div className="mt-1 text-xs text-muted">{m.marketSize.label}{m.projected ? ' · Projected for this season' : ''}</div>
      </div>
    </Card>
  )
}

function MessagesCard({ d }: { d: OwnerData }) {
  const [shown, setShown] = useState(10)
  return (
    <Card title="From the owner">
      {d.messages.length === 0
        ? <div className="px-2 py-2 text-muted">No messages yet.</div>
        : (
          <ul>
            {d.messages.slice(0, shown).map((m, i) => (
              <li key={i} className="flex items-start gap-2 border-t border-line/50 px-2 py-1.5 text-[13px] first:border-t-0">
                <span aria-hidden className={`mt-1.5 h-2 w-2 shrink-0 rounded-full ${dot[m.tone] ?? 'bg-muted'}`} />
                <div className="min-w-0">
                  <div className="text-xs text-muted tabular-nums">{m.year} · Day {m.day}</div>
                  <div>{m.text}</div>
                </div>
              </li>
            ))}
          </ul>
        )}
      {d.messages.length > shown && <div className="p-2"><button className="btn" onClick={() => setShown((n) => n + 10)}>Show more</button></div>}
    </Card>
  )
}

function HistoryCard({ d }: { d: OwnerData }) {
  return (
    <Card title="History">
      {d.ledger.length === 0
        ? <div className="px-2 py-2 text-muted">No finished seasons with this owner yet.</div>
        : (
          <div className="overflow-x-auto">
            <table className="tbl min-w-[640px]">
              <thead>
                <tr><th>Season</th><th>Record</th><th>Playoffs</th><th className="r">Revenue</th><th className="r">Costs</th><th className="r">Profit</th><th className="r">Trust</th><th>Goals met</th></tr>
              </thead>
              <tbody>
                {d.ledger.map((r) => {
                  const met = r.goals.filter((g) => g.met).length
                  return (
                    <tr key={r.year}>
                      <td className="tabular-nums">{r.year}</td>
                      <td className="tabular-nums">{r.wins}-{r.losses}</td>
                      <td>{r.playoffs}</td>
                      <td className="r tabular-nums">{money(r.revenue)}</td>
                      <td className="r tabular-nums">{money(r.costs)}</td>
                      <td className={`r tabular-nums ${r.profit < 0 ? 'text-loss' : ''}`}>{money(r.profit)}</td>
                      <td className="r whitespace-nowrap tabular-nums">{r.trustStart} {'→'} {r.trustEnd}</td>
                      <td title={r.goals.map((g) => `${g.met ? 'Met' : 'Missed'}: ${g.title}`).join('\n')}>{met} of {r.goals.length}</td>
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

export default function Owner() {
  const { state } = useFlow()
  const q = useQuery({ queryKey: ownerKey(state), queryFn: api.owner })
  return (
    <Page title="Owner">
      <div className="space-y-4">
        <Q q={q} minH={240}>{(d) => !d.active
          ? <div className="card p-4 text-muted">Your owner will introduce themselves when the next season starts.</div>
          : (
            <>
              <Card>
                <div className="flex flex-wrap items-center gap-x-3 gap-y-1 px-2 pb-1">
                  <span className="text-lg font-extrabold">{d.owner.name}</span>
                  <Chip>{d.owner.personality.label}</Chip>
                  <Chip>{d.money.marketSize.label}</Chip>
                </div>
                <div className="px-2 pb-2 text-[13px] text-muted">{d.owner.personality.blurb}</div>
              </Card>
              <TrustCard d={d} />
              <GoalsCard goals={d.goals} />
              <MoneyCard m={d.money} />
              <MessagesCard d={d} />
              <HistoryCard d={d} />
              <p className="text-sm text-muted"><Link className="link text-accent" to="/settings">Settings</Link> has the switch for firing.</p>
            </>
          )}</Q>
      </div>
    </Page>
  )
}
