package main

import (
	"net/http"
	"sort"
	"strconv"

	"hoopsim/engine"
)

// injuryObj is the JSON form of a player's injury.
func injuryObj(in *engine.Injury) M {
	return M{"name": in.Name, "gamesLeft": in.GamesLeft, "serious": in.Serious}
}

// addInjury puts an "injury" key on a player row, only when injured.
func addInjury(row M, p *engine.Player) {
	if p != nil && p.Injured() {
		row["injury"] = injuryObj(p.Injury)
	}
}

// devChange picks the change to show: this season's in-season change if there
// is one (scope "season"), else the last yearly change (scope "year").
func devChange(p *engine.Player) (*engine.DevChange, string) {
	if p == nil {
		return nil, ""
	}
	if p.SeasonDev != nil {
		return p.SeasonDev, "season"
	}
	if p.Dev != nil {
		return p.Dev, "year"
	}
	return nil, ""
}

// addDev puts "ovrChange", "potChange" and "changeScope" on a non-prospect
// player row, once the player has had any progression.
func addDev(row M, p *engine.Player) {
	if d, scope := devChange(p); d != nil {
		row["ovrChange"], row["potChange"], row["changeScope"] = d.Overall, d.Potential, scope
	}
}

func (s *server) injuryLines(lines []engine.InjuryLine) []M {
	out := []M{}
	for _, l := range lines {
		row := M{"playerId": l.PlayerID, "name": l.Name, "team": s.team(l.TeamID),
			"injury": l.Injury, "gamesLeft": l.GamesLeft, "serious": l.Serious}
		pl := s.lg.FindPlayer(l.PlayerID)
		addDev(row, pl)
		s.addPhoto(row, pl)
		out = append(out, row)
	}
	return out
}

func (s *server) injuries(r *http.Request) (any, error) {
	tid := s.lg.UserTeamID
	if v := r.URL.Query().Get("team"); v == "all" {
		tid = -1
	} else if v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 || n >= len(s.lg.Teams) {
			return nil, bad("bad team")
		}
		tid = n
	}
	lines := s.lg.InjuryReport(tid)
	sort.SliceStable(lines, func(i, j int) bool {
		if lines[i].GamesLeft != lines[j].GamesLeft {
			return lines[i].GamesLeft > lines[j].GamesLeft
		}
		return lines[i].PlayerID < lines[j].PlayerID
	})
	return M{"injuries": s.injuryLines(lines)}, nil
}

func (s *server) adminPlayer(r *http.Request) (any, error) {
	id, err := pathID(r)
	if err != nil {
		return nil, err
	}
	var in struct {
		Age       *int           `json:"age"`
		Potential *int           `json:"potential"`
		Ratings   map[string]int `json:"ratings"`
		Heal      *bool          `json:"heal"`
		Overall   *int           `json:"overall"`
	}
	if err := readJSON(r, &in); err != nil {
		return nil, err
	}
	if !s.lg.Options.Commissioner {
		// an empty edit returns the engine's own refusal message
		return nil, conflict("%v", s.lg.EditPlayer(id, engine.PlayerEdit{}))
	}
	if s.findPlayer(id) == nil {
		return nil, missing("no such player")
	}
	if s.inPool(id) {
		return nil, conflict("prospects cannot be edited before the draft")
	}
	e := engine.PlayerEdit{Age: in.Age, Potential: in.Potential, Ratings: in.Ratings, Heal: in.Heal, Overall: in.Overall}
	if err := s.lg.EditPlayer(id, e); err != nil {
		return nil, bad("%v", err)
	}
	return s.player(r)
}
