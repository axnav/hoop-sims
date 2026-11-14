export type Team = {
  id: number; abbr: string; city: string; name: string; fullName: string
  primary: string; secondary: string; conf: number; confName: string
}
export type Injury = { name: string; gamesLeft: number; serious: boolean }
export type InjuryLine = { playerId: number; name: string; team: Team; injury: string; gamesLeft: number; serious: boolean }
export type Award = { playerId: number; name: string; team: Team; gp: number; ppg: number; rpg: number; apg: number } | null
export type Action = { id: string; label: string }
export type Phase = 'regular' | 'regular_done' | 'playoffs' | 'playoffs_done' | 'offseason' | 'draft' | 'draft_done'

export type SeriesGame = { n: number; home: number; homeScore: number; awayScore: number; winner: number; ot: number; box: number }
export type Series = {
  round: number; conf: number; slot: number; high: Team; low: Team; highSeed: number; lowSeed: number
  highWins: number; lowWins: number; done: boolean; winner: number; mine: boolean; games: SeriesGame[]
}
export type DayResult = {
  day: number; w: number; l: number; rank: number
  opp?: Team; home?: boolean; me?: number; them?: number; win?: boolean
}
export type GameState = {
  year: number; phase: Phase; day: number; totalDays: number; mode: 'real' | 'fictional'; conferences: string[]
  user: { team: Team; w: number; l: number; pd: number; confRank: number; confName: string }
  next: { kind: string; opp: Team; home: boolean; oppW: number; oppL: number; label: string; seriesMe?: number; seriesThem?: number } | null
  last?: { opp: Team; me: number; them: number; win: boolean; home: boolean; label: string; boxId: string; playoff: boolean; year: number }
  playoffs?: {
    active: boolean; round: number; roundName: string; userAlive: boolean; userResult: string
    champion?: Team; finalsMvp?: Award; mvp?: Award; finals?: Series
  }
  summary?: { w: number; l: number; rank: number; madePlayoffs: boolean; mvp: Award; scoringLeader: Award }
  draft?: { made: number; total: number; done: boolean; onClock?: Team; userOnClock?: boolean }
  nextAction: Action
  actions: Action[] | null
  offers: number
  injured: number
}
export type Meta = {
  loaded: boolean; realAvailable: boolean; saveExists: boolean
  fictionalTeams: Team[]; realTeams: Team[]; conferences: string[]
}
export type StandRow = { rank: number; team: Team; w: number; l: number; pct: number; gb: number; pd: number; inPlayoffs: boolean }
export type Standings = { year: number; conferences: { conf: number; name: string; rows: StandRow[] }[]; playoffLine: number; userTeam: number }
export type Leader = { playerId: number; name: string; team: Team; gp: number; value: number }
export type Leaders = { minGP: number; pts: Leader[]; reb: Leader[]; ast: Leader[] }
export type RosterRow = {
  id: number; name: string; age: number; pos: string; ovr: number; pot: number; starter: boolean
  gp: number; mpg: number; ppg: number; rpg: number; apg: number
  injury?: Injury
}
export type TeamPage = { team: Team; w: number; l: number; pd: number; confRank: number; isUser: boolean; roster: RosterRow[] }
export type StatLine = { gp: number; mpg: number; ppg: number; rpg: number; apg: number; tpg: number; fg: number | null; tp: number | null; ft: number | null } | null
export type PlayerPage = {
  id: number; name: string; age: number; pos: string; ovr: number; pot: number; status: string; team: Team | null
  role: string; usage: number
  injury?: Injury; editable: boolean; limits: { minRating: number; maxRating: number; minAge: number; maxAge: number }
  ratings: { key: string; label: string; value: number }[]; experience: number; draftYear: number; draftPick: number
  season: StatLine; playoffs: StatLine
  career: { year: number; team: Team; age: number; ovr: number; pot: number; gp: number; ppg: number; rpg: number; apg: number }[]
}
export type ScheduleGame = { day: number; opp: Team; home: boolean; played: boolean; me?: number; them?: number; win?: boolean }
export type Schedule = {
  team: Team; games: ScheduleGame[]
  playoffGames: { label: string; opp: Team; home: boolean; me: number; them: number; win: boolean; box: number }[]
}
export type BoxLine = {
  id: number; name: string; pos: string; min: number; pts: number; reb: number; ast: number; tov: number
  fgm: number; fga: number; tpm: number; tpa: number; ftm: number; fta: number
}
export type Box = {
  id: string; year: number; day: number; playoff: boolean; label: string; overtimes: number; hasReplay: boolean
  teams: { team: Team; score: number; lines: BoxLine[] }[]
}
export type GameEvent = { p: number; c: number; t: number; pl: number; a: number; y: string; n?: number; hs: number; as: number }
export type GameEvents = Box & { events: GameEvent[]; ot: number }
export type PlayoffsData = {
  year: number; live: boolean; started: boolean; canStart: boolean; active: boolean; roundName: string
  champion: Team | null; userAlive: boolean; series: Series[]; conferences: string[]
}
export type Change = { playerId: number; name: string; team: Team; age: number; oldOvr: number; newOvr: number }
export type OffseasonReport =
  | { available: false }
  | {
      available: true; year: number; risers: Change[]; fallers: Change[]; trades?: Transaction[]
      retirements: { playerId: number; name: string; age: number; ovr: number; team: Team }[]
    }
