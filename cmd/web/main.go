// Command web serves the HoopSim JSON API and the built React app (web/dist).
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"hoopsim/engine"
)

// M is a loose JSON object; responses are built from these so the hidden
// number can never leak through an engine struct.
type M = map[string]any

var confNames = [2]string{"West", "East"}

type server struct {
	mu         sync.Mutex
	lg         *engine.League
	reportSeen bool // the offseason report has been looked at; draft may start
	savePath   string
	realDir    string
	webDir     string

	metaOnce sync.Once
	fictTeam []M
	realTeam []M
}

type apiErr struct {
	code int
	msg  string
}

func (e apiErr) Error() string { return e.msg }

func bad(f string, a ...any) error      { return apiErr{400, fmt.Sprintf(f, a...)} }
func conflict(f string, a ...any) error { return apiErr{409, fmt.Sprintf(f, a...)} }
func missing(f string, a ...any) error  { return apiErr{404, fmt.Sprintf(f, a...)} }

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

// api wraps a handler: locks, requires a league if asked, maps errors to JSON.
func (s *server) api(needLeague bool, fn func(r *http.Request) (any, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		if needLeague && s.lg == nil {
			writeJSON(w, 409, M{"error": "no league loaded"})
			return
		}
		v, err := fn(r)
		if err != nil {
			var ae apiErr
			if errors.As(err, &ae) {
				writeJSON(w, ae.code, M{"error": ae.msg})
			} else {
				writeJSON(w, 500, M{"error": err.Error()})
			}
			return
		}
		writeJSON(w, 200, v)
	}
}

func readJSON(r *http.Request, v any) error {
	b, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil || len(b) == 0 {
		return err
	}
	if err := json.Unmarshal(b, v); err != nil {
		return bad("invalid JSON: %v", err)
	}
	return nil
}

func pathID(r *http.Request) (int, error) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		return 0, bad("bad id")
	}
	return id, nil
}

func fileExists(p string) bool { _, err := os.Stat(p); return err == nil }

// ---- teams ----

func (s *server) isReal() bool {
	if s.lg == nil {
		return false
	}
	for _, t := range s.lg.Teams {
		for _, p := range t.Roster {
			return p.RealID != ""
		}
	}
	return false
}

func teamRef(t *engine.Team, real bool) M {
	abbr, col := fictionalAbbr(t.City), fictionalColors(t.ID)
	if real {
		if a := engine.RealTeamAbbrevs(); t.ID < len(a) {
			abbr = a[t.ID]
			if c, ok := realColors[abbr]; ok {
				col = c
			}
		}
	}
	conf := engine.Conference(t.ID)
	return M{"id": t.ID, "abbr": abbr, "city": t.City, "name": t.Name, "fullName": t.FullName(),
		"primary": col[0], "secondary": col[1], "conf": conf, "confName": confNames[conf]}
}

func (s *server) team(id int) M {
	if s.lg == nil || id < 0 || id >= len(s.lg.Teams) {
		return M{"id": -1, "abbr": "-", "fullName": "-", "primary": "#444444", "secondary": "#888888", "conf": 0, "confName": ""}
	}
	return teamRef(s.lg.Teams[id], s.isReal())
}

func (s *server) loadTeamLists() {
	s.metaOnce.Do(func() {
		g := engine.GenerateLeague(1)
		for _, t := range g.Teams {
			s.fictTeam = append(s.fictTeam, teamRef(t, false))
		}
		if rl, err := engine.NewRealLeague(s.realDir, 1); err == nil {
			for _, t := range rl.Teams {
				s.realTeam = append(s.realTeam, teamRef(t, true))
			}
		}
	})
}

// ---- meta / new / load / save ----

func (s *server) meta(r *http.Request) (any, error) {
	s.loadTeamLists()
	return M{
		"loaded": s.lg != nil, "realAvailable": len(s.realTeam) > 0, "saveExists": fileExists(s.savePath),
		"fictionalTeams": s.fictTeam, "realTeams": s.realTeam, "conferences": confNames,
	}, nil
}

