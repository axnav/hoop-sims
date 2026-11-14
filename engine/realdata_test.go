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

func realPotentials(lg *League) map[string][2]int {
	out := map[string][2]int{}
	for _, p := range lg.allPlayers() {
		out[p.RealID] = [2]int{p.Potential, p.Overall()}
	}
	for _, cls := range lg.DraftClasses {
		for _, p := range cls {
			out[p.RealID] = [2]int{p.Potential, p.Overall()}
		}
	}
	return out
}

func TestPotentialBlur(t *testing.T) {
	old := RealTuning.PotentialBlurSD
	defer func() { RealTuning.PotentialBlurSD = old }()

	RealTuning.PotentialBlurSD = 0
	l1, l2 := realLeagueOrSkip(t, 1), realLeagueOrSkip(t, 2)
	z1, z2 := realPotentials(l1), realPotentials(l2)
	// (A prospect with no stat row gets seeded random ratings: not compared.)
	for _, p := range l1.allPlayers() {
		id, v := p.RealID, z1[p.RealID]
		if z2[id] != v {
			t.Fatalf("blur 0 differs across seeds for %s", id)
		}
	}

	RealTuning.PotentialBlurSD = 4
	lg1 := realLeagueOrSkip(t, 1)
	a, a2, b := realPotentials(lg1), realPotentials(realLeagueOrSkip(t, 1)), realPotentials(realLeagueOrSkip(t, 2))
	diffYoung := 0
	for id, v := range a {
		if a2[id] != v {
			t.Fatalf("same seed differs for %s", id)
		}
		if v[0] < v[1] || v[0] > 99 || v[0] < 25 {
			t.Fatalf("%s potential %d vs overall %d", id, v[0], v[1])
		}
		if v[0] != z1[id][0] {
			diffYoung++
		}
	}
	if diffYoung == 0 {
		t.Error("blur changed nothing")
	}
	changed := 0
	for id, v := range a {
		if b[id][0] != v[0] {
			changed++
		}
	}
	if changed == 0 {
		t.Error("different seeds gave the same potentials")
	}
	// Players 27 and older are untouched.
	for _, p := range lg1.allPlayers() {
		if p.Age >= PotentialEqual() && (a[p.RealID] != b[p.RealID] || p.Potential != p.Overall()) {
			t.Fatalf("%s (age %d) was blurred", p.Name, p.Age)
		}
	}
	// A well-known young player across three seeds.
	for _, p := range lg1.allPlayers() {
		if p.Name == "Blake Griffin" {
			t.Logf("Blake Griffin potential: hindsight %d", z1[p.RealID][0])
			for _, s := range []uint64{1, 2, 3} {
				RealTuning.PotentialBlurSD = 4
				t.Logf("seed %d: %d", s, realPotentials(realLeagueOrSkip(t, s))[p.RealID][0])
			}
		}
	}
}
