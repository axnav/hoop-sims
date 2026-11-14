# HoopSim architecture

This file explains how HoopSim is put together and where to change things. Read it once from top to bottom, then use it as a lookup. It describes the code as it is now. For the working rules (what you must never do, how to test, how to use helpers) read `RULES.md`; this file does not repeat them. `HANDOFF.md` records the current state and what is planned next.

Money in the code is an integer number of thousands of dollars. Day and year numbers are explained where they appear.

## 1. What HoopSim is

HoopSim is a basketball general-manager game. You run one team of a 30-team league: you set the lineup and minutes, trade players and draft picks, re-sign or release players, scout and make draft picks, bid for free agents, and then advance the calendar while the computer plays every game. There is no on-court play and no graphics of play beyond an optional replay of a finished game.

There are two league modes. The default is a fictional league made from generated players and team names. The second mode builds the league from private Basketball-Reference CSV files in `data/real/` (the 2010-11 season is the starting year); it only works if those files are present on the machine, and they are never committed (`.gitignore` excludes `/data/real/` and `/saves/`). The files there are `Advanced.csv`, `Draft Pick History.csv`, `Player Career Info.csv`, `Player Totals.csv`, `Team Abbrev.csv` and `Team Summaries.csv`. Everything after the start is simulated by the same engine in both modes.

The game is simulation only: results come from a possession-by-possession model in `engine/game.go`. Every player also has a hidden number, `Player.Impact`, that shifts his real performance away from his visible ratings. It exists by design and is never shown.

## 2. The three layers

```
  web/  (React + TypeScript screens, built to web/dist)
    |   HTTP + JSON  (/api/...)
    v
  cmd/web/  (Go: JSON API and static file server, one league in memory)
    |   function calls
    v
  engine/  (Go: all rules, data model, simulation, saving)

  cmd/sim, cmd/realcheck  (command-line tools; call engine/ directly)
```

Dependencies point downward only. `engine/` imports nothing from the project. `cmd/web/`, `cmd/sim/` and `cmd/realcheck/` import `hoopsim/engine`. The browser code knows the server only through HTTP.

| Layer | Its one rule |
|---|---|
| `engine/` | Holds every game rule and all state, and knows nothing about HTTP or JSON routes. |
| `cmd/web/` | Only translates: read a request, call the engine, build a JSON response (plain maps, so engine structs and the hidden number never leak). |
| `web/src/` | Only shows what the API returns; no game rules in the browser. |
| `cmd/sim/` | Developer tool: runs league-average games, multi-season runs and rating tables, printed to the terminal. |
| `cmd/realcheck/` | Developer tool: compares simulated real-data seasons with real 2010-11 results. Needs `data/real/`. |

The standard library is the only Go dependency (`go.mod` says `module hoopsim`, `go 1.22`).

## 3. How to run, build and test

Your shell may need `export PATH="/usr/local/bin:$PATH"`. Run everything from the project root.

| Task | Command |
|---|---|
| Build the front end (once, and after any UI change) | `cd web && npm install && npm run build` (runs `tsc --noEmit && vite build`, output in `web/dist`) |
| Run the game | `go run ./cmd/web`, then open http://localhost:8080 |
| Front-end dev server | `cd web && npm run dev` (Vite), and in another terminal `go run ./cmd/web -port 8095` |
| All Go checks | `go vet ./... && go test ./... -count=1` |
| League-average game check | `go run ./cmd/sim -mode sanity` |
| Multi-season run | `go run ./cmd/sim -mode seasons -seasons 10` |
| Rating and rookie tables | `go run ./cmd/sim -mode ratings` |
| Real-data comparison | `go run ./cmd/realcheck -runs 30` |

`cmd/web` flags: `-port` (default 8080), `-save` (save file path, default `saves/league.json`), `-web` (built app directory, default `web/dist`). The real-data folder is fixed at `data/real`, relative to the working directory. `cmd/sim` flags: `-seed`, `-games`, `-mode` (`sanity`, `seasons` or `ratings`), `-seasons`, and a debug flag `-reveal` that only affects that terminal tool. `cmd/realcheck` flags: `-runs`, `-data`.

The Go server serves `web/dist` for any path that is not `/api/...` and falls back to `index.html`, so the browser router can use real URLs. If `web/dist/index.html` is missing it answers 503 with the build command. The Vite dev server (`web/vite.config.ts`) proxies `/api` to `http://localhost:8095`, which is why the dev instance of the Go server must use port 8095.

The league lives in memory in the running server. It is written to disk only when the player presses save (`POST /api/save`) and read back by `POST /api/load`. Unsaved progress is lost when the server stops.

## 4. A season, stage by stage

The league is always in one stage, called a phase (`engine.Phase`, in `engine/phase.go`). The phase is not stored; `League.Phase(reportSeen)` works it out from the data. `reportSeen` is a flag the server keeps (in `server.reportSeen`, never saved) saying whether the player has looked at the offseason report. Until he has, a league with a draft set up reports phase `offseason` instead of `draft`. Starting or loading a game sets it to true.

`League.NextActions(reportSeen)` returns the main "continue" button and a menu of extra actions for the current phase. The server sends them as `nextAction` and `actions` in the `/api/state` response; the front end runs them through `FlowProvider.run(id)` in `web/src/flow.tsx`. The `Can...` guard functions in `phase.go` (`CanSimDay`, `CanStartPlayoffs`, `CanPlayoffAction`, `CanBeginOffseason`, `CanGoToDraft`, `CanFinishOffseason`, `CanAdvanceFreeAgency`, `CanDraft`, `CanUserPick`) decide whether a step is allowed; handlers turn a refusal into a 409.

| Phase string | Meaning | Engine call that moves it on |
|---|---|---|
| `regular` | Regular-season days remain (82 days; every team plays each day, 15 games per day) | `League.SimDay` (and `SimDays`, `SimRegularSeason`) |
| `regular_done` | Season over, playoffs not started | `League.StartPlayoffs` |
| `playoffs` | Playoffs in progress (best-of-seven, 8 seeds per conference, 4 rounds) | `SimPlayoffDay`, `SimUserSeries`, `SimRound`, `SimAllPlayoffs` |
| `playoffs_done` | Champion decided (`Season.Champion >= 0`) | `League.BeginOffseason` |
| `offseason` | Offseason report not yet seen; draft is set up | the server sets `reportSeen` (no engine call) |
| `draft` | Draft in progress (60 picks, two rounds) | `DraftPick`, `DraftAutoUntilUser`, `DraftAutoAll` |
| `draft_done` | Draft finished, free agency not open | `League.BeginFreeAgency` |
| `free_agency` | `League.FA != nil`, 10 days | `AdvanceFreeAgency` per day; `FinishOffseason` ends it |
| (back to `regular`) | New season | `FinishOffseason` then `StartSeason` |

What happens in each stage:

