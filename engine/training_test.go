package engine

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAptitudeDeterministicAndSpread(t *testing.T) {
	lg := GenerateLeague(1)
	var slow, normal, fast, n float64
	for _, p := range lg.allPlayers() {
		a, b := lg.Aptitude(p), lg.Aptitude(p)
		if a != b {
			t.Fatalf("aptitude of %d changed between calls", p.ID)
		}
		for _, x := range a {
			n++
			switch AptitudeLabel(x) {
			case "Slow":
				slow++
			case "Fast":
				fast++
			default:
				normal++
			}
		}
	}
	if s, f := slow/n, fast/n; s < 0.2 || s > 0.3 || f < 0.2 || f > 0.3 {
		t.Errorf("spread slow %.2f normal %.2f fast %.2f", slow/n, normal/n, fast/n)
	}
	// a different seed gives a different mix for the same player
	other := GenerateLeague(2)
	p := lg.Teams[0].Roster[0]
	if lg.Aptitude(p) == other.Aptitude(p) {
		t.Error("aptitude does not depend on the league seed")
	}
}

func TestReweightKeepsTotalAndDecline(t *testing.T) {
	d := [NumRatings]float64{3, 1, 2, -1.5, 0, 4, -0.5}
	w := [NumRatings]float64{2, 0.5, 1, 3, 1, 0.5, 2}
	before := d
	reweightGrowth(&d, &w)
	var sb, sa float64
	for i := range d {
		if before[i] > 0 {
			sb += before[i]
			sa += d[i]
		} else if d[i] != before[i] {
			t.Errorf("decline entry %d changed: %v -> %v", i, before[i], d[i])
		}
	}
	if sa < sb-1e-9 || sa > sb+1e-9 {
		t.Errorf("positive total %v -> %v", sb, sa)
	}
	if d[0]/before[0] <= d[5]/before[5] {
		t.Error("heavier weight did not get a larger share")
	}
}

func youngPlayer(id int) *Player {
	p := buildPlayer(NewRNG(7), id, SG, 50, 20)
	p.Age, p.Potential, p.Impact = 20, 85, 0
	return p
}

func TestUnitWeightsEqualOldBehaviour(t *testing.T) {
	ones := [NumRatings]float64{1, 1, 1, 1, 1, 1, 1}
	for s := uint64(0); s < 300; s++ {
		for _, age := range []int{20, 24, 28, 33} {
			a, b := youngPlayer(900), youngPlayer(900)
			a.Age, b.Age = age, age
			a.ProgressMinutes(NewRNG(s), 20)
			b.ProgressWeighted(NewRNG(s), 20, &ones)
			if a.Ratings != b.Ratings || a.Potential != b.Potential {
				t.Fatalf("seed %d age %d: %v vs %v", s, age, a.Ratings, b.Ratings)
			}
		}
	}
}

func TestDeclineUnaffectedByWeights(t *testing.T) {
	lg := GenerateLeague(1)
	for s := uint64(0); s < 300; s++ {
		a, b := youngPlayer(901), youngPlayer(901)
		a.Age, b.Age = 38, 38
		b.Focus = FocusShooting
		a.ProgressMinutes(NewRNG(s), -1)
		b.ProgressWeighted(NewRNG(s), -1, lg.growthWeights(b))
		if a.Ratings != b.Ratings {
			t.Fatalf("seed %d: decline changed: %v vs %v", s, a.Ratings, b.Ratings)
		}
	}
}

func TestFocusMovesGrowthNotTotal(t *testing.T) {
	lg := GenerateLeague(1)
	run := func(f Focus) (fav, other, total float64) {
		for s := uint64(0); s < 600; s++ {
			p := youngPlayer(902)
			old := p.Ratings.array()
			p.Focus = f
			p.ProgressWeighted(NewRNG(s), 20, lg.growthWeights(p))
			n := p.Ratings.array()
			for i := range n {
				d := float64(n[i] - old[i])
				total += d
				if i == 1 || i == 2 {
					fav += d
				} else if i != 6 {
					other += d
				}
			}
		}
		return fav / 600, other / 600, total / 600
	}
	bf, bo, bt := run(FocusBalanced)
	sf, so, st := run(FocusShooting)
	t.Logf("balanced fav %.2f other %.2f total %.2f; shooting fav %.2f other %.2f total %.2f", bf, bo, bt, sf, so, st)
	if sf < bf+0.8 || so > bo-0.8 {
		t.Errorf("focus did not move growth toward shooting")
	}
	if st < bt*0.9 || st > bt*1.1 {
		t.Errorf("total growth moved: %.2f vs %.2f", st, bt)
	}
}