export type TradeStatus = { open: boolean; reason: string; reopens: string; deadlineDay: number; day: number; maxPlayers: number; maxPicks: number; maxTeams: number; rosterSize: number }
export type TradeAssetRow = { id: number; name: string; pos: string; age: number; ovr: number; pot: number; value: number; untouchable: boolean; tier: string; injury?: Injury }
export type TradePickRow = { year: number; round: number; original: Team; own: boolean; label: string; value: number; tier: string }
export type TradeBlockData = { team: Team; isUser: boolean; players: TradeAssetRow[]; picks: TradePickRow[] }
export type DealMove =
  | { from: number; to: number; playerId: number }
  | { from: number; to: number; pick: { year: number; round: number; original: number } }
export type DealVerdict = {
  valid: boolean; problem: string; accept: boolean
  teams: {
    team: Team; accept: boolean; reason: string; gives: number; gets: number; needed: number
    mood: 'accept' | 'close' | 'far' | 'no'; askText: string; ask?: DealMove; askLabel?: string
  }[]
}
export type AssetRow =
  | { kind: 'player'; playerId: number; name: string; pos: string; age: number; ovr: number; pot: number; value: number; tier: string; injury?: Injury }
  | { kind: 'pick'; year: number; round: number; original: Team; label: string; value: number; tier: string }
export type Offer = {
  id: number; team: Team; message: string; day: number; expires: number; daysLeft: number
  give: AssetRow[]; get: AssetRow[]; moves: DealMove[]
}
export type SimPause = { reason: 'offer' | 'deadline'; text: string }
export type SimResult = {
  days: DayResult[]; strip: { rank: number; team: Team; w: number; l: number }[]; state: GameState
  newOffers: number; pause?: SimPause; injuries: InjuryLine[]
}
export type Options = { noDeadlineStop: boolean; noOfferStop: boolean; noInjuries: boolean; commissioner: boolean }
export type AdminEdit = { age?: number; potential?: number; ratings?: Record<string, number>; heal?: boolean }
export type TxPick = { year: number; round: number; original: Team; label: string; from: Team; to: Team }
export type TradeOfferIn = { team: number; give: number[]; get: number[] }
export type TradeVerdict = {
  valid: boolean; problem: string; accept: boolean; reason: string
  theyGive: number; theyGet: number; youGive: number; youGet: number; needed: number
}
export type TxPlayer = { playerId: number; name: string; from: Team; to: Team }
export type Transaction = { year: number; day: number; teamA: Team; teamB: Team; moved: TxPlayer[]; released: TxPlayer[]; signed: TxPlayer[]; teams?: Team[]; picks?: TxPick[] }
export type TradeResult = Transaction & { state: GameState }
export type Pick = { pick: number; round: number; team: Team; made: boolean; player?: { id: number; name: string; age: number; ovr: number; pot: number } }
export type DraftData =
  | { active: false }
  | {
      active: true; total: number; made: number; done: boolean; picks: Pick[]; started: boolean; userTeam: number
      prospects: { id: number; name: string; age: number; pos: string; ovr: number; pot: number }[]
      onClock?: Team; userOnClock?: boolean
    }
export type HistoryRow = {
  year: number; champion: Team; mvp: Award; finalsMvp: Award; scoringLeader: Award
  finals: { winner: Team; loser: Team; winnerWins: number; loserWins: number } | null
}

