# HoopSim rules

The rules everyone working on this project follows, person or AI session. `CLAUDE.md` has the
short version that is loaded every session; `ARCHITECTURE.md` explains how the code is built;
`HANDOFF.md` records the current state and what is next. If a rule here conflicts with those
files, this file wins for rules and `HANDOFF.md` wins for current state.

## 1. Hard limits (never break these)

1. **The hidden number stays hidden.** `Player.Impact` must never appear in an API response or on
   a screen, and must not be editable. The test helper `checkNoImpact` in `cmd/web/api_test.go`
   guards this: run every new or changed response through it.
2. **Prospects are secret until drafted.** For a player still in the draft pool, no API response
   may carry his true overall, potential or individual ratings. Send scouting ranges only
   (`ScoutView`, `ScoutRatings`), and never sort a prospect list by true values.
3. **Real data is private.** `data/real/` is for local testing only. Never commit, ship or
   redistribute it. The default game is fictional, with no lookalike names of real players.
4. **The owner's game is not a test bed.** Never start, stop or restart the server on port 8080
   without asking, and never write to `saves/league.json`. Test on a copy.
5. **Ask before anything leaves the machine**: downloads, pushes, any outside service. A push
   needs the owner's go-ahead each time unless they have given a standing one.

## 2. Code structure

1. **`engine/` holds every game rule** and knows nothing about HTTP or JSON routes. If a handler
   needs a decision (is this allowed, what is this worth, who is eligible), add an engine function
   and call it. Plain sums for display are the only arithmetic allowed in `cmd/web`.
2. **`cmd/web/` only translates**: read the request, call the engine, build the response. Stage
   rules ("which phase is the league in, what can happen next") live in `engine/phase.go`.
3. **`web/src/` only shows what the API returns.** No game rules in the browser, no guessing
   hidden values, no recomputing what the server decides.
4. **One feature, one file where possible**: `engine/<feature>.go` with `engine/<feature>_test.go`,
   `cmd/web/<feature>.go`, `web/src/pages/<Feature>.tsx`.
5. **Standard library only on the Go side.** No new Go dependencies, and no new npm packages
   without asking.

## 3. Randomness and repeatability

1. All engine randomness goes through `engine.RNG`. Same seed, same result.
2. A new feature that needs random numbers gets **its own stream**, so it cannot change game
   results for a seed. Derive it with `lg.dayRNG(year, offset+n)` and record the offset here:

   | Offset | Used by |
   |---|---|
   | 0 to 81 | Games of each regular-season day |
   | 1000 + day | Playoff games |
   | 5000, 5007 | Offseason progression, retirement and rookies; padding a real draft class |
   | 7000, 7500 | Trades between computer teams (7500 is defined but unused) |
   | 9000 | Schedule |
   | 20000 + day | Trade proposals from computer teams |
   | 30000 + day | Injuries (playoffs use 30500 + day) |
   | 40000 | Contract lengths at league creation |
   | 50000 + day | Free agency bidding |
   | 60000 | Draft lottery |
   | 70000 to 70004 | Scouting errors, computer draft noise, repair of old mid-draft saves |

   | 80000 + step | In-season development (hashed per player, steps 1 to 4) |

   Take the next free ten-thousand (90000) for a new feature and add it here and to
   `ARCHITECTURE.md` section 9, which has the full table with the function that uses each.
3. Never use `math/rand` or the clock in `engine/` or `cmd/`.
4. Do not draw random numbers in an order that depends on map iteration.

## 4. Save files

1. `SaveVersion` and the `saveUpgrades` list are in `engine/save.go`. A save with no version is
   version 0.
2. When you add or change a saved field, first try to make its **zero value mean "the old
   behaviour"** (this is why options are named `NoInjuries`, `NoDeadlineStop`). Then no upgrade
   step is needed.
3. If an old save would be wrong without help, **bump `SaveVersion` and append one upgrade step**,
   and add a test that loads an older-style save.
4. A save made by a newer build is refused, never half-loaded.

