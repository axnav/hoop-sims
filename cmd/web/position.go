package main

import (
	"net/http"

	"hoopsim/engine"
)

var posByName = map[string]engine.Position{"PG": engine.PG, "SG": engine.SG, "SF": engine.SF, "PF": engine.PF, "C": engine.C}

// addPosition adds posOverall and canSetPosition to a player page (not for
// prospects in the pool, whose true ratings stay secret).
func (s *server) addPosition(out M, p *engine.Player, retired bool) {
	if s.inPool(p.ID) {
		return
	}
	po := p.PositionOveralls()
	out["posOverall"] = M{"PG": po[0], "SG": po[1], "SF": po[2], "PF": po[3], "C": po[4]}
	out["canSetPosition"] = !retired && s.lg.CanSetPositions(p)
}

// setPosition handles POST /api/player/{id}/position {pos, alt}.
func (s *server) setPosition(r *http.Request) (any, error) {
	id, err := pathID(r)
	if err != nil {
		return nil, err
	}
	var in struct {
		Pos string `json:"pos"`
		Alt string `json:"alt"`
	}
	if err := readJSON(r, &in); err != nil {
		return nil, err
	}
	main, ok := posByName[in.Pos]
	if !ok {
		return nil, bad("pos must be PG, SG, SF, PF or C")
	}
	var second *engine.Position
	if in.Alt != "" {
		a, ok := posByName[in.Alt]
		if !ok {
			return nil, bad("alt must be empty, PG, SG, SF, PF or C")
		}
		if a == main {
			return nil, bad("the second position must differ from the main one")
		}
		second = &a
	}
	p := s.findPlayer(id)
	if p == nil {
		return nil, missing("no such player")
	}
	if !s.lg.CanSetPositions(p) {
		return nil, conflict("you can only change the position of your own players")
	}
	if err := s.lg.SetPositions(id, main, second); err != nil {
		return nil, conflict("%v", err)
	}
	return s.player(r)
}
