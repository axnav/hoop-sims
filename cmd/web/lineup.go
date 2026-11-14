package main

import (
	"net/http"
	"sort"
	"strconv"

	"hoopsim/engine"
)

// lineupObj describes the user's lineup: stored plan (or the automatic one),
// plus what each player would get automatically and in the next game.
func (s *server) lineupObj() M {
	lg := s.lg
	u := lg.UserTeamID
	t := lg.Teams[u]
	auto := lg.AutoLineup(u)
	custom := t.Lineup != nil
	cur := auto
	if custom {
		cur = *t.Lineup
	}
	next := map[int]int{}
	for _, pm := range t.EffectiveMinutes() {
		next[pm.Player.ID] = pm.Min
	}
	slot := map[int]int{}
	for i, id := range cur.Starters {
		if id != engine.NoPlayer {
			slot[id] = i
		}
	}
	ps := append([]*engine.Player(nil), t.Roster...)
	order := func(p *engine.Player) int {
		if i, ok := slot[p.ID]; ok {
			return i - 10
		}
		return 0
	}
	sort.SliceStable(ps, func(i, j int) bool {
		a, b := ps[i], ps[j]
		if order(a) != order(b) {
			return order(a) < order(b)
		}
		if cur.Minutes[a.ID] != cur.Minutes[b.ID] {
			return cur.Minutes[a.ID] > cur.Minutes[b.ID]
		}
		return a.Overall() > b.Overall()
	})
	rows := []M{}
	for _, p := range ps {
		sl := -1
		if i, ok := slot[p.ID]; ok {
			sl = i
		}
		row := M{"id": p.ID, "name": p.Name, "pos": p.PosLabel(), "age": p.Age, "ovr": p.Overall(), "pot": p.Potential,
			"tier": engine.PlayerTier(p), "minutes": cur.Minutes[p.ID], "autoMinutes": auto.Minutes[p.ID],
			"nextMinutes": next[p.ID], "starterSlot": sl, "gp": 0, "mpg": 0.0}
		if st := lg.Season.Stats[p.ID]; st != nil && st.GP > 0 {
			row["gp"], row["mpg"] = st.GP, float64(st.Min)/float64(st.GP)
		}
		addInjury(row, p)
		addDev(row, p)
		rows = append(rows, row)
	}
	return M{"custom": custom, "total": engine.TeamMinutes, "maxMinutes": engine.MaxPlayerMinutes,
		"minPlayers": engine.MinLineupPlayers, "slots": []string{"PG", "SG", "SF", "PF", "C"},
		"starters": cur.Starters[:],
		"dev": M{"maxAge": engine.Progression.MinutesDevMaxAge, "fullMinutes": engine.Progression.MinutesDevFull,
			"noneMinutes": engine.Progression.MinutesDevNone},
		"players": rows}
}

func (s *server) lineupGet(r *http.Request) (any, error) { return s.lineupObj(), nil }

func (s *server) lineupSet(r *http.Request) (any, error) {
	var in struct {
		Starters []int          `json:"starters"`
		Minutes  map[string]int `json:"minutes"`
	}
	if err := readJSON(r, &in); err != nil {
		return nil, err
	}
	if len(in.Starters) != 5 {
		return nil, bad("starters must list exactly 5 players")
	}
	l := engine.Lineup{Minutes: make(map[int]int, len(in.Minutes))}
	copy(l.Starters[:], in.Starters)
	for k, v := range in.Minutes {
		id, err := strconv.Atoi(k)
		if err != nil {
			return nil, bad("bad player id %q in minutes", k)
		}
		l.Minutes[id] = v
	}
	if err := s.lg.SetLineup(s.lg.UserTeamID, l); err != nil {
		return nil, bad("%v", err)
	}
	return s.lineupObj(), nil
}

func (s *server) lineupAuto(r *http.Request) (any, error) {
	s.lg.ClearLineup(s.lg.UserTeamID)
	return s.lineupObj(), nil
}
