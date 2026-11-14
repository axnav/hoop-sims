# HoopSim handoff

Last updated: 2026-11-14. Module `hoopsim`, Go (standard library only). Everything below was built on that one day.

Legend: **[verified]** = checked by running or reading the code while writing this file. **[reported]** = taken from the owner's session notes (helpers' or earlier measurements), not re-run now.

---

## 1. What this project is and the goal

HoopSim (working title) is a text/web **basketball franchise simulation**. The owner enjoys franchise and career-simulation videos of big basketball games and wants to build only the simulation part: ratings, potential, progression, seasons, drafts. There are no on-court graphics.

- Chosen mode: **GM mode** (run one team). A single-player career mode is a later idea.
- Must be an **original game**: fictional league by default (30 fictional teams, generated players). Do not copy an existing game's branding, menus or look.
- The owner may publish it later. **Real-player data is for private local testing only.** The files in `data/real/` must never be shipped or redistributed (source licence unclear; the underlying data is Basketball-Reference's). The published default stays fictional; real data stays an optional local import. Lookalike names of real players were rejected as a workaround, because identifiability, not spelling, is the legal risk.

## 2. How the owner wants to work

- **Language and level**: Go, their choice. They are comfortable with TypeScript, less so with Go, and are learning. Standard library only.
- **Communication**: they speak informally and often by voice, so messages can be garbled. Restate your understanding in plain words. Ask simple either/or questions. They could not follow a request to write a function themselves, so ask design decisions as plain choices and write the code for them.
- Explanations in plain language, short. They like brief "insight" notes about why a design choice was made.
- **Token economy matters a lot** (limited allowance; the main model's limit resets around the 17th and they may switch to another model). Rules that emerged:
  - The main session designs, briefs and reviews. Smaller-model subagents write and test code.
  - ONE layer only: subagents must not spawn their own subagents.
  - One or two helpers at a time. Detailed briefs with file boundaries so two helpers never edit the same files.
  - No separate sessions (they cost too much).
  - No nitpicking polish, but verify results independently before reporting.
- **Always ask before**: downloading anything; restarting the game server when a league may be in progress (unsaved progress is lost); anything that leaves the machine.
- **No git repository exists.** Never commit unless asked. (If one is ever created, add `data/real/` and `saves/` to `.gitignore` first.)
- **Report honestly**: say what was verified by running it and what was only reported by a helper.
- **Never show the hidden number** (`Impact`) in the interface.

## 3. Current state

What exists and works:
- Engine: players, seeded RNG, fictional names/teams, league generator, possession-based game sim with box scores and play-by-play, 82-game season over two conferences, playoffs (steppable), awards (MVP, Finals MVP), offseason (progression, retirements, 60-pick two-round draft where the user picks for their team, roster trim to 13, free agents), career history, save/load.
- Web UI (`cmd/web`): start page, dashboard with sim buttons, roster, player page with career table, standings, leaders, playoff bracket page (West left, East right, Finals centre), live game viewer replaying recorded play-by-play (pause, 1x/4x/16x, skip to end), offseason report, draft page, history, box scores, save/load to `saves/league.json`.
- Real-data mode: `NewRealLeague` builds the real 2010-11 league from the CSVs in `data/real/`. Offered on the start page only when the files exist.
- Validation tool `cmd/realcheck` compares simulated 2010-11 against real standings.

Verification done while writing this file (2026-11-14):
- **[verified]** `go vet ./...` clean and `go test -count=1 ./...` passes (`ok hoopsim/engine`, about 6 s; the `cmd/*` packages have no tests). Go is go1.27.4 at `/usr/local/bin/go`; `go.mod` says `go 1.22`.
- **[verified]** File list, exported names, flags and HTTP routes in section 5 were read from the code.
- **[reported]** Browser check of the live viewer (runs, skip works, final score matched the bracket).
- **[reported]** All numbers in section 7.

The web server (port 8080) may be running with a league in progress. Do not restart it without asking.

## 4. How to run everything

Shell may need `export PATH="/usr/local/bin:$PATH"`. Run from `/Users/apple/Documents/sim`.

| What | Command |
|---|---|
| Web game (http://localhost:8080; `-port N` to change) | `go run ./cmd/web` |
| Preview server definition | `.claude/launch.json`, name `hoopsim`, port 8080 |
| Engine sanity check (league-average game stats; `-games N`, `-seed N`) | `go run ./cmd/sim -mode sanity` |
| Simulate several seasons, prospect tracker | `go run ./cmd/sim -mode seasons -seasons 10` |
| Same, with the hidden number shown (debug only) | `go run ./cmd/sim -mode seasons -seasons 10 -reveal` |
| Real-data validation vs real 2010-11 (`-data` dir, default `data/real`) | `go run ./cmd/realcheck -runs 30` |
| Tests | `go test ./...` (add `-count=1` to bypass cache) |

## 5. Code map

Root: `go.mod`, `.claude/` (agents, `launch.json`), `cmd/`, `engine/`, `data/real/` (private CSVs), `saves/league.json` (current save).

### `engine/` (all game logic; no I/O except real-data loading and save/load)
- `rng.go`: `RNG` (the only randomness source; every function takes one so a seed reproduces everything), `Between`, `Normal`, `Chance`, `Pick`, `PickWeighted`, `clamp`.
- `player.go`: `Position` (PG, SG, SF, PF, C), `Ratings` (Inside, Mid, Three, Playmaking, Rebounding, Defense, Athleticism), `Player` (incl. `AltPos`, `Potential`, hidden `Impact`, `RealID`). `overallWeights` per position; `Overall()` = best over positions the player can play; `Effective()` = ratings shifted by `Impact` (what the sim uses); `TrueOverall()`.
- `names.go`: fictional name lists.
- `generate.go`: `RosterSize = 13`, `GeneratePlayer`, `Team`, `League` struct, `GenerateLeague(seed)`, `dealEvenly` (deals talent evenly to teams at creation; this is the main team-balance fix), `Team.Rotation()`.
- `game.go`: possession model constants (shooting bases, turnover, foul, rebound, home edge, `shootExp`, `minutesByRank`), `SimGame` / `SimGameRec` (with play-by-play recording), `StatLine`, `TeamBox`, `GameResult`, `Event`. Contains `possession()`.
- `season.go`: `NumTeams = 30`, `SeasonDays`, `Conference`, `Season`, `PlayerSeason`, `StartSeason`, `SimDay` / `SimDays` / `SimRegularSeason`, `Standings`, `Leaders`, `computeAwards`.
- `playoffs.go`: `Series`, `PGame`, `StartPlayoffs`, `SimPlayoffDay`, `SimUserSeries`, `SimRound`, `SimAllPlayoffs`, `finishPlayoffs` (Finals MVP).
- `history.go`: `CareerRow`, `SeasonRecord`, `Box` / `BoxTeam` / `BoxLine`, `recordBox`, `recordSeason`, `FindPlayer`.
- `progression.go`: `Progression` (the `ProgressionConfigT` with every tunable for development, retirement, rookies, roster management), `Player.Progress`, `RetireChance`, `TrueCeiling`, `GenerateRookie`.
- `offseason.go`: `BeginOffseason`, `Offseason`, `Draft`, `DraftPick`, `DraftAutoUntilUser`, `DraftAutoAll`, `FinishOffseason`, `OffseasonReport` (`Retiree`, `PickResult`, `Change`).
- `save.go`: `League.Save(path)` / `Load(path)` (JSON).
- `realdata.go` (890 lines): CSV loading, percentile models per season, `RealTuning` (`RealTuningT`), `rate()` (stats to ratings), `NewRealLeague`, `takeDraftClass`, `hindsightPeak`, and helpers `RatingsFromStats`, `RealPerGameStats`, `RealTeamWins`, `RealLeaguePPG` used by `realcheck`.
- `realteams.go`: the 2010-11 franchise table and `RealTeamAbbrevs()`.
- Tests: `game_test.go` (determinism, invariants, league averages), `season_test.go` (season basics, determinism, 30-season stability, team balance), `history_test.go` (save/load round trip, stepped draft), `playoffs_test.go` (step equivalence, save mid-playoffs, recording does not change results, events rebuild the box), `realdata_test.go` (real league shape and determinism; skipped if the CSVs are absent).

### `cmd/`
- `cmd/web/main.go` (850 lines): HTTP server with in-memory league. Routes: `/` (start), `/new`, `/load`, `/save`, `/dashboard`, `/sim`, `/offseason`, `/draft`, `/draft/pick`, `/draft/auto`, `/team`, `/player`, `/standings`, `/leaders`, `/playoffs`, `/playoffs/do`, `/watch` (live viewer), `/history`, `/box`. Templates in `cmd/web/templates/` (base, start, dashboard, team, player, standings, leaders, playoffs, watch, box, history, offseason, draft), CSS in `cmd/web/static/style.css`; both embedded.
- `cmd/sim/main.go`: command-line sanity and multi-season runs (flags: `-seed`, `-games`, `-mode`, `-seasons`, `-reveal`).
- `cmd/realcheck/main.go`: runs the real 2010-11 league many times; prints win correlation, mean absolute error, per-team misses, scoring comparison and ratings.

## 6. Design decisions and why

**Ratings.** Seven ratings per player. Players can have several positions (`Pos` + `AltPos`). The visible overall is position-weighted: the best of the weighted overalls over the positions he can play, not a plain average.

**Hidden number (`Impact`, about -8..+8, never shown).** It shifts real in-game performance (`Effective()`), and with `Potential` decides whether a prospect reaches, beats or misses visible potential (true ceiling = Potential + Impact x `CeilingImpactMult`). Age, overall and potential all feed development. `ImpactSD` was weakened to 1.8 (was 3) during tuning, so booms and busts come more from yearly luck than from the hidden number. Revisit after play-testing.

**Progression** (all in `Progression`): growth to about 27 (rate falls with age), prime 27-30, decline from 31, retirement from 34 with a forced end at 40. Leap and stall events for players under 25 (6% each). The owner chose development "in the middle" between predictable and swingy: target 55-65% of top-10 picks finishing within +/-4 of draft-day potential. Visible potential converges to overall by age 27.

**Balance.** Stars were first too dominant (36 PPG with 16 rebounds; 74-win teams). Fixed by retuning individual shares, then team balance mainly by dealing talent evenly at league creation (`dealEvenly`). Game constants were tuned to modern league averages.

**Interactive playoffs.** Steppable (a game day, my series, this round, to the end). Games record play-by-play so the viewer can replay them; recording is tested not to change results.

**Real-data mode** (private testing only). Source: GitHub repository `sumitrodatta/bball-reference-datasets`, downloaded with the owner's approval into `data/real/` (six CSVs: Player Totals, Advanced, Player Career Info, Draft Pick History, Team Summaries, Team Abbrev; 10.6 MB). The season column is the END year (2011 = 2010-11). The recipe, from `engine/realdata.go` (`rate`, `baseRatings`, `RealTuning`):
1. Each rating comes from a percentile of the league that season among qualified players (500+ minutes): Three from three-point accuracy (shrunk toward league average) and attempt rate; Mid mostly from free-throw ability; Inside from two-point efficiency, free-throw rate, two-point attempts and offensive rebound rate; Playmaking from assist rate (and low turnovers); Rebounding from rebound rate; Defense from defensive BPM, steals, blocks and defensive win shares per 48; Athleticism from steals, blocks, offensive rebounds, free-throw rate, attempts, plus an age term. A usage term (`UsageInside/Mid/Three/Play` = 30/22/22/8) adds to the offensive ratings.
2. Two target overalls are computed: one from the box-score profile, one from a value composite (BPM 0.6, WS/48 0.3, PER 0.1 as z-scores). Each maps through `Mean 63.8 + Sd 12 x z` with compression above z = 1.8 (`CompressZ`, `CompressSlop 1.0`). Low-minute players are shrunk toward `Replacement 50` by `(mp/500)^0.6`.
3. Visible overall target = 0.4 x box + 0.6 x value (`BoxWeight 0.4`). All ratings are shifted by a common delta, iterated until the position-weighted overall matches the target.
4. Hidden `Impact` = value target minus the resulting visible overall (gap between value stats and box-score ratings), clamped to +/-5 (`ImpactMax`).
5. Positions come from the listed position, height and assist/usage profile (`position()`).
6. Teams: 13 players per team by minutes on the team where each played most. Potential is set with hindsight (best later visible level, `hindsightPeak`). Real draft classes from 2011 onward replace generated rookies (`takeDraftClass`; `RookieDrop 4` lowers prospect targets before their first season).

## 7. Measured results (all [reported], 2026-11-14)

- Engine league averages after tuning: about 110 points per team, FG 47%, 3P 36.6%, home win rate 55.8%.
- Development: 65% of top-10 picks finished within +/-4 of draft-day potential (seed 1; target 55-65%).
- Balance after the fix (first seasons): best records 55-59 wins, worst 23-31, scoring leader about 31 PPG.
- `realcheck` (30 runs, 2010-11): win correlation 0.941, mean absolute error 5.3 wins vs real standings. Biggest misses: Heat +10, Hawks -9.7, Trail Blazers -9.6, Nets -9.3, Celtics +9.3. League scoring 113.3 simulated vs 99.6 real (the engine is tuned to modern pace; deliberately not retuned). Win spread SD 16.4 simulated vs 13.0 real (Heat 69 wins, Cavaliers 10). Top-rated: LeBron James 95, Dwight Howard 94, Derrick Rose 91, Chris Paul 91, Pau Gasol 90.
- Scorer/passer distribution misses: Carmelo Anthony 25.6 PPG real vs about 17-19 simulated (Monta Ellis and Amar'e Stoudemire similar); Rajon Rondo 11.2 assists real vs 6.7.

## 8. Known problems and things never verified

1. **Shot and assist distribution inside a team.** A per-player shot tendency now affects who shoots, separately from shooting skill; historical usage percentiles seed it in real-data mode. Playmaking no longer boosts shot volume directly. The new behavior still needs season-level calibration against scorer and passer distributions. `go run ./cmd/sim -mode ratings` prints current player and rookie rating distributions.
2. **Win spread too wide in real mode** (16.4 vs 13.0). No injuries or rest is part of the cause.
3. **Hidden number weakened** (`ImpactSD` 1.8). To revisit after play-testing.
4. **30-season stability tested on one seed only**; other seeds drifted slightly outside the bounds.
5. **Never tested**: real mode over many seasons (possible shortage of free agents by the late 2010s); loading a saved real league; loading a save made by an older build.
6. **Lineups**: the computer sets them for every team, including the user's; starter choice is by position slot and can look odd.
7. **Playoff stats** are tracked but not shown on player pages; box scores of a previous year's playoffs are not kept.
8. **UI not individually checked**: pause/speed buttons of the live viewer; narrow-window layout.
9. Engine pace is modern; real-mode 2010-11 scoring is about 14 points per team too high by design.

## 9. Next steps (owner: "build something fun, figure it out while playing")

Done: engine, interface, save/load, interactive playoffs, real-data mode. Recommended order:
1. Run `go run ./cmd/sim -mode ratings` to review the rating scale, then validate and tune the shot-tendency model against scorer/passer distributions and re-run `realcheck`.
2. Injuries and rest (also narrows the win spread).
3. User control of lineup and minutes.
4. Contracts and salary cap.
5. Free agency.
6. Trades with sensible computer teams.
7. Player traits, morale/chemistry, visible form.
8. News feed, records, hall of fame.
9. Draft scouting with fuzzy rookie ratings and a lottery.
10. Single-player career mode.
11. Browser-hosted version for publishing (real data excluded).

## 10. Glossary

- **Overall**: the visible single number; position-weighted best across a player's positions.
- **Potential**: the visible estimate of the overall a player can grow into; equals overall from age 27.
- **Impact / hidden MMR**: hidden number (-8..+8) shifting real performance and the real ceiling. Never displayed.
- **True overall / true ceiling**: overall and ceiling including Impact; internal only.
- **Effective ratings**: visible ratings plus Impact; what the game sim uses.
- **Real mode**: league built from real 2010-11 data; private testing only.
- **Hindsight potential**: in real mode, potential set to the player's best later level.
- **realcheck**: tool comparing simulated and real 2010-11 results.
- **GM mode**: the user runs one team.
- **dealEvenly**: league-creation step that spreads talent evenly across teams.

## Update log (newest first; overrides anything above that conflicts)

### 2026-11-14, night: merged to main, save versioning, plan agreed
- Owner agreed the order of work below and gave standing permission to commit and merge to `main` locally when the code is correct. Pushing still needs a yes. Owner also asked that checks (`go vet`, `go test`, web build) run only once per finished piece of work, not between small steps.
- `main` now holds the roles/usage/rookie/UI work (commit `89afcee`), checked by the main session with vet, test and web build; not play-tested in a browser.
- Save versioning (step 1, branch `feat/save-version`): `League.Version`, `SaveVersion = 1` and the `saveUpgrades` step list in `engine/save.go`; a save with no version is treated as 0 and upgraded on load, a newer save is refused. Tests in `engine/save_test.go`. Helper ran vet and tests (pass); main session read the diff only. Rule from now on: any new or changed saved field bumps `SaveVersion` and appends one upgrade step.
- Step 2a done (branch `feat/draft-pick-assets`, merged to `main`): `engine/picks.go` adds `DraftPick` {Year, Round, Original, Owner}, `League.Picks`, `PickYearsAhead = 4`, `PicksOwnedBy`, `PickOwner`, `TransferPick`, `rollPicks`. `Draft.Order` still lists original teams (slot order, also used for the free-agent fill); `DraftOnClock` returns the pick's owner. `SaveVersion` is 2 (step 1 to 2 creates picks). Tests in `engine/picks_test.go`. Helper ran vet and tests (pass); main session read the diff only. Nothing in the UI shows picks yet, and nothing can trade them yet.
- Was "next" when written: step 2a, draft picks as owned objects (engine only), then 2b multi-team asset trades, then 2c API and trade screen. Defaults chosen by the main session, owner may change: picks tradeable four drafts ahead, two rounds, no protections or swaps yet, at most three teams in a trade, simple cap later.

### 2026-11-14, late: investigations only (no code changed), decisions pending
All findings below are helper reports; the main session only spot-checked three rows of the measurement output.
- Why the same players always star in real mode: young players' Potential comes from their real later peak (`hindsightPeak`, realdata.go), growth pulls toward that ceiling, there are no injuries or situation effects, and the default seed is 1. Measured over 10 seeds x 10 seasons: Rose ends 85-95 every time, Harden is #1 in 7 of 10, per-player spread 2.5-5 points, below-average youngsters reach 80+ in 2.9% of cases. Ryan Anderson reaches 86-94 every seed (Impact +5 doubled into the ceiling; looks wrong, not traced). Throwaway measuring program was in the session scratchpad, not kept.
- Owner decisions still open: blur real-player ceilings per league or keep hindsight; injuries permanent or games-only; random default seed; simple cap or full rulebook.
- Feature gap vs a 2K-style GM mode: sim core is largely done; missing are contracts/salary cap (nothing exists), user free agency, picks as trade assets, 3+ team trades, AI offers to the user, lottery, scouting, lineup control, injuries. Save has one slot and no version field.
- Structure audit: `engine/` is clean (no HTTP, RNG discipline holds, trade rules in engine, Impact cannot leak via API; vet and tests pass, 26 tests). Weak points in order: player-only two-team `TradeOffer` and no asset/contract model; save has no `Version`/migration; phase and flow rules (`phase()`, `state()`, `playoffAction`, seeding) live in `cmd/web` instead of the engine; API responses are untyped maps with hand-kept TS types in `web/src/api.ts`.
- Proposed order (not yet agreed): save version + migration; asset model with multi-team trades and picks; contracts and simple cap, then user free agency; move phase rules into the engine; injuries and rest; lottery, scouting and blurred ceilings; product basics.
- Stale parts of this file: section 5 (html/template pages), "cmd/* has no tests" and "no git repository" are out of date.

### 2026-11-14, evening: narrow layout, 3D formation, offseason trades card
- Narrow layout (top bar stacks, nav strip scrolls, cards scroll inside themselves, sticky Start button): main session saw the dashboard at 560px with no clipping and no console errors; helper measured no sideways scroll at 520px and 800px on the main pages. Start screen at narrow width and the 1280px pages other than the game page were not looked at. Visible scrollbars on the nav strip remain.
- 3D formation (`web/src/court3d/Court3D.tsx`): offence now heads for the hoop it attacks and defenders mark their men, but at the fast replay speed the figures are still mostly in transit near half-court because possessions change ends faster than they can run (a whole game replays in roughly 30 seconds). Helper reports a proper half-court set at 1x; main session could not confirm 1x. Open: slow the default pace or guarantee figures arrive before each shot.
- "Offseason trades around the league" card on the dashboard (first 5 days of a season, from `/api/transactions`): compiled only, never seen with real trades.
- Only `web/src` changed; `npm run build` passes. Uncommitted.

### 2026-11-14, evening: state check after the owner's changes
- A git repository now exists with one commit ("Initial HoopSim commit"); `.gitignore` excludes `data/real/` and `saves/`. The usage-tendency work and later handoff edits are uncommitted. Never commit or push unless asked.
- `go vet`, `go test -count=1 ./...` and `npm run build` pass on the current code (run by the main session).
- `realcheck -runs 10` after the usage-tendency change: win correlation 0.943, MAE 5.1 wins, sim win SD 16.2 vs real 13.0. Scaled scoring improved for Stoudemire (22.0 vs 25.3 real, was 19.5) and slightly for Carmelo Anthony (18.0 vs 25.6, was 16.6); Monta Ellis unchanged (17.2 vs 24.1). Assists still low for pure passers (Rondo 7.1 vs 11.2, Nash 8.9 vs 11.4). Still open.
- In progress: a helper fixing the 3D formation (players bunching at half-court), the layout below 1000px (owner views in a narrow side pane), and where offseason computer trades are shown. Verify in a browser before calling done.

### 2026-11-14, evening: trade screen and 3D court reviewed
- Trade screen (`web/src/pages/Trade.tsx`, API in `cmd/web/trade.go`: `/api/trade/status|block/{id}|evaluate|counter|execute`, `/api/transactions`) and lazy-loaded 3D court (`web/src/court3d/Court3D.tsx`; `three`, `@react-three/fiber`, `@react-three/drei`) are built. `go vet`, `go test ./...` pass on the current code.
- Seen in a browser by the main session at 1280px: trade screen layout with both rosters, value bars and offer trays; 3D court rendering with hoops, figures, ball, scoreboard, play list, speed controls, camera presets, floating "+2", no console errors.
- Not seen: an accepted trade's animation and summary, the counter-offer preview, "Trade for this player" in the player drawer, Baseline/Overhead cameras, 16x, the 2D fallback.
- Problems found: (1) in 3D the ten figures bunch around half-court instead of setting up around the attacking hoop, most visible at 4x; (2) below about 1000px wide the top bar and nav are clipped, so the app needs a wide window; (3) computer trades of the offseason only show on `/offseason` after the new season starts, and nothing links there then.
- The game server on port 8080 may predate later code changes; restart it (ask first) to pick up engine changes.

### 2026-11-14, simulation role and rating diagnostics
- Added `Player.UsageTendency` and a derived fallback for old saves/imported players. Generated players get a role tendency based on shooting skills and position, with independent variation; real-data players use historical usage percentiles.
- Shot selection in `engine/game.go` now depends on playing time, shooting skill, and shot tendency, with less concentration toward the top shooter. Playmaking still affects turnovers and assist selection, but no longer adds directly to shot volume.
- Broadly lifting rookie ability or potential caused long-run stability failures. The potential curve is unchanged; current rookie base talent is 43.5 (from 44) to keep the generated league near its tested development equilibrium. A fuller rookie tier rebalance remains open and should use the ratings report as its calibration aid.
- Player details show a plain-language scoring role and shot-tendency value. `go run ./cmd/sim -mode ratings` reports overall/usage percentiles by primary position and rookie overall/potential percentiles by draft tier.
- Verification: `go test ./...` passes and `cd web && npm run build` succeeds. Seed 1 rookie median Overall/Potential by draft tier is 68/87 (picks 1-5), 62/67 (6-14), 56/58 (15-30), and 45/47 (31-60); this confirms late-pick rookies remain weak and needs a deliberate future rebalance.

### 2026-11-14, later the same day
- **Interface rebuilt in React + TypeScript** (owner's choice; they are proficient in React). `web/` is a Vite app (React, TanStack Query, TanStack Table v8, Motion, Tailwind v4, lucide-react); `cmd/web` is now a JSON API under `/api` that serves `web/dist`. The old `html/template` pages are deleted. Build: `cd web && npm install && npm run build`; run: `go run ./cmd/web`. Node is at `~/.nvm/versions/node/v22.23.2/bin`.
- **Flow:** top bar with a phase stepper and one Continue button driven by `GET /api/state` (`nextAction`), plus a menu of larger steps; multi-day sims call `POST /api/sim` one day at a time to animate progress. Phases: `regular`, `regular_done`, `playoffs`, `playoffs_done`, `offseason`, `draft`, `draft_done`.
- **Checked in a browser by the main session:** start screen, dashboard, standings, playoff bracket, 2D live game viewer, player drawer. Not seen: sim overlay (helper saw it), draft board, offseason report, champion screen, schedule, leaders, history.
- **Conference bug fixed:** real teams reordered in `engine/realteams.go` so West is indices 0-14. Real-league saves made before this show wrong colours; start fresh. Do not load `saves/league.json` from the old build.
- **Trades (engine, `engine/trade.go`):** players-for-players, 1-3 per side, judged on VISIBLE ratings only; `EvaluateTrade`, `SuggestCounter`, `ExecuteTrade`, `TradeBlock`, `TradeWindowOpen` (deadline day 55), `SimAITrades` (about one computer-to-computer trade per offseason, called in `FinishOffseason`), `League.Transactions`. Mid-season computer trades are not wired in. No contracts or picks as assets yet.
- **Star supply fix (`engine/progression.go`):** owner decided the stability test keeps its minimum of 2 players rated 90+. Each draft class now has exactly two star prospects (`RookieStarEvery` 30, `RookieStarMean` 89, `RookieStarSD` 2; `RookiePotMean` 48 → 43). Over 8 seeds × 30 seasons the minimum is ≥ 2 (one seed exactly 2); one seed starts with 17 (above the 15 target); one seed drifts +3.2 in average level.
- **In progress when this was written:** a helper building the trade screen (API + React) and a 3D court game viewer (`three`, `@react-three/fiber`, `@react-three/drei`, lazy-loaded, 2D fallback). Verify both in a browser before calling them done.
- **Still open:** real-mode win spread too wide (a test Lakers team went 69-13); scorer/passer distribution (usage tendency) not built; only the latest regular-season game keeps a box score.
- **Owner feedback:** wants an interactive, animated interface with a flow that never dead-ends, and "something 3D". Keep `HANDOFF.md` updated as a habit (see `CLAUDE.md`).

## 11. Prompt to paste into a new session

```
You are continuing HoopSim, a Go basketball GM simulation in /Users/apple/Documents/sim.
First read HANDOFF.md fully, then CLAUDE.md. They replace the old conversation.
Today's state: engine, web UI, save/load, interactive playoffs and a private real-data
mode all work; go vet and go test pass. The next planned task is the per-player usage
tendency (section 8, problem 1, and section 9, step 1).
Working rules: the owner is a beginner in Go and speaks informally (often by voice), so
restate your understanding and ask plain either/or questions; explain briefly in plain
language. Save tokens: you design, brief and review; use small-model subagents for
writing and testing code, one layer only, one or two at a time, with file boundaries.
Never show the hidden Impact number in the UI. Never ship or commit data/real/.
Ask before downloading, restarting the web server (a league may be in progress),
or anything leaving the machine. Do not commit unless asked. Report honestly what you
verified by running versus what a helper reported.
Start by confirming go test passes (export PATH="/usr/local/bin:$PATH"), then propose the
first step of the usage-tendency change in a few plain sentences and wait for a yes.
```
