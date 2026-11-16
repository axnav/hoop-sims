package main

import (
	"errors"
	"fmt"
	"net/http"

	"hoopsim/engine"
)

// ---- owner goals and finances (engine/owner.go) ----

func ownerPersonalityObj(key string) M {
	label, blurb := engine.OwnerPersonalityText(key)
	return M{"key": key, "label": label, "blurb": blurb}
}

func ownerMarketObj(key string) M {
	return M{"key": key, "label": engine.OwnerMarketLabel(key)}
}

func ownerWeightLabel(w int) string {
	switch {
	case w >= 3:
		return "High"
	case w == 2:
		return "Medium"
	}
	return "Low"
}

// ownerStateObj is the small "owner" key of /api/state.
func (s *server) ownerStateObj() M {
	lg := s.lg
	o := lg.Owner
	if o == nil {
		return M{"trust": 0, "label": "", "fired": false, "alert": nil}
	}
	var alert any
	if a := lg.OwnerAlert(); a != "" {
		alert = a
	}
	return M{"trust": o.Trust, "label": engine.OwnerTrustLabel(o.Trust), "fired": o.Fired, "alert": alert}
}

// ownerBody is the body of GET /api/owner.
func (s *server) ownerBody() M {
	lg := s.lg
	o := lg.Owner
	out := M{"firing": !lg.Options.NoFiring}
	if o == nil {
		out["active"] = false
		out["owner"] = M{"name": "", "personality": M{"key": "", "label": "", "blurb": ""}}
		out["trust"] = M{"score": 0, "label": "", "trend": "flat", "lastChange": 0}
		out["goals"] = []M{}
		out["money"] = M{"budget": 0, "payroll": 0, "overBudget": 0, "marketSize": M{"key": "", "label": ""},
			"revenue": M{"market": 0, "wins": 0, "playoffs": 0, "total": 0}, "costs": M{"salaries": 0, "deadMoney": 0, "tax": 0, "total": 0},
			"profit": 0, "projected": false}
		out["ledger"], out["messages"], out["jobs"] = []M{}, []M{}, []M{}
		out["fired"], out["firedText"] = false, ""
		return out
	}
	u := lg.UserTeamID
	info := lg.OwnerOf(u)
	out["active"] = true
	out["owner"] = M{"name": info.Name, "personality": ownerPersonalityObj(info.Personality)}
	out["trust"] = M{"score": o.Trust, "label": engine.OwnerTrustLabel(o.Trust), "trend": engine.OwnerTrend(o.LastChange), "lastChange": o.LastChange}

	goals := []M{}
	for i, g := range o.Goals {
		progress, status := lg.OwnerGoalStatus(i)
		goals = append(goals, M{"key": g.Key, "kind": g.Kind, "title": g.Title, "detail": g.Detail,
			"weight": ownerWeightLabel(g.Weight), "progress": progress, "status": status})
	}
	out["goals"] = goals

	m := lg.OwnerFinances()
	budget := lg.OwnerBudget()
	pay := lg.Payroll(u)
	out["money"] = M{"budget": budget, "payroll": pay, "overBudget": max(0, pay-budget), "marketSize": ownerMarketObj(info.Market),
		"revenue": M{"market": m.Market, "wins": m.Wins, "playoffs": m.Playoffs, "total": m.Revenue},
		"costs":   M{"salaries": m.Salaries, "deadMoney": m.DeadMoney, "tax": m.Tax, "total": m.Costs},
		"profit":  m.Profit, "projected": m.Projected}

	ledger := []M{}
	for i := len(o.Ledger) - 1; i >= 0; i-- {
		r := o.Ledger[i]
		gs := []M{}
		for _, g := range r.Goals {
			gs = append(gs, M{"title": g.Title, "met": g.Met})
		}
		ledger = append(ledger, M{"year": r.Year, "wins": r.Wins, "losses": r.Losses, "playoffs": r.Playoffs,
			"revenue": r.Revenue, "costs": r.Costs, "profit": r.Profit, "trustStart": r.TrustStart, "trustEnd": r.TrustEnd, "goals": gs})
	}
	out["ledger"] = ledger

	msgs := []M{}
	for i := len(o.Messages) - 1; i >= 0 && len(msgs) < 30; i-- {
		m := o.Messages[i]
		msgs = append(msgs, M{"year": m.Year, "day": min(m.Day+1, engine.SeasonDays), "text": m.Text, "tone": m.Tone})
	}
	out["messages"] = msgs

	out["fired"], out["firedText"] = o.Fired, o.FiredText
	jobs := []M{}
	for _, id := range lg.OwnerJobs() {
		oi := lg.OwnerOf(id)
		st := lg.TeamStatusOf(id)
		jobs = append(jobs, M{"team": s.team(id), "owner": M{"name": oi.Name, "personality": ownerPersonalityObj(oi.Personality)},
			"marketSize": ownerMarketObj(oi.Market), "record": fmt.Sprintf("%d-%d", lg.Season.Wins[id], lg.Season.Losses[id]),
			"payroll": lg.Payroll(id), "direction": M{"key": st.Key, "label": st.Label}})
	}
	out["jobs"] = jobs
	return out
}

func (s *server) ownerGet(r *http.Request) (any, error) { return s.ownerBody(), nil }

// ownerJob takes one of the jobs offered after the user was let go.
func (s *server) ownerJob(r *http.Request) (any, error) {
	var in struct {
		TeamID int `json:"teamId"`
	}
	if err := readJSON(r, &in); err != nil {
		return nil, err
	}
	msg, err := s.lg.TakeJob(in.TeamID)
	switch {
	case errors.Is(err, engine.ErrOwnerNotFired):
		return nil, conflict("%v", err)
	case err != nil:
		return nil, bad("%v", err)
	}
	return M{"message": msg, "owner": s.ownerBody(), "state": s.state()}, nil
}
