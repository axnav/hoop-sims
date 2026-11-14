# HoopSim

Text/web basketball GM simulation in Go (module `hoopsim`, standard library only). Simulation only, no on-court graphics. Mode: GM (run one team). Original game: fictional league by default. Full briefing: **read `HANDOFF.md` first** (state, code map, design decisions, known problems, next steps).

## Standing rules
- The owner is learning Go, speaks informally (often by voice). Restate your understanding in plain words, ask simple either/or design questions, write the code for them. Short plain-language explanations; brief "insight" notes on why.
- Save tokens: the main session designs, briefs and reviews; small-model subagents write and test code. ONE layer only (subagents never spawn subagents), one or two at a time, briefs name the files each may edit so two never touch the same file. No separate sessions. No nitpicking polish, but verify results independently.
- Ask before: downloading anything; restarting the web server (port 8080; a league may be in progress and unsaved progress is lost); anything that leaves the machine.
- A git repository exists (one initial commit; `data/real/` and `saves/` are ignored). Never commit or push unless asked.
- Report honestly: what you ran and saw vs what a helper reported.
- Keep the handoff current: after each finished piece of work or owner decision, add a few lines to the "Update log" in `HANDOFF.md` yourself (cheap, no subagent). Run the `handoff-writer` subagent only for a full refresh when the owner asks or the log gets long.

## Hard constraints
- The hidden number (`Player.Impact`) must never appear in the UI.
- `data/real/` (Basketball-Reference-derived CSVs) is for private local testing only: never ship, redistribute or commit it. The default game stays fictional; no lookalike names of real players.
- Engine randomness only through `engine.RNG` (same seed, same result). Tunables live in `Progression` (progression.go) and `RealTuning` (realdata.go); `game.go` holds the possession constants.

## Commands
Shell may need `export PATH="/usr/local/bin:$PATH"`. Run from the project root.
- `cd web && npm install && npm run build` : build the React app into `web/dist` (needed once, and after UI changes)
- `go run ./cmd/web` : JSON API + built app on http://localhost:8080 (`-port`, `-save`, `-web` flags; preview name `hoopsim` in `.claude/launch.json`)
- `cd web && npm run dev` : Vite dev server (proxies `/api` to Go on port 8095, so run `go run ./cmd/web -port 8095` too)
- `go run ./cmd/sim -mode sanity` : league-average game check
- `go run ./cmd/sim -mode ratings` : rating and rookie distributions by position/draft tier
- `go run ./cmd/sim -mode seasons -seasons 10` : multi-season run (add `-reveal` to debug the hidden number)
- `go run ./cmd/realcheck -runs 30` : compare real-data league vs real 2010-11 results
- `go vet ./... && go test ./...` : checks (engine has the tests; add `-count=1` to skip cache)

## Layout
- `engine/` game logic (player, generate, game, season, playoffs, offseason, progression, history, save, realdata, realteams, rng)
- `cmd/web/` JSON API (main.go routes, state.go flow/sim, league.go pages, flow.go playoffs/offseason/draft, trade.go trades API, colors.go, api_test.go), `cmd/sim/`, `cmd/realcheck/`
- `web/` React + TypeScript app (Vite, Tailwind v4, react-query, motion); `src/replay.ts` is the plain-TS game replay logic (shared by the 2D and lazy 3D court in `src/court3d/`); `pages/Trade.tsx` is the trade screen; `web/dist` is the build output
- `data/real/` private CSVs, `saves/league.json` current save

## Next
Per-player usage tendency (scorer/passer distribution), then injuries and rest; full ordered list in `HANDOFF.md` section 9.
