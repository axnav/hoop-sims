package main

import (
	"net/http"

	"hoopsim/engine"
)

// ---- trade assistant (engine/assistant.go) ----

func (s *server) tradeNeeds(r *http.Request) (any, error) {
	id, err := pathID(r)
	if err != nil {
		return nil, err
	}
	if id < 0 || id >= len(s.lg.Teams) {
		return nil, missing("no such team")
	}
	n := s.lg.NeedsOf(id)
	return M{"team": s.team(id), "direction": M{"key": n.Status.Key, "label": n.Status.Label, "wants": n.Status.Wants},
		"values": n.Values, "lacks": n.Lacks, "summary": n.Summary}, nil
}

// takeAsset is one asset of a package, as a row with a kind.
func (s *server) takeAsset(owner int, a engine.Asset) M {
	row := s.assetRow(engine.Move{From: owner, To: -1, Asset: a})
	if a.Kind == engine.PlayerAsset {
		row["id"] = a.PlayerID
	}
	return row
}

func (s *server) whatWouldItTake(r *http.Request) (any, error) {
	id, err := pathID(r)
	if err != nil {
		return nil, err
	}
	if s.lg.FindPlayer(id) == nil {
		return nil, missing("no such player")
	}
	res, err := s.lg.WhatWouldItTake(id)
	if err != nil {
		return nil, bad("%v", err)
	}
	user := s.lg.UserTeamID
	var row M
	for _, a := range engine.TradeBlock(s.lg, res.Team) {
		if a.PlayerID == id {
			row = s.playerRow(res.Team, a)
		}
	}
	if row == nil { // draft night: an expiring contract is not on the block
		p := res.Player
		row = M{"id": p.ID, "name": p.Name, "pos": p.PosLabel(), "age": p.Age, "ovr": p.Overall(), "pot": p.Potential}
		addContract(row, p)
	}
	pkgs := []M{}
	for _, pk := range res.Packages {
		give, receive := []M{}, []M{}
		for _, a := range pk.Give {
			give = append(give, s.takeAsset(user, a))
		}
		for _, a := range pk.Receive {
			receive = append(receive, s.takeAsset(res.Team, a))
		}
		moves := []M{}
		for _, m := range pk.Deal(user, res.Team).Moves {
			moves = append(moves, s.wireMove(m))
		}
		pkgs = append(pkgs, M{"label": pk.Label, "give": give, "receive": receive, "note": pk.Note,
			"cost": round1(pk.Cost), "cheapest": pk.Cheapest, "deal": M{"moves": moves}})
	}
	return M{"player": row, "team": s.team(res.Team), "possible": res.Possible, "reason": res.Reason, "packages": pkgs}, nil
}

func (s *server) tradeAdvisor(r *http.Request) (any, error) {
	rows, summary := s.lg.AdvisorOf()
	user := s.lg.UserTeamID
	out := []M{}
	for _, a := range rows {
		p := a.Player
		out = append(out, M{"id": p.ID, "name": p.Name, "pos": p.PosLabel(), "age": p.Age, "ovr": p.Overall(), "pot": p.Potential,
			"salary": p.Contract.Salary, "years": p.Contract.Years, "advice": a.Advice, "reasons": a.Reasons})
	}
	return M{"team": s.team(user), "direction": statusM(s.lg.TeamStatusOf(user)), "players": out, "summary": summary}, nil
}
