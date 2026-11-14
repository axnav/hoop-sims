package main

import (
	"net/http"
	"sort"

	"hoopsim/engine"
)

// payrollObj is the money summary of one team (thousands of dollars).
func (s *server) payrollObj(id int) M {
	lg := s.lg
	return M{"payroll": lg.Payroll(id), "cap": engine.Cap.SalaryCap, "taxLine": engine.Cap.TaxLine,
		"ceiling": engine.Cap.HardCeiling, "room": lg.CapRoom(id), "taxBill": lg.TaxBill(id),
		"minSalary": engine.Cap.MinSalary}
}

// addContract puts salary and years on a player row.
func addContract(row M, p *engine.Player) {
	if p != nil {
		row["salary"], row["years"] = p.Contract.Salary, p.Contract.Years
	} else {
		row["salary"], row["years"] = 0, 0
	}
}

// offseasonOpen is true in the offseason phases, when re-signing is possible.
func (s *server) offseasonOpen() bool {
	if s.lg.FA != nil {
		return false
	}
	switch s.lg.Phase(s.reportSeen) {
	case engine.PhaseOffseason, engine.PhaseDraft, engine.PhaseDraftDone:
		return true
	}
	return false
}

func (s *server) expiringView() M {
	lg := s.lg
	u := lg.UserTeamID
	ps := lg.Expiring(u)
	sort.SliceStable(ps, func(i, j int) bool { return ps[i].Overall() > ps[j].Overall() })
	rows := []M{}
	for _, p := range ps {
		sal, yrs := engine.AskingContract(p)
		problem := ""
		if err := lg.CanSign(u, sal, engine.SignOwn); err != nil {
			problem = err.Error()
		}
		row := M{"id": p.ID, "name": p.Name, "pos": p.PosLabel(), "age": p.Age, "ovr": p.Overall(),
			"pot": p.Potential, "tier": engine.PlayerTier(p), "askSalary": sal, "askYears": yrs,
			"canReSign": problem == "", "problem": problem}
		addDev(row, p)
		rows = append(rows, row)
	}
	return M{"players": rows, "payroll": s.payrollObj(u), "open": s.offseasonOpen()}
}

func (s *server) expiring(r *http.Request) (any, error) { return s.expiringView(), nil }

func (s *server) contractAction(fn func(team, id int) error) func(r *http.Request) (any, error) {
	return func(r *http.Request) (any, error) {
		var in struct {
			PlayerID int `json:"playerId"`
		}
		if err := readJSON(r, &in); err != nil {
			return nil, err
		}
		if err := fn(s.lg.UserTeamID, in.PlayerID); err != nil {
			return nil, conflict("%v", err)
		}
		return M{"expiring": s.expiringView(), "state": s.state()}, nil
	}
}
