package engine

import (
	"os"
	"testing"
)

const realDir = "../data/real"

func realLeagueOrSkip(t *testing.T, seed uint64) *League {
	t.Helper()
	if _, err := os.Stat(realDir); err != nil {
		t.Skip("real data not present")
	}
	lg, err := NewRealLeague(realDir, seed)
	if err != nil {
		t.Fatal(err)
	}
	return lg
}

func TestRealLeagueShape(t *testing.T) {
	lg := realLeagueOrSkip(t, 1)
	if len(lg.Teams) != 30 {
		t.Fatalf("teams = %d", len(lg.Teams))
	}
	seen := map[int]bool{}
	real := map[string]bool{}
	var east int
	for i, tm := range lg.Teams {
		if len(tm.Roster) != RosterSize {
			t.Fatalf("%s has %d players", tm.FullName(), len(tm.Roster))
		}
		if Conference(i) == 0 {
			east++
		}
		for _, p := range tm.Roster {
			if seen[p.ID] || real[p.RealID] {
				t.Fatalf("duplicate player %s", p.Name)
			}
			seen[p.ID], real[p.RealID] = true, true
		}
	}
	for _, p := range lg.FreeAgents {
		if seen[p.ID] || real[p.RealID] {
			t.Fatalf("duplicate free agent %s", p.Name)
		}
		seen[p.ID], real[p.RealID] = true, true
	}
	if east != 15 {
		t.Fatalf("east teams = %d", east)
	}
	check := func(p *Player) {
		for _, v := range p.Ratings.array() {
			if v < 25 || v > 99 {
				t.Fatalf("%s rating %d out of range", p.Name, v)
			}
		}
		if p.Potential < p.Overall() {
			t.Fatalf("%s potential below overall", p.Name)
		}
	}
	for _, tm := range lg.Teams {
		for _, p := range tm.Roster {
			check(p)
		}
	}
	if n := len(lg.DraftClasses[2011]); n < 30 {
		t.Fatalf("2011 class has %d players", n)
	}
	for _, p := range lg.DraftClasses[2011] {
		check(p)
	}
}

func TestRealLeagueSeasonDeterministic(t *testing.T) {
	a := realLeagueOrSkip(t, 7)
	b := realLeagueOrSkip(t, 7)
	for _, lg := range []*League{a, b} {
		lg.StartSeason()
		lg.SimRegularSeason()
	}
	for i := range a.Season.Wins {
		if a.Season.Wins[i] != b.Season.Wins[i] || a.Season.PointDiff[i] != b.Season.PointDiff[i] {
			t.Fatalf("standings differ for team %d", i)
		}
	}
	// The offseason uses the stored 2011 class.
	n := len(a.DraftClasses[2011])
	a.Season.Champion = 0
	a.Offseason()
	if len(a.DraftClasses[2011]) != 0 || n == 0 {
		t.Fatal("2011 class not consumed")
	}
	if a.Year != 2012 {
		t.Fatalf("year = %d", a.Year)
	}
}
