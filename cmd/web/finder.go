package main

import (
	"net/http"

	"hoopsim/engine"
)

// ---- trade finder (engine/finder.go) ----

type findIn struct {
	Players []int        `json:"players"`
	Picks   []dealPickIn `json:"picks"`
	Want    string       `json:"want"`
}

func (s *server) tradeFind(r *http.Request) (any, error) {
	var in findIn
	if err := readJSON(r, &in); err != nil {
		return nil, err
	}
	if in.Want == "" {
		in.Want = "any"
	}
	if !engine.ValidFindWant(in.Want) {
		return nil, bad("Unknown filter.")
	}
	if len(in.Players)+len(in.Picks) == 0 {
		return nil, bad("Tick the players and picks you want to offer.")
	}
	if len(in.Players) > engine.MaxTradePlayers {
		return nil, bad("You can offer at most %d players in one deal.", engine.MaxTradePlayers)
	}
	if len(in.Picks) > engine.MaxDealPicks {
		return nil, bad("You can offer at most %d draft picks in one deal.", engine.MaxDealPicks)
	}
	user := s.lg.UserTeamID
	var give []engine.Asset
	for _, id := range in.Players {
		give = append(give, engine.PlayerOf(id))
	}
	for _, k := range in.Picks {
		give = append(give, engine.PickOf(k.Year, k.Round, k.Original))
	}
	res, err := s.lg.FindOffers(give, in.Want)
	if err != nil {
		return nil, bad("%v", err)
	}

	wants := []M{}
	for _, w := range engine.FindWants() {
		wants = append(wants, M{"key": w[0], "label": w[1]})
	}
	giveRows := []M{}
	for _, a := range give {
		giveRows = append(giveRows, s.takeAsset(user, a))
	}
	offers := []M{}
	for _, o := range res.Offers {
		recv := []M{}
		for _, a := range o.Receive {
			recv = append(recv, s.takeAsset(o.Team, a))
		}
		moves := []M{}
		for _, m := range o.Deal(user, give).Moves {
			moves = append(moves, s.wireMove(m))
		}
		offers = append(offers, M{"team": s.team(o.Team), "direction": statusM(s.lg.TeamStatusOf(o.Team)),
			"receive": recv, "value": round1(o.Value), "salaryIn": o.SalaryIn, "salaryOut": o.SalaryOut,
			"note": o.Note, "deal": M{"moves": moves}})
	}
	passed := []M{}
	for _, p := range res.Passed {
		passed = append(passed, M{"team": s.team(p.Team), "reason": p.Reason})
	}
	return M{"possible": res.Possible, "reason": res.Reason, "want": in.Want, "wants": wants,
		"give": giveRows, "giveValue": round1(res.GiveValue), "offers": offers, "passed": passed}, nil
}
