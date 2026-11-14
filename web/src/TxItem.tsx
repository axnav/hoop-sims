import type { Transaction } from './api'
import { PlayerLink, TeamBadge } from './ui'

export const txWhen = (t: Transaction) => (t.day === 0 ? `${t.year} offseason` : `${t.year}, day ${t.day}`)

export function TxItem({ t }: { t: Transaction }) {
  const side = (id: number) => t.moved.filter((m) => m.to.id === id)
  const col = (team: Transaction['teamA']) => (
    <div className="flex-1">
      <div className="flex items-center gap-2"><TeamBadge team={team} size={16} /><span className="text-muted">receives</span></div>
      <div>{side(team.id).map((m, i) => <span key={m.playerId}>{i > 0 && ', '}<PlayerLink id={m.playerId} name={m.name} /></span>)}</div>
    </div>
  )
  const extra = [...t.released.map((r) => `${r.name} released by ${r.from.abbr}`), ...t.signed.map((r) => `${r.name} signed by ${r.to.abbr}`)]
  return (
    <li className="border-b border-line/50 px-2 py-2 text-[13px]">
      <div className="mb-1 text-[11px] uppercase tracking-wider text-muted">{txWhen(t)}</div>
      <div className="flex gap-4">{col(t.teamA)}{col(t.teamB)}</div>
      {extra.length > 0 && <div className="mt-1 text-xs text-muted">{extra.join(' · ')}</div>}
    </li>
  )
}
