package main

import (
	"fmt"
	"net/http"

	"hoopsim/engine"
)

// motivationObj is what a player cares about, as words.
func (s *server) motivationObj(p *engine.Player) M {
	m := s.lg.MotivationOf(p)
	return M{"money": m.Money.String(), "winning": m.Winning.String(), "role": m.Role.String(), "loyalty": m.Loyalty.String()}
}

// addStance puts the short stance (no reasons) and the extended flag on a
// row for a player on the user's team.
func (s *server) addStance(row M, p *engine.Player) {
	st := s.lg.StanceOf(p)
	row["stance"] = M{"key": st.Key, "label": st.Label}
	if p.Contract.NextYears > 0 {
		row["extended"] = true
	}
}

// extensionAsk answers GET /api/contracts/extension/{id}: the ask, or why not
// (a refusal is a normal answer here, so it is a 200).
func (s *server) extensionAsk(r *http.Request) (any, error) {
	id, err := pathID(r)
	if err != nil {
		return nil, err
	}
	lg := s.lg
	sal, years, err := lg.ExtensionAsk(lg.UserTeamID, id)
	if err != nil {
		return M{"ok": false, "reason": err.Error()}, nil
	}
	p := s.lg.FindPlayer(id)
	return M{"ok": true, "salary": sal, "maxSalary": lg.ExtensionLimit(p), "maxYears": years, "current": p.Contract.Salary}, nil
}

// extend answers POST /api/contracts/extend.
func (s *server) extend(r *http.Request) (any, error) {
	var in struct {
		PlayerID int `json:"playerId"`
		Salary   int `json:"salary"`
		Years    int `json:"years"`
	}
	if err := readJSON(r, &in); err != nil {
		return nil, err
	}
	if err := s.lg.Extend(s.lg.UserTeamID, in.PlayerID, in.Salary, in.Years); err != nil {
		return nil, conflict("%v", err)
	}
	p := s.lg.FindPlayer(in.PlayerID)
	return M{"message": fmt.Sprintf("%s agreed to an extension of $%.1fM for %s.", p.Name, float64(in.Salary)/1000, yearsStr(in.Years)), "state": s.state()}, nil
}

func yearsStr(n int) string {
	if n == 1 {
		return "1 year"
	}
	return fmt.Sprintf("%d years", n)
}
