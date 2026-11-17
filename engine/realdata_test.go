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

func TestRealTeamAbbrevsCoverEverySeason(t *testing.T) {
	if _, err := os.Stat(realDir); err != nil {
		t.Skip("real data not present")
	}
	rows, err := readCSV(realDir + "/Team Abbrev.csv")
	if err != nil {
		t.Fatal(err)
	}
	bySeason := map[int]map[string]bool{}
	for _, m := range rows {
		if m["lg"] == "NBA" && m["abbreviation"] != "NA" {
			s := int(num(m["season"]))
			if bySeason[s] == nil {
				bySeason[s] = map[string]bool{}
			}
			bySeason[s][m["abbreviation"]] = true
		}
	}
	for s := firstRealSeason; s <= lastRealSeason; s++ {
		ab := RealTeamAbbrevsFor(s)
		if len(ab) != 30 {
			t.Fatalf("season %d: %d abbreviations", s, len(ab))
		}
		seen := map[string]bool{}
		for _, a := range ab {
			if seen[a] || !bySeason[s][a] {
				t.Fatalf("season %d: abbreviation %q duplicated or not in Team Abbrev.csv", s, a)
			}
			seen[a] = true
		}
		if len(bySeason[s]) != 30 {
			t.Fatalf("season %d: data has %d teams", s, len(bySeason[s]))
		}
	}
	if a := RealTeamAbbrevsFor(2011); a[24] != "NJN" || a[8] != "NOH" || a[17] != "CHA" {
		t.Fatalf("2011 table wrong: %v", a)
	}
	if a := RealTeamAbbrevsFor(2026); a[24] != "BRK" || a[8] != "NOP" || a[17] != "CHO" {
		t.Fatalf("2026 table wrong: %v", a)
	}
	if got := RealTeamAbbrevs(); got[24] != "NJN" {
		t.Fatal("RealTeamAbbrevs is not the 2011 list")
	}
	infos, err := RealSeasons(realDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 16 || infos[0].Season != 2011 || infos[15].Season != 2026 || len(infos[15].Teams) != 30 {
		t.Fatalf("RealSeasons shape wrong: %d entries", len(infos))
	}
	if tm := infos[15].Teams[24]; tm.City != "Brooklyn" || tm.Name != "Nets" {
		t.Fatalf("2026 franchise 24 = %+v", tm)
	}
}

func TestRealLeagueSeasonInvalid(t *testing.T) {
	for _, s := range []int{0, 1990, 2010, 2027, 3000} {
		if _, err := NewRealLeagueSeason(realDir, 1, s); err == nil {
			t.Fatalf("season %d accepted", s)
		}
	}
}

func TestRealLeagueEverySeasonBuilds(t *testing.T) {
	if _, err := os.Stat(realDir); err != nil {
		t.Skip("real data not present")
	}
	for s := firstRealSeason; s <= lastRealSeason; s++ {
		lg, err := NewRealLeagueSeason(realDir, 3, s)
		if err != nil {
			t.Fatalf("season %d: %v", s, err)
		}
		if lg.RealSeason != s || lg.Year != s || len(lg.Teams) != 30 {
			t.Fatalf("season %d: RealSeason %d Year %d teams %d", s, lg.RealSeason, lg.Year, len(lg.Teams))
		}
		seen := map[string]bool{}
		for _, tm := range lg.Teams {
			if len(tm.Roster) < 13 {
				t.Fatalf("season %d: %s has %d players", s, tm.FullName(), len(tm.Roster))
			}
			for _, p := range tm.Roster {
				if seen[p.RealID] {
					t.Fatalf("season %d: %s appears twice", s, p.Name)
				}
				seen[p.RealID] = true
			}
		}
		for y := range lg.DraftClasses {
			if y < s {
				t.Fatalf("season %d: stored draft class of %d", s, y)
			}
		}
		if s == 2020 && lg.Teams[24].City != "Brooklyn" {
			t.Fatalf("2020 franchise 24 is %s", lg.Teams[24].FullName())
		}
	}
}

func TestRealCeilingBlend(t *testing.T) {
	if _, err := os.Stat(realDir); err != nil {
		t.Skip("real data not present")
	}
	if ceilingWeight(2011, 18) != 1 || ceilingWeight(2018, 19) != 1 || ceilingWeight(2026, 20) != 0 {
		t.Fatal("ceilingWeight bounds wrong")
	}
	if w := ceilingWeight(2022, 21); w < 0.66 || w > 0.67 {
		t.Fatalf("ceilingWeight(2022, 21) = %v, want 4/6", w)
	}
	// 2011: every young player's potential is exactly the old rule (blurred hindsight peak).
	d, err := loadRealDataFrom(realDir, 2011)
	if err != nil {
		t.Fatal(err)
	}
	lg, err := NewRealLeagueSeason(realDir, 5, 2011)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, p := range lg.allPlayers() {
		if p.Age >= PotentialEqual() || p.RealID == "" {
			continue
		}
		want := clamp(blurPotential(5, p.RealID, d.hindsightPeak(p.RealID, 2011, p.Overall())), p.Overall(), 99)
		if p.Potential != want {
			t.Fatalf("2011 %s: potential %d, old rule %d", p.Name, p.Potential, want)
		}
		n++
	}
	if n < 50 {
		t.Fatalf("only %d young players checked", n)
	}
	// 2026: no later data, so potential is the projection from age and overall.
	l26, err := NewRealLeagueSeason(realDir, 5, 2026)
	if err != nil {
		t.Fatal(err)
	}
	above := 0
	for _, p := range l26.allPlayers() {
		if p.Age < 22 && p.Potential > p.Overall() {
			above++
		}
		if p.Age < PotentialEqual() && p.Potential < p.Overall() {
			t.Fatalf("%s potential below overall", p.Name)
		}
	}
	if above < 10 {
		t.Fatalf("only %d young 2026 players have potential above overall", above)
	}
}
