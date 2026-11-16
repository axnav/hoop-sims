# HoopSim handoff

Last full refresh: 2026-11-14. Sections 0, 3, 4, 7 and 10 updated 2026-11-16. Module `hoopsim`, Go (standard library only) with a React + TypeScript front end.

Legend: **verified** means a person or session ran it or read the code and saw it. **reported** means a helper said so and it was not re-run.

## 0. Where things stand (2026-11-16), read this first

**Built and merged to `main` on 2026-11-15 and 2026-11-16** (details in the update log, newest first):
- Player photos and avatars (real mode), what players want and contract extensions, training focus and learning speeds, automatic lineup starts the best five.
- Badges in the style of a console basketball game (40 badges, five tiers), coaches with six team systems and roster fit (save version 4), weekly morale and trade requests with a player news list.
- Release any time with dead money; trades on draft night with trade-up offers; player search (top bar and Players page).
- **Work in progress when the session moved to a new window (2026-11-16): the trade assistant** (section 16 piece B) on branch `feat/trade-assistant`. A Sonnet helper in the old window was writing it and had NOT reported back; its files are uncommitted in the working tree (`engine/assistant.go`, `cmd/web/assistant.go`, `cmd/web/assistant_test.go`, edits to `cmd/web/main.go`, `ARCHITECTURE.md`, `web/src/api.ts`, `ui.tsx`, `PlayerDrawer.tsx`, `pages/Trade.tsx`, `pages/Players.tsx`). The branch's only commit so far is this handoff refresh. Next window: check `git status`; if the old helper may still be writing, wait or ask the owner; then review the code against the contract in section 16, finish what is missing, run `go vet ./... && go test ./... -count=1` and `cd web && npm run build`, do the browser pass on 8097 (needs panel, "What would it take?" package loaded and accepted, advisor chips, the Players/drawer entry point, 1400 and 390 px), merge locally, update this file.
- Test changes the owner approved: `TestLongStability` drift 2.5, 90+ limit 20, FT% ceiling 81; `TestTeamBalance` allows one team-season outside 12-70 wins (hard limits 9-73).
- Effects kept small for now (owner's choice): coach and fit, morale. Raise later in `CoachTuning`/`SystemTuning`/`MoraleTuning`.

**Pushed:** `main` up to `2f8e965` (through coaches) on 2026-11-16. Later commits (morale, release, draft-night trades, search, documents) are local; push needs the owner's go-ahead.

**Next:** finish the trade assistant (section 16), then ask the owner. Open choices: owner goals and team finances, a fuller news feed (the league events list from morale is the start), the tuning batch (section 5 problems 1, 3 to 6, 15), the two clear buttons in the phone search box.

**Earlier (2026-11-14 to 2026-11-15):**
- Progression arrows on every player list (last change, this season or last year) and per rating in the player drawer.
- In-season progression: four steps a season; switch on the Settings page; `go run ./cmd/sim -mode develop -seasons 16` measures the development target.
- Sim window: live game ("Watch my games", speeds 1x to 64x), trade offers answered in the window, a Development box. "Watch next game" in the sim menu.
- Teams tab and team direction labels (Contender, Buyer, Seller, Rebuilding).
- Court view on the Lineup page (plan piece 1).
- Free agency: mid-level exception and restricted free agency with offer sheets.
- `TestLongStability` reworked to five seeds and a drift check (owner approved).
- Player photos downloaded to `data/real/photos/` (1,754), NOT shown in the game yet.

The owner will play a test stretch of games and may ask for changes; treat that feedback as higher priority than the next piece.

**Cost preference (owner, 2026-11-15):** keep token use low. Helpers on a named cheaper model; a small model for downloads, searching and measuring; scripts instead of an AI watching a long job; short answers.

**Lessons from this stretch:**
- Two helpers on one feature need the exact JSON shape written out, including what each route returns. "Same as the other routes" caused a page-blanking bug.
- The browser pass on port 8097 found two bugs that the tests and the type check missed. Keep doing it.
- A small-model helper told to "stop your background work" may stop other jobs too, and can loop. Give it one foreground task, one report, no background processes.
- Timings taken while another test run is going are not trustworthy; re-time on a quiet machine before calling something slow.

## 1. Summary

- HoopSim is a text and web basketball franchise simulation: you run one team (GM mode) through seasons, trades, free agency and drafts. There is no on-court graphics engine, only simulation (plus a replay viewer of recorded games).
- State today: the whole approved roadmap (trade negotiation, injuries, commissioner mode, contracts and soft cap, free agency, lineups, draft lottery, scouting) is built and merged to `main`. `go vet` is clean and all tests pass (verified 2026-11-14).
- The most important thing to know: the question that started this work, "why do the same real players always become stars in real-data mode?", is only partly solved (see problem 1 in section 5).
- Read next: `RULES.md` (the rules everyone follows, including the random-stream table and save rules) and `ARCHITECTURE.md` (how the code is built, file map, API routes, save format). Use `ARCHITECTURE.md` whenever you need to find or change code.
- This file is about state: what exists, what was decided and why, what is verified, what is wrong, what is next.

## 2. Get going in ten minutes

Your shell may need `export PATH="/usr/local/bin:$PATH"`. Run everything from the project root.

| What | Command |
|---|---|
| Checks (Go) | `go vet ./... && go test ./... -count=1` (engine tests take 30 to 40 seconds; verified result on 2026-11-14: all pass) |
| Build the web app (needed once, and after screen changes) | `cd web && npm install && npm run build` |
| Run the game | `go run ./cmd/web` then open http://localhost:8080 (flags `-port`, `-save`, `-web`) |
| Front-end dev server | `cd web && npm run dev` (proxies `/api` to port 8095, so also run `go run ./cmd/web -port 8095`) |
| League-average game check | `go run ./cmd/sim -mode sanity` (flags `-seed`, `-games`) |
| Rating distributions | `go run ./cmd/sim -mode ratings` |
| Multi-season run | `go run ./cmd/sim -mode seasons -seasons 10` (`-reveal` shows the hidden number; debug only) |
| Real-data check vs real 2010-11 | `go run ./cmd/realcheck -runs 30` (needs `data/real/`) |

How to test safely without touching the owner's game:
- Port 8080 and `saves/league.json` belong to the owner. Do not start, stop or write to them without asking.
- Build the server to a place outside the repository, for example `go build -o <scratch dir>/hoopsim-test ./cmd/web`. Never run a plain `go build` inside `cmd/web`: it writes a file called `web` there, and that file is tracked in git (problem 10).
- Run it on port 8097 with a scratch save: `<scratch dir>/hoopsim-test -port 8097 -save <scratch dir>/test.json`. Drive it with a script and headless Chromium. `RULES.md` section 7 describes the end-to-end pass.
- To try a real old save, copy it to the scratch folder first.

## 3. What the game has now

Everything below was built on 2026-11-14 unless noted. "Verified" means the main session ran an end-to-end pass on a throwaway server (port 8097, scratch save, headless Chromium) on that date. File names are the main engine file; `ARCHITECTURE.md` has the rest.

- **Core simulation** (older work). Players with seven ratings, a visible potential and a hidden `Impact`; possession-based games with box scores and play-by-play; 82-game season, two conferences, steppable playoffs, awards; offseason with progression, retirements and a 60-pick two-round draft; career history; save and load. Fictional league by default (30 teams). Verified by the test suite.
- **Save versioning** (`engine/save.go`). Saves carry a version; an older one is upgraded on load, a newer one is refused. Current version 4 (since 2026-11-16: coaches; the owner's own save has not been loaded with version 4 yet). Verified: a COPY of the owner's own save (no version, year 2032) loaded, was upgraded with picks and contracts, and ran a full season cycle.
- **Draft picks as owned, tradeable objects** (`engine/picks.go`). Four drafts ahead, two rounds, no protections or swaps.
- **Multi-team deals** (`engine/deal.go`). Up to three teams, players and picks, each computer team judges its own side and all must accept. Verified: pick-for-player, a rejected lowball, a three-team evaluate, a bad request, an executed deal. Not tried: executing a three-team deal by hand in the browser.
- **Trade negotiation** (`engine/negotiate.go`). Tier labels beside the numbers, a mood (accept, close, far, no) and a concrete ask, offers from computer teams, the sim pauses when an offer arrives and before the deadline (day 50), with "don't stop" switches. Verified: tiers shown, a lowball got an ask and adding it made the team accept, an offer paused an auto sim, Accept executed a trade, the deadline dialog appeared on day 50. Not tried: the Decline and "Change it" buttons by hand.
- **Injuries** (`engine/injury.go`). Mostly missed games (about 85 player-games per team per season), 0.4% serious with a permanent athleticism drop of 2 to 4. Can be switched off. Verified: injured players sat out and counted down; switching off stopped new ones. Not tried: serious injuries, playoffs with injuries, offseason carry-over.
- **Commissioner mode** (`engine/commissioner.go`). Option switches (Settings page) and editing a player's visible ratings, age, potential and healing. Refused when the mode is off. Cannot touch `Impact`. Verified: edit refused when off, worked when on, out-of-range refused.
- **Stage rules in the engine** (`engine/phase.go`). Which stage the league is in and what may happen next now lives in the engine, not the web layer. Evidence it changed no behaviour: the API tests ran unedited. No browser pass was done for this piece.
- **Contracts, soft cap, tax, hard ceiling** (`engine/contract.go`). Cap 140,000, tax line 170,000, hard ceiling 190,000 (thousands of dollars), minimum 1,200, maximum 49,000; trades must roughly match salaries; contract value adjusts trade value. Verified: league payroll averaged $137.4M, a swap breaking the hard ceiling was refused with a sentence, re-sign and release worked, rosters stayed at 13 with everyone under contract.
- **Free agency over ten days** (`engine/freeagency.go`). A stage between the draft and the new season: expiring players go to a pool, you make offers, rival teams bid, asking prices soften over the days. Verified: confirmations appeared, an unaffordable offer was refused, a lowball lost to another team, an offer at the ask signed a player, stars signed (a first play-through left a 98-rated star unsigned; that was fixed and re-checked). Six seasons of payroll stayed between 97% and 107% of the cap (helper report). Not tried: withdrawing an offer by hand.
- **Lineups** (`engine/lineup.go`). Pick five starters and minutes per player (total 240, no more than 42 each, at least 8 players). Injured players' minutes are redistributed. Young players (24 and under) develop faster with many minutes and slower with few. Verified: saving at 248 minutes blocked, 240 allowed, a custom lineup produced exactly the minutes set, an injury redistributed minutes, Reset returned to automatic.
- **Draft lottery** (`engine/lottery.go`). Fourteen lottery teams, four slots drawn, round two in record order. Verified: the lottery moved teams.
- **Scouting** (`engine/scouting.go`). Prospects show ranges that always contain the truth; 12 scouting points per draft narrow them. The Draft page waits for "Start the draft" so there is time to scout. Verified: two scouts narrowed the top prospect from 53-67 / 81-99 to 58-64 / 85-93; the browser never received true prospect values or `Impact`; after the draft all true values were inside the ranges shown.
- **Blurred ceilings in real data** (`RealTuning.PotentialBlurSD` 4.0 in `engine/realdata.go`). Each real-data league gets its own noise on young players' potentials, so ceilings depend on the seed.
- **Real-data mode** (older work, private). Builds the real 2010-11 league from `data/real/`. Verified: a full cycle ran cleanly on 2026-11-14.
- **Screens** (React in `web/`). Dashboard, roster, lineup, trade, free agency, draft, offseason, injuries, settings, standings, playoff bracket, game viewer (2D and a lazy-loaded 3D court), history, Coach page, Players page (search).
- **Added 2026-11-15 and 2026-11-16** (each verified in a browser pass on port 8097; details and what was not tried are in the update log): photos and avatars (`cmd/web/photos.go`); motivations, stances and extensions (`engine/motivation.go`); training focus (`engine/training.go`); best-five automatic lineup (`Team.Rotation`, `StarterSlots`); badges (`engine/badges.go`); coaches, systems, fit (`engine/coach.go`, `engine/system.go`); morale, trade requests, league events (`engine/morale.go`); release with dead money (`League.Release`, `Team.DeadMoney`); draft-night trades (`engine/drafttrade.go`); player search (`cmd/web/players.go`).

## 4. Decisions made and why

Owner decisions and standing preferences:
- Simulation only. The default game is fictional. Real-data mode is for private local testing and must never ship (the data's licence is unclear; lookalike names were rejected because being identifiable, not spelling, is the legal risk).
- Trade screen keeps its numbers and also shows plain tier labels. Teams negotiate (a mood and a concrete ask) instead of flatly refusing.
- Soft cap with a luxury tax and a hard ceiling, no long list of exceptions. Free agency by offers over several days. Scouting by fuzzy ranges narrowed with points. Lineups by starters plus minutes sliders. Injuries mostly missed games with rare serious ones, switchable off. A commissioner mode.
- Later, not built: owner goals and job security, a full news feed. (Morale, trade requests and coaches were built on 2026-11-16.)
- 2026-11-16: badges in the 2K style (about 40, five tiers, effects measured against the league average, earned by ratings plus position); release any time with dead money; trades on draft night; a built-in trade advisor (rules, offline), not an AI chat.
- The interface is React (the owner knows it) with an animated flow that never dead-ends; a 3D court viewer was asked for.
- Checks run once per finished piece of work. Every feature gets one end-to-end pass on port 8097. The owner gave standing permission to commit and merge to `main` locally when the code is correct, and on 2026-11-14 asked for `main` to be pushed.

Design decisions from earlier work (checked against the code):
- **Ratings.** Seven ratings per player, players may have a second position. The visible overall is position-weighted and takes the best position the player can play.
- **Hidden number `Impact`** (about -8 to +8, never shown). It shifts real in-game performance and, with potential, sets the true ceiling (ceiling = potential + Impact x `CeilingImpactMult`). `ImpactSD` was weakened during tuning so booms and busts come more from yearly luck. All tunables are in `Progression` in `engine/progression.go`.
- **Progression.** Growth to about 27, prime 27 to 30, decline from 31, retirement from 34, forced at 40. Leap and stall events for young players. The owner chose development "in the middle" (target 55 to 65% of top-10 picks finishing within 4 points of their draft-day potential).
- **Balance.** Stars were first too dominant. Fixed by retuning individual shares, and mainly by dealing talent evenly at league creation (`dealEvenly` in `engine/generate.go`). Each draft class has exactly two star prospects so a minimum of two 90+ players holds over 30 seasons.
- **Shot tendency.** Each player has a usage tendency that affects who shoots, separate from shooting skill; real-data mode seeds it from historical usage.
- **Real-data recipe.** Ratings come from season percentiles of box-score and value statistics; the hidden `Impact` is the gap between value statistics and the box-score rating (clamped to 5); potential is set with hindsight (the player's later best level, then blurred); real draft classes from 2011 replace generated rookies. The tunables are in `RealTuning`.
- **Interactive playoffs.** Steppable; games record play-by-play so the viewer can replay them; tests confirm recording does not change results.
- **Pace.** The engine is tuned to modern scoring, so real-mode 2010-11 scoring runs about 14 points per team too high on purpose.
- **Saves.** Any new or changed saved field follows the rules in `RULES.md` section 4. New random-number features get their own stream (table in `RULES.md` section 3).

## 5. Known problems and open decisions

1. **The same real players become stars.** On real data over 10 seeds and 10 seasons, after all the work: six different players finish as the best young player (before: one player in 7 of 10), but the elite group is the same names (top-ten overlap 0.48, was 0.51) and Derrick Rose still ends 87 to 96 in every seed. Ryan Anderson (hidden Impact +5, doubled into his ceiling by `CeilingImpactMult` 2.0) reaches 88 to 95 in every league. Below-median youngsters peaking at 70 or more went from 12.8% to 15.4%. Blurring changed the order at the top, not who is at the top. Untried levers: a larger `RealTuning.PotentialBlurSD` (now 4.0), a smaller `CeilingImpactMult`, more consequential injuries. Decision needed: which lever to pull first.
2. **Automatic lineup and positions. FIXED (branch `feat/autolineup`): the automatic rotation now starts the five best healthy players by overall, and `StarterSlots` only seats them in PG..C for the court view; positions stay out of the sim.** Original problem: The game sim ignores positions (only ratings and minutes count), but the automatic rotation picks starters by position, so weak players can start over far better ones (a 48-rated forward started over an 80-rated 20-year-old). This is why some weak players get heavy minutes league-wide. Choose between "the automatic lineup plays the best players" and "make positions matter in the game sim". The first is simpler.
3. **Pick values are small next to stars.** A top pick is worth about 32, a 99-rated star about 180. Untuned; nobody measured what top picks are worth right after the draft, so picks may be under- or over-priced (possible exploit either way).
4. **Fictional rosters are very top-heavy.** For example a 97 next to several players under 50 on one team.
5. **Retirement may be too eager for stars.** A 91-rated 35-year-old retired in a test league.
6. **Few trades in real-data mode.** Only one trade was logged in a year, against 26 in the owner's fictional save. Computer teams may trade less under the salary rules there.
7. **Hands-off teams suffer.** A team that ignores re-signing and free agency ends with leftovers (a hands-off team went 4-78 in a helper test). An over-the-cap team can only sign players whose ask is the minimum. This is intended but harsh; the screens warn before both steps.
8. **Money is thin.** A released player's contract simply ends (no money still owed). The luxury tax is recorded but has no consequence because teams have no finances. When a full roster receives extra players the game releases the lowest-valued ones without asking.
9. **Tests.** `TestFreeAgencyMarket` was loosened by a helper (top ten signing early: at least 4, was 5). `TestLongStability`'s home-win band (54 to 60%) sits near its edge; the test was reworked on 2026-11-15 to check drift on five seeds (see the update log), and seed 3 sits close to the drift band (+1.93 of 2.0). The engine trade tests halve salaries in a fixture. Engine tests take 30 to 40 seconds. `cmd/web` has API tests; `cmd/sim` and `cmd/realcheck` have none.
10. **Tracked binary.** The compiled file `cmd/web/web` is tracked in git (verified) and is rewritten by a build inside that folder. The owner was asked whether to untrack it and has not answered.
11. **Old save mode label.** The owner's old save reports mode "fictional" although it holds real player names.
12. **Cheaper helpers.** Two changes were proposed for helper cost and not applied: an environment setting in the user's Claude settings making helpers default to a cheaper model, and a note that helpers must always be given a named model. The note is now in `CLAUDE.md` and `RULES.md`; the settings change was not made and needs the owner's say.
13. **Front end.** The 3D court at fast replay speed (figures still mostly in transit near half-court because possessions change ends faster than they can run) and visible scrollbars on the nav strip are open. The trade screen's counter-offer button, accepted-trade animation and several other buttons were never clicked by hand. Playoff stats are tracked but not on player pages; only the latest regular-season game keeps a box score.
14. **Product basics missing.** API responses are untyped maps with hand-kept TypeScript types (`web/src/api.ts`); there is one save slot; there is no packaged build that leaves out `data/real/`.
15. **Scoring distribution.** Real mode still under-scores pure scorers and under-passes pure passers (Carmelo Anthony 18.0 vs 25.6 real PPG, Rajon Rondo 7.1 vs 11.2 assists, `realcheck -runs 10`). The simulated win spread is wider than real (SD 16.2 vs 13.0).
16. **Never tested.** Real mode over many seasons (possible free-agent shortage late), the tax display with a taxed team, withdrawing an offer by hand, serious injuries.

## 6. Measured results

- Engine league averages (reported, earlier): about 110 points per team, FG 47%, 3P 36.6%, home win rate 55.8%.
- `realcheck -runs 10` (reported, 2026-11-14): win correlation 0.943, mean absolute error 5.1 wins vs real 2010-11, simulated win SD 16.2 vs real 13.0.
- Free agency: league payroll 107%, 105%, 103%, 99%, 97%, 99% of the cap over six seasons (helper). Contracts pass: average payroll $137.4M in year one, $134.7M in year two.
- Star variety (the 10 seeds x 10 seasons run in problem 1): the "before" numbers were measured by a helper and spot-checked by the main session; the "after" numbers were run by the main session on the finished code (verified 2026-11-14). The measuring program was a throwaway in a scratch folder and is not in the repository; it would need to be rewritten to repeat the run.
- Checks run for this refresh (verified 2026-11-14): `go vet ./...` clean; `go test ./... -count=1` passes (`cmd/web` 3.8 s, `engine` 34.9 s).

## 7. What to do next, in recommended order

A. Open decisions that block nothing (ask the owner, with a recommendation):
   1. Pick a variety lever (problem 1). Recommended: first lower `CeilingImpactMult` or cap its effect for real players, then raise `PotentialBlurSD` to 5 or 6, then re-run the 10 seeds x 10 seasons measurement and compare the top-ten overlap (target clearly below 0.4).
   2. Automatic lineup (problem 2). Recommended: make the automatic lineup pick the best players by rating, and leave positions out of the sim for now. Change `AutoLineup` in `engine/lineup.go` and `Team.Rotation()`, then check the league's minutes by rating.
B. Tuning:
   3. Pick values (problem 3): measure what drafted rookies are worth, then adjust `PickValueFor` in `engine/deal.go`.
   4. Real-mode trade frequency (problem 6): find why computer-to-computer trades are rare under the salary rules, using a copy of a real league.
   5. Retirement of stars (problem 5), roster balance at league creation (problem 4), the scoring distribution (problem 15).
   6. Tests: restore the stricter free agency assertion if possible and widen seeds on the long-stability test.
C. Later features the owner listed: owner goals and job security (this would also give the luxury tax a consequence through team finances); a news feed. (Morale, trade requests and coaches: done 2026-11-16.)
D. Product basics: multiple save slots; typed API responses (generate TypeScript types from Go or add schemas); a packaged build that leaves out `data/real/`; front-end tests; untrack `cmd/web/web` once the owner agrees.

## 8. How the owner likes to work

The owner is learning Go and often dictates by voice, so restate the request in plain words, ask either/or questions with a recommendation, write the code for them and explain briefly. The main session designs and reviews, helpers write code with a named model, checks run once per finished piece, and results are reported honestly. The owner holds the main session responsible for what helpers do. Full list: `RULES.md` sections 9 to 11.

## 9. Glossary

- **Overall**: the visible single number, position-weighted, best across a player's positions.
- **Potential**: the visible estimate of what a player can grow into; equals overall from about age 27.
- **Impact**: the hidden number (-8 to +8) shifting real performance and the true ceiling. Never shown.
- **True ceiling**: potential plus Impact times `CeilingImpactMult`. Internal only.
- **Real mode**: a league built from real 2010-11 data; private testing only.
- **Hindsight potential**: in real mode, potential set from the player's best later level.
- **Blur**: random per-league noise added to real players' potentials.
- **Soft cap**: you may exceed the cap to re-sign your own players or sign minimum deals; the tax line and hard ceiling limit how far.
- **Throwaway server**: a test copy of the game on port 8097 with a scratch save.
- **Stream**: a separate random-number sequence for one feature (table in `RULES.md`).
- **dealEvenly**: league-creation step spreading talent across teams.

## 10. Prompt to paste into a new session

```
You are taking over HoopSim, a Go basketball GM simulation in /Users/apple/Documents/sim.
Read, in this order: CLAUDE.md, HANDOFF.md (state, decisions, problems, next steps),
RULES.md (the rules), and ARCHITECTURE.md when you need to find or change code.
They replace any earlier conversation.
Start by running: export PATH="/usr/local/bin:$PATH"; go vet ./... && go test ./... -count=1
and tell me the result.
Then restate in two or three sentences what you understand the state to be (HANDOFF.md
section 0). If the trade assistant (section 16 piece B, branch feat/trade-assistant)
is not merged yet, finish and verify it first; then ask me what to build next
(section 0 lists the open choices). Keep token use low: cheaper named models for
helpers, scripts for long jobs, short answers.
The owner is learning Go and may dictate by voice: use plain words, write the code for them.
Never touch port 8080 or saves/league.json; test on port 8097 with a scratch save.
Never show the hidden Impact number or true prospect values; never commit data/real/.
Do not commit or push unless the rules in RULES.md section 8 allow it.
Report what you ran and saw separately from what a helper reported.
```

## 11. Reference: what a full console-style GM mode has that HoopSim lacks

Written 2026-11-14 from the main session's general knowledge of console franchise modes (the NBA 2K MyGM / MyNBA family). It was not checked against any one release online; treat it as a checklist of ideas, not a spec. Items already planned in section 7 are marked (planned).

**Have already:** trades with picks and three teams, negotiation and incoming offers, soft cap with tax and ceiling, free agency by days, draft lottery, scouting ranges, injuries, lineups and minutes, steppable playoffs, awards, history, commissioner edits, replay viewer.

| Area | Missing |
|---|---|
| Players | In-season progression; training or practice focus; badges or traits; hot and cold streaks; fatigue and durability; morale, chemistry, roles and promises, trade requests (planned); personalities |
| Staff | Coaches and coaching systems (planned); scouts and trainers with their own ratings; hiring and firing |
| Owner and GM | Owner goals and job security (planned); team finances, budget, ticket prices; GM skills; relocation and rebranding |
| Contracts | Player and team options; no-trade clauses; restricted free agency and offer sheets; in-season extensions; exceptions (mid-level and others); dead money and buyouts; two-way and ten-day deals; a development league |
| Trades | Pick protections and swaps; a trade finder ("who would give me X"); a public trading block; draft-day trades; cash and trade exceptions |
| Draft | Workouts and a combine; a mock draft and big board; draft-and-stash |
| League | All-Star weekend; All-League teams and more awards (defensive, rookie, sixth man, most improved); Hall of Fame and records; play-in games; expansion; rule changes; several save slots |
| Presentation | News feed (planned); live game view while simming (being built, see the update log); box scores for every game; player comparison; milestones |
| Game model | Positions that matter (problem 2); playbooks and strategy sliders; fatigue; clutch |

**Ideas to improve player progression, in recommended order:**
1. Show it: save each player's last yearly change and show it beside the overall on every screen (being built). Then a per-season overall line on the player page.
2. In-season progression: done 2026-11-14 (four steps a season; see the update log).
3. Training focus: the user picks a focus per player (shooting, defense, body) that tilts which ratings get the growth, not how much.
4. Performance-based growth: playing above expectation adds a little, for all ages (today only minutes count, and only for ages 24 and under).
5. Different peak ages per player (late bloomers, early decliners) from a hidden per-player number, which would also help problem 1 (star variety).
6. Mentors (a veteran at the same position helps a young player) and, once coaches exist, coach development ratings.
7. Injuries that cost development time (a long absence slows that year's growth).

## 12. Approved plan (owner approved 2026-11-15)

Six pieces, in this order, one at a time, each with checks, a pass on port 8097 and a local merge. Ideas come from researching the NBA 2K franchise modes and EA FC career mode online; no formulas or names are copied.

1. **Court view for the lineup.** Half-court picture on the Lineup page with five spots and the bench; tap or drag a player onto a spot; out-of-position warning. Screens only.
2. **What players want, and extensions.** Visible motivations per player (Money, Winning, Role, Loyalty: Low/Medium/High). Each of the user's players shows a stance: Will re-sign, Open to it, Will test free agency, with reasons. "Offer extension" for a player in his last contract year, in season or offseason (first-year salary limit like the NBA's 140% rule). The same motivations weight a free agent's choice between offers (extend the appeal score in `playersDecide`).
3. **Coaches, systems and roster fit.** One head coach per team (Offense, Defense, Development, a preferred system), hired from a pool in the offseason, no salary. Six systems of our own: Balanced, Pace and Space, Run and Gun, Inside-Out, Star-Led, Defense First; each sets pace, three-point share, star usage and defensive effort and names the skills it needs. Per-player fit grade and a team fit score from the players who get minutes; small game effect. The user may run a system other than the coach's favourite at a cost. Needs a before/after measurement of scoring and home-win rate.
4. **Training focus.** A focus per player (Balanced, Shooting, Inside scoring, Playmaking, Defense, Rebounding, Body). Each player has his own learning speed per skill (derived from seed and player ID, no saved field) and age windows. The focus moves where growth goes; it does not add growth. The coach's Development rating feeds in.
5. **Badges.** About twelve of our own in four groups (finishing, shooting, playmaking, defense/rebounding), tiers Bronze/Silver/Gold, earned by rating thresholds and by performance, small effect on the matching kind of play; three personality badges (Leader, Mentor, Star). Badges count toward system fit.
6. **Player pictures.** Photos are downloaded (2026-11-15): 1,754 of 1,966 real-data players, in `data/real/photos/<RealID>.jpg|png`. NOT yet shown in the game. Still to build: generated avatars for everyone, and in real-data mode the photo when a file exists. The exact changes are listed in `ARCHITECTURE.md` section 16. The owner said the photos are for local use and will say when to remove them before going live (section 16 has the removal steps).

Left for later on purpose: fatigue on back-to-backs, chemistry between personalities, promises with a trust rating, the room and bi-annual exceptions, more 3D court work (the owner parked it for cost).

## 13. Build plan for pieces 6, 2 and 4 (owner approved 2026-11-15)

Run each piece the usual way: its own `feat/` branch, two helpers on a named mid-size model with ONE written contract that spells out every JSON key and what each route returns, checks once, a browser pass on port 8097, local fast-forward merge, update log. Push only on the owner's say.

**Owner decisions for these pieces:**
1. A player whose stance is "Will test free agency" refuses to re-sign or extend early. The user can still keep him by matching an offer sheet if he is restricted, or by winning the bidding.
2. Extensions have a pay limit like the NBA's: the first-year salary may be at most 140% of the current salary. A player whose market value is well above that will not extend.
3. Learning speeds are visible for every player except draft prospects.

### Piece 6: photos and avatars (small; no engine change, no saved field)
- `cmd/web/photos.go` (new): `GET /api/photo/{playerId}` serves `data/real/photos/<RealID>.jpg|png` for a player found by id, else 404; `addPhoto(row, p)` sets `"photo": true` on player rows. Full step list, path-safety and prospect rules, credits route and removal steps: `ARCHITECTURE.md` section 16.
- `web/src/ui.tsx`: `Avatar` (photo when `photo` is true, cropped from the top; otherwise a drawn avatar from initials and team colour; same fallback when the image fails). Use it in the player drawer, roster rows and the court tokens.
- Tests: 404 in a fictional league and for an unknown id; `photo` only when a file exists (temporary folder).
- Browser pass needs a real-data league (`data/real/` is present on this machine).

### Piece 2: what players want, and extensions (medium)
- `engine/motivation.go` (new): `Motivation{Money, Winning, Role, Loyalty}` each Low/Medium/High, derived from the league seed and player ID with a hashed stream (offset 90000; add it to the tables in `RULES.md` section 3 and `ARCHITECTURE.md` section 9), so nothing is saved. `Stance(player)` returns a key (`resign`, `open`, `test`), a label ("Will re-sign", "Open to it", "Will test free agency") and reasons in plain words, from: minutes against what his rank on the roster deserves, the team's direction (`TeamStatusOf`), and years with the team.
- New saved field `Player.TeamSince` (league year he joined; 0 = unknown, treated as neutral; set on draft, signing and trade). No version bump.
- `engine/contract.go`: `ExtensionAsk(player)` and `Extend(team, player, salary, years)`, allowed when `Contract.Years == 1`, in the regular season and in the offseason before free agency. New saved contract fields `NextSalary`, `NextYears` (zero = none) that take over when the current deal runs out in `BeginOffseason`. Ask: market salary for `resign`, about 10% more for `open`, refused for `test`; refused when the ask is above 140% of the current salary (tunables in `Cap`). `ReSign` also refuses a `test` player. Computer teams follow the same stance rule in `reSignComputerTeams`.
- `engine/freeagency.go`: in `playersDecide` the existing appeal score (salary, years, team quality, top-eight role) is weighted by the player's motivations; Loyalty adds a bonus for his previous team.
- API: `motivation` on the player page and on free-agent rows; `stance` on the user's roster and expiring rows; `GET /api/contracts/extension/{id}` (ask, or the refusal sentence) and `POST /api/contracts/extend`.
- Screens: "What he wants" panel and "Offer extension" in the player drawer; a stance chip on roster rows; what each free agent values on the Free Agency page.
- Measure: league payroll over six seasons before and after; how many players re-sign, extend, and reach free agency per offseason.

### Piece 4: training focus (medium)
- `engine/training.go` (new): saved field `Player.Focus` (Balanced, Shooting, Inside scoring, Playmaking, Defense, Rebounding, Body; zero = Balanced = today's behaviour). `Aptitude(player)`: a learning speed per rating from a hashed stream (offset 90001), shown as Fast/Normal/Slow. Age windows as tunables in `Progression` (Body closes at 25 like athleticism today).
- `engine/progression.go`, `engine/inseason.go`: both already send every rating change through `ratingStep`. Add a weight per rating there (focus x learning speed x age window), scaled so total growth is unchanged; decline is not affected. `ProgressMinutes` is a `Player` method with no league, so the weights must be passed in. Computer teams pick a focus each offseason.
- API: `POST /api/training {playerId, focus}`; a `training` block on the player page (focus, options, each skill's speed and whether its window is open, a rough "about N steps for +2"); `focus` on roster rows.
- Screens: a Training panel in the player drawer with the focus picker.
- Measure: `go run ./cmd/sim -mode develop -seasons 16` with everyone Balanced against automatic focus; the share must stay in 55 to 65%.

## 14. Build plan for pieces 5 and 3 (owner approved 2026-11-15)

Order: piece 5 (badges) first, because piece 3's system fit uses badges. Each piece the usual way: `feat/` branch, an engine + API helper then a screens helper (named mid-size model) coding against the final JSON, checks once, browser pass on port 8097, local merge, update log.

**Owner decisions:** (1) Mentor makes teammates aged 24 and under grow about 5% faster. (2) Coach Development makes growth up to about 8% faster or slower, centred so the league average is unchanged. (3) Coaches are hired and fired in the offseason only. (4) System fit effect is small: perfect against terrible fit is about 1.5 points of team shooting %.

### Piece 5: badges, redesigned 2026-11-16 (owner: "like NBA 2K, about 40 badges, stars with 25 to 35")
Owner choices (2026-11-16): 2K-size (about 40 badges, 6 groups, 5 tiers Bronze/Silver/Gold/Hall of Fame/Legend; stars 25-35 badges, starters about 12-25, bench 5-15); effects measured against the league average (a player with an average badge load plays like today, more helps, fewer hurts, so league scoring stays put); earned by ratings plus position limits (no height field exists, position stands in for height; real heights would be a later piece), last season's stats can raise a badge one tier. Names are our own, never 2K's.
- Forty badges. Finishing: Rim Finisher, Power Inside, Foul Magnet, Soft Touch, Lob Target, Post Craft, Above the Rim, Contact Finisher. Shooting: Spot-Up, Midrange Maestro, Deep Range, Catch and Fire, Off the Dribble, Corner Specialist, Clutch Shooter, Charity Stripe. Playmaking: Floor General, Ball Security, Lob Passer, Needle Threader, Transition Pusher, Breakdown Dribbler, Steady Hands. Defense: Lockdown, Rim Protector, Pickpocket, Passing Lane, Perimeter Wall, Post Anchor, Help Defender, Charge Taker. Rebounding: Glass Cleaner, Box-Out Boss, Tip Master, Outlet Starter. General: Motor, Iron Will, Leader, Mentor, Star (the last three are personality badges without tiers, offset 100000).
- Each skill badge feeds one of the sim's existing hooks (shot make chance by kind, fouls drawn, assists, turnovers, steals/forced turnovers, team defense, rebounds, shot share, pace). Our sim has no separate contests or dunks, so badges that share a hook each carry part of it. Per hook: player effect = scale x (his badge points on that hook with diminishing returns - the minutes-weighted league mean), the mean cached per season (derived, not saved).
- The first build (twelve badges, plain bonuses) failed `TestTeamBalance` (a 73-9 team) because stars stacked the badges; centring plus diminishing returns is the answer to that.

### Piece 5 (first plan, replaced by the redesign above)
- `engine/badges.go` (new): twelve badges of our own in four groups (finishing: e.g. Rim Finisher, Power Inside, Foul Magnet; shooting: Spot-Up, Midrange Maestro, Deep Range; playmaking: Floor General, Ball Security, Lob Passer; defense/rebounding: Lockdown, Rim Protector, Glass Cleaner), tiers Bronze/Silver/Gold.
- Rating badges derived from rating thresholds (not saved). Performance badges from last season's stats, awarded at season end, saved field `Player.PerfBadges` (omitempty, no version bump). Personality badges Leader, Mentor, Star from a hashed stream at offset 100000 plus age and experience (not saved). Add the offset to `RULES.md` section 3 and `ARCHITECTURE.md` section 9.
- Small game effects through existing sim hooks (`engine/game.go`): shot badges add to that shot kind's make chance, playmaking badges lower turnovers and raise assists, defense badges add to team defense, Glass Cleaner to offensive rebounds, Star raises shot share, Leader a tiny team boost; Mentor +5% growth for teammates 24 and under (through the training weights / growth path). All as tunables.
- API and screens: `badges` on the player page (name, group, tier, rule, effect) and a count on roster rows; chips in the drawer; badge filter on Leaders. No badges for draft prospects.
- Measure: points, home wins, FT%/3P% before and after (`-mode sanity`); badges per tier league-wide; `TestLongStability` must pass.

### Piece 3: coaches, systems and roster fit (large)
- `engine/coach.go` (new): `Coach{Name, Offense, Defense, Development, System}`, fictional names in both modes, no salary. Saved `Team.Coach` and an offseason coach pool: **SaveVersion 3 -> 4** with an upgrade that gives every team a generated coach (`RULES.md` section 4). Random stream offset 110000.
- `engine/system.go` (new): six systems of our own (Balanced, Pace and Space, Run and Gun, Inside-Out, Star-Led, Defense First), each setting pace (`possMean`), three-point share (`threeShotWt`), star usage (`shootExp`) and defensive effort, and naming the ratings and badges it needs.
- Fit: grade A to D per player, team fit = minutes-weighted average of players with minutes; small effect (decision 4). Running a system other than the coach's favourite costs fit. Coach Offense/Defense small game effect; Development per decision 2 (feeds training growth).
- Offseason only: the user may fire the coach and hire from the pool; computer teams replace weak coaches.
- API and screens: Coach page (coach, system picker, team fit, player grades with reasons), fit grades on the Lineup page, hiring step in the offseason flow.
- Measure: scoring, home-win rate and the spread of wins before and after; `TestLongStability`.

## 15. Build plan for morale and trade requests (owner approved 2026-11-16)

Owner choices: morale moves week by week (saved); a small on-court effect measured against the league average (about +-1 point of shooting at the extremes); a trade request builds pressure (known league-wide, so his trade value to computer teams drops a little; if he stays, his mood sinks further and teammates dip slightly; trading him or fixing his situation ends it).
- `engine/morale.go` (new): saved `Player.Morale` (int, -50..+50, zero = content, omitempty, no version bump) and `Player.TradeRequest` (omitempty). Each sim week morale drifts part of the way toward a target built from role vs minutes, team winning, pay vs market value, years with the team, weighted by the player's motivations (piece 2), plus Leader teammates. Offseason events: re-signed or extended (up), traded (back toward content). Random stream offset 120000 (timing of requests).
- Effects: game make chance centred on the league mean; low morale pushes the stance toward "test" and raises extension asks; a request lowers his trade value to computer teams; computer teams move their own requesters more readily.
- API and screens: `morale` (score, label, trend, reasons, tradeRequest) on the player page and roster rows, team morale on the team page, a request flag on trade screens, an event in the existing transactions/news list.
- Measure: sanity before/after, morale spread and requests per season, `TestLongStability` and `TestTeamBalance` with no limit changes.

## 16. Build plan for draft-night trades and the trade assistant (owner approved 2026-11-16)

Found on checking: `TradeWindowOpen` (engine/trade.go) closes trades for the whole draft and between the end of the playoffs and the offseason; the draft already lets a pick's owner select (`PickOwner`), so trading picks during the draft mostly needs the window opened. The trade screen shows each team's block, an accept/reject evaluation and a counter, but not what a team wants or who the user should move.

Owner choices: (1) trades open during the draft, including this year's unused picks and just-drafted players, and a computer team may offer to trade up when the user is on the clock; (2) on another team: a needs panel (direction, what they value, missing positions/skills) plus "What would it take?" on any of their players, which builds the package from the user's roster they would accept; (3) a built-in advisor (rules, offline, advice only) labelling each of the user's players Keep / Shop / Sell with a reason.

**Contract for piece B (as briefed to the helper):** new `engine/assistant.go` (no saved fields, deterministic, tunables in `AdvisorTuning`). Routes: `GET /api/trade/needs/{teamId}` → `{team, direction:{key,label}, values:[..], lacks:[..], summary}`; `GET /api/trade/whatwouldittake/{playerId}` → `{player, team, possible, reason, packages:[{label, give:[..], receive:[..], note, deal}]}` where `deal` is a body the trade evaluate/execute routes accept and every package must pass `EvaluateTrade` and `ValidateDeal` (bounded search, at most 3 outgoing players and 2 picks, up to 3 alternatives); `GET /api/trade/advisor` → `{players:[{id,name,pos,age,ovr,advice:"keep"|"shop"|"sell",reasons}], summary}` for the user team. Screens: "What they want" panel on the Trade page for the selected partner; "What would it take?" on their players with "Load this offer"; the same action from the drawer of a computer team's player and from Players page rows (`/trade?team=<id>&want=<playerId>`); advisor chips (Keep green, Shop amber, Sell red) on the user's side of the Trade page and in the drawer. Tests: every package accepted (several seeds), impossible case has a reason, advisor cases (rebuilding team's old expensive vet sells, young cornerstone keeps, requester sells/shops), speed budget, API keys, no impact.

Pieces, in order (shared files): A. draft-night trades (`feat/draft-trades`); C. player search (owner asked 2026-11-16: no search existed; a search box in the top bar plus a Players page filtering every player in the league and free agency by name, team, position, age, overall, contract, badge, morale/trade request, sortable; `GET /api/players` with query parameters; prospects excluded, as their true values are secret); B. trade assistant (`feat/trade-assistant`), whose "What would it take?" can then start from a search result.

## Update log (newest first)

### 2026-11-16, handoff made a core rule; session moved to a new window
- Owner: keeping `HANDOFF.md` current is a core principle for every session and window. Written as hard limit 6 in `RULES.md` section 1 (log plus sections 0, 3, 4, 7, 10 after every piece, decision, push, and before a session ends or moves; work in progress noted in section 0; a piece is not finished while the handoff is stale), section 11 points to it, and `CLAUDE.md` carries the short version.
- Earlier the same day the main session had kept the log but let sections 0, 3, 4, 7 and 10 go a day stale; they were refreshed (commit 2aaea74 on `feat/trade-assistant`).
- State at the move: trade assistant unfinished on `feat/trade-assistant` (see section 0). `main` local is ahead of origin by morale, release, draft-night trades and search (pushed only through `2f8e965`).

### 2026-11-16, player search (section 16 piece C)
- Owner asked for a way to search players. Built by one Sonnet helper (API and screens, no engine change). `GET /api/players` (cmd/web/players.go): name (case and accents ignored), team or `fa`, position (primary or alternate), overall, age, salary, years left, badge, mood, trade request, sort, paging; rows through the same helpers as the team page; prospects and retired players excluded; 400 on bad values. Top-bar search box with a dropdown of 8 matches (icon below 700 px), Players page with URL filters, sortable headers and 50-row pages.
- Verified by the main session: `go vet`, `go test ./cmd/web -count=1` (engine untouched, confirmed by `git status`), web build. Port 8097 seed 1: `q=kow` finds Jamal Kowalski, `pos=C&minOvr=80&sort=age` gives 8 rows ascending by age, unknown badge 400, no impact key in 200 rows; top-bar search "kowal" shows 8 matches and Enter lists 11 at 1400 and 390 px, no console errors, no horizontal scroll.
- Rough edges: at 390 px the search input shows two clear buttons (the browser's own and ours); free agents' salary filters compare against an empty contract; not tried in real-data mode or with free agents in the browser.

### 2026-11-16, draft-night trades (section 16 piece A)
- Built by one Sonnet helper (engine, API, screens). `TradeWindowOpen` is open while the draft has picks left ("Draft night: trades are open.") and closes after the last pick until the offseason ends; the playoffs-to-offseason gap stays closed. Unused current-year picks are valued by their known slot (same units as `PickValueFor`), labelled "2025 1st round, pick 18"; used picks are hidden and refused. Just-drafted players tradable; expiring contracts (0 years) refused during the draft (helper's addition: they would walk away). `fixRosters` does nothing during the draft (no trims, signings or dead money). Trade-up offers (`engine/drafttrade.go`, `DraftTradeTuning`, offset 130000, next free 140000): once per user pick on the clock, 0.6 chance, from a computer team whose favourite prospect clearly beats its own later pick (within 12 slots), 1-3 picks worth 95-135% of the user's pick; about 22% of user picks drew one over 20 seeds; saved `Draft.OfferChecked`, `Proposal.DraftPick` (omitempty). Draft page: Trade button and the offer card.
- Verified by the main session: `go vet`, `go test ./... -count=1` (engine 325 s under load), web build. Port 8097 seed 1 team 0: status open with draftNight during the draft, pick labels with slots, one draft offer when on the clock at pick 18, Draft page shows the Trade link and the offer at 1400 and 390 px, no console errors, no horizontal scroll, no impact key. Helper traded pick 18 through the Trade page and saw the new owner select.
- Not tried: real-data league, an old mid-draft save, "Change it" on a draft offer.

### 2026-11-16, release any time, with dead money
- Owner noticed there was no way to release players (only "Let go" on expiring contracts in the offseason) and chose: release any time from the drawer; the team still pays him (dead money).
- Built by one Sonnet helper (engine, API, screen). `League.Release` works in season; in season the best free agent willing to take the minimum is signed to keep 13 (shared `bestFreeAgent`, never the player just released); refused if that signing would pass the hard ceiling. Saved `Team.DeadMoney []DeadContract` (omitempty, no version bump), counted in `Payroll` everywhere, counts down at rollover. Every cut of a player with years left leaves dead money, computer trims included (after offseasons 3 to 16 computer teams carry some, average $0.4-2.3M, max about $15.6M). An agreed extension is cancelled; no set-off if he signs elsewhere. `GET /api/contracts/release/{id}` preview (allowed, reason, deadSalary, deadYears, refill, refillName, short); team page `deadMoney` and `payrollObj.dead`. Release button with a confirm box in the drawer, dead-money list on the team page.
- **Test change, owner approved 2026-11-16:** `TestTeamBalance` keeps the 12-70 band but allows one team-season in the whole run outside it, never below 9 or above 73 (this change gave a single 11-71 season; badges, coaches and morale had each tipped it before). Helper also changed two fixtures to the new rules (`makeRoom` clears dead money; the API test's in-season release now expects 200).
- Verified by the main session: `go vet`, `go test ./... -count=1` (engine 239 s), web build. Port 8097 seed 1 team 0, first season, 390 px: confirm box names $24.2M x 3 years and warns the roster drops below 13 (a new league has no free agents in season one), release done, dead money listed on the team page, other-team release 409, no console errors, no horizontal scroll, no impact key. Helper saw the in-season refill back to 13 in season two at both widths.

### 2026-11-16, morale and trade requests (section 15)
- Owner picked this as the next piece and chose: weekly saved morale, a small on-court effect against the league average, trade requests that build pressure.
- Built by two Sonnet helpers (engine + API, then screens). `engine/morale.go`: saved `Player.Morale` (-50..+50, zero = content), `Player.TradeRequest`, `Player.MoraleStreak` (all omitempty, no version bump); every 7th sim day morale moves part way toward a target from role vs minutes, winning, pay vs market (skipped in the first 3 seasons), tenure, Leader teammates and request pressure, weighted by motivations; injuries never punished. Offseason: re-sign/extend +10 and ends a request, new team or release keeps half, new season keeps 75%. Requests after 3 weeks at -28 or lower, 30% a week (offset 120000; next free 130000); value to computer teams x0.88, own mood -2 a week (cap -16), teammates -1.5 each (cap -6); computer teams try to move requesters weekly and after offseason trades; ends on trade or recovery above -12. Morale feeds `StanceOf` and `ExtensionAsk`. New saved `League.Events` (newest 300) in `/api/transactions` as `events`. Screens: Morale block in the drawer (red banner on a request), mood chips and "Wants out" tags on roster and trade rows, team morale line, Player news card on the Trade page, Trade requests card on the Dashboard.
- **Effect cut, as with coaches:** the full on-court effect failed `TestTeamBalance` (an 11-win team); after both allowed cuts the cap is about 0.3 points of FG% (target was 1). Raise `MoraleTuning.EffectScale`/`EffectCap` later if wanted. No test limit changed.
- Helper measurement: sanity unchanged at a fresh league (everyone content). Seed 1, 10 seasons hands-off: about 4 requests a season, 13 of 41 traded, 1 withdrawn; season 10 end labels Furious 6, Unhappy 75, Content 188, Happy 106, Thrilled 15. TestTeamBalance most wins up to 68, fewest 12.
- Verified by the main session: `go vet`, `go test ./... -count=1` (engine 254 s under load), web build. Port 8097 fictional seed 3 team 4 after 280 days: 4 events (trade_request, request_traded), labels spread across all five, a Furious requester's drawer shows the banner and four reasons at 1400 and 390 px, no horizontal page scroll on dashboard/team/drawer/trade, no console errors, no impact key.
- Rough edges: the reasons list mixes positives into an unhappy player's list ("Looks up to a Leader on the team" for a Furious player); `request_withdrawn` events never seen in the browser.

### 2026-11-16, pushed
- Owner said "push it": `main` pushed to origin (`axnav/hoop-sims`), d804fcd..2f8e965, 11 commits (photos, motivations, training, lineup, badges, coaches). Private folders checked: none tracked.

### 2026-11-16, coaches, systems and roster fit (plan piece 3)
- Built by two Sonnet helpers (engine + API, then screens). `engine/coach.go` (`Coach` with Offense/Defense/Development and a favourite system, `CoachTuning`, offseason pool, `HireCoach`, `SetSystem`, computer teams replace weak coaches in `FinishOffseason` after the user's pick), `engine/system.go` (six systems: balanced, pace_space, run_gun, inside_out, star_led, defense_first; shifts to pace, three share, star shooting, defense; fit score/grade A-D with reasons from ratings and badges; per-system `Bias` so Balanced does not win everywhere; -8 off the coach's favourite). Saved `Team.Coach`, `Team.System` ("" = favourite), `League.CoachPool`; **SaveVersion 3 -> 4** with an upgrade giving every team a coach (test loads a v3 save). Offset 110000 (next free 120000). Coach Offense/Defense and team fit act against the league mean like badges; Development +-8% growth centred (decision 1). Routes `GET /api/coach/{id}`, `POST /api/coach/system` (any time), `GET /api/coach/pool`, `POST /api/coach/hire` (offseason only); `fit`/`teamFit` on lineup rows, `coach`/`teamFit` on the team page. Screens: Coach page (coach card, six system cards with fit grade each, player fit table, coaching market with confirm), fit column on Lineup, coach line on Team page, market card on Offseason, nav link. `go run ./cmd/sim -mode coaches`.
- **Owner decision 2026-11-16: keep the effects small for now.** To pass `TestTeamBalance` the helper used both allowed cuts: best vs worst realistic fit is about 0.65 points of FG% (owner's target was 1.5), coach Offense about 0.5, Defense about 0.7. Raise later in `CoachTuning`/`SystemTuning` after play-testing. No test limit changed.
- Helper measurement: sanity 20000 games before/after points 110.2/110.4, FG% 47.3/47.3, 3P% 36.8/36.9, home wins 57.1/56.2. All teams on one system: defense_first 103.8 to run_gun 115.1 points; the computer mix stays within 0.1 of balanced. Coaches replaced per offseason 0-2. Wins vs team fit correlation 0.16 after roster strength. TestTeamBalance most wins up to 65, fewest 13 (limits 70/12): thin margins; the helper saw 75-win and 12-win seasons in longer runs even with every new effect off, so this test is sensitive to any change in the random sequence.
- Verified by the main session: `go vet`, `go test ./... -count=1` (engine 83 s), web build. Port 8097 fictional seed 1: Coach page system changed by clicking at 1400 (Defense First) and 390 (Run and Gun), grade updated, hire in season 409, pool closed in season, unknown system 400, other team not editable, no impact key, no console errors, no horizontal scroll on Coach and Lineup. Helper's browser pass covered the offseason hire flow at both widths (coach changed, pool count dropped).
- Not tried: loading the owner's real v3 save (only the unit test), real-data league, fit grades on court tokens.

### 2026-11-16, badges built in the 2K style (plan piece 5)
- Owner redirected mid-build: "like NBA 2K, about 40 badges, stars with many". A first twelve-badge version with plain bonuses failed `TestTeamBalance` (73-9 team); replaced. Choices recorded in section 14 (2K-size, effects against the league average, ratings plus position, a web search on how 2K26 badges work was allowed).
- Built by two Sonnet helpers (engine + API, then screens). `engine/badges.go` (40 badges: 37 skill badges with tiers bronze/silver/gold/hof/legend, Leader/Mentor/Star personality badges at offset 100000), `engine/badge_thresholds.go`, saved `Player.PerfBadges map[string]int` (one-tier boosts from last season's stats; omitempty, no version bump). Each badge feeds one sim hook with a share; per hook effect = scale x (points^0.5 - league mean weighted by who uses the hook), means rebuilt each sim day, never saved. Clutch badges act in the last 12% of regulation or overtime within 5 points. Mentor: +5% growth for teammates 24 and under. Routes: `badges` on the player page (with `next`), `badgeCount` on roster rows, `GET /api/badges`, `?badge=` on Leaders. Screens: Badges section in the drawer (tier counts, grouped chips, tap for rule/effect/next, show all), count pill on roster rows, badge select on Leaders. `go run ./cmd/sim -mode badges`.
- Helper measurement: sanity 8000 games before/after points 109.9/110.2, FG% 47.3/47.3, 3P% 36.7/36.8, FT% 79.1/79.4, home wins 57.6/57.2. Seed 1 day 0: Bronze 3030, Silver 1684, Gold 757, HoF 289, Legend 31; 80+ players average 34-35 badges, other starters 26-27 (above the 12-25 target; lowering them would push the bench under 5), bench about 6. TestTeamBalance most wins 57-65 (limit 70) after one 25% effect cut; the first full-strength try failed both stability tests, so margins are thin. No test limit changed (only a wins log line added to TestTeamBalance).
- Verified by the main session: `go vet`, `go test ./... -count=1` (engine 187 s), web build. Port 8097 fictional seed 1: badge counts up to 40, median 12; Legend player (33 badges, 4 Legend, 17 HoF) drawer renders at 1400 and 390 px with show-all, no console errors, no horizontal scroll; catalogue 40; unknown `?badge=` gives 400; no impact key on any player page.
- Rough edges: a badge whose position limit is below Legend for every position (e.g. Rim Finisher, max Hall of Fame) still lists a Legend threshold in its rule text. Real-data league not tried.

### 2026-11-15, plan approved for pieces 5 and 3
- Section 14 records the build plan and four owner decisions (Mentor growth boost, coach Development speeds growth, offseason-only coaching changes, small fit effect). Nothing built yet.

### 2026-11-15, automatic lineup plays the best five (section 5 problem 2)
- Owner chose "the automatic lineup plays the best players". A Sonnet helper changed `Team.Rotation()` (engine/generate.go): the five healthy players with the highest overall start, bench best-first; new `Team.StarterSlots()` seats them in PG..C by trying all 120 seatings for the best fit, used by `AutoLineup`. Positions still do not affect the sim.
- **Test change, owner approved** ("over 30 seasons a little leeway is fine"): `TestLongStability` drift limit 2.15 -> 2.5 (seed 5 at -2.19) and final-season FT% ceiling 80 -> 81 (80.10).
- Helper measurement, one season, minutes per game by overall band, fictional seed 1 before/after: <50 6.3/5.8, 50-59 12.3/10.4, 60-69 23.9/24.4, 70-79 31.2/33.0, 80+ 34.2/34.4 (real data similar). Sanity 2000 games: points 110.0/109.9, home wins 56.5/57.7%, FT% 78.7/79.0.
- Verified by the main session: `go vet`, `go test ./... -count=1`, web build. Port 8097 fictional seed 1 team 0: starters are the five best (80, 77, 77, 68, 65), the 60-rated Marcus Bello is on the bench with 14 minutes, Lineup page has no console errors at 1400 and 390 px.
- Rough edge: on the court view the "out of position" caption can overlap a neighbouring token's avatar.

### 2026-11-15, training focus and learning speeds (plan piece 4)
- Built by two Sonnet helpers (engine + API, then screens). `engine/training.go`: saved `Player.Focus` (omitempty, zero = Balanced), `Aptitude` per rating from offset 90001 (Slow 0.9 / Normal 1.0 / Fast 1.12, nothing saved), age windows `Progression.GrowthWindowClose` (Inside and Rebounding close at 29, Athleticism at 25, others open). Growth steps are weighted (favoured x1.6, others x0.85, times speed and window) and rescaled so total growth is unchanged; decline untouched. `ProgressWeighted` beside `ProgressMinutes`. Computer teams pick a focus each offseason by the largest raw gap in open favoured ratings (no position weights; 30+ stay Balanced). Route `POST /api/training`; `training` on the player page, `focus` on user roster rows. Training panel in the drawer, focus label on roster rows. `cmd/sim -mode develop -focus auto|balanced`.
- Main session found that the first auto rule (position weight x gap) pushed growth into the ratings that count most in Overall (seed 3 drift +2.59); the helper switched it to the raw gap and softened speeds.
- **Test change, owner approved:** `TestLongStability` drift limit 2.0 -> 2.15 (seed 5 at -2.03; drifts -0.29, -1.34, +1.22, -0.11, -2.03).
- Helper measurement, develop share within 4 (seeds 1-4, 16 seasons): main 60.9%, balanced 59.4%, auto 65.0% (at the band edge).
- Verified by the main session: `go vet`, `go test ./... -count=1` (engine 120 s), web build. Port 8097 fictional seed 1: focus changed in the drawer at 1400 and 390 px (saved, message, roster label), other team read-only, bad key 400, other team 409, no impact key, no console errors, no horizontal scroll.
- Not tried: real-data league, an old save, how a focus plays out over seasons in the browser. The drawer header is cramped at 390 px (older issue).

### 2026-11-15, what players want, and extensions (plan piece 2)
- Built by two Sonnet helpers in turn (engine + API, then screens against the final JSON). `engine/motivation.go`: `MotivationOf` (offset 90000, nothing saved), `StanceOf` (role vs rank and planned minutes, team direction, tenure, can the team pay), `ExtensionAsk`/`Extend`; saved `Player.TeamSince`, `Contract.NextSalary/NextYears`, `FreeAgency.Prev` (all omitempty, no version bump). `ReSign` and computer re-signing refuse "test" players. `playersDecide` weights salary/winning/role by motivation plus a loyalty bonus. Routes `GET /api/contracts/extension/{id}`, `POST /api/contracts/extend`. Screens: "What he wants" panel and extension form in the drawer, stance chips on roster and expiring rows, High values on free-agent rows. New `go run ./cmd/sim -mode resign`.
- **Test change, owner approved:** `TestLongStability` 90+ limit for seeds 2 to 5 raised from 18 to 20 (seed 4 reached 19; peaks now 14/18/10/19/13 against 13/16/11/14/14; no drift in averages).
- Helper measurement (seed 1, 6 offseasons, before/after): re-signed 59/53, 44/38, 70/54, 105/92, 75/74, 78/76; reaching free agency up 6 to 12 per offseason; league payroll up to 3 points of the cap lower; about 22 to 25% of expiring players say "test". Computer teams never extend.
- Verified by the main session: `go vet`, `go test ./... -count=1` (engine 50 s), web build. Port 8097 fictional seed 1: stance chips on the roster, a test-stance/not-last-year refusal shown in the drawer at 1400 and 390 px, team 1 player extended through the drawer form (Extended line, Ext badge, second ask refused), no impact key, no console errors, no horizontal scroll.
- Not tried in the browser: the Free Agency page values chips (no free agents in season), extension in the offseason, an old save. Note for the owner's test stretch: on team 0 at day 0, 6 of 13 players say "test" (a Seller team with a deep bench); tunables are in `MotivationTuning` if it feels too harsh.

### 2026-11-15, player photos and avatars shown (plan piece 6)
- Built by one Sonnet helper (server and screens together, one written contract; cheaper than two for a piece this small). New `cmd/web/photos.go`, `-photos` flag, `photo` key on player rows, `GET /api/photo/{id}` and `/credit`, `Avatar` in the drawer, roster rows and court tokens. Main session moved the credit line to a full-width line in the drawer and added `cleanArtist` (Commons author text had "File:x.jpg:" prefixes and line breaks).
- Verified by the main session: `go vet`, `go test ./... -count=1` (engine 50 s), web build. Port 8097, real-data league seed 1: 13 of 13 Dallas roster rows have photos, all 13 images load at 1400 and 390 px, drawer shows photo and credit, court tokens show photos, photo route 200 image/jpeg, unknown id and `..%2Fx` 404, no impact key, no console errors; fictional league: 0 rows with photos (initials avatars).
- Not tried: a photo that fails to load in the browser (fallback only read in code), free agency and trade screens (no Avatar there yet), the Leaders page (rows not built with `addDev`, no `photo`).

### 2026-11-15, build plan approved for pieces 6, 2 and 4
- The owner approved the plan in section 13 with the three recommended choices, and will play a test stretch and report changes. Pieces 5 and 3 wait. Nothing was built for these pieces yet. The owner is moving to a new chat; section 0 and the prompt in section 10 are current.

### 2026-11-15, player photos downloaded (not yet shown in the game)
- Result (main session ran the status command and read the log): 1,754 photos saved, 0 left, 212 players without one (205 have no image on Wikidata, 7 had a missing file or a licence we do not accept; listed in `data/real/photos/skipped.csv`). 85 MB. Every photo has a row in `data/real/photos/credits.csv` (author, licence, source page). Licences are Creative Commons or public domain.
- How: the owner asked for real player pictures for local use and allowed browsing and downloading. Official NBA and Basketball-Reference headshots were not used (copyrighted, and both sites limit bots). One Wikidata query on property P2685 (Basketball-Reference NBA player ID, the same ID as `Player.RealID`) maps each player to a Wikimedia Commons file, so matching is exact. A first name-search script was slow and was replaced.
- Tool: `data/real/phototool/photos.py` with `README.txt`, written by a separate Haiku session as the owner asked. Standalone, standard library only, self-pacing on HTTP 429, resumable, lock file, `--status`, `--refresh-wikidata`. The last run saved 1,446 photos in about 53 minutes with no block. Earlier runs were blocked after about 150 thumbnails; the cause was not proven (the first script asked for a non-standard 300 pixel size and retried quickly).
- Private: everything is under `data/real/`, which git ignores. Nothing was committed or pushed.
- Not done: the game does not display photos or avatars yet (plan piece 6; changes listed in `ARCHITECTURE.md` section 16). Only two photos were opened by eye; the rest are trusted to the exact ID match. The script's 429 waiting, size fallback and batch splitting were never exercised in a real run.
- Before going live (owner will say when): follow the removal steps in `ARCHITECTURE.md` section 16.

### 2026-11-15, mid-level exception, restricted free agency, stability test reworked
- Owner asked why an over-the-cap team can only sign minimum players and wanted the real NBA rules. Checked online (NBA 2026-27: cap $164.96M, mid-level $15.04M up to 4 years, taxpayer mid-level $6.06M up to 2 years, none above the second apron; restricted free agents after a rookie deal or 3 or fewer years, old team may match an offer sheet of 2+ years) and scaled to our $140M cap.
- **Exception** (`engine/contract.go`): `Cap.MidLevel` 12800 / 4 years if payroll after signing stays at or under the tax line, else `Cap.TaxMidLevel` 5200 / 2 years under the hard ceiling. One signing per offseason per team (`Team.ExceptionYear`, no version bump). New `SignException` kind, `ExceptionFor`. Computer teams use the mid-level version only, with an open roster spot.
- **Restricted free agency** (`engine/freeagency.go`): a player whose contract expires with `Experience <= 4` (`FATuning.RFAMaxExperience`) is restricted; `FreeAgency.Rights` records the rights team. Another team's offer must be 2+ years and is an offer sheet. A computer rights team matches if it values the player and stays at or under the tax line. For the user a pending sheet (`FreeAgency.Sheets`) blocks the next day and the season start until answered (`AnswerSheet`; route `POST /api/freeagents/sheet`). An unanswered sheet at `FinishOffseason` counts as declined; only the fully automatic `Offseason()` matches for the user with the computer rule. Not built: the room and bi-annual exceptions, qualifying offers as a separate step.
- **Screens**: offer sheet card (Match / Let him go), exception box, RFA chips and filter, minimum years on the offer form, "via" on offers, matched lines in the log, RFA note on the expiring card, dashboard notice.
- **`TestLongStability` reworked with the owner's approval** (this changes assertions, so it is recorded here). Before: seed 1 only, every single season's healthy-rotation average within 2.0 of the start. The new rules made season 3 read 2.52; a helper traced it to 49 injured players on the measuring day, with roster quality slightly higher, and showed the code on `main` already exceeded 2.0 on four of five other seeds. Now: seeds 1 to 5 run side by side; drift is the last-ten-season average within 2.0 of the start; any single season within 4.0; players rated 90+ between 2 and 15 on seed 1 as before, and 2 to 18 on seeds 2 to 5, which were measured for the first time (peaks 13, 16, 11, 14, 14 with the new rules; 13, 14, 11, 13, 15 on `main`). The final-season averages check still runs on seed 1. The suite is slower for it: five leagues of 30 seasons.
- Measured (main session, new code, five seeds): last-ten-season drift +0.23, +0.06, +1.93, -0.28, -1.45; top-10 picks within 4 of draft potential 633/1050 (60%; 62% on `main`). Helper measurement (seed 5, hands-off): league payroll 107, 105, 102, 100, 98, 100, 98% of the cap against 107, 105, 103, 101, 97, 100 before; 4 to 9 computer exception signings and 0 to 1 offer sheets per offseason; unsigned players at the end 42 to 109 against 47 to 122 before.
- Verified by the main session: `go vet`, `go test ./... -count=1` (engine 181 s on a loaded machine), web build. On port 8097: a user's restricted player got a $9.4M 4-year sheet, the next day was refused until answered, Match kept him over the cap, Let him go sent him to the offering team; with the user $16.3M over the cap a $4.1M offer went out and signed via the exception, a second was refused, the box read "Already used this offseason"; no hidden number. Found and fixed: Match blanked the page (the screen expected the bare view but the route returns `{signings, freeAgency, state, voided}`; the brief was ambiguous).
- Not tried: the taxpayer exception, a sheet the user cannot afford, a voided sheet, phone width for the new cards, an old save mid-free-agency in the browser, real-data mode. Rough edges: when an exception offer is already pending, a second offer's refusal talks only about cap room; the note under the payroll bar still says offers above the minimum need cap room.
- Photos: a separate Haiku session wrote `data/real/phototool/photos.py` (standalone, self-pacing, resumable, lock file, `--status`; README beside it) and left it running detached. Source is Wikimedia Commons through one Wikidata query on the Basketball-Reference ID (1,761 of 1,966 players have a free photo). Wikimedia's image server allows about 150 thumbnails then blocks for minutes, so the run is slow. Private, never committed.

### 2026-11-15, plan approved, court view for the lineup (plan piece 1)
- The owner approved the six-piece plan in section 12. A Haiku helper is downloading free-licence player photos into `data/real/photos/` (private; see piece 6).
- Piece 1 built: `web/src/CourtLineup.tsx`, a half-court picture on the Lineup page with five spots and the bench. Tap a spot then a bench player (or the other way round), tap two spots to swap, × to remove, drag and drop on desktop. A player in a slot that is not in his position label gets an amber "out of position" ring. It edits the same starters state as the dropdowns; no API or engine change.
- Verified by the main session: web build; on port 8097 (server built from `main`, screens from the working tree) the court drew, a bench centre placed at PG showed out of position and "Unsaved changes", × removed a starter, phone width had no sideways scroll, no console errors.
- Not tried: drag and drop, saving a lineup made on the court, swapping two starters by tapping two spots. Rough edge: at phone width the last name in a token is clipped by the × button.

### 2026-11-15, Teams tab and team direction
- Owner asked to see which teams are proposing trades, a Teams tab with everything, and which teams are buying, selling or chasing a title. Engine: `engine/teamstatus.go` (`TeamStatuses`, `TeamStatusOf`, bands in `StatusBands`: Contender 0.8, Buyer 0.5, Seller 0.2 on the contend factor; the Buyer line is the same constant the proposal code uses). API: `GET /api/teams` (record, rank, payroll, cap room, top-8 average, best player, status, pending offers) and `status` on the team page, trade block and offers. Screens: `pages/Teams.tsx` (offers at the top, filters, sortable table, legend), `StatusChip` on the team page, Trade screen and offers.
- Verified by the main session: `go vet`, `go test ./... -count=1`, web build; on port 8097 an offer arrived on day 4 and the Teams page showed it with the proposer's status, the table, filters row and team page chip, at wide and phone width, no console errors. Status counts seen: 6 contenders, 9 buyers, 9 sellers, 6 rebuilding.
- Not tried: clicking each filter and column sort by hand, the chip on the Trade screen, Accept/Decline from the Teams page. `status` on `/api/offers` has no test.
- Owner decisions the same day: add a mid-level exception and restricted free agency, modelled on the real NBA rules (next piece). The owner allowed browsing to check the real rules.

### 2026-11-14, watch the next game
- Owner asked to be able to watch the game when playing just the next day. New menu action `watch_day` ("Watch next game") in `NextActions` (`engine/phase.go`); the front end opens the sim window for one day with the live game forced on (`SimPlan.watch`), titled "Game day". "Sim next day" itself is unchanged (instant). `phase_test.go` expects the new action in the menu list (a spec change, not a weakened test).
- Verified by the main session: `go vet`, `go test ./... -count=1`, web build; on port 8097 the menu showed the entry, the game played live, 64x finished it, the window showed the final and standings, no console errors. `main` was pushed earlier today at 84cace9 with the owner's go-ahead; this piece is merged locally only.
- Not tried: the entry on the day before the trade deadline (the deadline pause should show first), phone width for this entry.

### 2026-11-14, in-season progression
- Owner asked for it (idea 2 in section 11). Design chosen by the main session: four steps a season (after days 16, 32, 48, 64), half of the expected yearly trend arrives in them, and the offseason subtracts what already arrived, so the yearly total keeps its average. Own random stream 80000 (hashed per player). Switch `NoInSeasonDev` on the Settings page. New saved field `Player.SeasonDev` (nil in old saves, no version bump). Old saves get the feature from their next step day.
- Arrows now show this season's change once the first step has happened, otherwise last year's total (`changeScope`). The sim window lists the user's players who moved ("Development"). Live game got a 64x speed. Risers and fallers in the offseason report now use the whole-year change.
- New measuring mode kept in the repository: `go run ./cmd/sim -mode develop -seasons 16` (flag `-noinseason` to compare). It prints the share of top-10 picks whose best overall ends within 4 points of draft-day potential, over 4 leagues.
- Verified by the main session: `go vet`, `go test ./... -count=1`, web build. Measurement (seeds 1 to 4, 16 seasons, 320 picks): ON 63.4% within 4 points (per seed 59, 66, 60, 69), OFF 68.4% (72, 75, 64, 62); 90+ players at the end 4 to 10 ON, 7 to 9 OFF; average overall 63.6 to 65.7 ON, 63.2 to 65.0 OFF. The gap between ON and OFF is inside the seed-to-seed spread; no drift seen. Note this metric uses the best overall reached, which may not be exactly how the original 55 to 65% target was measured (OFF reads 68%). API pass on port 8097: no changes before day 16, six user players moved on day 16, year total equals season plus offseason, no hidden number. Browser: Development box in the sim window, Settings switch shown, no console errors.
- Helper change to a test fixture (reported, read by the main session): `TestFreeAgencyFlowAndOffers` (seed 21) could no longer find a suitable star free agent because the league evolves differently, so its fixture league now runs with `NoInSeasonDev` on. No assertion or number was changed.
- Not tried: toggling the Settings switch by hand, the arrow tooltips, a mid-season old save, real-data mode, a commissioner edit mid-season (it is not counted in `SeasonDev`), more than four seeds. A serious injury's rating drop is included in the year total shown.

### 2026-11-14, progression arrows, live game in the sim window, offers in the sim window
- Owner decisions: the live game plays inside the sim window (not the full viewer); the +/- shows last offseason's change; in-season progression is the next progression piece, not built yet. Section 11 (GM-mode gap list and progression ideas) was added as a reference.
- Engine: `Player.Dev` (`DevChange`, set in `ProgressMinutes`, nil for rookies and old saves, no save-version bump). `SimDay` records play-by-play for the user's game only (`SimGameRec`), kept in `LastBox`.
- API: `ovrChange` / `potChange` on player rows through `addDev` (roster, trade block, offers, lineup, free agents, expiring, injuries), `ratingChanges` on the player page, `hasReplay` on sim day results. Never on prospects.
- Screens: `Delta` arrows on TeamPage, Lineup, Trade block, offers, FreeAgency, ExpiringCard and the player drawer; `LiveGame.tsx` with a "Watch my games" switch in `SimOverlay`; offers with Accept / Decline / Change it inside the sim window (`OfferList`, `useOffers` in `OffersCard.tsx`).
- Verified by the main session: `go vet`, `go test ./... -count=1`, `npm run build`; a full season cycle on port 8097 through the API (deltas present, none for rookies, no hidden number, 328 events for a regular-season game); in the browser pane: arrows on the roster and drawer, a live game playing, Skip to final, an offer shown in the window, Decline, Accept ("Trade complete"), Keep simming, phone width, no console errors. One layout fault found and fixed (player names squeezed out of the offer rows).
- Not tried: Stop during a live game, the 1x and 16x speed buttons, "Change it" from the sim window, the deadline pause after this change, an old save in the browser, real-data mode. No arrows on the Trade page's selected-asset list, Leaders, Injuries card or Dashboard (they show no overall).
- Known rough edge: a full game at the default 4x takes about a minute, so watching every game of a season is slow; use 16x or Skip.

### 2026-11-14, documents and push
- Added `RULES.md` (rules) and `ARCHITECTURE.md` (how the code is built). The owner asked for `main` to be pushed to GitHub with these documents; see the git log for whether the push went through.

### 2026-11-14, full refresh
- Rewrote this file from the long log below: stale sections (old html pages, "no tests in cmd", "no git repository", old next steps) removed; code maps moved out to `ARCHITECTURE.md`, rules to `RULES.md`. Verified: `go vet` and `go test ./... -count=1` pass; flags in `cmd/*/main.go` read; `SaveVersion` 3, `TradeDeadlineDay` 50, `PotentialBlurSD` 4.0 and the tracked `cmd/web/web` confirmed in the code. Nothing was run in a browser for this refresh.

### 2026-11-14, whole day in one entry
Built and merged to `main`, in order: save versioning; draft picks as assets; multi-team deals; trade negotiation; injuries and commissioner mode; stage rules into `engine/phase.go`; contracts with cap, tax and ceiling; free agency; lineups with minutes-based development; draft lottery, scouting and blurred real-data ceilings. Each had an end-to-end pass on a throwaway server (details in section 3). A copy of the owner's old save and real-data mode each ran a full season cycle. Earlier the same day: the interface was rebuilt in React, a 3D court viewer and trade screen were added, shot tendency was added, stars per draft class were fixed at two. The findings and open items are all in section 5.