- **Regular season.** `SimDay` plays that day's games with `SimGame`, updates records, point differences and season stats, rolls injuries (`afterGame`), computes awards on the last day, and may create a trade proposal from a computer team (`updateProposals`). The trade window closes after 50 days played (`TradeDeadlineDay`). Multi-day sims run by the server stop for the deadline and for new offers unless the options `NoDeadlineStop` or `NoOfferStop` are on.
- **Playoffs.** `StartPlayoffs` builds the bracket from conference standings. Each playoff day plays the next game of every active series; the home pattern is `HHAAHAH`. The champion, finals MVP and the season record are set in `finishPlayoffs`.
- **Offseason report and draft setup.** `BeginOffseason` records career rows and the season record, records the luxury tax, ages and progresses every player, retires some, uses up one contract year, builds the report (retirements, five biggest risers and fallers), generates the draft pool (or takes a stored real-data class), runs the lottery, and sets up scouting. The player may re-sign expiring players during `offseason`, `draft` and `draft_done`.
- **Draft.** Picks are made one at a time; the owner of each pick selects (picks can be traded). Rookies sign scale contracts automatically.
- **Free agency.** `BeginFreeAgency` lets computer teams re-sign who they want, trims them to 13 players, then releases every still-expiring player (the user's too) into the pool. Each `AdvanceFreeAgency` day lets computer teams bid and lets players choose among offers made before that day.
- **New season.** `FinishOffseason` plays out any remaining free-agency days, trims and fills rosters with minimum deals, rolls the pick window forward, clears most injuries, adds one to `Year`, and makes 0 to 2 trades between computer teams. The handler then calls `StartSeason` to build the new schedule.

Action ids and the route each calls (from `exec` in `web/src/flow.tsx`; the API functions are in `web/src/api.ts`):

| Action id | Phase it appears in | What the front end calls |
|---|---|---|
| `sim_day` | `regular` | `POST /api/sim` with `{days: 1}` |
| `sim_week` | `regular` | opens `SimOverlay` for 7 days (it calls `POST /api/sim` one day at a time with `auto: true`) |
| `sim_to_end` | `regular` | opens `SimOverlay` with `toEnd` |
| `start_playoffs` | `regular_done` | `POST /api/playoffs/start` |
| `playoff_day` | `playoffs` | `POST /api/playoffs/day` |
| `playoff_series` | `playoffs` | `POST /api/playoffs/series` |
| `playoff_round` | `playoffs` | `POST /api/playoffs/round` |
| `playoff_all` | `playoffs` | `POST /api/playoffs/all` |
| `playoff_watch` | `playoffs` | `POST /api/playoffs/watch`, then goes to `/game/{boxId}` |
| `begin_offseason` | `playoffs_done` | `POST /api/offseason/begin` |
| `go_draft` | `offseason` | `POST /api/offseason/to-draft`, then goes to `/draft` |
| `make_pick`, `draft_run` | `draft` | only navigates to `/draft`; the Draft page itself calls `POST /api/draft/pick` and `POST /api/draft/step` |
| `draft_auto` | `draft` | `POST /api/draft/auto` |
| `begin_free_agency` | `draft_done` | `POST /api/freeagents/begin`, then goes to `/free-agency` (asks for confirmation first if players are still expiring) |
| `fa_day` | `free_agency` | `POST /api/freeagents/day` |
| `start_season` | `free_agency` | `POST /api/offseason/finish` (asks first if roster spots are open) |

After any action the front end stores the returned state under the react-query key `['state']`, refreshes every other query, and navigates to the home route of the new phase (`homeRoute` in `flow.tsx`) if the phase changed.

## 5. Engine map

All files are in `engine/`. "Tunable" names the config block that holds the numbers for that area. Only the main exported names are listed.

### League and players

| File | Owns | Main exports | Tunable |
|---|---|---|---|
| `player.go` | The `Player` type, positions, ratings, overall maths | `Player`, `Ratings`, `Position` (`PG`..`C`), `Overall`, `OverallAt`, `Effective` (ratings shifted by hidden `Impact`), `TrueOverall`, `ShotUsage`, `ScoringRole`, `CanPlay`, `PosLabel` | `overallWeights` (per-position weights) |
| `generate.go` | Teams, the `League` type, new fictional league | `League`, `Team`, `GenerateLeague`, `GeneratePlayer`, `Team.Rotation` (auto starters by position, then bench), `RosterSize` (13) | uses `Progression` |
| `names.go` | Name lists | (unexported lists) | none |
| `rng.go` | The only random source | `RNG`, `NewRNG`, `Between`, `Normal`, `Chance`, `Pick`, `PickWeighted` | none |
| `phase.go` | League stages and guards | `Phase`, `Action`, `League.Phase`, `NextActions`, `Can...` guards, `PlayoffSeeds` (8), `UserPlayoffResult` | none |
| `commissioner.go` | Rating keys and the cheat-style player editor | `RatingKeys`, `GetRating`, `SetRating`, `EditPlayer`, `PlayerEdit`, `MinRating`/`MaxRating` (25/99), `MinAge`/`MaxAge` (18/45) | none |
| `history.go` | Career rows, season records, stored box scores | `CareerRow`, `SeasonRecord`, `Box`, `FindPlayer` | none |

### Games and season

| File | Owns | Main exports | Tunable |
|---|---|---|---|
| `game.go` | The possession model for one game | `SimGame`, `SimGameRec` (adds a play-by-play log without changing the random stream), `GameResult`, `StatLine`, `Event` | constants at the top (`baseTov`, `foulChance`, `shootExp`, ...) and `minutesByRank` |
| `season.go` | Schedule, standings, stats, awards | `Season`, `Game`, `PlayerSeason`, `StartSeason`, `SimDay`, `Standings`, `ConferenceStandings`, `Leaders`, `Conference` (teams 0-14 West, 15-29 East), `NumTeams`, `SeasonGames` | `playoffOffset`, `offseasonOffset`, `scheduleOffset` |
| `playoffs.go` | Bracket and series | `Series`, `PGame`, `StartPlayoffs`, `SimPlayoffDay`, `SimUserSeries`, `SimRound`, `SimAllPlayoffs`, `RoundName` | `homePattern` |
| `injury.go` | Injuries and the injury report | `Injury`, `InjuryLine`, `InjuryReport`, `NewInjuriesLastDay` | `InjuryTuning` |
| `lineup.go` | Custom starting five and minutes | `Lineup`, `SetLineup`, `ClearLineup`, `AutoLineup`, `Team.EffectiveMinutes` | constants `TeamMinutes` (240), `MaxPlayerMinutes` (42), `MinLineupPlayers` (8) |

### Trades

| File | Owns | Main exports | Tunable |
|---|---|---|---|
| `trade.go` | Player valuation, contend factors, the window, the two-team path, trades between computer teams | `TradeWindowOpen`, `TradeValue`, `TradeBlock`, `TradeOffer`, `EvaluateTrade`, `SuggestCounter`, `ExecuteTrade`, `SimAITrades`, `TradeResult`, `TradeDeadlineDay` | constants at the top (`TradeMargin`, `UntouchableMargin`, `MaxTradePlayers`, `tradeOffset`) |
| `deal.go` | Multi-asset deals (players and picks, 2 or 3 teams) | `Asset`, `Move`, `Deal`, `ValidateDeal`, `EvaluateDeal`, `ExecuteDeal`, `PickValueFor`, `PickValue` | pick-value constants at the top (`pickR1Late`, `pickYearDiscount`, ...), `MaxDealTeams`, `MaxDealPicks` |
| `negotiate.go` | Tiers, asks, proposals from computer teams, user options, deadline pause | `PlayerTier`, `PickTier`, `Options`, `Proposal`, `PendingProposals`, `AcceptProposal`, `RejectProposal`, `ShouldPauseForDeadline` | constants (`CloseFraction`, `ProposalChancePerDay`, `MaxPendingProposals`, `ProposalLifeDays`, `proposalOffset`) |
| `picks.go` | Ownership of draft picks | `DraftPick`, `PicksOwnedBy`, `PickOwner`, `TransferPick`, `PickYearsAhead` (4) | none |

### Contracts and free agency

| File | Owns | Main exports | Tunable |
|---|---|---|---|
| `contract.go` | Contracts, cap rules, payroll, re-signing, release | `Cap`, `Contract`, `MarketSalary`, `RookieContract`, `Payroll`, `CapRoom`, `TaxBill`, `CanSign`, `Sign`, `SignKind`, `AskingContract`, `ReSign`, `Release`, `Expiring` | `Cap` |
| `freeagency.go` | The free-agency period | `FreeAgency`, `FAOffer`, `FASigning`, `BeginFreeAgency`, `AdvanceFreeAgency`, `MakeOffer`, `WithdrawOffer`, `OffersBy`, `FreeAgentAsk` | `FATuning`, `freeAgencyOffset` |

### Draft

| File | Owns | Main exports | Tunable |
|---|---|---|---|
| `offseason.go` | Offseason report, draft state, finishing the offseason | `OffseasonReport`, `Draft`, `BeginOffseason`, `DraftPick`, `DraftOnClock`, `DraftDone`, `DraftAutoUntilUser`, `DraftAutoAll`, `FinishOffseason`, `Offseason` (runs the whole offseason with no screens; used by tests and `cmd/sim`) | uses `Progression` (`DraftSize` 60) |
| `lottery.go` | Draft lottery | `LotteryOdds`, `LotteryRow`, `LotteryDrawn` (4) | `LotteryOdds`, `lotteryOffset` |
| `scouting.go` | What the user may see of a prospect | `ScoutView`, `ScoutRatings`, `Scout`, `ScoutPointsLeft`, `MaxScoutLevel` (3) | `ScoutTuning`, `ComputerDraftNoise`, `scoutOffset` |

### Offseason and progression

| File | Owns | Main exports | Tunable |
|---|---|---|---|
| `progression.go` | Yearly development, retirement, rookie generation | `Player.Progress`, `ProgressMinutes`, `RetireChance`, `TrueCeiling`, `MinutesAdjust`, `GenerateRookie` | `Progression` (also roster and draft sizes, `FreeAgentPool`) |

### Saving

| File | Owns | Main exports | Tunable |
|---|---|---|---|
| `save.go` | JSON save and load with version upgrades | `Save`, `Load`, `SaveVersion`, `SavePath`, `saveUpgrades` | none |

### Real data

| File | Owns | Main exports | Tunable |
|---|---|---|---|
| `realdata.go` | Reads the CSVs, turns stats into ratings, builds the real league and its draft classes, helpers for `cmd/realcheck` | `NewRealLeague`, `RatingsFromStats`, `RealPerGameStats`, `RealTeamWins`, `RealLeaguePPG` | `RealTuning` |
| `realteams.go` | The 30 real franchises (names, abbreviations) | `RealTeamAbbrevs` | none |

## 6. The data model

The whole game is one `League` value (`engine/generate.go`). Engine structs have no `json:` tags except `League.Version`, so the save file uses the Go field names. `omitempty` fields are left out of the save when empty.

### League

| Field | Meaning |
|---|---|
| `Version` | Save format number (see section 10) |
| `Seed` | The league's random seed |
| `Year` | Current league year. During an offseason it is still the year of the season just played; `FinishOffseason` adds one |
| `NextID` | Next unused player ID (player ID 0 is a real player) |
| `Teams` | 30 teams; the index equals `Team.ID` |
| `FreeAgents` | The free-agent pool |
| `Season` | The current `Season` |
| `UserTeamID` | The team the player runs |
| `Retired` | Retired players, kept so their pages still work |
| `Careers` | `map[playerID][]CareerRow`, one row per finished season |
| `Records` | One `SeasonRecord` per finished season (champion, awards, bracket) |
| `LastBox` | The user's most recent game |
| `Draft` | Non-nil from `BeginOffseason` until `FinishOffseason` (so it is also non-nil during free agency) |
| `Report` | The current offseason report |
| `FA` | Non-nil only during free agency |
| `DraftClasses` | Real-data mode: stored future rookies by year |
| `Picks` | Every tradable pick in the next four draft years |
| `Transactions` | Log of executed trades, oldest first |
| `Proposals`, `NextProposalID`, `DeadlineStopYear` | Computer-team trade offers waiting for the user, their ID counter, and the season in which the deadline pause already fired |
| `Options` | User switches (see below) |

Two unexported fields, `newProposals` and `newInjuries`, hold what the most recent day produced; they are not saved.

### Team

`ID`, `City`, `Name`, `Roster` (up to 13 `*Player`), `TaxPaid` (luxury tax by season year), and `Lineup` (nil means the automatic rotation).

### Player

| Field | Meaning |
|---|---|
| `ID`, `Name`, `Age`, `TeamID` | Identity; `TeamID` is -1 for free agents |
| `Pos`, `AltPos` | Primary position and other positions he can play |
| `Ratings` | Seven visible ratings, 25 to 99: `Inside`, `Mid`, `Three`, `Playmaking`, `Rebounding`, `Defense`, `Athleticism` |
| `Potential` | The overall he can grow into (visible to the user, except for prospects) |
| `UsageTendency` | How often he looks for his own shot; zero means "derive it from ratings" (`ShotUsage`) |
| `Experience`, `DraftYear`, `DraftPick` | Career facts; `DraftPick` 0 means undrafted |
| `RealID` | Real-data player id; empty in fictional leagues (the server uses it to tell which mode a league is in) |
| `Impact` | The hidden number, roughly -8 to +8. Used by `Effective` in the game sim and by progression. Never sent to the browser |
| `Contract` | `Contract{Salary, Years}`; `Years` counts seasons left including the current one, 0 means none or expiring in the offseason |
| `Injury` | `*Injury{Name, GamesLeft, Serious}`; nil means healthy |

### Lineup

`Lineup{Starters [5]int, Minutes map[int]int}`: starter player IDs for slots PG, SG, SF, PF, C, and target minutes per game by player ID. `NoPlayer` (-1) marks an empty starter slot. `Team.pruneLineup` drops departed players when saving and when computing minutes.

### Season (also `Game`, `Series`, `PlayerSeason`, `Box`)

`Season` holds `Year`, `Games` (ordered by day, 15 per day, each `Game{Day, Home, Away, Played, HomeScore, AwayScore}`), `Day` (next day to play, 0-based), `Wins`, `Losses`, `PointDiff` (indexed by team), `Stats` (`map[playerID]*PlayerSeason`), `Champion` (-1 until decided), `Awards`. Playoff-only fields: `PlayoffsStarted`, `PlayoffDay` (days played, also the random stream number), `PlayoffGames`, `Series` (flat list of `Series`), `PlayoffStats`, `PlayoffBoxes` (every playoff game with play-by-play events). Regular-season box scores are kept only for the user's team (`LastBox`).

### Draft, DraftPick, OffseasonReport

`DraftPick{Year, Round, Original, Owner}` is one tradable pick: `Original` decides the slot, `Owner` makes the choice. `Year` is the league year during whose offseason the draft happens.

`Draft` exists only between `BeginOffseason` and `FinishOffseason`: `Pool` (the prospects with their true values), `Order` (round-1 original teams after the lottery), `Order2` (round 2, worst record first), `Made`, `Total`, `Results`, `Lottery` (`[]LotteryRow`), and scouting state `OvrErr`, `PotErr`, `ScoutPoints`, `ScoutLevel`. The scouting state and the true values of prospects are saved but never sent to the browser. `OffseasonReport` (`Retirements`, `Draft` picks as made, `Risers`, `Fallers`) lives in `League.Report`.

### FreeAgency

`FreeAgency{Day, Days, NextOfferID, Offers, Log}` exists only during free agency. `Day` is days already resolved; `Days` is 10. `FAOffer{ID, Team, PlayerID, Salary, Years, Day}` is a standing offer; `FASigning` is a log row.

### Proposal and trade records

`Proposal{ID, Team, Deal, Day, Expires, Message}` is an offer from a computer team, present only while pending. `TradeResult` (kept in `Transactions`) lists moved, released and signed players, and the picks that changed hands (`Picks` and `Teams` are empty for old two-team trades).

### Options

`Options{NoDeadlineStop, NoOfferStop, NoInjuries, Commissioner}`. Each flag is named so that `false` (the zero value) is the old default behaviour. `Commissioner` allows `EditPlayer` (never for prospects, and never touches `Impact`).

### What the browser never receives

`Player.Impact`, `TrueOverall`, `Draft.OvrErr`, `Draft.PotErr`, true overall, potential and ratings of undrafted prospects (the API sends ranges from `ScoutView` and `ScoutRatings` instead), and the raw engine structs themselves. Responses are built by hand from maps in `cmd/web`; the test `TestNoHiddenNumber` and the helper `checkNoImpact` in `cmd/web/api_test.go` guard this.

## 7. Trades in one page

**Assets.** `Asset` is either a player (`Kind: PlayerAsset`, `PlayerID`) or a draft pick (`Kind: PickAsset`, `Year`, `Round`, `Original`); build them with `PlayerOf` and `PickOf`. A `Move` sends one asset from one team to another. A `Deal` is a list of moves among 2 or 3 teams (`MaxDealTeams`).

**Validation** (`ValidateDeal`): the trade window must be open; the deal is not empty; teams and assets exist and the sender really owns each asset; nothing is listed twice; every team both sends and receives something; the user's team is in the deal; each team sends at most 3 players (`MaxTradePlayers`) and 4 picks (`MaxDealPicks`); rosters can be refilled (a team left short signs free agents, a team left over releases players); and the salary rules below hold. The window (`TradeWindowOpen`) is open in the regular season while at most 50 days have been played (`TradeDeadlineDay`); it is closed after that, during the playoffs and the offseason report, and while a draft exists (which includes free agency). The code also has an "offseason window" state (`Season.Year < League.Year`), but through the web API `offseasonFinish` calls `StartSeason` straight after `FinishOffseason`, so in practice trading reopens on day 0 of the new season.

**Salary rules** (`salaryProblem`, in `contract.go`): after the trade a team must be at or under the hard ceiling. If it ends over the salary cap it may take back at most 125% of the salary it sends out plus $250K (`TradeMatchPct`, `TradeMatchExtra`). Contracts that have run out count as zero.

**How a computer team judges its side.** `contendFactors` gives each team a number from 0 (rebuilding) to 1 (contending), from the rank of its top-eight average overall, blended with its record after 20 games. `tradeValue` turns a player into a number: it rises steeply with overall, adds some of a young player's potential (more for rebuilding teams), falls for age from 30, falls a little for old players on rebuilding teams, gets a small bonus for filling a thin position, and is scaled by `contractAdjust` (a bargain contract up to +30%, an overpay down to -30%). Each team judges only its own side: value coming in must be at least `TradeMargin` (5%) above value going out, and 25% above if it gives up one of its two most valuable players ("untouchables"). Giving up a player that leaves fewer than two at a position adds an 8% penalty. Picks are valued by `PickValueFor`: a first-round value of 8 up to 32 depending on how weak the original team looks (cubed curve), a second-round value of 1 to 2.5, each further year away worth 88% of the one before, and the valuer's contend factor tilts it (rebuilders value picks 1.25 times, contenders 0.75).

**Tiers.** `PlayerTier` labels a player from visible data only: Franchise (overall 90+), Star (77+), Starter (66+), Prospect (age 22 or younger, under 62 now, potential at least 10 higher and 68 or more), Rotation (57+), Bench. `PickTier` labels a pick as Lottery pick, Mid first, Late first or Second-rounder from the original team's expected slot.

**Asks.** When a team refuses, `negotiate` sets a mood (`accept`, `close` at 85% or more of what it needs, `far`, or `no` for a refused core player) and `findAsk` looks for the cheapest single extra asset from the user's side that would make it accept, producing a plain sentence ("Add ... and we have a deal."). If no single piece works it says roughly how much more is needed.

**Proposals from computer teams.** After each regular-season day, if fewer than 2 are pending and the window is open before the deadline, there is a 1-in-8 chance (random stream offset 20000 plus the day) that one computer team, tried in random order, builds an offer: contenders target the user's best players (overall 58 or more), rebuilding teams target young players with potential (age 25 or less, potential 64 or more). `findPackage` looks for at most one player and three picks that the proposer accepts and that land within 90% to 115% of what the user gives, by the user's valuation. A proposal lives 6 days (`ProposalLifeDays`). `AcceptProposal` re-validates and executes it; `RejectProposal` removes it.

**Between computer teams.** `SimAITrades` runs once per offseason at the end of `FinishOffseason`. It makes 0 to 2 one-for-one trades where both teams gain by their own valuation and both pass salary rules, never involving the user's team.

**The older two-team path.** `TradeOffer`, `EvaluateTrade`, `SuggestCounter` and `ExecuteTrade` handle a plain swap of 1 to 3 players each between the user and one computer team, with no picks. The routes `/api/trade/evaluate` and `/api/trade/execute` still exist for it, and the current Trade screen calls only `/api/trade/counter` from this path (for "suggest a counter" on a two-team player swap); evaluating and executing go through `/api/deal/evaluate` and `/api/deal/execute`.

## 8. Money in one page

All numbers are in the `Cap` block in `engine/contract.go`, in thousands of dollars per season.

| `Cap` field | Value | Meaning |
|---|---|---|
| `SalaryCap` | 140000 | Soft cap: cap-room signings and trades must fit under it |
| `TaxLine` | 170000 | Payroll above this pays luxury tax |
| `HardCeiling` | 190000 | Payroll may never go above this (draft picks excepted) |
| `MinSalary` / `MaxSalary` | 1200 / 49000 | Smallest and largest contract |
| `TaxRate` | 1.5 | Tax dollars per dollar over the tax line (`TaxBill`) |
| `TradeMatchPct`, `TradeMatchExtra` | 125, 250 | Over-the-cap trade matching |
| `RookieTop`, `RookieLastFirst` | 10000, 2200 | First-round scale, sliding by pick; second-round picks get the minimum |
| `RookieYearsRound1/2`, `MaxContractYears` | 4, 2, 4 | Contract lengths |
| `Market...` fields | various | The market salary curve (`MarketSalary`) |
| `ContractSwing`, `ContractFullYears`, `ExpiringRebuildCut` | 0.30, 4, 0.08 | How contracts change trade value |
| `ReSignMinValue` | 4.5 | A computer team re-signs an expiring player only if his value to it is at least this |

`Payroll(team)` is the sum of contracts with years left. `MarketSalary` depends only on visible overall and age (with a bonus for young players whose potential is well above their overall, and a discount from age 31).

**What is allowed over the cap.** `CanSign` checks a signing by kind (`SignKind`): `SignCapRoom` needs payroll plus the salary to fit under the cap; `SignOwn` (re-signing your own expiring player, "Bird rights") ignores the cap; `SignMinimum` must be exactly the minimum salary; `SignDraft` always works. Every kind except `SignDraft` is still stopped by the hard ceiling. Trades follow the matching rule in section 7.

**Re-signing.** After `BeginOffseason`, a player whose `Contract.Years` has dropped to 0 is "expiring" and stays on his roster. `ReSign` signs him at `AskingContract` (his market salary now; 4 years if 25 or younger, 3 if 28 or younger, 2 if 31 or younger, else 1). It is refused once free agency has begun. Computer teams do this in `reSignComputerTeams` when free agency begins, keeping players worth their ask while staying under the tax line.

**Release.** `Release` sends a player to free agency and deletes the contract with no dead money; it is allowed only in the offseason.

**Free agency days.** `BeginFreeAgency` opens a 10-day period (`FATuning.Days`). A free agent's asking salary starts at his `AskingContract` and softens linearly to 70% by the last day (`soften`, never below the minimum). The user places standing offers with `MakeOffer` (replaces his earlier offer for that player, needs a roster spot, must respect cap room after his other pending offers, and can only be accepted from the next day). Rival bids are hidden; only a count of offers is shown. Each `AdvanceFreeAgency` day, computer teams with open spots make up to 2 offers (plus a chance of a second) at 92% to 112% of the ask, minimum offers start on day 5, and a full team may bid for a clear upgrade and cut its worst player if it signs him. Then `playersDecide` runs for each free agent with offers made before today: he considers offers at or above 95% of his current ask (late in the period, 50% of his original ask), ranks them by an appeal score (salary, a small bonus per contract year, team quality, and a bonus if he would be among the team's best eight), and signs the best one that passes the cap rules. After the last day, players whose original ask was at least four times the minimum take the best real offer they have. Anyone left unsigned is a free agent when the season starts, and short rosters are filled with minimum deals by `fillRosters`.

