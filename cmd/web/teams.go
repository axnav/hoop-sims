package main

import (
	"net/http"

	"hoopsim/engine"
)

// statusObj is a team's direction (contender, buyer, seller, rebuilding).
func (s *server) statusObj(id int) M {
	return statusM(s.lg.TeamStatusOf(id))
}

func statusM(st engine.TeamStatus) M {
	return M{"key": st.Key, "label": st.Label, "wants": st.Wants}
}

// teamsList is one row per team, ordered by team ID.
func (s *server) teamsList(r *http.Request) (any, error) {
	lg := s.lg
	sts := lg.TeamStatuses()
	offers := map[int]int{}
	for _, p := range lg.PendingProposals() {
		offers[p.Team]++
	}
	rows := []M{}
	for id, t := range lg.Teams {
		st := lg.Season
		row := M{"team": s.team(id), "w": st.Wins[id], "l": st.Losses[id], "confRank": s.rank(id),
			"payroll": lg.Payroll(id), "capRoom": lg.CapRoom(id), "rosterSize": len(t.Roster),
			"top8": round1(lg.TeamTop8(id)), "status": statusM(sts[id]), "offers": offers[id],
			"isUser": id == lg.UserTeamID}
		var best *engine.Player
		for _, p := range t.Roster {
			if best == nil || p.Overall() > best.Overall() {
				best = p
			}
		}
		if best != nil {
			row["star"] = M{"id": best.ID, "name": best.Name, "pos": best.PosLabel(), "ovr": best.Overall()}
		}
		rows = append(rows, row)
	}
	return M{"teams": rows}, nil
}
