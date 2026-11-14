package main

import (
	"net/http"
	"sort"

	"hoopsim/engine"
)

// faDay is the 1-based day shown to the user.
func faDay(fa *engine.FreeAgency) int { return min(fa.Day+1, fa.Days) }

// faState is the small summary on the main state, only while free agency is open.
func (s *server) faState() M {
	fa := s.lg.FA
	return M{"day": faDay(fa), "days": fa.Days, "daysLeft": fa.DaysLeft()}
}

func (s *server) signingRow(g engine.FASigning) M {
	return M{"day": g.Day + 1, "playerId": g.PlayerID, "name": g.Name, "pos": g.Pos, "ovr": g.Overall,
		"team": s.team(g.Team), "salary": g.Salary, "years": g.Years, "mine": g.Team == s.lg.UserTeamID}
}

func (s *server) freeAgencyView() M {
	lg := s.lg
	u := lg.UserTeamID
	out := M{"open": lg.FA != nil, "day": 0, "days": 0, "daysLeft": 0, "payroll": s.payrollObj(u),
		"rosterCount": len(lg.Teams[u].Roster), "rosterSize": engine.RosterSize,
		"minSalary": engine.Cap.MinSalary, "maxYears": engine.Cap.MaxContractYears,
		"players": []M{}, "offers": []M{}, "log": []M{}}
	fa := lg.FA
	if fa == nil {
		return out
	}
	out["day"], out["days"], out["daysLeft"] = faDay(fa), fa.Days, fa.DaysLeft()
	mine := map[int]engine.FAOffer{}
	names := map[int]string{}
	offers := []M{}
	for _, o := range lg.OffersBy(u) {
		mine[o.PlayerID] = o
	}
	pool := append([]*engine.Player(nil), lg.FreeAgents...)
	sort.SliceStable(pool, func(i, j int) bool {
		if a, b := pool[i].Overall(), pool[j].Overall(); a != b {
			return a > b
		}
		return pool[i].ID < pool[j].ID
	})
	rows := []M{}
	for _, p := range pool {
		names[p.ID] = p.Name
		sal, yrs := lg.FreeAgentAsk(p)
		row := M{"id": p.ID, "name": p.Name, "pos": p.PosLabel(), "age": p.Age, "ovr": p.Overall(), "pot": p.Potential,
			"tier": engine.PlayerTier(p), "askSalary": sal, "askYears": yrs, "interest": lg.OffersFor(p.ID)}
		if o, ok := mine[p.ID]; ok {
			row["myOffer"] = M{"id": o.ID, "salary": o.Salary, "years": o.Years, "day": o.Day + 1}
		}
		addInjury(row, p)
		addDev(row, p)
		rows = append(rows, row)
	}
	for _, o := range lg.OffersBy(u) {
		offers = append(offers, M{"id": o.ID, "playerId": o.PlayerID, "name": names[o.PlayerID],
			"salary": o.Salary, "years": o.Years, "day": o.Day + 1})
	}
	log := []M{}
	for i := len(fa.Log) - 1; i >= 0; i-- {
		log = append(log, s.signingRow(fa.Log[i]))
	}
	out["players"], out["offers"], out["log"] = rows, offers, log
	return out
}

func (s *server) freeAgents(r *http.Request) (any, error) { return s.freeAgencyView(), nil }

func (s *server) faBegin(r *http.Request) (any, error) {
	if err := s.lg.CanBeginFreeAgency(s.reportSeen); err != nil {
		return nil, conflict("%v", err)
	}
	if err := s.lg.BeginFreeAgency(); err != nil {
		return nil, conflict("%v", err)
	}
	return M{"freeAgency": s.freeAgencyView(), "state": s.state()}, nil
}

func (s *server) faOffer(r *http.Request) (any, error) {
	var in struct {
		PlayerID int `json:"playerId"`
		Salary   int `json:"salary"`
		Years    int `json:"years"`
	}
	if err := readJSON(r, &in); err != nil {
		return nil, err
	}
	if _, err := s.lg.MakeOffer(s.lg.UserTeamID, in.PlayerID, in.Salary, in.Years); err != nil {
		return nil, conflict("%v", err)
	}
	return s.freeAgencyView(), nil
}

func (s *server) faWithdraw(r *http.Request) (any, error) {
	var in struct {
		OfferID int `json:"offerId"`
	}
	if err := readJSON(r, &in); err != nil {
		return nil, err
	}
	if err := s.lg.WithdrawOffer(s.lg.UserTeamID, in.OfferID); err != nil {
		return nil, conflict("%v", err)
	}
	return s.freeAgencyView(), nil
}

func (s *server) faAdvance(r *http.Request) (any, error) {
	if err := s.lg.CanAdvanceFreeAgency(); err != nil {
		return nil, conflict("%v", err)
	}
	signed, err := s.lg.AdvanceFreeAgency()
	if err != nil {
		return nil, conflict("%v", err)
	}
	rows := []M{}
	for _, g := range signed {
		rows = append(rows, s.signingRow(g))
	}
	return M{"signings": rows, "freeAgency": s.freeAgencyView(), "state": s.state()}, nil
}
