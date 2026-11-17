import { Chip, PlayerLink, TeamBadge, type Tone } from './ui'
import type { NewsItem, NewsKind } from './api'

export const newsTone: Record<NewsKind, Tone> = {
  trade: 'info', signing: 'good', release: 'neutral', injury: 'bad', request: 'warn', game: 'neutral', streak: 'neutral',
  playoffs: 'violet', draft: 'info', award: 'good', retire: 'neutral', coach: 'neutral', league: 'neutral',
}
export const newsKindLabel: Record<NewsKind, string> = {
  trade: 'Trade', signing: 'Signing', release: 'Release', injury: 'Injury', request: 'Request', game: 'Game', streak: 'Streak',
  playoffs: 'Playoffs', draft: 'Draft', award: 'Award', retire: 'Retired', coach: 'Coach', league: 'League',
}
const stageLabel: Record<string, string> = { playoffs: 'Playoffs', draft: 'Draft', free_agency: 'Free agency', offseason: 'Offseason' }

export const newsDate = (n: NewsItem) => `${n.year} · ${n.day > 0 ? `Day ${n.day}` : stageLabel[n.stage] ?? 'Season'}`

export function NewsStory({ n, compact = false }: { n: NewsItem; compact?: boolean }) {
  const teams = n.teams ?? []
  const players = n.players ?? []
  return (
    <div className={`border-b border-line/60 last:border-b-0 ${compact ? 'px-2 py-1.5' : 'px-3 py-2.5'} ${n.mine ? 'border-l-2' : ''}`} style={n.mine ? { borderLeftColor: 'var(--team)' } : undefined}>
      <div className="flex flex-wrap items-center gap-1.5">
        <Chip tone={newsTone[n.kind] ?? 'neutral'}>{newsKindLabel[n.kind] ?? n.kind}</Chip>
        {n.mine && <Chip tone="strong">Your team</Chip>}
        {teams.map((t) => <TeamBadge key={t.id} team={t} size={compact ? 18 : 22} />)}
      </div>
      <div className={`mt-1 min-w-0 break-words ${n.big ? 'text-base font-bold' : compact ? 'text-[13px]' : ''}`}>{n.text}</div>
      {!compact && players.length > 0 && (
        <div className="mt-1 flex flex-wrap gap-x-3 gap-y-0.5 text-[13px]">
          {players.map((p) => <PlayerLink key={p.id} id={p.id} name={p.name} />)}
        </div>
      )}
    </div>
  )
}
