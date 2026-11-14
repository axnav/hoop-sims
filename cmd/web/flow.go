package main

import (
	"net/http"
	"sort"

	"hoopsim/engine"
)

// ---- playoffs ----

func (s *server) playoffs(r *http.Request) (any, error) {
	lg := s.lg
	st := lg.Season
	series, year, champ, live := st.Series, st.Year, st.Champion, true
	if len(series) == 0 {
		series, champ, live = nil, -1, false
		if n := len(lg.Records); n > 0 {
			rec := lg.Records[n-1]
			series, year, champ = rec.Series, rec.Year, rec.Champion
		}
	}
	views := []M{}
	for _, sr := range sortedSeries(series) {
		sr := sr
		views = append(views, s.seriesView(&sr))
	}
	var champion any
	if champ >= 0 {
		champion = s.team(champ)
	}
	return M{"year": year, "live": live, "started": st.PlayoffsStarted, "canStart": s.phase() == "regular_done",
		"active": st.PlayoffsActive(), "roundName": engine.RoundName(st.CurrentRound()), "champion": champion,
		"userAlive": s.userSeries() != nil, "series": views, "conferences": confNames}, nil
}

func (s *server) playoffAction(r *http.Request) (any, error) {
	lg := s.lg
	what := r.PathValue("action")
	out := M{}
	if what == "start" {
		if s.phase() != "regular_done" {
			return nil, conflict("the regular season is not finished")
		}
		if err := lg.StartPlayoffs(); err != nil {
			return nil, conflict("%v", err)
		}
		out["state"] = s.state()
		return out, nil
	}
	if s.phase() != "playoffs" {
		return nil, conflict("no playoffs to sim")
	}
	switch what {
	case "day":
		lg.SimPlayoffDay()
	case "series":
		if s.userSeries() == nil {
			return nil, conflict("your team has no series in progress")
		}
		lg.SimUserSeries()
	case "round":
		lg.SimRound()
	case "all":
		lg.SimAllPlayoffs()
	case "watch":
		i := lg.UserSeriesIdx()
		if i < 0 {
			return nil, conflict("your team has no game to watch")
		}
		lg.SimPlayoffDay()
		sr := lg.Season.Series[i]
		out["boxId"] = sr.Games[len(sr.Games)-1].Box
	default:
		return nil, bad("unknown playoff action")
	}
	out["state"] = s.state()
	return out, nil
}

// ---- offseason ----

func (s *server) changes(in []engine.Change) []M {
	out := []M{}
	for _, c := range in {
		out = append(out, M{"playerId": c.PlayerID, "name": c.Name, "team": s.team(c.TeamID), "age": c.Age, "oldOvr": c.OldOvr, "newOvr": c.NewOvr})
	}
	return out
}

func (s *server) pickDTO(p engine.PickResult) M {
	return M{"pick": p.Pick, "round": p.Round, "team": s.team(p.TeamID), "made": true,
		"player": M{"id": p.PlayerID, "name": p.Name, "age": p.Age, "ovr": p.Overall, "pot": p.Potential}}
}

func (s *server) offseasonReport(r *http.Request) (any, error) {
	rep := s.lg.Report
	if rep == nil {
		return M{"available": false}, nil
	}
	ret := []M{}
	for _, x := range rep.Retirements {
		ret = append(ret, M{"playerId": x.ID, "name": x.Name, "age": x.Age, "ovr": x.Overall, "team": s.team(x.TeamID)})
	}
	out := M{"available": true, "year": rep.Year, "retirements": ret, "risers": s.changes(rep.Risers), "fallers": s.changes(rep.Fallers)}
	out["trades"] = s.offseasonTrades(rep.Year)
	return out, nil
}

func (s *server) offseasonBegin(r *http.Request) (any, error) {
	if s.phase() != "playoffs_done" {
		return nil, conflict("the playoffs are not finished")
	}
	s.lg.BeginOffseason()
	s.reportSeen = false
	return s.state(), nil
}

func (s *server) offseasonToDraft(r *http.Request) (any, error) {
	if s.lg.Draft == nil {
		return nil, conflict("no offseason in progress")
	}
	s.reportSeen = true
	return s.state(), nil
}

func (s *server) offseasonFinish(r *http.Request) (any, error) {
	if s.phase() != "draft_done" {
		return nil, conflict("the draft is not finished")
	}
	s.lg.FinishOffseason()
	s.lg.StartSeason()
	return s.state(), nil
}

// ---- draft ----

func (s *server) draftState(r *http.Request) (any, error) {
	lg := s.lg
	d := lg.Draft
	if d == nil {
		return M{"active": false}, nil
	}
	picks := []M{}
	for i := 0; i < d.Total; i++ {
		if i < len(d.Results) {
			picks = append(picks, s.pickDTO(d.Results[i]))
			continue
		}
		picks = append(picks, M{"pick": i + 1, "round": i/len(d.Order) + 1, "team": s.team(d.Order[i%len(d.Order)]), "made": false})
	}
	pool := append([]*engine.Player(nil), d.Pool...)
	sort.SliceStable(pool, func(i, j int) bool { return pool[i].Overall() > pool[j].Overall() })
	prospects := []M{}
	for _, p := range pool {
		prospects = append(prospects, M{"id": p.ID, "name": p.Name, "age": p.Age, "pos": p.PosLabel(), "ovr": p.Overall(), "pot": p.Potential})
	}
	out := M{"active": true, "total": d.Total, "made": d.Made, "done": lg.DraftDone(), "picks": picks, "prospects": prospects,
		"started": s.reportSeen, "userTeam": lg.UserTeamID}
	if !lg.DraftDone() {
		out["onClock"] = s.team(lg.DraftOnClock())
		out["userOnClock"] = lg.DraftOnClock() == lg.UserTeamID
	}
	return out, nil
}

func (s *server) needDraft() error {
	if s.lg.Draft == nil || !s.reportSeen {
		return conflict("no draft in progress")
	}
	return nil
}

func (s *server) draftPick(r *http.Request) (any, error) {
	var in struct {
		PlayerID int `json:"playerId"`
	}
	if err := readJSON(r, &in); err != nil {
		return nil, err
	}
	if err := s.needDraft(); err != nil {
		return nil, err
	}
	lg := s.lg
	if lg.DraftDone() || lg.DraftOnClock() != lg.UserTeamID {
		return nil, conflict("you are not on the clock")
	}
	if err := lg.DraftPick(in.PlayerID); err != nil {
		return nil, bad("%v", err)
	}
	pk := lg.Draft.Results[len(lg.Draft.Results)-1]
	return M{"pick": s.pickDTO(pk), "state": s.state()}, nil
}

// draftStep lets the computer make up to max picks, stopping when the user is on the clock.
func (s *server) draftStep(r *http.Request) (any, error) {
	var in struct {
		Max int `json:"max"`
	}
	if err := readJSON(r, &in); err != nil {
		return nil, err
	}
	if err := s.needDraft(); err != nil {
		return nil, err
	}
	if in.Max < 1 {
		in.Max = 1
	}
	lg := s.lg
	picks := []M{}
	for i := 0; i < in.Max && !lg.DraftDone() && lg.DraftOnClock() != lg.UserTeamID; i++ {
		lg.DraftPick(-1)
		picks = append(picks, s.pickDTO(lg.Draft.Results[len(lg.Draft.Results)-1]))
	}
	return M{"picks": picks, "state": s.state()}, nil
}

func (s *server) draftAuto(r *http.Request) (any, error) {
	if err := s.needDraft(); err != nil {
		return nil, err
	}
	s.lg.DraftAutoAll()
	return s.state(), nil
}
