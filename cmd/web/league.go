package main

import (
	"math"
	"net/http"
	"sort"
	"strconv"

	"hoopsim/engine"
)

func pctOf(m, a int) any {
	if a == 0 {
		return nil
	}
	return math.Round(1000*float64(m)/float64(a)) / 10
}

func winPct(w, l int) float64 {
	if w+l == 0 {
		return 0
	}
	return float64(w) / float64(w+l)
}

func statLine(ps *engine.PlayerSeason) any {
	if ps == nil || ps.GP == 0 {
		return nil
	}
	return M{"gp": ps.GP, "mpg": float64(ps.Min) / float64(ps.GP), "ppg": ps.PPG(), "rpg": ps.RPG(), "apg": ps.APG(),
		"tpg": float64(ps.Tov) / float64(ps.GP), "fg": pctOf(ps.FGM, ps.FGA), "tp": pctOf(ps.TPM, ps.TPA), "ft": pctOf(ps.FTM, ps.FTA)}
}

func (s *server) standings(r *http.Request) (any, error) {
	st := s.lg.Season
	confs := []M{}
	for c := 0; c < 2; c++ {
		ids := st.ConferenceStandings(c)
		rows := []M{}
		for i, id := range ids {
			top := ids[0]
			gb := float64(st.Wins[top]-st.Wins[id]+st.Losses[id]-st.Losses[top]) / 2
			rows = append(rows, M{"rank": i + 1, "team": s.team(id), "w": st.Wins[id], "l": st.Losses[id],
				"pct": winPct(st.Wins[id], st.Losses[id]), "gb": gb, "pd": st.PointDiff[id], "inPlayoffs": i < 8})
		}
		confs = append(confs, M{"conf": c, "name": confNames[c], "rows": rows})
	}
	return M{"year": s.lg.Year, "conferences": confs, "playoffLine": 8, "userTeam": s.lg.UserTeamID}, nil
}

func (s *server) leaders(r *http.Request) (any, error) {
	st := s.lg.Season
	badge := r.URL.Query().Get("badge")
	if badge != "" && !engine.ValidBadgeKey(badge) {
		return nil, bad("unknown badge %q", badge)
	}
	list := func(stat string, f func(*engine.PlayerSeason) float64) []M {
		out := []M{}
		rows := st.Leaders(stat, 10)
		if badge != "" {
			// Only players who have the badge: filter the full ranking, then take ten.
			rows = rows[:0:0]
			for _, ps := range st.Leaders(stat, len(st.Stats)) {
				if p := s.lg.FindPlayer(ps.PlayerID); p != nil && s.lg.HasBadge(p, badge) {
					rows = append(rows, ps)
				}
			}
			if len(rows) > 10 {
				rows = rows[:10]
			}
		}
		for _, ps := range rows {
			out = append(out, M{"playerId": ps.PlayerID, "name": ps.Name, "team": s.team(ps.TeamID), "gp": ps.GP, "value": f(ps)})
		}
		return out
	}
	return M{"minGP": engine.MinGPLeaders, "pts": list("pts", (*engine.PlayerSeason).PPG),
		"reb": list("reb", (*engine.PlayerSeason).RPG), "ast": list("ast", (*engine.PlayerSeason).APG)}, nil
}

func (s *server) teamPage(r *http.Request) (any, error) {
	id, err := pathID(r)
	if err != nil {
		return nil, err
	}
	lg := s.lg
	if id < 0 || id >= len(lg.Teams) {
		return nil, missing("no such team")
	}
	t := lg.Teams[id]
	starters := map[int]bool{}
	for i, p := range t.Rotation() {
		if i < 5 {
			starters[p.ID] = true
		}
	}
	roster := []M{}
	expiring := map[int]bool{}
	for _, p := range lg.Expiring(id) {
		expiring[p.ID] = true
	}
	for _, p := range t.Roster {
		row := M{"id": p.ID, "name": p.Name, "age": p.Age, "pos": p.PosLabel(), "ovr": p.Overall(), "pot": p.Potential,
			"starter": starters[p.ID], "gp": 0, "mpg": 0.0, "ppg": 0.0, "rpg": 0.0, "apg": 0.0}
		if ps := lg.Season.Stats[p.ID]; ps != nil && ps.GP > 0 {
			row["gp"], row["mpg"], row["ppg"], row["rpg"], row["apg"] = ps.GP, float64(ps.Min)/float64(ps.GP), ps.PPG(), ps.RPG(), ps.APG()
		}
		addInjury(row, p)
		addContract(row, p)
		addDev(row, p)
		s.addPhoto(row, p)
		row["expiring"] = expiring[p.ID]
		if id == lg.UserTeamID {
			s.addStance(row, p)
			row["focus"] = p.Focus.Key()
		}
		roster = append(roster, row)
	}
	st := lg.Season
	return M{"team": s.team(id), "w": st.Wins[id], "l": st.Losses[id], "pd": st.PointDiff[id], "confRank": s.rank(id),
		"isUser": id == lg.UserTeamID, "roster": roster, "payroll": s.payrollObj(id), "status": s.statusObj(id),
		"coach": s.coachTeamObj(t), "teamFit": fitObj(lg.TeamFit(t))}, nil
}

