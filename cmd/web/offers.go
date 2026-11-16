package main

import (
	"net/http"

	"hoopsim/engine"
)

func (s *server) wireMove(m engine.Move) M {
	out := M{"from": m.From, "to": m.To}
	if m.Kind == engine.PlayerAsset {
		out["playerId"] = m.PlayerID
	} else {
		out["pick"] = M{"year": m.Year, "round": m.Round, "original": m.Original}
	}
	return out
}

// askLabel is a short display name for an asset the user would add.
func (s *server) askLabel(a engine.Asset, user int) string {
	if a.Kind == engine.PlayerAsset {
		if p := s.lg.FindPlayer(a.PlayerID); p != nil {
			return p.Name
		}
		return "player"
	}
	return s.pickLabelFor(user, a.Year, a.Round, a.Original)
}

// assetRow builds an offer asset row, using the owner's trade-block view.
func (s *server) assetRow(m engine.Move) M {
	owner := m.From
	if m.Kind == engine.PlayerAsset {
		for _, a := range engine.TradeBlock(s.lg, owner) {
			if a.PlayerID == m.PlayerID {
				r := s.playerRow(owner, a)
				row := M{"kind": "player", "playerId": a.PlayerID, "name": r["name"], "pos": r["pos"], "age": r["age"],
					"ovr": r["ovr"], "pot": r["pot"], "value": r["value"], "tier": r["tier"],
					"salary": r["salary"], "years": r["years"]}
				if inj, ok := r["injury"]; ok {
					row["injury"] = inj
				}
				for _, k := range []string{"ovrChange", "potChange"} {
					if v, ok := r[k]; ok {
						row[k] = v
					}
				}
				return row
			}
		}
		return M{"kind": "player", "playerId": m.PlayerID, "name": "?", "pos": "", "age": 0, "ovr": 0, "pot": 0, "value": 0.0, "tier": ""}
	}
	r := s.pickRow(owner, m.Year, m.Round, m.Original)
	return M{"kind": "pick", "year": m.Year, "round": m.Round, "original": r["original"], "label": r["label"],
		"value": r["value"], "tier": r["tier"]}
}

func (s *server) offerDTO(p engine.Proposal) M {
	user := s.lg.UserTeamID
	give, get, moves := []M{}, []M{}, []M{}
	for _, m := range p.Deal.Moves {
		if m.From == user {
			give = append(give, s.assetRow(m))
		} else {
			get = append(get, s.assetRow(m))
		}
		moves = append(moves, s.wireMove(m))
	}
	left := p.Expires - s.lg.Season.Day
	if left < 0 {
		left = 0
	}
	if p.DraftPick > 0 {
		left = 0
	}
	return M{"draftPick": p.DraftPick, "id": p.ID, "team": s.team(p.Team), "message": p.Message, "day": p.Day, "expires": p.Expires,
		"status": s.statusObj(p.Team), "daysLeft": left, "give": give, "get": get, "moves": moves}
}

func (s *server) offerList() []M {
	rows := []M{}
	for _, p := range s.lg.PendingProposals() {
		rows = append(rows, s.offerDTO(p))
	}
	return rows
}

func (s *server) offersList(r *http.Request) (any, error) {
	s.lg.EnsureDraftOffer()
	return M{"offers": s.offerList()}, nil
}

func (s *server) offerAccept(r *http.Request) (any, error) {
	id, err := pathID(r)
	if err != nil {
		return nil, err
	}
	if !s.hasOffer(id) {
		return nil, missing("no such offer")
	}
	res, err := s.lg.AcceptProposal(id)
	if err != nil {
		return nil, conflict("%v", err)
	}
	// On draft night the draft simply carries on: the new owner of the pick is on the clock.
	out := s.txDTO(res)
	out["state"] = s.state()
	return out, nil
}

func (s *server) hasOffer(id int) bool {
	for _, p := range s.lg.PendingProposals() {
		if p.ID == id {
			return true
		}
	}
	return false
}

func (s *server) offerReject(r *http.Request) (any, error) {
	id, err := pathID(r)
	if err != nil {
		return nil, err
	}
	if !s.lg.RejectProposal(id) {
		return nil, missing("no such offer")
	}
	return M{"ok": true, "offers": s.offerList()}, nil
}

func (s *server) optionsGet(r *http.Request) (any, error) {
	o := s.lg.Options
	return M{"noDeadlineStop": o.NoDeadlineStop, "noOfferStop": o.NoOfferStop,
		"noInjuries": o.NoInjuries, "noInSeasonDev": o.NoInSeasonDev, "commissioner": o.Commissioner, "noFiring": o.NoFiring}, nil
}

func (s *server) optionsSet(r *http.Request) (any, error) {
	var in struct {
		NoDeadlineStop *bool `json:"noDeadlineStop"`
		NoOfferStop    *bool `json:"noOfferStop"`
		NoInjuries     *bool `json:"noInjuries"`
		NoInSeasonDev  *bool `json:"noInSeasonDev"`
		Commissioner   *bool `json:"commissioner"`
		NoFiring       *bool `json:"noFiring"`
	}
	if err := readJSON(r, &in); err != nil {
		return nil, err
	}
	if in.NoDeadlineStop != nil {
		s.lg.Options.NoDeadlineStop = *in.NoDeadlineStop
	}
	if in.NoOfferStop != nil {
		s.lg.Options.NoOfferStop = *in.NoOfferStop
	}
	if in.NoInjuries != nil {
		s.lg.Options.NoInjuries = *in.NoInjuries
	}
	if in.NoInSeasonDev != nil {
		s.lg.Options.NoInSeasonDev = *in.NoInSeasonDev
	}
	if in.Commissioner != nil {
		s.lg.Options.Commissioner = *in.Commissioner
	}
	if in.NoFiring != nil {
		s.lg.Options.NoFiring = *in.NoFiring
	}
	return s.optionsGet(r)
}
