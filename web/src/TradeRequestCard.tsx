import { Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from './api'
import { useFlow } from './flow'
import { Card, MoraleChip, PlayerLink } from './ui'

// Players on the user's team who currently have a trade request open (from the roster rows).
export function TradeRequestCard() {
  const { state } = useFlow()
  const id = state?.user.team.id
  const q = useQuery({ queryKey: ['team', id], queryFn: () => api.team(id!), enabled: id != null })
  const rows = (q.data?.roster ?? []).filter((r) => r.tradeRequest)
  if (rows.length === 0) return null
  return (
    <Card title="Trade requests" className="mb-4" right={<Link className="link text-accent" to="/trade#player-news">Player news</Link>}>
      <ul className="px-2 pb-1">
        {rows.map((r) => (
          <li key={r.id} className="flex flex-wrap items-center gap-x-3 gap-y-0.5 border-b border-line/50 py-1 text-[13px]">
            <span className="min-w-0 flex-1 truncate"><PlayerLink id={r.id} name={r.name} /> <span className="text-xs text-muted">{r.pos} · OVR {r.ovr}</span></span>
            <MoraleChip morale={r.morale} />
            <span className="text-xs text-loss">Wants out</span>
          </li>
        ))}
      </ul>
    </Card>
  )
}
