export type ChangeScope = 'season' | 'year'
export type DevLine = { playerId: number; name: string; ovr: number; change: number }
export type Team = {
  id: number; abbr: string; city: string; name: string; fullName: string
  primary: string; secondary: string; conf: number; confName: string
}
export type Payroll = { budget?: number; dead?: number; payroll: number; cap: number; taxLine: number; ceiling: number; room: number; taxBill: number; minSalary: number }
export type Injury = { name: string; gamesLeft: number; serious: boolean }
export type InjuryLine = { photo?: boolean; ovrChange?: number; potChange?: number; playerId: number; name: string; team: Team; injury: string; gamesLeft: number; serious: boolean }
export type Award = { playerId: number; name: string; team: Team; gp: number; ppg: number; rpg: number; apg: number } | null
export type Action = { id: string; label: string }
export type Phase = 'regular' | 'regular_done' | 'playoffs' | 'playoffs_done' | 'offseason' | 'draft' | 'draft_done' | 'free_agency'

export type SeriesGame = { n: number; home: number; homeScore: number; awayScore: number; winner: number; ot: number; box: number }
export type Series = {
  round: number; conf: number; slot: number; high: Team; low: Team; highSeed: number; lowSeed: number
  highWins: number; lowWins: number; done: boolean; winner: number; mine: boolean; games: SeriesGame[]
}
export type DayResult = {
  day: number; w: number; l: number; rank: number
  opp?: Team; home?: boolean; me?: number; them?: number; win?: boolean; hasReplay?: boolean
}
export type GameState = {
  year: number; phase: Phase; day: number; totalDays: number; mode: 'real' | 'fictional'; realSeason?: number; conferences: string[]
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
  expiring: number
  payroll: Payroll
  fa?: { day: number; days: number; daysLeft: number }
  pendingSheets?: number
  gm?: { count: number }
  owner?: { trust: number; label: string; fired: boolean; alert: string | null }
}
export type OwnerPersonality = { key: 'win_now' | 'patient' | 'cheap' | ''; label: string; blurb: string }
export type OwnerGoal = {
  key: string; kind: 'results' | 'money' | 'building'; title: string; detail: string
  weight: 'Low' | 'Medium' | 'High'; progress: string; status: 'on_track' | 'at_risk' | 'met' | 'missed'
}
export type OwnerLedgerRow = {
  year: number; wins: number; losses: number; playoffs: string; revenue: number; costs: number; profit: number
  trustStart: number; trustEnd: number; goals: { title: string; met: boolean }[]
}
export type OwnerTone = 'good' | 'warn' | 'bad' | 'info'
export type OwnerMessage = { year: number; day: number; text: string; tone: OwnerTone }
export type OwnerJob = {
  team: Team; owner: { name: string; personality: OwnerPersonality }; marketSize: { key: string; label: string }
  record: string; payroll: number; direction: { key: string; label: string }
}
export type Owner = {
  active: boolean; firing: boolean
  owner: { name: string; personality: OwnerPersonality }
  trust: { score: number; label: 'Secure' | 'Steady' | 'Uneasy' | 'Hot seat' | ''; trend: 'up' | 'flat' | 'down'; lastChange: number }
  goals: OwnerGoal[]
  money: {
    budget: number; payroll: number; overBudget: number; marketSize: { key: string; label: string }
    revenue: { market: number; wins: number; playoffs: number; total: number }
    costs: { salaries: number; deadMoney: number; tax: number; total: number }
    profit: number; projected: boolean
  }
  ledger: OwnerLedgerRow[]; messages: OwnerMessage[]
  fired: boolean; firedText: string; jobs: OwnerJob[]
}
export type Level = 'Low' | 'Medium' | 'High'
export type Motivation = { money: Level; winning: Level; role: Level; loyalty: Level }
export type Training = {
  focus: string; options: { key: string; label: string }[]; editable: boolean
  skills: { rating: string; speed: 'Slow' | 'Normal' | 'Fast'; open: boolean; stepsFor2: string }[]
}
export type Stance = { key: 'resign' | 'open' | 'test'; label: string }
export type FAPlayer = {
  motivation?: Motivation
  id: number; name: string; pos: string; age: number; ovr: number; pot: number; tier: string
  askSalary: number; askYears: number; interest: number
  photo?: boolean; ovrChange?: number; changeScope?: ChangeScope; potChange?: number
  myOffer?: { id: number; salary: number; years: number; day: number; via?: FAVia }; injury?: Injury
  restricted?: boolean; rightsTeam?: Team; mine?: boolean; minYears?: number
}
export type FAVia = 'room' | 'exception' | 'minimum' | 'own'
export type FAOfferRow = { id: number; playerId: number; name: string; salary: number; years: number; day: number; via?: FAVia }
export type FASigning = { day: number; playerId: number; name: string; pos: string; ovr: number; team: Team; salary: number; years: number; mine: boolean; matched?: boolean; via?: string; sheetFrom?: Team }
export type FAException = { kind: 'mid' | 'taxpayer' | 'none'; label: string; maxSalary: number; maxYears: number; used: boolean; reason: string }
export type FASheet = { playerId: number; name: string; pos: string; age: number; ovr: number; pot: number; team: Team; salary: number; years: number; canMatch: boolean; problem: string }
export type FreeAgencyData = {
  open: boolean; day: number; days: number; daysLeft: number; payroll: Payroll
  rosterCount: number; rosterSize: number; minSalary: number; maxYears: number
  players: FAPlayer[]; offers: FAOfferRow[]; log: FASigning[]
  exception?: FAException; sheets?: FASheet[]
}
export type Meta = {
  loaded: boolean; realAvailable: boolean; saveExists: boolean
  fictionalTeams: Team[]; realTeams: Team[]; conferences: string[]
  realSeasons?: { season: number; label: string; teams: Team[] }[]
}
export type StandRow = { rank: number; team: Team; w: number; l: number; pct: number; gb: number; pd: number; inPlayoffs: boolean }
export type Standings = { year: number; conferences: { conf: number; name: string; rows: StandRow[] }[]; playoffLine: number; userTeam: number }
export type Leader = { ovrChange?: number; changeScope?: ChangeScope; potChange?: number; playerId: number; name: string; team: Team; gp: number; value: number }
export type Badge = { key: string; name: string; group: string; tier: string; kind: 'rating' | 'performance' | 'personality'; rule: string; effect: string; next: string }
export type BadgeDef = { key: string; name: string; group: string; kind: string; rule: string; effect: string }
export type Leaders = { minGP: number; pts: Leader[]; reb: Leader[]; ast: Leader[] }
export type MoraleLabel = 'Furious' | 'Unhappy' | 'Content' | 'Happy' | 'Thrilled'
export type MoraleBrief = { score: number; label: MoraleLabel }
export type Morale = MoraleBrief & { trend: 'up' | 'down' | 'steady'; reasons: string[]; tradeRequest: boolean }
export type RosterRow = {
  morale?: MoraleBrief; tradeRequest?: boolean
  id: number; name: string; age: number; pos: string; ovr: number; pot: number; starter: boolean
  gp: number; mpg: number; ppg: number; rpg: number; apg: number
  injury?: Injury
  salary: number; years: number; expiring: boolean
  photo?: boolean; ovrChange?: number; changeScope?: ChangeScope; potChange?: number
  stance?: Stance; extended?: boolean; focus?: string; badgeCount?: number
}
export type SearchRow = {
  id: number; name: string; pos: string; age: number; ovr: number; pot: number; team: Team | null
  salary: number; years: number; injury?: Injury; photo?: boolean; ovrChange?: number; changeScope?: ChangeScope; potChange?: number
  badgeCount?: number; morale?: MoraleBrief; tradeRequest?: boolean
}
export type PlayerSearch = { total: number; players: SearchRow[] }
export type TeamStatus = { key: 'contender' | 'buyer' | 'seller' | 'rebuilding'; label: string; wants: string }
export type TeamRow = {
  team: Team; w: number; l: number; confRank: number; payroll: number; capRoom: number; rosterSize: number; top8: number
  star?: { id: number; name: string; pos: string; ovr: number }; status: TeamStatus; offers: number; isUser: boolean
}
export type TeamPage = { teamMorale?: MoraleBrief; status?: TeamStatus; team: Team; w: number; l: number; pd: number; confRank: number; isUser: boolean; roster: RosterRow[]; payroll: Payroll; deadMoney?: { name: string; salary: number; years: number }[]; coach?: { name: string; system: string; systemName: string }; teamFit?: Fit }
export type StatLine = { gp: number; mpg: number; ppg: number; rpg: number; apg: number; tpg: number; fg: number | null; tp: number | null; ft: number | null } | null
export type PlayerPage = {
  awards?: PlayerAward[] | null
  morale?: Morale
  id: number; name: string; age: number; pos: string; ovr: number; pot: number; status: string; team: Team | null
  photo?: boolean; ovrChange?: number; changeScope?: ChangeScope; potChange?: number; ratingChanges?: Record<string, number>
  role: string; usage: number; salary: number; years: number
  posOverall?: Record<'PG' | 'SG' | 'SF' | 'PF' | 'C', number>; canSetPosition?: boolean
  injury?: Injury; editable: boolean; limits: { minRating: number; maxRating: number; minAge: number; maxAge: number }
  ratings: { key: string; label: string; value: number; lo?: number; hi?: number }[]; experience: number; draftYear: number; draftPick: number
  badges?: Badge[]
  prospect?: boolean; scout?: { level: number; ovrLo: number; ovrHi: number; potLo: number; potHi: number }
  season: StatLine; playoffs: StatLine
  motivation?: Motivation; stance?: Stance & { reasons: string[] }; extension?: { salary: number; years: number }; training?: Training
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
export type TradeStatus = { open: boolean; reason: string; reopens: string; deadlineDay: number; day: number; maxPlayers: number; maxPicks: number; maxTeams: number; rosterSize: number; draftNight?: boolean }
export type TradeAssetRow = { morale?: MoraleBrief; tradeRequest?: boolean; id: number; name: string; pos: string; age: number; ovr: number; pot: number; value: number; untouchable: boolean; tier: string; injury?: Injury; salary: number; years: number; photo?: boolean; ovrChange?: number; changeScope?: ChangeScope; potChange?: number }
export type TradePickRow = { year: number; round: number; original: Team; own: boolean; label: string; value: number; tier: string }
export type TradeBlockData = { status?: TeamStatus; team: Team; isUser: boolean; players: TradeAssetRow[]; picks: TradePickRow[]; payroll: Payroll }
export type DealMove =
  | { from: number; to: number; playerId: number }
  | { from: number; to: number; pick: { year: number; round: number; original: number } }
export type DealPayroll = { team: Team; before: number; after: number; in: number; out: number }
export type DealVerdict = {
  valid: boolean; problem: string; accept: boolean
  payrolls?: DealPayroll[]
  teams: {
    team: Team; accept: boolean; reason: string; gives: number; gets: number; needed: number
    mood: 'accept' | 'close' | 'far' | 'no'; askText: string; ask?: DealMove; askLabel?: string
  }[]
}
export type AssetRow =
  | { kind: 'player'; playerId: number; name: string; pos: string; age: number; ovr: number; pot: number; value: number; tier: string; injury?: Injury; salary: number; years: number; photo?: boolean; ovrChange?: number; changeScope?: ChangeScope; potChange?: number }
  | { kind: 'pick'; year: number; round: number; original: Team; label: string; value: number; tier: string }
export type Offer = {
  status?: TeamStatus; id: number; team: Team; message: string; day: number; expires: number; daysLeft: number; draftPick?: number
  give: AssetRow[]; get: AssetRow[]; moves: DealMove[]
}
export type GMAutoKey = 'lineup' | 'training' | 'contracts' | 'freeAgency' | 'draft' | 'tradeAlerts'
export type GMArea = { key: string; label: string; blurb: string; canAuto: boolean; autoKey: GMAutoKey; autoLabel: string }
export type GMSuggestion = { id: string; area: string; title: string; reasons: string[]; action: string; playerId: number | null; link: string | null }
export type GMLogEntry = { year: number; day: number; area: string; text: string; auto: boolean }
export type GM = {
  plan: string; plans: { key: string; label: string; blurb: string }[]
  auto: Record<GMAutoKey, boolean>; areas: GMArea[]; suggestions: GMSuggestion[]
  counts: Record<string, number>; log: GMLogEntry[]
}
export type SimPause = { reason: 'offer' | 'deadline' | 'assistant'; text: string }
export type SimResult = {
  days: DayResult[]; strip: { rank: number; team: Team; w: number; l: number }[]; state: GameState
  newOffers: number; pause?: SimPause; injuries: InjuryLine[]; development?: DevLine[]
}
export type ExpiringPlayer = {
  id: number; name: string; pos: string; age: number; ovr: number; pot: number; tier: string
  askSalary: number; askYears: number; canReSign: boolean; problem: string
  photo?: boolean; ovrChange?: number; changeScope?: ChangeScope; potChange?: number; restricted?: boolean
  stance?: Stance; extended?: boolean
}
export type ExtensionAsk = { ok: true; salary: number; maxSalary: number; maxYears: number; current: number } | { ok: false; reason: string }
export type ReleasePreview = { allowed: boolean; reason: string; deadSalary: number; deadYears: number; refill: boolean; refillName: string; short?: boolean }
export type Expiring = { players: ExpiringPlayer[]; payroll: Payroll; open: boolean }
export type Options = { noDeadlineStop: boolean; noOfferStop: boolean; noInjuries: boolean; noInSeasonDev: boolean; commissioner: boolean; noFiring: boolean }
export type AdminEdit = { age?: number; potential?: number; ratings?: Record<string, number>; heal?: boolean; overall?: number }
export type TxPick = { year: number; round: number; original: Team; label: string; from: Team; to: Team }
export type TradeOfferIn = { team: number; give: number[]; get: number[] }
export type TeamNeeds = { team: Team; direction: TeamStatus; values: string[]; lacks: string[]; summary: string }
export type TakeAsset =
  | { kind: 'player'; id: number; playerId: number; name: string; pos: string; age: number; ovr: number; pot: number; salary: number; years: number; value: number }
  | { kind: 'pick'; year: number; round: number; original: Team; label: string; value: number }
export type TakePackage = { label: string; give: TakeAsset[]; receive: TakeAsset[]; note: string; cost: number; cheapest: boolean; deal: { moves: DealMove[] } }
export type WhatWouldItTake = { player: TradeAssetRow; team: Team; possible: boolean; reason: string; packages: TakePackage[] }
export type UpgradeRow = { player: TradeAssetRow; team: Team; direction: TeamStatus; gain: number; shopped: boolean; shopReason: string; possible: boolean; reason: string; package?: TakePackage }
export type Upgrades = { player: TradeAssetRow; possible: boolean; reason: string; upgrades: UpgradeRow[] }
export type FindOffer = { team: Team; direction: TeamStatus; receive: TakeAsset[]; value: number; salaryIn: number; salaryOut: number; note: string; deal: { moves: DealMove[] } }
export type FindOffers = {
  possible: boolean; reason: string; want: string; wants: { key: string; label: string }[]
  give: TakeAsset[]; giveValue: number; offers: FindOffer[]; passed: { team: Team; reason: string }[]
}
export type FindBody = { players: number[]; picks: { year: number; round: number; original: number }[]; want?: string }
export type Advice = 'keep' | 'shop' | 'sell'
export type AdvisorRow = { id: number; name: string; pos: string; age: number; ovr: number; pot: number; salary: number; years: number; advice: Advice; reasons: string[] }
export type Advisor = { team: Team; direction: TeamStatus; players: AdvisorRow[]; summary: string }
export type TradeVerdict = {
  valid: boolean; problem: string; accept: boolean; reason: string
  theyGive: number; theyGet: number; youGive: number; youGet: number; needed: number
}
export type TxPlayer = { playerId: number; name: string; from: Team; to: Team }
export type NewsEvent = { type: 'trade_request' | 'request_withdrawn' | 'request_traded'; text: string; year: number; day: number; playerId: number; name: string; team: Team }
export type Transaction = { year: number; day: number; teamA: Team; teamB: Team; moved: TxPlayer[]; released: TxPlayer[]; signed: TxPlayer[]; teams?: Team[]; picks?: TxPick[] }
export type TradeResult = Transaction & { state: GameState }
export type Pick = { pick: number; round: number; team: Team; via?: Team; made: boolean; player?: { id: number; name: string; age: number; ovr: number; pot: number } }
export type Prospect = { id: number; name: string; age: number; pos: string; ovrLo: number; ovrHi: number; potLo: number; potHi: number; scoutLevel: number }
export type LotteryRow = { team: Team; owner: Team; seed: number; slot: number; odds: number; change: number }
export type DraftData =
  | { active: false }
  | {
      active: true; total: number; made: number; done: boolean; picks: Pick[]; started: boolean; userTeam: number
      prospects: Prospect[]
      scouting: { points: number; maxLevel: number; perDraft: number }
      lottery: LotteryRow[]
      onClock?: Team; userOnClock?: boolean
    }
export type CoachAward = { name: string; team: Team; w: number; l: number } | null
export type SeasonAwards = {
  roy?: Award; dpoy?: Award; sixthMan?: Award; mip?: Award
  coach?: CoachAward; allLeague?: Award[][] | null; allRookie?: Award[] | null
}
export type CurrentAwards = { year: number; mvp?: Award; scoringLeader?: Award; awards?: SeasonAwards | null }
export type HistoryResponse = { seasons: HistoryRow[]; current?: CurrentAwards | null }
export type NewsKind = 'trade' | 'signing' | 'release' | 'injury' | 'request' | 'game' | 'streak' | 'playoffs' | 'draft' | 'award' | 'retire' | 'coach' | 'league'
export type NewsItem = {
  id: number; year: number; day: number; stage: 'season' | 'playoffs' | 'draft' | 'free_agency' | 'offseason'
  kind: NewsKind; text: string; big: boolean; mine: boolean
  teams?: Team[] | null; players?: { id: number; name: string }[] | null
}
export type NewsFilter = { key: string; label: string; count: number }
export type NewsResponse = { items: NewsItem[] | null; filters?: NewsFilter[] | null; total: number }
export type PlayerAward = { year: number; title: string }
export type HistoryRow = {
  awards?: SeasonAwards | null
  year: number; champion: Team; mvp: Award; finalsMvp: Award; scoringLeader: Award
  finals: { winner: Team; loser: Team; winnerWins: number; loserWins: number } | null
}

export type Fit = { score: number; grade: string }
export type FitFull = Fit & { reasons: string[] }
export type CoachInfo = { name: string; age: number; offense: number; defense: number; development: number; system: string; systemName: string }
export type CoachSystem = {
  key: string; name: string; description: string; pace: string; threes: string; starUsage: string; defense: string
  wants: string[]; teamFit: Fit
}
export type CoachData = {
  coach: CoachInfo; system: string; systemName: string; favourite: boolean; teamFit: Fit
  canChangeSystem: boolean; canHire: boolean; editable: boolean
  systems: CoachSystem[]
  players: { id: number; name: string; pos: string; minutes: number; fit: FitFull }[]
}
export type CoachPool = { open: boolean; candidates: (CoachInfo & { id: number })[] }

export type LineupPlayer = {
  fit?: Fit;
  id: number; name: string; pos: string; age: number; ovr: number; pot: number; tier: string
  minutes: number; autoMinutes: number; nextMinutes: number; starterSlot: number
  gp: number; mpg: number; injury?: Injury
  photo?: boolean; ovrChange?: number; changeScope?: ChangeScope; potChange?: number
}
export type LineupData = {
  teamFit?: Fit; custom: boolean; total: number; maxMinutes: number; minPlayers: number; slots: string[]
  starters: number[]
  dev: { maxAge: number; fullMinutes: number; noneMinutes: number }
  players: LineupPlayer[]
}

export type PhotoCredit = { artist: string; licence: string; source: string }

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
  newGame: (b: { mode: string; seed: number; team: number; season?: number }) => post<GameState>('/api/new', b),
  load: () => post<GameState>('/api/load'),
  save: () => post<{ ok: boolean }>('/api/save'),
  state: () => get<GameState>('/api/state'),
  gm: () => get<GM>('/api/gm'),
  gmSettings: (b: { plan?: string; auto?: Partial<Record<GMAutoKey, boolean>> }) => post<GM>('/api/gm/settings', b),
  owner: () => get<Owner>('/api/owner'),
  ownerJob: (teamId: number) => post<{ message: string; owner: Owner; state: GameState }>('/api/owner/job', { teamId }),
  gmApply: (id: string) => post<{ message: string; gm: GM; state: GameState }>('/api/gm/apply', { id }),
  sim: (b: { days?: number; toEnd?: boolean; auto?: boolean }) => post<SimResult>('/api/sim', b),
  standings: () => get<Standings>('/api/standings'),
  leaders: (badge = '') => get<Leaders>('/api/leaders' + (badge ? `?badge=${encodeURIComponent(badge)}` : '')),
  badges: () => get<{ badges: BadgeDef[] }>('/api/badges'),
  players: (qs: string) => get<PlayerSearch>('/api/players' + (qs ? `?${qs}` : '')),
  teams: () => get<{ teams: TeamRow[] }>('/api/teams'),
  team: (id: number) => get<TeamPage>(`/api/team/${id}`),
  player: (id: number) => get<PlayerPage>(`/api/player/${id}`),
  photoCredit: (id: number) => get<PhotoCredit>(`/api/photo/${id}/credit`),
  schedule: (team?: number) => get<Schedule>('/api/schedule' + (team === undefined ? '' : `?team=${team}`)),
  box: (id: string) => get<Box>(`/api/box/${id}`),
  events: (id: string) => get<GameEvents>(`/api/game/${id}/events`),
  history: () => get<HistoryResponse>('/api/history'),
  news: (filter = 'all', limit = 100) => get<NewsResponse>(`/api/news?filter=${encodeURIComponent(filter)}&limit=${limit}`),
  playoffs: () => get<PlayoffsData>('/api/playoffs'),
  playoffAction: (a: string) => post<{ state: GameState; boxId?: number }>(`/api/playoffs/${a}`),
  offseason: () => get<OffseasonReport>('/api/offseason'),
  offseasonBegin: () => post<GameState>('/api/offseason/begin'),
  offseasonToDraft: () => post<GameState>('/api/offseason/to-draft'),
  offseasonFinish: () => post<GameState>('/api/offseason/finish'),
  draft: () => get<DraftData>('/api/draft'),
  draftScout: (playerId: number) => post<DraftData>('/api/draft/scout', { playerId }),
  draftPick: (playerId: number) => post<{ pick: Pick; state: GameState }>('/api/draft/pick', { playerId }),
  draftStep: (max = 1) => post<{ picks: Pick[]; state: GameState }>('/api/draft/step', { max }),
  tradeStatus: () => get<TradeStatus>('/api/trade/status'),
  tradeBlock: (id: number) => get<TradeBlockData>(`/api/trade/block/${id}`),
  tradeNeeds: (id: number) => get<TeamNeeds>(`/api/trade/needs/${id}`),
  whatWouldItTake: (id: number) => get<WhatWouldItTake>(`/api/trade/whatwouldittake/${id}`),
  findUpgrades: (id: number) => get<Upgrades>(`/api/trade/upgrade/${id}`),
  tradeFind: (b: FindBody) => post<FindOffers>('/api/trade/find', b),
  tradeAdvisor: () => get<Advisor>('/api/trade/advisor'),
  tradeEvaluate: (o: TradeOfferIn) => post<TradeVerdict>('/api/trade/evaluate', o),
  tradeCounter: (o: TradeOfferIn) =>
    post<{ found: false } | { found: true; give: number[]; get: number[]; verdict: TradeVerdict }>('/api/trade/counter', o),
  tradeExecute: (o: TradeOfferIn) => post<TradeResult>('/api/trade/execute', o),
  dealEvaluate: (moves: DealMove[]) => post<DealVerdict>('/api/deal/evaluate', { moves }),
  dealExecute: (moves: DealMove[]) => post<TradeResult>('/api/deal/execute', { moves }),
  transactions: () => get<{ transactions: Transaction[]; events: NewsEvent[]; userTeam: number }>('/api/transactions'),
  offers: () => get<{ offers: Offer[] }>('/api/offers'),
  offerAccept: (id: number) => post<TradeResult>(`/api/offers/${id}/accept`),
  offerReject: (id: number) => post<{ ok: true; offers: Offer[] }>(`/api/offers/${id}/reject`),
  options: () => get<Options>('/api/options'),
  setOptions: (o: Partial<Options>) => post<Options>('/api/options', o),
  injuries: (team?: number | 'all') => get<{ injuries: InjuryLine[] }>('/api/injuries' + (team === undefined ? '' : `?team=${team}`)),
  adminEditPlayer: (id: number, b: AdminEdit) => post<PlayerPage>(`/api/admin/player/${id}`, b),
  contractsExpiring: () => get<Expiring>('/api/contracts/expiring'),
  contractResign: (playerId: number) => post<{ expiring: Expiring; state: GameState }>('/api/contracts/resign', { playerId }),
  extensionAsk: (id: number) => get<ExtensionAsk>(`/api/contracts/extension/${id}`),
  contractExtend: (playerId: number, salary: number, years: number) => post<{ message: string; state: GameState }>('/api/contracts/extend', { playerId, salary, years }),
  releasePreview: (id: number) => get<ReleasePreview>(`/api/contracts/release/${id}`),
  contractRelease: (playerId: number) => post<{ expiring: Expiring; state: GameState }>('/api/contracts/release', { playerId }),
  setPosition: (id: number, pos: string, alt: string) => post<PlayerPage>(`/api/player/${id}/position`, { pos, alt }),
  setTraining: (playerId: number, focus: string) => post<{ training: Training; message: string }>('/api/training', { playerId, focus }),
  draftAuto: () => post<GameState>('/api/draft/auto'),
  freeAgents: () => get<FreeAgencyData>('/api/freeagents'),
  faBegin: () => post<{ freeAgency: FreeAgencyData; state: GameState }>('/api/freeagents/begin'),
  faOffer: (b: { playerId: number; salary: number; years: number }) => post<FreeAgencyData>('/api/freeagents/offer', b),
  faSheet: (playerId: number, match: boolean) => post<{ signings: FASigning[]; freeAgency: FreeAgencyData; state: GameState; voided?: boolean }>('/api/freeagents/sheet', { playerId, match }),
  faWithdraw: (offerId: number) => post<FreeAgencyData>('/api/freeagents/withdraw', { offerId }),
  coach: (teamId: number) => get<CoachData>(`/api/coach/${teamId}`),
  coachSystem: (system: string) => post<CoachData>('/api/coach/system', { system }),
  coachPool: () => get<CoachPool>('/api/coach/pool'),
  coachHire: (id: number) => post<CoachData>('/api/coach/hire', { id }),
  lineup: () => get<LineupData>('/api/lineup'),
  setLineup: (b: { starters: number[]; minutes: Record<string, number> }) => post<LineupData>('/api/lineup', b),
  autoLineup: () => post<LineupData>('/api/lineup/auto'),
  faDay: () => post<{ signings: FASigning[]; freeAgency: FreeAgencyData; state: GameState }>('/api/freeagents/day'),
}