func (s *server) findPlayer(id int) *engine.Player {
	if p := s.lg.FindPlayer(id); p != nil {
		return p
	}
	if d := s.lg.Draft; d != nil {
		for _, p := range d.Pool {
			if p.ID == id {
				return p
			}
		}
	}
	return nil
}

func (s *server) player(r *http.Request) (any, error) {
	id, err := pathID(r)
	if err != nil {
		return nil, err
	}
	lg := s.lg
	p := s.findPlayer(id)
	if p == nil {
		return nil, missing("no such player")
	}
	status, team := "Free agent", any(nil)
	for _, q := range lg.Retired {
		if q.ID == p.ID {
			status = "Retired"
		}
	}
	if status != "Retired" {
		for _, t := range lg.Teams {
			for _, q := range t.Roster {
				if q.ID == p.ID {
					status, team = "Active", s.team(t.ID)
				}
			}
		}
		if lg.Draft != nil {
			for _, q := range lg.Draft.Pool {
				if q.ID == p.ID {
					status = "Draft prospect"
				}
			}
		}
	}
	rt := p.Ratings
	ratings := []M{}
	for _, k := range engine.RatingKeys() {
		v, _ := engine.GetRating(rt, k.Key)
		ratings = append(ratings, M{"key": k.Key, "label": k.Label, "value": v})
	}
	career := []M{}
	for _, c := range lg.Careers[p.ID] {
		career = append(career, M{"year": c.Year, "team": s.team(c.TeamID), "age": c.Age, "ovr": c.Overall, "pot": c.Potential,
			"gp": c.GP, "ppg": c.PPG, "rpg": c.RPG, "apg": c.APG})
	}
	out := M{"id": p.ID, "name": p.Name, "age": p.Age, "pos": p.PosLabel(), "ovr": p.Overall(), "pot": p.Potential,
		"status": status, "team": team, "role": p.ScoringRole(), "usage": p.ShotUsage(),
		"ratings": ratings, "experience": p.Experience, "draftYear": p.DraftYear, "draftPick": p.DraftPick,
		"season": statLine(lg.Season.Stats[p.ID]), "playoffs": statLine(lg.Season.PlayoffStats[p.ID]), "career": career,
		"editable": lg.Options.Commissioner,
		"limits":   M{"minRating": engine.MinRating, "maxRating": engine.MaxRating, "minAge": engine.MinAge, "maxAge": engine.MaxAge}}
	if s.inPool(p.ID) && !s.prospectPlayer(p, out) {
		return nil, missing("no such player")
	}
	addInjury(out, p)
	addContract(out, p)
	if !s.inPool(p.ID) {
		out["badges"] = s.badgesObj(p)
		s.addPhoto(out, p)
		if status != "Retired" {
			out["motivation"] = s.motivationObj(p)
		}
		out["training"] = s.trainingObj(p, p.TeamID == lg.UserTeamID && status == "Active")
		if p.TeamID == lg.UserTeamID && status == "Active" {
			st := lg.StanceOf(p)
			reasons := st.Reasons
			if reasons == nil {
				reasons = []string{}
			}
			out["stance"] = M{"key": st.Key, "label": st.Label, "reasons": reasons}
			if p.Contract.NextYears > 0 {
				out["extension"] = M{"salary": p.Contract.NextSalary, "years": p.Contract.NextYears}
			}
		}
	}
	if d, _ := devChange(p); !s.inPool(p.ID) && d != nil {
		addDev(out, p)
		rc := M{}
		for _, k := range engine.RatingKeys() {
			v, _ := engine.GetRating(d.Ratings, k.Key)
			rc[k.Key] = v
		}
		out["ratingChanges"] = rc
	}
	return out, nil
}

