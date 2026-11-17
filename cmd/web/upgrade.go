package main

import (
	"net/http"

	"hoopsim/engine"
)

// ---- find an upgrade (engine/upgrade.go) ----

// upgradeRow is a trade-block row for a player on team owner's roster, the
// same row whatWouldItTake builds (a plain row when the block leaves him out,
// as on draft night for an expiring contract).
func (s *server) upgradeRow(owner int, p *engine.Player, block map[int][]engine.TradeAsset) M {
	if _, ok := block[owner]; !ok {
		block[owner] = engine.TradeBlock(s.lg, owner)
	}
	for _, a := range block[owner] {
		if a.PlayerID == p.ID {
			return s.playerRow(owner, a)
		}
	}
	row := M{"id": p.ID, "name": p.Name, "pos": p.PosLabel(), "age": p.Age, "ovr": p.Overall(), "pot": p.Potential}
	addContract(row, p)
	return row
}

// takePackageM is one package as whatWouldItTake builds it.
func (s *server) takePackageM(user, team int, pk engine.TakePackage) M {
	give, receive := []M{}, []M{}
	for _, a := range pk.Give {
		give = append(give, s.takeAsset(user, a))
	}
	for _, a := range pk.Receive {
		receive = append(receive, s.takeAsset(team, a))
	}
	moves := []M{}
	for _, m := range pk.Deal(user, team).Moves {
		moves = append(moves, s.wireMove(m))
	}
	return M{"label": pk.Label, "give": give, "receive": receive, "note": pk.Note,
		"cost": round1(pk.Cost), "cheapest": pk.Cheapest, "deal": M{"moves": moves}}
}

func (s *server) tradeUpgrade(r *http.Request) (any, error) {
	id, err := pathID(r)
	if err != nil {
		return nil, err
	}
	res, err := s.lg.FindUpgrades(id)
	if err != nil {
		return nil, bad("%v", err)
	}
	user := s.lg.UserTeamID
	block := map[int][]engine.TradeAsset{}
	ups := []M{}
	for _, u := range res.Upgrades {
		row := M{"player": s.upgradeRow(u.Team, u.Player, block), "team": s.team(u.Team),
			"direction": statusM(u.Direction), "gain": u.Gain, "shopped": u.Shopped, "shopReason": u.ShopReason,
			"possible": u.Possible, "reason": u.Reason}
		if u.Package != nil {
			row["package"] = s.takePackageM(user, u.Team, *u.Package)
		}
		ups = append(ups, row)
	}
	return M{"player": s.upgradeRow(user, res.Player, block), "possible": res.Possible, "reason": res.Reason, "upgrades": ups}, nil
}
