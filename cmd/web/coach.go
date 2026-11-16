package main

import (
	"math"
	"net/http"
	"sort"

	"hoopsim/engine"
)

// fitObj is a fit score (0-100, rounded) and its letter grade.
func fitObj(score float64) M {
	return M{"score": int(math.Floor(score + 0.5)), "grade": engine.GradeOf(score)}
}

func systemName(key string) string {
	if sy := engine.SystemByKey(key); sy != nil {
		return sy.Name
	}
	return ""
}

func coachRow(c *engine.Coach) M {
	return M{"name": c.Name, "age": c.Age, "offense": c.Offense, "defense": c.Defense, "development": c.Development,
		"system": c.System, "systemName": systemName(c.System)}
}

// coachTeamObj is the short coach block on the team page: the coach's name and
// the system the team runs now.
func (s *server) coachTeamObj(t *engine.Team) M {
	if t.Coach == nil {
		return nil
	}
	return M{"name": t.Coach.Name, "system": t.SystemKey(), "systemName": systemName(t.SystemKey())}
}

// canChangeSystem says whether the user may change the system now. Games are
// played a whole day at a time, so between calls the user is always between
// games; kept as one place to forbid it later.
func (s *server) canChangeSystem() error { return nil }

// coachObj describes a team's coach, systems and fit (see the JSON contract in
// ARCHITECTURE.md). Only the user's team is editable.
func (s *server) coachObj(id int) M {
	lg := s.lg
	t := lg.Teams[id]
	user := id == lg.UserTeamID
	sys := t.CurrentSystem()

	systems := []M{}
	for i := range engine.Systems {
		sy := &engine.Systems[i]
		systems = append(systems, M{"key": sy.Key, "name": sy.Name, "description": sy.Desc,
			"pace": sy.PaceWord(), "threes": sy.ThreesWord(), "starUsage": sy.StarWord(), "defense": sy.DefenseWord(),
			"wants": sy.Wants(), "teamFit": fitObj(lg.TeamFitUnder(t, sy.Key))})
	}

	mins := map[int]int{}
	for _, pm := range t.EffectiveMinutes() {
		mins[pm.Player.ID] = pm.Min
	}
	ps := append([]*engine.Player(nil), t.Roster...)
	sort.SliceStable(ps, func(i, j int) bool {
		a, b := ps[i], ps[j]
		if mins[a.ID] != mins[b.ID] {
			return mins[a.ID] > mins[b.ID]
		}
		if a.Overall() != b.Overall() {
			return a.Overall() > b.Overall()
		}
		return a.ID < b.ID
	})
	players := []M{}
	for _, p := range ps {
		f := lg.PlayerFit(t, p)
		fo := fitObj(f.Score)
		fo["reasons"] = append([]string{}, f.Reasons...)
		players = append(players, M{"id": p.ID, "name": p.Name, "pos": p.PosLabel(), "minutes": mins[p.ID], "fit": fo})
	}

	out := M{"system": sys.Key, "systemName": sys.Name, "favourite": t.OnFavourite(), "systems": systems,
		"teamFit": fitObj(lg.TeamFit(t)), "players": players,
		"canChangeSystem": user && s.canChangeSystem() == nil,
		"canHire":         user && lg.CoachPoolOpen(),
		"editable":        user}
	if t.Coach != nil {
		out["coach"] = coachRow(t.Coach)
	} else {
		out["coach"] = nil
	}
	return out
}

func (s *server) coachGet(r *http.Request) (any, error) {
	id, err := pathID(r)
	if err != nil {
		return nil, err
	}
	if id < 0 || id >= len(s.lg.Teams) {
		return nil, missing("no such team")
	}
	return s.coachObj(id), nil
}

func (s *server) coachSetSystem(r *http.Request) (any, error) {
	var in struct {
		System string `json:"system"`
	}
	if err := readJSON(r, &in); err != nil {
		return nil, err
	}
	if !engine.ValidSystemKey(in.System) {
		return nil, bad("unknown system %q", in.System)
	}
	if err := s.canChangeSystem(); err != nil {
		return nil, conflict("%v", err)
	}
	if err := s.lg.SetSystem(s.lg.UserTeamID, in.System); err != nil {
		return nil, bad("%v", err)
	}
	return s.coachObj(s.lg.UserTeamID), nil
}

func (s *server) coachPool(r *http.Request) (any, error) {
	list := []M{}
	for _, c := range s.lg.CoachPool {
		row := coachRow(c)
		row["id"] = c.ID
		list = append(list, row)
	}
	return M{"open": s.lg.CoachPoolOpen(), "candidates": list}, nil
}

func (s *server) coachHire(r *http.Request) (any, error) {
	var in struct {
		ID *int `json:"id"`
	}
	if err := readJSON(r, &in); err != nil {
		return nil, err
	}
	if in.ID == nil {
		return nil, bad("id is required")
	}
	if err := s.lg.HireCoach(*in.ID); err != nil {
		return nil, conflict("%v", err)
	}
	return s.coachObj(s.lg.UserTeamID), nil
}
