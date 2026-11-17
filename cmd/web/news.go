package main

import (
	"net/http"
	"strconv"

	"hoopsim/engine"
)

var newsFilterLabels = map[string]string{
	"all": "All", "mine": "My team", "trades": "Trades", "signings": "Signings",
	"injuries": "Injuries", "games": "Games", "awards": "Awards", "draft": "Draft",
}

// news serves GET /api/news?filter=<key>&limit=<n>. The stories are the ones the
// engine wrote when things happened; this only translates them.
func (s *server) news(r *http.Request) (any, error) {
	filter := r.URL.Query().Get("filter")
	if filter == "" {
		filter = "all"
	}
	if !engine.ValidNewsFilter(filter) {
		return nil, bad("unknown filter %q", filter)
	}
	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return nil, bad("bad limit")
		}
		limit = min(n, 400)
	}
	items, counts, total := s.lg.NewsFor(filter, limit)
	rows := []M{}
	for _, it := range items {
		teams := []M{}
		mine := false
		for _, t := range it.TeamIDs {
			teams = append(teams, s.team(t))
			mine = mine || t == s.lg.UserTeamID
		}
		players := []M{}
		for i, id := range it.PlayerIDs {
			name := ""
			if i < len(it.Names) {
				name = it.Names[i]
			}
			players = append(players, M{"id": id, "name": name})
		}
		rows = append(rows, M{"id": it.ID, "year": it.Year, "day": max(0, it.Day+1), "stage": it.Stage, "kind": it.Kind,
			"text": it.Text, "big": it.Big, "mine": mine, "teams": teams, "players": players})
	}
	filters := []M{}
	for _, k := range engine.NewsFilterKeys() {
		filters = append(filters, M{"key": k, "label": newsFilterLabels[k], "count": counts[k]})
	}
	return M{"items": rows, "filters": filters, "total": total}, nil
}

// awardRows turns a list of awards into the JSON shape (never null).
func (s *server) awardRows(as []engine.Award) []any {
	out := []any{}
	for _, a := range as {
		if v := s.award(a); v != nil {
			out = append(out, v)
		}
	}
	return out
}

// awardsObj is the "awards" object of a history season row and of "current".
func (s *server) awardsObj(a engine.Awards) M {
	return s.awardsFrom(a.ROY, a.DPOY, a.SixthMan, a.MIP, a.Coach, a.AllLeague, a.AllRookie)
}

func (s *server) awardsFrom(roy, dpoy, six, mip engine.Award, coach engine.CoachAward, all [][]engine.Award, rookie []engine.Award) M {
	var c any
	if coach.Name != "" {
		c = M{"name": coach.Name, "team": s.team(coach.TeamID), "w": coach.W, "l": coach.L}
	}
	al := []any{}
	for _, tm := range all {
		al = append(al, s.awardRows(tm))
	}
	return M{"roy": s.award(roy), "dpoy": s.award(dpoy), "sixthMan": s.award(six), "mip": s.award(mip),
		"coach": c, "allLeague": al, "allRookie": s.awardRows(rookie)}
}

// playerAwardRows is the player page's award list, newest first (never null).
func (s *server) playerAwardRows(id int) []M {
	rows := []M{}
	for _, a := range s.lg.PlayerAwards(id) {
		rows = append(rows, M{"year": a.Year, "title": a.Title})
	}
	return rows
}
