package main

import (
	"hoopsim/engine"
)

// addMorale puts "morale" ({score, label}) on a row for a player with a team
// (not prospects or free agents), and "tradeRequest": true only while he has
// asked to be traded.
func (s *server) addMorale(row M, p *engine.Player) {
	if p == nil || s.lg == nil {
		return
	}
	score, label, ok := s.lg.MoraleBrief(p)
	if !ok {
		return
	}
	row["morale"] = M{"score": score, "label": label}
	if p.TradeRequest > 0 {
		row["tradeRequest"] = true
	}
}

// teamMoraleObj is the team's mood, minutes weighted.
func (s *server) teamMoraleObj(t *engine.Team) M {
	score, label := s.lg.TeamMorale(t)
	return M{"score": score, "label": label}
}

// eventRows lists the morale events (trade requests, withdrawals, trades of
// requesters), newest first, for the transactions list. Day is 1-based.
func (s *server) eventRows(limit int) []M {
	rows := []M{}
	ev := s.lg.Events
	for i := len(ev) - 1; i >= 0 && len(rows) < limit; i-- {
		e := ev[i]
		row := M{"type": e.Type, "year": e.Year, "day": e.Day + 1, "playerId": e.PlayerID, "name": e.Name, "team": s.team(e.TeamID)}
		switch e.Type {
		case engine.EvTradeRequest:
			row["text"] = e.Name + " has asked to be traded."
		case engine.EvRequestWithdrawn:
			row["text"] = e.Name + " withdrew his trade request."
		case engine.EvRequestTraded:
			row["text"] = e.Name + " was traded after asking out."
		}
		rows = append(rows, row)
	}
	return rows
}
