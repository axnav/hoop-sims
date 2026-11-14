import type { Transaction } from './api'
import { PlayerLink, TeamBadge } from './ui'

export const txWhen = (t: Transaction) => (t.day === 0 ? `${t.year} offseason` : `${t.year}, day ${t.day}`)

export function TxItem({ t }: { t: Transaction }) {
  const teams = t.teams && t.teams.length > 2 ? t.teams : [t.teamA, t.teamB]
  const side = (id: number) => t.moved.filter((m) => m.to.id === id)
  const pickSide = (id: number) => (t.picks ?? []).filter((p) => p.to.id === id)
  const col = (team: Transaction['teamA']) => (
    <div className="min-w-0 flex-1">
      <div className="flex items-center gap-2"><TeamBadge team={team} size={16} /><span className="text-muted">receives</span></div>
      <div>{side(team.id).map((m, i) => <span key={m.playerId}>{i > 0 && ', '}<PlayerLink id={m.playerId} name={m.name} /></span>)}</div>
      {pickSide(team.id).length > 0 && (
        <div className="text-muted">{pickSide(team.id).map((p) => `${p.label} (from ${p.from.abbr})`).join(', ')}</div>
      )}
    </div>
  )
  const extra = [...t.released.map((r) => `${r.name} released by ${r.from.abbr}`), ...t.signed.map((r) => `${r.name} signed by ${r.to.abbr}`)]
  return (
    <li className="border-b border-line/50 px-2 py-2 text-[13px]">
      <div className="mb-1 text-[11px] uppercase tracking-wider text-muted">{txWhen(t)}</div>
      <div className={`flex gap-4 ${teams.length > 2 ? 'flex-col sm:flex-row' : ''}`}>{teams.map((tm) => <div key={tm.id} className="contents">{col(tm)}</div>)}</div>
      {extra.length > 0 && <div className="mt-1 text-xs text-muted">{extra.join(' · ')}</div>}
    </li>
  )
}