## 5. Tunables

Numbers that shape the game live in named, commented config blocks, not scattered in code:
`Progression` (progression.go), `RealTuning` (realdata.go), `Cap` (contract.go), `FATuning`
(freeagency.go), `InjuryTuning` (injury.go), `ScoutTuning` and `LotteryOdds` (scouting.go,
lottery.go), the possession constants in game.go, the trade constants at the top of trade.go,
deal.go and negotiate.go. A new feature adds its own block the same way.

## 6. API conventions

1. Changes are **additive**: add keys, do not rename or remove ones the screens use.
2. Money is an integer in **thousands of dollars**. Screens format it with `money()` in
   `web/src/ui.tsx`.
3. Day numbers are **1-based in the API** (the engine counts from 0).
4. Errors carry a sentence the user can read, in the `error` key. Use 400 for a bad request,
   404 for something that does not exist, 409 for "not allowed right now".
5. Team objects come from `s.team(id)`; player rows come from the shared row helpers, so every
   screen shows the same fields.
6. TypeScript types in `web/src/api.ts` are hand-kept copies of the responses. When a response
   changes, change the type in the same piece of work.

## 7. Testing

1. **Run the checks once per finished piece of work**, not after every small step:
   `go vet ./... && go test ./... -count=1`, plus `cd web && npm run build` for screen changes.
   Run them again before a merge to `main` only if code changed since.
2. **Every feature gets one end-to-end pass before it is merged**: a throwaway server on another
   port with a scratch save (`go build -o <scratch>/hoopsim-test ./cmd/web`, then run it with
   `-port 8097 -save <scratch file>`), driven by a short script and headless Chromium, looking
   for wrong behaviour and console errors at wide and narrow widths. Unit tests have passed with
   real bugs present more than once; this pass is what found them.
3. For anything that plays out over years (payroll, ratings, free agency), **run several
   seasons** and report the numbers per season. Look for drift.
4. **Do not weaken a test to make it pass.** Fix the code or the fixture. If an assertion truly
   has to change, say so in the report and in `HANDOFF.md`.
5. Never run `go build` inside `cmd/web` without `-o` pointing outside the repository.
6. Say plainly what was run and seen, what a helper reported, and what was not tested.

## 8. Git

1. One branch per piece of work, named `feat/<topic>`. `main` only moves when the piece is
   finished and checked, by fast-forward.
2. Commit messages say what changed and why in plain words.
3. Never commit `data/real/`, `saves/`, `web/dist` or `node_modules`.
4. Pushing follows rule 1.5.

## 9. Working with AI helpers

1. The main session designs, writes the briefs, reviews and tests. Helpers write code.
   **The main session is responsible for the result.**
2. Give every helper a named model: a small model for searching, reading and measuring; a
   mid-size model for writing code. Never let a helper inherit the main model.
3. A brief states: the goal and why; the exact files the helper may edit; what it must not do
   (commit, start servers, touch saves or real data, spawn helpers); the exact function names or
   JSON keys it codes against; the tests to add; when to run the checks; and the report format.
4. One layer only: helpers never start helpers. Two helpers never edit the same file. When an
   API helper and a screen helper run together, both get the same written contract.
5. Resume a helper that was cut off instead of starting a new one.
6. Read what a helper changed, or test it, before merging. A helper's "it passes" is a report,
   not a verification.

## 10. Talking to the owner

1. The owner is learning Go and often speaks by voice. Restate the request in plain words first.
2. Ask simple either/or questions for design choices, with a recommendation. Decide the routine
   things yourself and say what you chose.
3. Write the code for them. Keep explanations short.
4. Report honestly, including what did not work.

## 11. Finishing a piece of work

A piece is done when: the code is in the right layer; tests cover the new rules; the checks
pass; the end-to-end pass has been run; old saves still load; `HANDOFF.md` has a few lines in
its update log (what changed, what was verified and how, what was not tried); and
`ARCHITECTURE.md` is updated if a file, route, saved field, random stream or stage was added.
