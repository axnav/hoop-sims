package main

import (
	"fmt"
	"net/http"

	"hoopsim/engine"
)

// trainingObj is the training block of the player page: the focus, the menu,
// whether the user can change it and each skill's learning speed and window.
func (s *server) trainingObj(p *engine.Player, editable bool) M {
	opts := []M{}
	for _, f := range engine.AllFocuses() {
		opts = append(opts, M{"key": f.Key(), "label": f.Label()})
	}
	skills := []M{}
	for _, l := range s.lg.TrainingView(p) {
		skills = append(skills, M{"rating": l.Key, "speed": l.Speed, "open": l.Open, "stepsFor2": l.StepsFor2})
	}
	return M{"focus": p.Focus.Key(), "options": opts, "skills": skills,
		"editable": editable}
}

// setTraining answers POST /api/training {playerId, focus}.
func (s *server) setTraining(r *http.Request) (any, error) {
	var in struct {
		PlayerID int    `json:"playerId"`
		Focus    string `json:"focus"`
	}
	if err := readJSON(r, &in); err != nil {
		return nil, err
	}
	f, ok := engine.FocusFromKey(in.Focus)
	if !ok {
		return nil, bad("That is not a training focus.")
	}
	if s.inPool(in.PlayerID) {
		return nil, missing("no such player")
	}
	if err := s.lg.SetFocus(s.lg.UserTeamID, in.PlayerID, f); err != nil {
		return nil, conflict("%v", err)
	}
	p := s.lg.FindPlayer(in.PlayerID)
	return M{"training": s.trainingObj(p, true), "message": fmt.Sprintf("%s will train: %s.", p.Name, f.Label())}, nil
}