## 9. Randomness and determinism

All engine randomness goes through `engine.RNG` (`engine/rng.go`, a wrapper over Go's `math/rand/v2` PCG generator). The same seed gives the same league and results.

A league is made from `NewRNG(seed)`. Everything later draws from a stream derived for one purpose: `lg.dayRNG(year, offset)` returns `NewRNG(lg.Seed*1000003 + year*10007 + offset)`. Regular-season day `d` uses offset `d`. A new feature gets its own offset so it cannot change existing results for a seed; recording a play-by-play log and turning injuries off also never change the game stream.

Streams in the code today:

| Offset | Used by | Where |
|---|---|---|
| 0 to 81 | Games of regular-season day `d` | `SimDay` |
| 1000 + playoff day | Playoff games | `SimPlayoffDay` |
| 5000 | Progression, retirement and rookie generation, in that order, in one stream | `BeginOffseason` |
| 5007 | Padding a stored real-data draft class with generated rookies | `takeDraftClass` |
| 7000 | Computer-to-computer trades (built with `NewRNG` and the same formula, using the already advanced `Year`) | `FinishOffseason` |
| 9000 | Schedule | `StartSeason` |
| 20000 + day | Trade proposals from computer teams | `updateProposals` |
| 30000 + day | Injuries in the regular season (playoffs use 30500 + playoff day) | `SimDay`, `SimPlayoffDay` |
| 40000 | Contract lengths at league creation | `assignInitialContracts` |
| 50000 + day | Free-agency bidding | `AdvanceFreeAgency` |
| 60000 | Draft lottery | `BeginOffseason` |
| 70000 | Per-prospect scouting errors, drawn once in pool order | `initScouting` |
| 70001, 70003 | Hashed streams (no draw order): computer team draft noise per team and prospect (70001), per-rating scouting errors (70003) | `computerDraftScore`, `ScoutRatings` |
| 70004 | Hashed stream used only to repair an old save's missing scouting errors | `ensureDraft` |

The hashed streams (`mix64`) are built from the seed, year, offset and ids, so they do not depend on the order in which picks are made or screens are opened. `MidSeasonTradeStep` (7500) is defined in `trade.go` as a suggested offset for a mid-season AI trade call, but no code uses it.

## 10. Saving and loading

`League.Save(path)` stamps `Version = SaveVersion`, prunes each team's lineup, marshals the whole league with `encoding/json`, creates the folder if needed, writes `path + ".tmp"` and renames it over the target (so a crash does not leave a half-written save). There is one save file; the web server uses `saves/league.json` unless `-save` says otherwise.

`SaveVersion` is currently 3. `saveUpgrades[i]` upgrades a league from version `i` to `i+1`:

| Step | What it does |
|---|---|
| 0 to 1 | Saves from before versioning: makes sure `Careers` exists |
| 1 to 2 | Draft picks became owned objects: if `Picks` is empty, gives every team its own picks (`initPicks`) |
| 2 to 3 | Contracts and the cap arrived: if nobody has a contract, assigns market-salary contracts (`assignInitialContracts`) |

`Load` reads the file and then:

- A save with no version field counts as version 0 and goes through every step.
- A version higher than `SaveVersion` is refused with a message that it was made by a newer build; nothing is half-loaded.
- A negative version is refused.
- Otherwise it applies the steps from the file's version up to the current one and sets `Version`.

Fields added after version 3 were designed so their zero value means the old behaviour (injuries, lineup, options, scouting, usage tendency, proposals), so they need no upgrade step. The server's `reportSeen` flag is not saved; after loading it is true, so a save made while the offseason report was unseen resumes at the draft.

**Lazy repair of an old mid-draft save.** A draft saved by an older build may lack the two-round order and the scouting state. `League.ensureDraft` (in `scouting.go`) repairs this the first time the draft is used (`DraftOnClock`, `DraftPick`, `ScoutView`, `ScoutRatings`, `Scout`, `ScoutPointsLeft` all call it): if `Order2` has a different length from `Order` it copies `Order`; if the scouting errors are missing it draws them from hashed per-prospect streams and gives the user the scouting points; and it makes sure `ScoutLevel` exists. It does nothing for a draft made by the current build.

## 11. API reference

All routes are registered in `routes()` in `cmd/web/main.go`. "Needs league" routes answer 409 `no league loaded` before a game is started or loaded. Every other path under `/api/` answers 404; anything else is served by `static`. File names below are in `cmd/web/`.

| Method | Path | What it does | Handler (file) |
|---|---|---|---|
| GET | `/api/meta` | Is a league loaded, is real data available, does a save exist, team lists for both modes | `meta` (main.go) |
| POST | `/api/new` | Start a game: body `{mode, seed, team}`; `mode` is `fictional` or `real` | `newGame` (main.go) |
| POST | `/api/load` | Load the save file | `load` (main.go) |
| POST | `/api/save` | Write the save file | `save` (main.go) |
| GET | `/api/state` | Phase, day, record, next game, last result, payroll summary, `nextAction` and `actions` | `state` (state.go) |
| POST | `/api/sim` | Sim days: `{days, toEnd, auto}`; returns day results, standings strip, new state, optional `pause` | `sim` (state.go) |
| GET | `/api/standings` | Conference standings | `standings` (league.go) |
| GET | `/api/leaders` | Top 10 in points, rebounds, assists | `leaders` (league.go) |
| GET | `/api/team/{id}` | A team's roster, record, payroll | `teamPage` (league.go) |
| GET | `/api/player/{id}` | A player's page (prospects get ranges only) | `player` (league.go) |
| GET | `/api/schedule` | A team's games; `?team=` (default user) | `schedule` (league.go) |
| GET | `/api/box/{id}` | A box score (`last` or a playoff game id) | `box` (league.go) |
| GET | `/api/game/{id}/events` | Play-by-play events for the replay | `gameEvents` (league.go) |
| GET | `/api/history` | Finished seasons, newest first | `history` (league.go) |
| GET | `/api/playoffs` | The bracket | `playoffs` (flow.go) |
| POST | `/api/playoffs/{action}` | `start`, `day`, `series`, `round`, `all`, `watch` | `playoffAction` (flow.go) |
| GET | `/api/offseason` | The offseason report | `offseasonReport` (flow.go) |
| POST | `/api/offseason/begin` | Run `BeginOffseason`; sets `reportSeen` false | `offseasonBegin` (flow.go) |
| POST | `/api/offseason/to-draft` | Marks the report seen | `offseasonToDraft` (flow.go) |
| POST | `/api/offseason/finish` | Finish the offseason and start the next season (opens free agency first if it was never opened) | `offseasonFinish` (flow.go) |
| GET | `/api/draft` | Picks, prospects (ranges), lottery, scouting points | `draftState` (flow.go) |
| POST | `/api/draft/pick` | The user's pick: `{playerId}` | `draftPick` (flow.go) |
| POST | `/api/draft/scout` | Spend a scouting point: `{playerId}` | `draftScout` (scouting.go) |
| POST | `/api/draft/step` | Computer picks up to `{max}`, stopping when the user is on the clock | `draftStep` (flow.go) |
| POST | `/api/draft/auto` | Auto-draft all remaining picks | `draftAuto` (flow.go) |
| GET | `/api/trade/status` | Is the window open, deadline day, limits | `tradeStatus` (trade.go) |
| GET | `/api/trade/block/{id}` | A team's players and picks with values and tiers | `tradeBlock` (trade.go) |
| POST | `/api/trade/evaluate` | Two-team swap verdict (older path) | `tradeEvaluate` (trade.go) |
| POST | `/api/trade/counter` | Suggest a counter to a refused two-team swap | `tradeCounter` (trade.go) |
| POST | `/api/trade/execute` | Execute a two-team swap (older path) | `tradeExecute` (trade.go) |
| POST | `/api/deal/evaluate` | Verdict on a multi-asset deal: `{moves}` | `dealEvaluate` (trade.go) |
| POST | `/api/deal/execute` | Execute a deal if every computer team accepts | `dealExecute` (trade.go) |
| GET | `/api/transactions` | Trade log, newest first; `?limit=` (default 100) | `transactions` (trade.go) |
| GET | `/api/offers` | Pending proposals from computer teams | `offersList` (offers.go) |
| POST | `/api/offers/{id}/accept` | Accept a proposal | `offerAccept` (offers.go) |
| POST | `/api/offers/{id}/reject` | Reject a proposal | `offerReject` (offers.go) |
| GET | `/api/options` | The four option switches | `optionsGet` (offers.go) |
| POST | `/api/options` | Change any of them (only keys sent are changed) | `optionsSet` (offers.go) |
| GET | `/api/lineup` | The user's lineup and minutes | `lineupGet` (lineup.go) |
| POST | `/api/lineup` | Set starters and minutes | `lineupSet` (lineup.go) |
| POST | `/api/lineup/auto` | Clear the custom lineup | `lineupAuto` (lineup.go) |
| GET | `/api/injuries` | Injury report; `?team=` an id or `all` (default user) | `injuries` (admin.go) |
| GET | `/api/contracts/expiring` | The user's expiring players and their asks | `expiring` (contracts.go) |
| POST | `/api/contracts/resign` | Re-sign `{playerId}` at his ask | closure over `League.ReSign` (main.go), `contractAction` (contracts.go) |
| POST | `/api/contracts/release` | Release `{playerId}` | closure over `League.Release` (main.go), `contractAction` (contracts.go) |
| GET | `/api/freeagents` | Pool, the user's offers, cap numbers, day | `freeAgents` (freeagency.go) |
| POST | `/api/freeagents/begin` | Open free agency | `faBegin` (freeagency.go) |
| POST | `/api/freeagents/offer` | Make an offer `{playerId, salary, years}` | `faOffer` (freeagency.go) |
| POST | `/api/freeagents/withdraw` | Withdraw `{offerId}` | `faWithdraw` (freeagency.go) |
| POST | `/api/freeagents/day` | Advance one day; returns signings | `faAdvance` (freeagency.go) |
| POST | `/api/admin/player/{id}` | Commissioner edit (age, potential, ratings, heal); refused unless the option is on | `adminPlayer` (admin.go) |

**Response conventions.**

- Responses are plain maps (`type M = map[string]any` in `main.go`), built by hand. The `api()` wrapper takes the one server lock, checks a league is loaded when asked, and turns errors into JSON.
- Money is an integer in thousands of dollars. The front end formats it with `money()` in `web/src/ui.tsx`.
- Days are 1-based in the API (`"Day 1"` is engine day 0; free-agency `day` is `min(fa.Day+1, fa.Days)`). The engine counts from 0.
- Errors are `{"error": "sentence"}` with 400 (bad request), 404 (does not exist), 409 (not allowed right now) or 500 (unexpected). Helpers: `bad`, `missing`, `conflict`.
- Team objects always come from `server.team(id)` (id, abbr, city, name, fullName, primary and secondary colours, conference); player rows come from shared row helpers (`playerRow`, `prospectRows`, `addContract`, `addInjury`) so every screen shows the same fields.
- Action routes usually return the new `state` object so the front end can update at once.
- Request bodies are limited to 1 MB and decoded by `readJSON`.

## 12. Front end map

The app is React 19 with TypeScript, built with Vite, styled with Tailwind v4, using `react-router-dom` for routes, `@tanstack/react-query` for server data, `@tanstack/react-table` for tables, `motion` for animation, and `three` with `@react-three/fiber` for the 3D court. Source is in `web/src/`.

**Routing** (`App.tsx`). `App` first loads `['meta']`. If no league is loaded it shows the `Start` page (new game or load). Otherwise it wraps `Shell` in `FlowProvider` and routes:

| Path | Page file | Purpose |
|---|---|---|
| `/` | `pages/Dashboard.tsx` | Home: next game, last result, standings snapshot, offers, injuries, the continue button |
| `/team/:id` | `pages/TeamPage.tsx` | A team's roster and contracts (`/roster` redirects to the user's team) |
| `/lineup` | `pages/Lineup.tsx` | Starters and minutes |
| `/trade` | `pages/Trade.tsx` | Build and evaluate deals, trade block, transactions |
| `/standings` | `pages/Standings.tsx` | Conference standings |
| `/leaders` | `pages/Leaders.tsx` | Statistical leaders |
| `/schedule`, `/schedule/:id` | `pages/Schedule.tsx` | A team's schedule and results |
| `/box/:id` | `pages/BoxScore.tsx` | Box score |
| `/playoffs` | `pages/Playoffs.tsx` | Bracket |
| `/game/:id` | `pages/GameViewer.tsx` | Replay of a recorded game (2D, with a lazy 3D court) |
| `/offseason` | `pages/Offseason.tsx` | Offseason report (retirements, risers, fallers, AI trades) |
| `/draft` | `pages/Draft.tsx` | Lottery, prospect list with scouting ranges, picks |
| `/free-agency` | `pages/FreeAgency.tsx` | Free-agent pool, offers, daily signings |
| `/history` | `pages/History.tsx` | Past seasons |
| `/settings` | `pages/Settings.tsx` | Options and commissioner switch |