func (s *server) newGame(r *http.Request) (any, error) {
	var in struct {
		Mode string `json:"mode"`
		Seed uint64 `json:"seed"`
		Team int    `json:"team"`
	}
	if err := readJSON(r, &in); err != nil {
		return nil, err
	}
	if in.Seed == 0 {
		in.Seed = 1
	}
	if in.Team < 0 || in.Team >= engine.NumTeams {
		return nil, bad("team must be 0-%d", engine.NumTeams-1)
	}
	var lg *engine.League
	switch in.Mode {
	case "real":
		var err error
		if lg, err = engine.NewRealLeague(s.realDir, in.Seed); err != nil {
			return nil, bad("real league unavailable: %v", err)
		}
	case "", "fictional":
		lg = engine.GenerateLeague(in.Seed)
	default:
		return nil, bad("mode must be fictional or real")
	}
	lg.UserTeamID = in.Team
	lg.StartSeason()
	s.lg, s.reportSeen = lg, true
	return s.state(), nil
}

func (s *server) load(r *http.Request) (any, error) {
	lg, err := engine.Load(s.savePath)
	if err != nil {
		return nil, bad("load failed: %v", err)
	}
	s.lg, s.reportSeen = lg, true
	return s.state(), nil
}

func (s *server) save(r *http.Request) (any, error) {
	if err := s.lg.Save(s.savePath); err != nil {
		return nil, err
	}
	return M{"ok": true}, nil
}

// ---- static app ----

func (s *server) static(w http.ResponseWriter, r *http.Request) {
	index := filepath.Join(s.webDir, "index.html")
	if !fileExists(index) {
		http.Error(w, "The web app is not built. Run: cd web && npm install && npm run build", 503)
		return
	}
	clean := filepath.Join(s.webDir, filepath.FromSlash(filepath.Clean("/"+r.URL.Path)))
	if st, err := os.Stat(clean); err == nil && !st.IsDir() {
		http.ServeFile(w, r, clean)
		return
	}
	http.ServeFile(w, r, index)
}

func (s *server) routes() *http.ServeMux {
	mux := http.NewServeMux()
	g := func(p string, need bool, fn func(r *http.Request) (any, error)) {
		mux.HandleFunc("GET "+p, s.api(need, fn))
	}
	p := func(p string, need bool, fn func(r *http.Request) (any, error)) {
		mux.HandleFunc("POST "+p, s.api(need, fn))
	}
	g("/api/meta", false, s.meta)
	p("/api/new", false, s.newGame)
	p("/api/load", false, s.load)
	p("/api/save", true, s.save)
	g("/api/state", true, func(r *http.Request) (any, error) { return s.state(), nil })
	p("/api/sim", true, s.sim)
	g("/api/standings", true, s.standings)
	g("/api/leaders", true, s.leaders)
	g("/api/team/{id}", true, s.teamPage)
	g("/api/player/{id}", true, s.player)
	g("/api/schedule", true, s.schedule)
	g("/api/box/{id}", true, s.box)
	g("/api/history", true, s.history)
	g("/api/playoffs", true, s.playoffs)
	p("/api/playoffs/{action}", true, s.playoffAction)
	g("/api/game/{id}/events", true, s.gameEvents)
	g("/api/offseason", true, s.offseasonReport)
	p("/api/offseason/begin", true, s.offseasonBegin)
	p("/api/offseason/to-draft", true, s.offseasonToDraft)
	p("/api/offseason/finish", true, s.offseasonFinish)
	g("/api/draft", true, s.draftState)
	p("/api/draft/pick", true, s.draftPick)
	p("/api/draft/step", true, s.draftStep)
	p("/api/draft/auto", true, s.draftAuto)
	g("/api/trade/status", true, s.tradeStatus)
	g("/api/trade/block/{id}", true, s.tradeBlock)
	p("/api/trade/evaluate", true, s.tradeEvaluate)
	p("/api/trade/counter", true, s.tradeCounter)
	p("/api/trade/execute", true, s.tradeExecute)
	g("/api/transactions", true, s.transactions)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 404, M{"error": "not found"})
	})
	mux.HandleFunc("/", s.static)
	return mux
}

func main() {
	port := flag.Int("port", 8080, "port to listen on")
	save := flag.String("save", engine.SavePath, "save file path")
	web := flag.String("web", "web/dist", "built web app directory")
	flag.Parse()
	s := &server{savePath: *save, realDir: "data/real", webDir: *web}
	addr := fmt.Sprintf(":%d", *port)
	log.Printf("listening on http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, s.routes()))
}