export class ApiError extends Error {
  status: number
  constructor(status: number, msg: string) {
    super(msg)
    this.status = status
  }
}

async function req<T>(method: string, url: string, body?: unknown): Promise<T> {
  let res: Response
  try {
    res = await fetch(url, {
      method,
      headers: body !== undefined ? { 'Content-Type': 'application/json' } : undefined,
      body: body !== undefined ? JSON.stringify(body) : undefined,
    })
  } catch {
    throw new ApiError(0, 'Cannot reach the game server')
  }
  const data = await res.json().catch(() => ({}))
  if (!res.ok) throw new ApiError(res.status, (data as { error?: string }).error ?? `Request failed (${res.status})`)
  return data as T
}
const get = <T,>(u: string) => req<T>('GET', u)
const post = <T,>(u: string, b?: unknown) => req<T>('POST', u, b ?? {})

export const api = {
  meta: () => get<Meta>('/api/meta'),
  newGame: (b: { mode: string; seed: number; team: number }) => post<GameState>('/api/new', b),
  load: () => post<GameState>('/api/load'),
  save: () => post<{ ok: boolean }>('/api/save'),
  state: () => get<GameState>('/api/state'),
  sim: (b: { days?: number; toEnd?: boolean; auto?: boolean }) => post<SimResult>('/api/sim', b),
  standings: () => get<Standings>('/api/standings'),
  leaders: () => get<Leaders>('/api/leaders'),
  team: (id: number) => get<TeamPage>(`/api/team/${id}`),
  player: (id: number) => get<PlayerPage>(`/api/player/${id}`),
  schedule: (team?: number) => get<Schedule>('/api/schedule' + (team === undefined ? '' : `?team=${team}`)),
  box: (id: string) => get<Box>(`/api/box/${id}`),
  events: (id: string) => get<GameEvents>(`/api/game/${id}/events`),
  history: () => get<{ seasons: HistoryRow[] }>('/api/history'),
  playoffs: () => get<PlayoffsData>('/api/playoffs'),
  playoffAction: (a: string) => post<{ state: GameState; boxId?: number }>(`/api/playoffs/${a}`),
  offseason: () => get<OffseasonReport>('/api/offseason'),
  offseasonBegin: () => post<GameState>('/api/offseason/begin'),
  offseasonToDraft: () => post<GameState>('/api/offseason/to-draft'),
  offseasonFinish: () => post<GameState>('/api/offseason/finish'),
  draft: () => get<DraftData>('/api/draft'),
  draftPick: (playerId: number) => post<{ pick: Pick; state: GameState }>('/api/draft/pick', { playerId }),
  draftStep: (max = 1) => post<{ picks: Pick[]; state: GameState }>('/api/draft/step', { max }),
  tradeStatus: () => get<TradeStatus>('/api/trade/status'),
  tradeBlock: (id: number) => get<TradeBlockData>(`/api/trade/block/${id}`),
  tradeEvaluate: (o: TradeOfferIn) => post<TradeVerdict>('/api/trade/evaluate', o),
  tradeCounter: (o: TradeOfferIn) =>
    post<{ found: false } | { found: true; give: number[]; get: number[]; verdict: TradeVerdict }>('/api/trade/counter', o),
  tradeExecute: (o: TradeOfferIn) => post<TradeResult>('/api/trade/execute', o),
  dealEvaluate: (moves: DealMove[]) => post<DealVerdict>('/api/deal/evaluate', { moves }),
  dealExecute: (moves: DealMove[]) => post<TradeResult>('/api/deal/execute', { moves }),
  transactions: () => get<{ transactions: Transaction[]; userTeam: number }>('/api/transactions'),
  offers: () => get<{ offers: Offer[] }>('/api/offers'),
  offerAccept: (id: number) => post<TradeResult>(`/api/offers/${id}/accept`),
  offerReject: (id: number) => post<{ ok: true; offers: Offer[] }>(`/api/offers/${id}/reject`),
  options: () => get<Options>('/api/options'),
  setOptions: (o: Partial<Options>) => post<Options>('/api/options', o),
  injuries: (team?: number | 'all') => get<{ injuries: InjuryLine[] }>('/api/injuries' + (team === undefined ? '' : `?team=${team}`)),
  adminEditPlayer: (id: number, b: AdminEdit) => post<PlayerPage>(`/api/admin/player/${id}`, b),
  draftAuto: () => post<GameState>('/api/draft/auto'),
}