Unknown paths show the Dashboard. A player's detail opens as a side drawer driven by the `?player=<id>` URL parameter (`PlayerDrawer.tsx`; `useOpenPlayer` in `Inline.tsx` sets it), so any page can open one.

**Shared pieces.**

- `api.ts`: one typed `api` object with a function per route, plus hand-written TypeScript types for every response (`GameState`, `Phase`, `DraftData`, ...). It is a hand-kept copy of the server's maps.
- `ui.tsx`: small components and formatters used everywhere: `Card`, `Page`, `DataTable`, `Q` (renders a query's loading and error states), `TeamBadge`, `TeamLink`, `PlayerLink`, `TierChip`, `InjuryTag`, `RangeBar` and `rangeText` (scouting ranges), `PayrollBar`, `ConfirmDialog`, `money`, `ovrColor`.
- `flow.tsx`: `FlowProvider` and `useFlow()`. It owns the `['state']` query, the `run(id)` action runner from section 4, the busy flag, the action error, and the latest free-agency signings. It decides whether to ask for confirmation first.
- `SimOverlay.tsx`: the full-screen progress view for multi-day sims; it calls `POST /api/sim` one day at a time with `auto: true` so the server can pause for the deadline or a new offer.
- `TopBar.tsx`: navigation, the season stepper, the continue button, save. `App.tsx` also binds Space and Enter to the continue button.
- Cards used on several pages: `ExpiringCard.tsx` (re-sign or release), `InjuriesCard.tsx`, `OffersCard.tsx` (computer proposals), `TxItem.tsx` (one trade in a log).
- `scout.ts`: the `useScout` hook that spends a scouting point and updates the cache.

**Fetching and refreshing.** `main.tsx` creates one `QueryClient` with no retries, no refetch on window focus and a 5-second stale time. Query keys worth knowing: `['meta']`, `['state']` (the central game state), `['team', id]`, `['player', id]`, `['standings']`, `['draft']`, `['freeagents']`, `['lineup']`, `['playoffs']`, `['offers', ...]`, `['injuries', ...]`, `['contracts', 'expiring', ...]`, `['trade-status', ...]`, `['trade-block']`, `['transactions']`, `['options']`, `['history']`, `['box', id]`, `['events', id]`. After every run action `FlowProvider.apply` writes the new state into `['state']` and invalidates every other key; some keys also include state values (day, phase, offer count) so they refetch when those change.

**Replay and 3D court.** `replay.ts` is plain TypeScript with no React: it turns the list of play-by-play events from `/api/game/{id}/events` into a running score, clock and box score at any event index. `GameViewer.tsx` uses it for the 2D view, and `court3d/Court3D.tsx` (loaded lazily, so the large three.js code is its own chunk) draws the same replay state on a 3D court.

## 13. Tests

Two packages have tests; `cmd/sim`, `cmd/realcheck` and the whole of `web/` have none. The whole suite runs in about 35 to 45 seconds (`engine` about 33 seconds, `cmd/web` about 4 seconds when run just now, uncached). The real-data tests skip themselves when `data/real/` is missing. The API tests use a temporary save folder and a temporary web folder.

| File | What it covers |
|---|---|
| `engine/game_test.go` | Determinism, league-average game numbers, basic invariants |
| `engine/season_test.go` | Season basics, determinism, long-run stability, team balance |
| `engine/playoffs_test.go` | Playoff step-by-step equals all-at-once, saving mid-playoffs, recording does not change results, events rebuild the box score |
| `engine/phase_test.go` | The full phase cycle |
| `engine/history_test.go` | Save and load round trip, stepped draft and history |
| `engine/save_test.go` | Round trip, old save without a version, newer save refused, version 2 save gets contracts |
| `engine/picks_test.go` | Initial picks, draft order matches standings, traded picks select, pick window rolls, version 1 save gets picks |
| `engine/lottery_test.go` | Lottery order, frequencies, traded pick keeps its slot |
| `engine/scouting_test.go` | Scouting ranges, spending points, computer draft noise and determinism, old save repair |
| `engine/contract_test.go` | Market salary, initial contracts, signing rules, rookie scale, trade salary rules, contract value effect, offseason contracts, computer re-signing, a six-season run |
| `engine/freeagency_test.go` | Free-agency flow and offers, market, saving during free agency, stars sign |
| `engine/trade_test.go` | Accept and refuse, trade window, save and load, computer trades, tiers, values, deadline |
| `engine/deal_test.go` | Deal validation, pick-for-player, three-team deals, agreement with the two-team path, determinism |
| `engine/negotiate_test.go` | Asks, proposals generated, accept, reject and drop, proposals leave game results alone, deadline pause |
| `engine/injury_test.go` | Determinism with and without injuries, sitting and healing, rates, serious injuries, offseason clearing, saving |
| `engine/lineup_test.go` | Automatic minutes, custom lineup rules and effect, injured starter, departed player, saving, minutes-based development |
| `engine/commissioner_test.go` | Player editing |
| `engine/realdata_test.go` | Real league shape, determinism, potential blur (skipped without data) |
| `cmd/web/api_test.go` | The API end to end through the handlers: a full flow, hidden number never present, real conferences, trades and deals, negotiation, offers, injuries and commissioner, contracts, free agency, lineup, lottery and scouting, prospect secrecy, scouting points running out |

Not covered: the front end (no tests; the type check in `npm run build` is the only automatic check), `cmd/sim`, `cmd/realcheck`, and the real-data mode through the API beyond conferences. `RULES.md` asks for an end-to-end pass with a scratch server and headless browser on top of these tests.

## 14. Where to change what

| Task | Files to touch, in order |
|---|---|
| Add an option switch | `engine/negotiate.go` (`Options`, name it so `false` is the old behaviour) -> the engine code that reads it -> `cmd/web/offers.go` (`optionsGet`, `optionsSet`) -> `web/src/api.ts` (`Options`) -> `web/src/pages/Settings.tsx` -> a test in `cmd/web/api_test.go` |
| Add a saved field | The struct in `engine/` (zero value = old behaviour, `omitempty` if optional) -> only if an old save would be wrong, bump `SaveVersion` and append a step in `engine/save.go` with a test in `engine/save_test.go` -> expose it in a `cmd/web/` handler if needed -> `web/src/api.ts` |
| Add a route and a screen | Engine function in `engine/<feature>.go` with a test -> handler in `cmd/web/<feature>.go` -> register it in `routes()` in `cmd/web/main.go` -> type and function in `web/src/api.ts` -> `web/src/pages/<Feature>.tsx` -> route in `web/src/App.tsx` and a link in `TopBar.tsx` -> an API test |
| Add a league stage | `engine/phase.go` (constant, `Phase`, `NextActions`, a `Can...` guard) -> the engine step -> `cmd/web/` handler and route -> `web/src/api.ts` (`Phase` type) -> `web/src/flow.tsx` (`homeRoute`, a `case` in `exec`) and the step list in `TopBar.tsx` -> update `phase_test.go` |
| Tune salaries, the cap or free agency | `Cap` in `engine/contract.go`; `FATuning` in `engine/freeagency.go` |
| Tune injuries | `InjuryTuning` in `engine/injury.go` |
| Tune player development, rookies, retirement, roster and draft size | `Progression` in `engine/progression.go` |
| Tune scoring in games | Constants at the top of `engine/game.go` |
| Tune the stats-to-ratings formula for real data | `RealTuning` in `engine/realdata.go` |
| Tune scouting or the lottery | `ScoutTuning` in `engine/scouting.go`; `LotteryOdds` in `engine/lottery.go` |
| Tune trade values and proposals | Constants at the top of `engine/trade.go`, `engine/deal.go`, `engine/negotiate.go` |
| Add a new random stream | A new offset constant in its engine file, `lg.dayRNG(year, offset+n)`, then record it in the table in `RULES.md` and in section 9 here |
| Add a field to a player row on every screen | The row helper in `cmd/web/` (for example `addContract`, `playerRow`) -> `web/src/api.ts` -> the page |

## 15. Known structural weak points

- **Untyped API, hand-kept types.** Handlers build `map[string]any` responses and `web/src/api.ts` holds hand-written copies of their shape. Nothing checks that they agree; a renamed key fails only at run time.
- **One lock around everything.** `server.mu` is held for the whole of every request, so a long action (simulating a season) blocks all other requests, and only one league can exist in the server.
- **A single save slot.** There is one save file and the server keeps one league; starting a new game replaces the one in memory, and saving overwrites the file.
- **`reportSeen` is not saved.** The phase also depends on a server-side flag outside the league. Loading always sets it to true, so a game saved on the unseen offseason report resumes at the draft.
- **Positions are not used in the game sim, but are used by the automatic rotation.** `newSide` in `game.go` weighs players by minutes and effective ratings only. `Team.Rotation` and `AutoLineup` do pick starters by position, and the overall formula is position-based, so position matters to who plays and how good a player looks, not to how a lineup plays together.
- **Engine trade tests halve salaries.** `tradeLeague` in `engine/trade_test.go` halves every salary so all teams have cap room; salary matching is covered separately in `contract_test.go` and `deal_test.go`, but the trade value tests do not run under realistic payrolls.
- **A compiled binary is tracked in git.** `cmd/web/web` (about 11 MB) is committed. `.gitignore` does not exclude it, and a plain `go build` inside `cmd/web` produces exactly that file, which is why `RULES.md` says to build with `-o` outside the repository.
- **Real data is detected indirectly.** `server.isReal()` looks at the first player on the first roster for a `RealID`; the mode is not stored on the league.
- **Prospect secrecy depends on each handler.** The draft pool is saved with true values and the API hides them handler by handler (`prospectRows`, `prospectPlayer`, `ScoutView`). A new screen that reads `Draft.Pool` directly would leak them; `RULES.md` rule 1.2 and `TestProspectSecrecy` exist for this.
- **Scattered phase logic.** Which trades, signings and releases are allowed is decided in several places by looking at `Draft`, `FA` and `Season` (`TradeWindowOpen`, `inOffseason`, `offseasonOpen` in `cmd/web`), not only through `Phase`. A new stage has to be checked against all of them.
- **Year means two things.** During an offseason `League.Year` is still the season just played, and `Season.Year < Year` is how several functions detect "offseason with no draft". The draft pick `Year` follows the same rule.
- **Old offsets are not in the rules table.** See section 9; a newcomer adding a stream should read the table here as well as `RULES.md`.
- **Player ID 0 is valid.** That is why an empty starter slot is `NoPlayer` (-1); code that treats 0 as "none" would be wrong.
