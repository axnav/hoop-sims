package main

import (
	"fmt"
	"net/http"
	"strconv"

	"hoopsim/engine"
)

func (s *server) rank(id int) int { return s.lg.Season.ConferenceRank(id) }

func (s *server) phase() string { return string(s.lg.Phase(s.reportSeen)) }

func act(a engine.Action) M { return M{"id": a.ID, "label": a.Label} }

func (s *server) userSeries() *engine.Series {
	if i := s.lg.UserSeriesIdx(); i >= 0 {
		return &s.lg.Season.Series[i]
	}
	return nil
}

// state is the single source for the app flow: phase, user summary and the
// one action the Continue button performs.
func (s *server) state() M {
	lg := s.lg
	st := lg.Season
	u := lg.UserTeamID
	ph := s.phase()
	out := M{
		"year": lg.Year, "phase": ph, "day": st.Day, "totalDays": engine.SeasonDays,
		"offers": len(lg.PendingProposals()), "expiring": len(lg.Expiring(u)), "payroll": s.payrollObj(u), "injured": len(lg.InjuryReport(u)),
		"mode": map[bool]string{true: "real", false: "fictional"}[s.isReal()], "conferences": confNames,
		"user": M{"team": s.team(u), "w": st.Wins[u], "l": st.Losses[u], "pd": st.PointDiff[u],
			"confRank": s.rank(u), "confName": confNames[engine.Conference(u)]},
	}
	// next game
	var next M
	if ph == "regular" {
		for _, g := range st.Games[st.Day*engine.GamesPerDay : (st.Day+1)*engine.GamesPerDay] {
			if g.Home == u || g.Away == u {
				opp := g.Away
				if g.Away == u {
					opp = g.Home
				}
				next = M{"kind": "regular", "opp": s.team(opp), "home": g.Home == u, "oppW": st.Wins[opp], "oppL": st.Losses[opp],
					"label": fmt.Sprintf("Day %d", st.Day+1)}
			}
		}
	} else if sr := s.userSeries(); sr != nil && ph == "playoffs" {
		opp, myWins, oppWins := sr.Low, sr.HighWins, sr.LowWins
		if sr.Low == u {
			opp, myWins, oppWins = sr.High, sr.LowWins, sr.HighWins
		}
		g := len(sr.Games)
		home := sr.HostOf(g) == u
		next = M{"kind": "playoff", "opp": s.team(opp), "home": home, "oppW": oppWins, "oppL": myWins,
			"label": fmt.Sprintf("%s, game %d", engine.RoundName(sr.Round), g+1), "seriesMe": myWins, "seriesThem": oppWins}
	}
	out["next"] = next
	// last result
	if b := lg.LastBox; b != nil {
		me, opp := 0, 1
		if b.Teams[1].TeamID == u {
			me, opp = 1, 0
		}
		label := fmt.Sprintf("Day %d", b.Day+1)
		boxID := "last"
		if b.Day < 0 {
			label, boxID = b.Label, strconv.Itoa(b.ID)
		}
		out["last"] = M{"opp": s.team(b.Teams[opp].TeamID), "me": b.Teams[me].Score, "them": b.Teams[opp].Score,
			"win": b.Teams[me].Score > b.Teams[opp].Score, "home": me == 0, "label": label, "boxId": boxID, "playoff": b.Day < 0, "year": b.Year}
	}
	// playoffs
	if st.PlayoffsStarted {
		po := M{"active": st.PlayoffsActive(), "round": st.CurrentRound(), "roundName": engine.RoundName(st.CurrentRound()),
			"userAlive": s.userSeries() != nil, "userResult": lg.UserPlayoffResult()}
		if st.Champion >= 0 {
			po["champion"] = s.team(st.Champion)
			po["finalsMvp"] = s.award(st.Awards.FinalsMVP)
			po["mvp"] = s.award(st.Awards.MVP)
			if fin := st.Series[len(st.Series)-1]; fin.Round == 4 {
				po["finals"] = s.seriesView(&fin)
			}
		}
		out["playoffs"] = po
	}
	if ph == "regular_done" {
		seed := st.ConferenceRank(u)
		out["summary"] = M{"w": st.Wins[u], "l": st.Losses[u], "rank": seed, "madePlayoffs": st.MadePlayoffs(u),
			"mvp": s.award(st.Awards.MVP), "scoringLeader": s.award(st.Awards.ScoringLeader)}
	}
	if lg.Draft != nil {
		d := lg.Draft
		dr := M{"made": d.Made, "total": d.Total, "done": lg.DraftDone()}
		if !lg.DraftDone() {
			dr["onClock"] = s.team(lg.DraftOnClock())
			dr["userOnClock"] = lg.DraftOnClock() == u
		}
		out["draft"] = dr
	}
	if lg.FA != nil {
		out["fa"] = s.faState()
	}
	// next action and menu
	var na M
	var menu []M
	nx, more := lg.NextActions(s.reportSeen)
	if nx != (engine.Action{}) {
		na = act(nx)
	}
	for _, m := range more {
		menu = append(menu, act(m))
	}
	out["nextAction"] = na
	out["actions"] = menu
	return out
}

