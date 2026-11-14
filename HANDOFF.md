# HoopSim handoff

Last full refresh: 2026-11-14. Module `hoopsim`, Go (standard library only) with a React + TypeScript front end.

Legend: **verified** means a person or session ran it or read the code and saw it. **reported** means a helper said so and it was not re-run.

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
- **Save versioning** (`engine/save.go`). Saves carry a version; an older one is upgraded on load, a newer one is refused. Current version 3. Verified: a COPY of the owner's own save (no version, year 2032) loaded, was upgraded with picks and contracts, and ran a full season cycle.
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
- **Screens** (React in `web/`). Dashboard, roster, lineup, trade, free agency, draft, offseason, injuries, settings, standings, playoff bracket, game viewer (2D and a lazy-loaded 3D court), history.

## 4. Decisions made and why

Owner decisions and standing preferences:
- Simulation only. The default game is fictional. Real-data mode is for private local testing and must never ship (the data's licence is unclear; lookalike names were rejected because being identifiable, not spelling, is the legal risk).
- Trade screen keeps its numbers and also shows plain tier labels. Teams negotiate (a mood and a concrete ask) instead of flatly refusing.
- Soft cap with a luxury tax and a hard ceiling, no long list of exceptions. Free agency by offers over several days. Scouting by fuzzy ranges narrowed with points. Lineups by starters plus minutes sliders. Injuries mostly missed games with rare serious ones, switchable off. A commissioner mode.
- Later, not built: player morale and trade requests, owner goals and job security, a news feed, coaches.
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
2. **Automatic lineup and positions.** The game sim ignores positions (only ratings and minutes count), but the automatic rotation picks starters by position, so weak players can start over far better ones (a 48-rated forward started over an 80-rated 20-year-old). This is why some weak players get heavy minutes league-wide. Choose between "the automatic lineup plays the best players" and "make positions matter in the game sim". The first is simpler.
3. **Pick values are small next to stars.** A top pick is worth about 32, a 99-rated star about 180. Untuned; nobody measured what top picks are worth right after the draft, so picks may be under- or over-priced (possible exploit either way).
4. **Fictional rosters are very top-heavy.** For example a 97 next to several players under 50 on one team.
5. **Retirement may be too eager for stars.** A 91-rated 35-year-old retired in a test league.
6. **Few trades in real-data mode.** Only one trade was logged in a year, against 26 in the owner's fictional save. Computer teams may trade less under the salary rules there.
7. **Hands-off teams suffer.** A team that ignores re-signing and free agency ends with leftovers (a hands-off team went 4-78 in a helper test). An over-the-cap team can only sign players whose ask is the minimum. This is intended but harsh; the screens warn before both steps.
8. **Money is thin.** A released player's contract simply ends (no money still owed). The luxury tax is recorded but has no consequence because teams have no finances. When a full roster receives extra players the game releases the lowest-valued ones without asking.
9. **Tests.** `TestFreeAgencyMarket` was loosened by a helper (top ten signing early: at least 4, was 5). `TestLongStability`'s home-win band (54 to 60%) sits near its edge, and 30-season stability was only fully checked on a few seeds. The engine trade tests halve salaries in a fixture. Engine tests take 30 to 40 seconds. `cmd/web` has API tests; `cmd/sim` and `cmd/realcheck` have none.
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
C. Later features the owner listed: player morale and trade requests; owner goals and job security (this would also give the luxury tax a consequence through team finances); a news feed; coaches.
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
Then restate in plain words what you understand the state to be, and propose the first
item from HANDOFF.md section 7 (the open decisions) as a simple either/or question
with your recommendation. Wait for my answer before changing code.
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
2. In-season progression: split the yearly change into monthly steps so ratings move during the season (needs its own random stream and a re-check of the "55 to 65% reach potential" target).
3. Training focus: the user picks a focus per player (shooting, defense, body) that tilts which ratings get the growth, not how much.
4. Performance-based growth: playing above expectation adds a little, for all ages (today only minutes count, and only for ages 24 and under).
5. Different peak ages per player (late bloomers, early decliners) from a hidden per-player number, which would also help problem 1 (star variety).
6. Mentors (a veteran at the same position helps a young player) and, once coaches exist, coach development ratings.
7. Injuries that cost development time (a long absence slows that year's growth).

## Update log (newest first)

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
