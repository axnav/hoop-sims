import { useQuery } from '@tanstack/react-query'
import { api } from './api'
import { useFlow } from './flow'
import { Card, PlayerLink } from './ui'

export function InjuriesCard() {
  const { state } = useFlow()
  const q = useQuery({ queryKey: ['injuries', 'user', state?.day, state?.phase, state?.injured], queryFn: () => api.injuries() })
  const rows = q.data?.injuries ?? []
  if (rows.length === 0) return null
  return (
    <Card title="Injuries" className="mb-4">
      <ul className="px-2 pb-1">
        {rows.map((r) => (
          <li key={r.playerId} className="flex flex-wrap items-center gap-x-3 gap-y-0.5 border-b border-line/50 py-1 text-[13px]">
            <span className="min-w-0 flex-1 truncate"><PlayerLink id={r.playerId} name={r.name} /></span>
            <span className={r.serious ? 'font-semibold text-loss' : ''}>{r.injury}{r.serious && ' (serious)'}</span>
            <span className="w-24 text-right text-xs text-muted tabular-nums">out {r.gamesLeft} {r.gamesLeft === 1 ? 'game' : 'games'}</span>
          </li>
        ))}
      </ul>
    </Card>
  )
}
