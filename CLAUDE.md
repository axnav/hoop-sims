# HoopSim

Text/web basketball GM simulation in Go (module `hoopsim`, standard library only) with a React + TypeScript front end. Simulation only, no on-court graphics. Mode: GM (run one team). The default league is fictional.

Read `HANDOFF.md` first (state, decisions, known problems, next steps), then `RULES.md` (the full rules). Open `ARCHITECTURE.md` when you need to find or change code.

## Standing rules (short; full text in RULES.md)
- The owner is learning Go and often dictates by voice. Restate the request in plain words, ask simple either/or design questions with a recommendation, write the code for them, keep explanations short.
- The main session designs, briefs and reviews. Helpers write code. ALWAYS give a helper a named model: small for searching and measuring, mid-size for writing code. One layer only (helpers never start helpers), one or two at a time, briefs name the files each may edit.
- Run the checks once per finished piece of work, not after every small step.
- Every feature gets one end-to-end pass on a throwaway server (port 8097, scratch save, headless Chromium) before merging.
- Do not weaken a test to make it pass.
- Commit and merge to `main` locally when the code is correct (standing permission). Push only with the owner's go-ahead. Never commit `data/real/`, `saves/`, `web/dist`, `node_modules`.
- Report honestly: what you ran and saw versus what a helper reported versus what was not tried.
- **Core rule: `HANDOFF.md` is always current** (RULES.md hard limit 6). After every finished piece, owner decision or push, and before a session ends or moves: update the log AND sections 0, 3, 4, 7, 10 where they changed; note work in progress (branch, done, left) in section 0. Write it yourself and commit it with the piece. A piece is not finished while the handoff is stale.
- **`ARCHITECTURE.md` matches the code** (RULES.md hard limit 7): a piece that adds or changes a file, route, page, saved field, stream, stage, test file or game rule updates it in the same commit; fix stale parts when you find them.
- Go tests stay beside the code (`engine/x.go` with `engine/x_test.go`); that is the Go standard, do not move them to a separate folder.
- Ask before: downloading anything; anything that leaves the machine; starting or restarting anything on port 8080.

## Hard constraints
- The hidden number `Player.Impact` never appears in the API or UI and is never editable (test helper `checkNoImpact` in `cmd/web/api_test.go`).
- Prospects in the draft pool are secret: send scouting ranges only, never true values.
- `data/real/` is private local testing data: never ship, redistribute or commit it. No lookalike names of real players.
- Randomness only through `engine.RNG`; a new feature gets its own random stream (table in `RULES.md` section 3). No `math/rand` or clock in `engine/` or `cmd/`.
- Saves are versioned (`SaveVersion` in `engine/save.go`, now 4): follow `RULES.md` section 4 when a saved field changes.
- Never touch port 8080 or `saves/league.json` without asking. Never run `go build` inside `cmd/web` without `-o` pointing outside the repo.
- Game rules live in `engine/`; `cmd/web/` only translates; `web/src/` only shows what the API returns.

## Commands
Shell may need `export PATH="/usr/local/bin:$PATH"`. Run from the project root.
- `go vet ./... && go test ./... -count=1` : checks (engine tests take 4 to 5 minutes; run in the background)
- `cd web && npm install && npm run build` : build the app into `web/dist` (once, and after screen changes)
- `go run ./cmd/web` : API plus built app on http://localhost:8080 (flags `-port`, `-save`, `-web`)
- `cd web && npm run dev` : Vite dev server (proxies `/api` to port 8095; also run `go run ./cmd/web -port 8095`)
- `go run ./cmd/sim -mode sanity|ratings|seasons` : engine checks (flags `-seed`, `-games`, `-seasons`, `-reveal`)
- `go run ./cmd/realcheck -runs 30` : compare the real-data league with real 2010-11 results (needs `data/real/`)

## Layout
- `engine/` game logic, one file per feature with its test
- `cmd/web/` JSON API; `cmd/sim/` and `cmd/realcheck/` command-line tools
- `web/` React app (Vite, Tailwind, react-query); `web/dist` is the build output
- `data/real/` private CSVs; `saves/league.json` the owner's current save

## Next
See `HANDOFF.md` section 0 (work in progress and next steps) and section 7.
