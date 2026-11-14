package engine

import (
	"path/filepath"
	"testing"
)

func teamPoints(lg *League) int {
	n := 0
	for _, g := range lg.Season.Games {
		n += g.HomeScore*3 + g.AwayScore
	}
	return n
}

func TestNoInjuriesDeterministic(t *testing.T) {
	run := func() *League {
		lg := GenerateLeague(5)
		lg.Options.NoInjuries = true
		lg.StartSeason()
		lg.SimRegularSeason()
		return lg
	}
	a, b := run(), run()
	if teamPoints(a) != teamPoints(b) {
		t.Fatal("not deterministic")
	}
	for _, tm := range a.Teams {
		for _, p := range tm.Roster {
			if p.Injured() {
				t.Fatal("injury with NoInjuries")
			}
		}
	}
}

func TestInjuriesSameSeedSameResult(t *testing.T) {
	run := func() (int, int) {
		lg := GenerateLeague(5)
		lg.StartSeason()
		lg.SimRegularSeason()
		return teamPoints(lg), len(lg.InjuryReport(-1))
	}
	a1, b1 := run()
	a2, b2 := run()
	if a1 != a2 || b1 != b2 {
		t.Fatal("injury run not deterministic")
	}
}

func TestInjuredSitsAndHeals(t *testing.T) {
	lg := GenerateLeague(8)
	lg.Options.NoInjuries = true
	lg.StartSeason()
	tm := lg.Teams[lg.Season.Games[0].Home]
	star := tm.Rotation()[0]
	star.Injury = &Injury{Name: "Test", GamesLeft: 2}
	for _, p := range tm.Rotation() {
		if p == star {
			t.Fatal("injured player in rotation")
		}
	}
	res := SimGame(NewRNG(1), tm, lg.Teams[lg.Season.Games[0].Away])
	for _, l := range res.Home.Lines {
		if l.Player == star {
			t.Fatal("injured player got a line")
		}
	}
	lg.afterGame(NewRNG(1), res)
	if star.Injury == nil || star.Injury.GamesLeft != 1 {
		t.Fatal("countdown wrong")
	}
	lg.afterGame(NewRNG(1), res)
	if star.Injured() {
		t.Fatal("not cleared")
	}
}

func TestInjuryRates(t *testing.T) {
	serious := 0
	for _, seed := range []uint64{1, 2} {
		lg := GenerateLeague(seed)
		lg.StartSeason()
		lost := 0
		for lg.SimDay() {
			for _, tm := range lg.Teams {
				for _, p := range tm.Roster {
					if p.Injured() {
						lost++
					}
				}
			}
			for _, l := range lg.NewInjuriesLastDay() {
				if l.Serious {
					serious++
				}
			}
		}
		avg := float64(lost) / NumTeams
		t.Logf("seed %d: player-games lost per team %.1f", seed, avg)
		if avg < 60 || avg > 110 {
			t.Errorf("seed %d: lost %.1f per team, want 60-110", seed, avg)
		}
	}
	if serious > 8 {
		t.Errorf("too many serious injuries: %d", serious)
	}
}

func TestSeriousInjuryDropsRatings(t *testing.T) {
	r := NewRNG(1)
	InjuryTuning.SeriousShare = 1
	defer func() { InjuryTuning.SeriousShare = 0.004 }()
	p := &Player{Age: 25, Ratings: Ratings{60, 60, 60, 60, 60, 60, 70}, Potential: 60}
	inj := rollInjury(r, p)
	if !inj.Serious || inj.GamesLeft <= 20 || p.Ratings.Athleticism >= 70-1 || p.Potential < p.Overall() {
		t.Fatalf("bad serious injury %+v %+v", inj, p.Ratings)
	}
}

func TestManyInjuredStillPlays(t *testing.T) {
	lg := GenerateLeague(4)
	tm := lg.Teams[0]
	for _, p := range tm.Roster[:11] {
		p.Injury = &Injury{Name: "x", GamesLeft: 9}
	}
	if len(tm.Rotation()) != 13-11 && len(tm.Rotation()) < 5 {
		t.Fatal("rotation too small")
	}
	for _, p := range tm.Roster {
		p.Injury = &Injury{Name: "x", GamesLeft: 9}
	}
	if len(tm.Rotation()) != 13 {
		t.Fatal("fallback should include everyone")
	}
	SimGame(NewRNG(1), tm, lg.Teams[1])
}

func TestOffseasonClearsInjuries(t *testing.T) {
	lg := GenerateLeague(6)
	lg.StartSeason()
	lg.SimRegularSeason()
	if err := lg.SimPlayoffs(); err != nil {
		t.Fatal(err)
	}
	a, b := lg.Teams[0].Roster[0], lg.Teams[0].Roster[1]
	a.Injury = &Injury{Name: "x", GamesLeft: 10}
	b.Injury = &Injury{Name: "Torn ACL", GamesLeft: 50, Serious: true}
	lg.BeginOffseason()
	lg.DraftAutoAll()
	lg.FinishOffseason()
	if a.Injured() {
		t.Error("ordinary injury survived offseason")
	}
	if b.Injury == nil || b.Injury.GamesLeft != 50 {
		t.Error("long serious injury should carry over")
	}
}

func TestSaveKeepsInjury(t *testing.T) {
	lg := GenerateLeague(7)
	lg.Teams[2].Roster[3].Injury = &Injury{Name: "Sore knee", GamesLeft: 4}
	path := filepath.Join(t.TempDir(), "l.json")
	if err := lg.Save(path); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if in := got.Teams[2].Roster[3].Injury; in == nil || in.GamesLeft != 4 || in.Name != "Sore knee" {
		t.Fatal("injury lost in save")
	}
}
