package main

import (
	"net/http"
	"sort"

	"hoopsim/engine"
)

// mid is the rounded midpoint of a range.
func mid(lo, hi int) int { return (lo + hi + 1) / 2 }

// inPool reports whether the player is an undrafted prospect.
func (s *server) inPool(id int) bool {
	if d := s.lg.Draft; d != nil {
		for _, p := range d.Pool {
			if p.ID == id {
				return true
			}
		}
	}
	return false
}

// prospectRows lists the pool with scouting ranges only, ordered by the
// midpoints of the ranges (never by the true values).
func (s *server) prospectRows() []M {
	type row struct {
		m              M
		potMid, ovrMid int
		id             int
	}
	rows := []row{}
	for _, p := range s.lg.Draft.Pool {
		v, ok := s.lg.ScoutView(p.ID)
		if !ok {
			continue
		}
		rows = append(rows, row{M{"id": p.ID, "name": p.Name, "age": p.Age, "pos": p.PosLabel(),
			"ovrLo": v.OvrLo, "ovrHi": v.OvrHi, "potLo": v.PotLo, "potHi": v.PotHi, "scoutLevel": v.Level},
			v.PotLo + v.PotHi, v.OvrLo + v.OvrHi, p.ID})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.potMid != b.potMid {
			return a.potMid > b.potMid
		}
		if a.ovrMid != b.ovrMid {
			return a.ovrMid > b.ovrMid
		}
		return a.id < b.id
	})
	out := []M{}
	for _, r := range rows {
		out = append(out, r.m)
	}
	return out
}

// lotteryRows is the lottery result, ordered by final slot.
func (s *server) lotteryRows() []M {
	rows := append([]engine.LotteryRow(nil), s.lg.Draft.Lottery...)
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Slot < rows[j].Slot })
	out := []M{}
	for _, r := range rows {
		out = append(out, M{"team": s.team(r.Team), "owner": s.team(r.Owner), "seed": r.Seed, "slot": r.Slot,
			"odds": float64(r.Odds) / 10, "change": r.Change})
	}
	return out
}

// prospectPlayer fills a player response for a prospect: midpoints, ranges
// and a role/usage worked out from the midpoint ratings (not the true ones).
func (s *server) prospectPlayer(p *engine.Player, out M) bool {
	v, ok := s.lg.ScoutView(p.ID)
	rg, ok2 := s.lg.ScoutRatings(p.ID)
	if !ok || !ok2 {
		return false
	}
	seen := engine.Player{Pos: p.Pos, AltPos: p.AltPos}
	ratings := []M{}
	for _, k := range engine.RatingKeys() {
		r := rg[k.Key]
		m := mid(r[0], r[1])
		engine.SetRating(&seen.Ratings, k.Key, m)
		ratings = append(ratings, M{"key": k.Key, "label": k.Label, "value": m, "lo": r[0], "hi": r[1]})
	}
	out["ratings"] = ratings
	out["ovr"], out["pot"] = mid(v.OvrLo, v.OvrHi), mid(v.PotLo, v.PotHi)
	out["role"], out["usage"] = seen.ScoringRole(), seen.ShotUsage()
	out["prospect"] = true
	out["editable"] = false
	out["scout"] = M{"level": v.Level, "ovrLo": v.OvrLo, "ovrHi": v.OvrHi, "potLo": v.PotLo, "potHi": v.PotHi}
	return true
}

func (s *server) draftScout(r *http.Request) (any, error) {
	var in struct {
		PlayerID int `json:"playerId"`
	}
	if err := readJSON(r, &in); err != nil {
		return nil, err
	}
	if err := s.lg.Scout(in.PlayerID); err != nil {
		return nil, conflict("%v", err)
	}
	return s.draftState(r)
}