func (s *server) award(a engine.Award) any {
	if a.Name == "" {
		return nil
	}
	return M{"playerId": a.PlayerID, "name": a.Name, "team": s.team(a.TeamID), "gp": a.GP, "ppg": a.PPG, "rpg": a.RPG, "apg": a.APG}
}

func (s *server) seriesView(sr *engine.Series) M {
	u := s.lg.UserTeamID
	games := []M{}
	for _, g := range sr.Games {
		hs, as, hw := g.HomeScore, g.AwayScore, g.Home
		if as > hs {
			hw = sr.High + sr.Low - g.Home
		}
		games = append(games, M{"n": g.N, "home": g.Home, "homeScore": g.HomeScore, "awayScore": g.AwayScore,
			"winner": hw, "ot": g.OT, "box": g.Box})
	}
	v := M{"round": sr.Round, "conf": sr.Conf, "slot": sr.Slot, "high": s.team(sr.High), "low": s.team(sr.Low),
		"highSeed": sr.HighSeed, "lowSeed": sr.LowSeed, "highWins": sr.HighWins, "lowWins": sr.LowWins,
		"done": sr.Done, "winner": sr.Winner, "mine": sr.High == u || sr.Low == u, "games": games}
	return v
}

// ---- sim ----

func (s *server) dayResult(day int) M {
	lg := s.lg
	st := lg.Season
	u := lg.UserTeamID
	res := M{"day": day + 1, "w": st.Wins[u], "l": st.Losses[u], "rank": s.rank(u)}
	for _, g := range st.Games[day*engine.GamesPerDay : (day+1)*engine.GamesPerDay] {
		if g.Home != u && g.Away != u {
			continue
		}
		me, them, opp := g.HomeScore, g.AwayScore, g.Away
		if g.Away == u {
			me, them, opp = g.AwayScore, g.HomeScore, g.Home
		}
		res["opp"], res["home"], res["me"], res["them"], res["win"] = s.team(opp), g.Home == u, me, them, me > them
	}
	return res
}

func (s *server) strip() []M {
	lg := s.lg
	conf := engine.Conference(lg.UserTeamID)
	rows := []M{}
	for i, id := range lg.Season.ConferenceStandings(conf) {
		if i >= 10 {
			break
		}
		rows = append(rows, M{"rank": i + 1, "team": s.team(id), "w": lg.Season.Wins[id], "l": lg.Season.Losses[id]})
	}
	return rows
}

func (s *server) sim(r *http.Request) (any, error) {
	var in struct {
		Days  int  `json:"days"`
		ToEnd bool `json:"toEnd"`
		Auto  bool `json:"auto"`
	}
	if err := readJSON(r, &in); err != nil {
		return nil, err
	}
	lg := s.lg
	if err := lg.CanSimDay(s.reportSeen); err != nil {
		return nil, conflict("%v", err)
	}
	n := in.Days
	if in.ToEnd {
		n = engine.SeasonDays
	}
	if n < 1 {
		n = 1
	}
	multi := in.Auto || in.Days > 1 || in.ToEnd
	days := []M{}
	newInj := []engine.InjuryLine{}
	created := 0
	var pause M
	for i := 0; i < n && !lg.Season.Done(); i++ {
		if multi && lg.ShouldPauseForDeadline() {
			pause = M{"reason": "deadline", "text": "The trade deadline is tomorrow. After the next game day, trades are closed until the offseason."}
			break
		}
		d := lg.Season.Day
		lg.SimDay()
		days = append(days, s.dayResult(d))
		for _, l := range lg.NewInjuriesLastDay() {
			if l.TeamID == lg.UserTeamID {
				newInj = append(newInj, l)
			}
		}
		k := lg.ProposalsCreatedLastDay()
		created += k
		if multi && k > 0 && !lg.Options.NoOfferStop {
			text := "You have new trade offers."
			if ps := lg.PendingProposals(); k == 1 && len(ps) > 0 {
				text = s.team(ps[len(ps)-1].Team)["fullName"].(string) + " made you a trade offer."
			}
			pause = M{"reason": "offer", "text": text}
			break
		}
	}
	out := M{"days": days, "strip": s.strip(), "state": s.state(), "newOffers": created,
		"injuries": s.injuryLines(newInj)}
	if pause != nil {
		out["pause"] = pause
	}
	return out, nil
}