func (s *server) schedule(r *http.Request) (any, error) {
	lg := s.lg
	tid := lg.UserTeamID
	if v := r.URL.Query().Get("team"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 || n >= len(lg.Teams) {
			return nil, bad("bad team")
		}
		tid = n
	}
	games := []M{}
	for _, g := range lg.Season.Games {
		if g.Home != tid && g.Away != tid {
			continue
		}
		opp, home := g.Away, true
		if g.Away == tid {
			opp, home = g.Home, false
		}
		row := M{"day": g.Day + 1, "opp": s.team(opp), "home": home, "played": g.Played}
		if g.Played {
			me, them := g.HomeScore, g.AwayScore
			if !home {
				me, them = them, me
			}
			row["me"], row["them"], row["win"] = me, them, me > them
		}
		games = append(games, row)
	}
	po := []M{}
	for _, sr := range lg.Season.Series {
		if sr.High != tid && sr.Low != tid {
			continue
		}
		for _, g := range sr.Games {
			opp := sr.Low
			if sr.Low == tid {
				opp = sr.High
			}
			me, them := g.HomeScore, g.AwayScore
			if g.Home != tid {
				me, them = them, me
			}
			po = append(po, M{"label": engine.RoundName(sr.Round) + ", game " + strconv.Itoa(g.N), "opp": s.team(opp),
				"home": g.Home == tid, "me": me, "them": them, "win": me > them, "box": g.Box})
		}
	}
	return M{"team": s.team(tid), "games": games, "playoffGames": po}, nil
}

func (s *server) boxDTO(b *engine.Box, id string) M {
	teams := []M{}
	for _, bt := range b.Teams {
		lines := []M{}
		for _, l := range bt.Lines {
			lines = append(lines, M{"id": l.PlayerID, "name": l.Name, "pos": l.Pos, "min": l.Min, "pts": l.Pts, "reb": l.Reb,
				"ast": l.Ast, "tov": l.Tov, "fgm": l.FGM, "fga": l.FGA, "tpm": l.TPM, "tpa": l.TPA, "ftm": l.FTM, "fta": l.FTA})
		}
		teams = append(teams, M{"team": s.team(bt.TeamID), "score": bt.Score, "lines": lines})
	}
	label := b.Label
	if label == "" {
		label = "Day " + strconv.Itoa(b.Day+1)
	}
	return M{"id": id, "year": b.Year, "day": b.Day, "playoff": b.Day < 0, "label": label, "overtimes": b.Overtimes,
		"teams": teams, "hasReplay": len(b.Events) > 0}
}

func (s *server) lookupBox(idStr string) (*engine.Box, error) {
	if idStr == "last" {
		if s.lg.LastBox == nil {
			return nil, missing("no game played yet")
		}
		return s.lg.LastBox, nil
	}
	n, err := strconv.Atoi(idStr)
	if err != nil {
		return nil, bad("bad box id")
	}
	b := s.lg.Season.PlayoffBoxes[n]
	if b == nil {
		return nil, missing("no such game")
	}
	return b, nil
}

func (s *server) box(r *http.Request) (any, error) {
	b, err := s.lookupBox(r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	return s.boxDTO(b, r.PathValue("id")), nil
}

func (s *server) gameEvents(r *http.Request) (any, error) {
	b, err := s.lookupBox(r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	if len(b.Events) == 0 {
		return nil, missing("no replay for this game")
	}
	d := s.boxDTO(b, r.PathValue("id"))
	d["events"] = b.Events
	d["ot"] = b.Overtimes
	return d, nil
}

func (s *server) history(r *http.Request) (any, error) {
	rows := []M{}
	for i := len(s.lg.Records) - 1; i >= 0; i-- {
		rec := s.lg.Records[i]
		row := M{"year": rec.Year, "champion": s.team(rec.Champion), "mvp": s.award(rec.MVP),
			"finalsMvp": s.award(rec.FinalsMVP), "scoringLeader": s.award(rec.ScoringLeader), "finals": nil}
		for _, sr := range rec.Series {
			if sr.Round == 4 && sr.Done {
				row["finals"] = M{"winner": s.team(sr.Winner), "loser": s.team(sr.High + sr.Low - sr.Winner),
					"winnerWins": max(sr.HighWins, sr.LowWins), "loserWins": min(sr.HighWins, sr.LowWins)}
			}
		}
		rows = append(rows, row)
	}
	return M{"seasons": rows}, nil
}

// sortedSeries returns series ordered by round, conference, slot.
func sortedSeries(in []engine.Series) []engine.Series {
	out := append([]engine.Series(nil), in...)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Round != b.Round {
			return a.Round < b.Round
		}
		if a.Conf != b.Conf {
			return a.Conf < b.Conf
		}
		return a.Slot < b.Slot
	})
	return out
}
