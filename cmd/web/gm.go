package main

import (
	"errors"
	"net/http"

	"hoopsim/engine"
)

// ---- assistant GM (engine/gm.go) ----

// gmAutoKeys maps the API's auto keys to the engine switches.
func gmAutoGet(a engine.GMAuto, key string) (bool, bool) {
	switch key {
	case "lineup":
		return a.Lineup, true
	case "training":
		return a.Training, true
	case "contracts":
		return a.Contracts, true
	case "freeAgency":
		return a.FreeAgency, true
	case "draft":
		return a.Draft, true
	case "tradeAlerts":
		return a.TradeAlerts, true
	}
	return false, false
}

func gmAutoSet(a *engine.GMAuto, key string, v bool) {
	switch key {
	case "lineup":
		a.Lineup = v
	case "training":
		a.Training = v
	case "contracts":
		a.Contracts = v
	case "freeAgency":
		a.FreeAgency = v
	case "draft":
		a.Draft = v
	case "tradeAlerts":
		a.TradeAlerts = v
	}
}

var gmAutoOrder = []string{"lineup", "training", "contracts", "freeAgency", "draft", "tradeAlerts"}

// gmAreaMeta is the per-area extras the engine list does not carry.
var gmAreaMeta = map[string][3]any{
	// key: autoKey, canAuto, autoLabel
	engine.GMAreaLineup:     {"lineup", true, "Keep my lineup right automatically"},
	engine.GMAreaTraining:   {"training", true, "Set training focuses automatically"},
	engine.GMAreaContracts:  {"contracts", true, "Re-sign and extend players automatically"},
	engine.GMAreaFreeAgency: {"freeAgency", true, "Make free agency offers automatically"},
	engine.GMAreaDraft:      {"draft", true, "Make my draft picks automatically"},
	engine.GMAreaTrades:     {"tradeAlerts", false, "Tell me about trade ideas while simming"},
}

func (s *server) gmView() M {
	lg := s.lg
	plans := []M{}
	for _, p := range engine.GMPlans() {
		plans = append(plans, M{"key": p[0], "label": p[1], "blurb": p[2]})
	}
	a := lg.GMAutoSettings()
	auto := M{}
	for _, k := range gmAutoOrder {
		v, _ := gmAutoGet(a, k)
		auto[k] = v
	}
	areas := []M{}
	for _, ar := range engine.GMAreas() {
		m := gmAreaMeta[ar[0]]
		areas = append(areas, M{"key": ar[0], "label": ar[1], "blurb": ar[2], "canAuto": m[1], "autoKey": m[0], "autoLabel": m[2]})
	}
	sugs := []M{}
	counts := M{"total": 0, "lineup": 0, "training": 0, "contracts": 0, "free_agency": 0, "draft": 0, "trades": 0}
	for _, g := range lg.GMSuggestions() {
		var pid, link any
		if g.PlayerID >= 0 {
			pid = g.PlayerID
		}
		if g.Link != "" {
			link = g.Link
		}
		reasons := g.Reasons
		if reasons == nil {
			reasons = []string{}
		}
		sugs = append(sugs, M{"id": g.ID, "area": g.Area, "title": g.Title, "reasons": reasons, "action": g.Action, "playerId": pid, "link": link})
		counts[g.Area] = counts[g.Area].(int) + 1
		counts["total"] = counts["total"].(int) + 1
	}
	log := []M{}
	entries := lg.GMLogEntries()
	for i := len(entries) - 1; i >= 0 && len(log) < 100; i-- {
		e := entries[i]
		log = append(log, M{"year": e.Year, "day": e.Day + 1, "area": e.Area, "text": e.Text, "auto": e.Auto})
	}
	return M{"plan": lg.GMPlan(), "plans": plans, "auto": auto, "areas": areas, "suggestions": sugs, "counts": counts, "log": log}
}

func (s *server) gmGet(r *http.Request) (any, error) { return s.gmView(), nil }

func (s *server) gmSettings(r *http.Request) (any, error) {
	var in struct {
		Plan *string         `json:"plan"`
		Auto map[string]bool `json:"auto"`
	}
	if err := readJSON(r, &in); err != nil {
		return nil, err
	}
	if in.Plan != nil && !engine.ValidGMPlan(*in.Plan) {
		return nil, bad("Unknown plan.")
	}
	a := s.lg.GMAutoSettings()
	for k, v := range in.Auto {
		if _, ok := gmAutoGet(a, k); !ok {
			return nil, bad("Unknown auto switch %q.", k)
		}
		gmAutoSet(&a, k, v)
	}
	if in.Plan != nil {
		s.lg.SetGMPlan(*in.Plan)
	}
	s.lg.SetGMAuto(a)
	return s.gmView(), nil
}

func (s *server) gmApply(r *http.Request) (any, error) {
	var in struct {
		ID string `json:"id"`
	}
	if err := readJSON(r, &in); err != nil {
		return nil, err
	}
	if in.ID == "" {
		return nil, bad("Say which suggestion.")
	}
	if len(in.ID) >= 6 && in.ID[:6] == "draft:" {
		if err := s.lg.CanDraft(s.reportSeen); err != nil {
			return nil, conflict("The draft has not started yet.")
		}
	}
	msg, err := s.lg.GMApply(in.ID)
	if err != nil {
		if errors.Is(err, engine.ErrGMBadID) {
			return nil, bad("%v", err)
		}
		return nil, conflict("%v", err)
	}
	return M{"message": msg, "gm": s.gmView(), "state": s.state()}, nil
}