func TestInSeasonFocusMovesGrowth(t *testing.T) {
	run := func(f Focus) (fav, total float64) {
		lg := GenerateLeague(3)
		lg.StartSeason()
		var ps []*Player
		for _, tm := range lg.Teams {
			for _, p := range tm.Roster {
				if p.Age <= 23 {
					p.Focus = f
					ps = append(ps, p)
				}
			}
		}
		start := map[int][NumRatings]int{}
		for _, p := range ps {
			start[p.ID] = p.Ratings.array()
		}
		for step := 1; step <= 4; step++ {
			lg.inSeasonDev(step)
		}
		for _, p := range ps {
			n, o := p.Ratings.array(), start[p.ID]
			for i := range n {
				d := float64(n[i] - o[i])
				total += d
				if i == 4 { // rebounding
					fav += d
				}
			}
		}
		return
	}
	bf, bt := run(FocusBalanced)
	rf, rt := run(FocusRebounding)
	t.Logf("balanced rebounding %.0f total %.0f; rebounding focus %.0f total %.0f", bf, bt, rf, rt)
	if rf <= bf {
		t.Errorf("in-season focus did not move growth: %.0f vs %.0f", rf, bf)
	}
	if rt < bt*0.85 || rt > bt*1.15 {
		t.Errorf("in-season total moved: %.0f vs %.0f", rt, bt)
	}
}

func TestSetFocus(t *testing.T) {
	lg := GenerateLeague(1)
	p := lg.Teams[1].Roster[0]
	if err := lg.SetFocus(1, p.ID, FocusDefense); err != nil || p.Focus != FocusDefense {
		t.Fatalf("set focus: %v %v", err, p.Focus)
	}
	if err := lg.SetFocus(2, p.ID, FocusDefense); err == nil {
		t.Error("another team's player accepted")
	}
	if err := lg.SetFocus(1, -5, FocusDefense); err == nil {
		t.Error("unknown player accepted")
	}
	if err := lg.SetFocus(1, p.ID, Focus(99)); err == nil {
		t.Error("bad focus accepted")
	}
	if err := lg.SetFocus(99, p.ID, FocusBody); err == nil {
		t.Error("bad team accepted")
	}
	for _, f := range AllFocuses() {
		g, ok := FocusFromKey(f.Key())
		if !ok || g != f || f.Label() == "" {
			t.Errorf("key round trip for %v", f)
		}
	}
	if _, ok := FocusFromKey("nope"); ok {
		t.Error("bad key parsed")
	}
}

func TestFocusSavedAndOldSaveLoads(t *testing.T) {
	p := youngPlayer(903)
	b, _ := json.Marshal(p)
	if strings.Contains(string(b), `"Focus"`) {
		t.Error("Balanced should not be written")
	}
	var q Player
	if err := json.Unmarshal(b, &q); err != nil || q.Focus != FocusBalanced {
		t.Fatalf("old save: %v %v", err, q.Focus)
	}
	p.Focus = FocusInside
	b, _ = json.Marshal(p)
	json.Unmarshal(b, &q)
	if q.Focus != FocusInside {
		t.Errorf("focus not kept: %v", q.Focus)
	}
}

func TestAutoFocusAndView(t *testing.T) {
	p := youngPlayer(904)
	p.Pos = C
	p.Ratings = Ratings{Inside: 40, Mid: 60, Three: 60, Playmaking: 60, Rebounding: 70, Defense: 70, Athleticism: 70}
	if f := autoFocus(p); f != FocusInside {
		t.Errorf("weak-inside centre got %v", f)
	}
	p.Age = 33
	if autoFocus(p) != FocusBalanced {
		t.Error("old player should stay Balanced")
	}
	lg := GenerateLeague(1)
	p.Age = 26
	v := lg.TrainingView(p)
	if len(v) != NumRatings || v[6].StepsFor2 != "closed" || v[6].Open {
		t.Errorf("athleticism should be closed at 26: %+v", v[6])
	}
	p.Age = 20
	for _, l := range lg.TrainingView(p) {
		if !l.Open || l.StepsFor2 == "closed" || l.Speed == "" {
			t.Errorf("young player line: %+v", l)
		}
	}
}
