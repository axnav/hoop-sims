package engine

import (
	"path/filepath"
	"testing"
)

func ratingsSnapshot(lg *League) map[int][7]int {
	m := map[int][7]int{}
	for _, p := range lg.allPlayers() {
		m[p.ID] = p.Ratings.array()
	}
	return m
}

func TestInSeasonDeterministic(t *testing.T) {
	run := func() map[int][7]int {
		lg := GenerateLeague(11)
		lg.StartSeason()
		lg.SimDays(40)
		return ratingsSnapshot(lg)
	}
	a, b := run(), run()
	changed := 0
	base := ratingsSnapshot(GenerateLeague(11))
	for id, v := range a {
		if v != b[id] {
			t.Fatalf("player %d differs between identical runs", id)
		}
		if v != base[id] {
			changed++
		}
	}
	if changed == 0 {
		t.Fatal("nobody's ratings moved in 40 days")
	}
}

func TestInSeasonOff(t *testing.T) {
	lg := GenerateLeague(12)
	lg.Options.NoInSeasonDev = true
	lg.Options.NoInjuries = true
	before := ratingsSnapshot(lg)
	lg.StartSeason()
	lg.SimRegularSeason()
	for _, p := range lg.allPlayers() {
		if p.Ratings.array() != before[p.ID] {
			t.Fatalf("%s changed with the option off", p.Name)
		}
		if p.SeasonDev != nil {
			t.Fatalf("%s has SeasonDev with the option off", p.Name)
		}
	}
}

func TestInSeasonLeavesGamesAlone(t *testing.T) {
	scores := func(off bool) []int {
		lg := GenerateLeague(13)
		lg.Options.NoInSeasonDev = off
		s := lg.StartSeason()
		lg.SimDays(16)
		var out []int
		for i := 0; i < 16*GamesPerDay; i++ {
			out = append(out, s.Games[i].HomeScore, s.Games[i].AwayScore)
		}
		return out
	}
	on, off := scores(false), scores(true)
	for i := range on {
		if on[i] != off[i] {
			t.Fatalf("score %d differs: %d vs %d", i, on[i], off[i])
		}
	}
}

func TestInSeasonOffseasonTotals(t *testing.T) {
	lg := GenerateLeague(14)
	lg.Options.NoInjuries = true // injuries change ratings outside SeasonDev
	start := map[int]int{}
	for _, p := range lg.allPlayers() {
		start[p.ID] = p.Overall()
	}
	lg.StartSeason()
	lg.SimRegularSeason()
	withDev := 0
	for _, p := range lg.allPlayers() {
		if p.SeasonDev != nil {
			withDev++
			if p.SeasonDev.Overall != p.Overall()-start[p.ID] {
				t.Fatalf("%s SeasonDev.Overall %d, want %d", p.Name, p.SeasonDev.Overall, p.Overall()-start[p.ID])
			}
		}
	}
	if withDev == 0 {
		t.Fatal("no SeasonDev after a season")
	}
	if err := lg.SimPlayoffs(); err != nil {
		t.Fatal(err)
	}
	lg.BeginOffseason()
	for _, p := range lg.allPlayers() {
		if p.SeasonDev != nil {
			t.Fatalf("%s still has SeasonDev after the offseason", p.Name)
		}
		if p.Dev == nil {
			t.Fatalf("%s has no Dev", p.Name)
		}
		if p.Dev.Overall != p.Overall()-start[p.ID] {
			t.Fatalf("%s Dev.Overall %d, want %d", p.Name, p.Dev.Overall, p.Overall()-start[p.ID])
		}
	}
}

func TestInSeasonYoungGainAndPotential(t *testing.T) {
	lg := GenerateLeague(15)
	lg.Options.NoInjuries = true
	type rec struct {
		p   *Player
		ovr int
	}
	var young []rec
	for _, p := range lg.allPlayers() {
		if p.Age <= 22 {
			p.Potential = min(99, p.Overall()+15)
			p.Impact = 2
			young = append(young, rec{p, p.Overall()})
		}
	}
	if len(young) < 20 {
		t.Fatalf("only %d young players", len(young))
	}
	lg.StartSeason()
	lg.SimDays(64)
	sum := 0
	for _, y := range young {
		sum += y.p.Overall() - y.ovr
		if y.p.Potential < y.p.Overall() {
			t.Fatalf("%s potential %d below overall %d", y.p.Name, y.p.Potential, y.p.Overall())
		}
	}
	if avg := float64(sum) / float64(len(young)); avg <= 0.5 {
		t.Fatalf("young high-ceiling players gained %.2f on average over 4 steps, want > 0.5", avg)
	}
	for _, p := range lg.allPlayers() {
		if p.Potential < p.Overall() {
			t.Fatalf("%s potential %d below overall %d", p.Name, p.Potential, p.Overall())
		}
	}
}

func TestInSeasonSaveLoad(t *testing.T) {
	lg := GenerateLeague(16)
	lg.StartSeason()
	lg.SimDays(20)
	path := filepath.Join(t.TempDir(), "league.json")
	if err := lg.Save(path); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, p := range lg.allPlayers() {
		g := got.FindPlayer(p.ID)
		if g == nil {
			t.Fatalf("player %d missing after load", p.ID)
		}
		if (p.SeasonDev == nil) != (g.SeasonDev == nil) || (p.SeasonDev != nil && *p.SeasonDev != *g.SeasonDev) {
			t.Fatalf("%s SeasonDev lost in save/load", p.Name)
		}
		if p.SeasonDev != nil {
			n++
		}
	}
	if n == 0 {
		t.Fatal("no SeasonDev to round-trip")
	}
}
