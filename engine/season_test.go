package engine

import (
	"math"
	"sync"
	"testing"
)

func playSeason(lg *League) {
	lg.StartSeason()
	lg.SimRegularSeason()
	if err := lg.SimPlayoffs(); err != nil {
		panic(err)
	}
}

func rotationAvg(lg *League) float64 {
	sum, n := 0, 0
	for _, t := range lg.Teams {
		rot := t.Rotation()
		for _, p := range rot[:min(10, len(rot))] {
			sum += p.Overall()
			n++
		}
	}
	return float64(sum) / float64(n)
}

func count90(lg *League) int {
	n := 0
	for _, t := range lg.Teams {
		for _, p := range t.Roster {
			if p.Overall() >= 90 {
				n++
			}
		}
	}
	return n
}

func TestSeasonBasics(t *testing.T) {
	lg := GenerateLeague(11)
	s := lg.StartSeason()
	home := make([]int, NumTeams)
	for _, g := range s.Games {
		home[g.Home]++
	}
	for i, h := range home {
		if h < 40 || h > 42 {
			t.Errorf("team %d has %d home games", i, h)
		}
	}
	lg.SimRegularSeason()
	w, l := 0, 0
	for i := 0; i < NumTeams; i++ {
		if s.Wins[i]+s.Losses[i] != 82 {
			t.Fatalf("team %d played %d", i, s.Wins[i]+s.Losses[i])
		}
		w += s.Wins[i]
		l += s.Losses[i]
	}
	if w != l {
		t.Fatalf("wins %d != losses %d", w, l)
	}
	if err := lg.SimPlayoffs(); err != nil || s.Champion < 0 || len(s.Series) != 15 {
		t.Fatalf("playoffs: %v champ %d series %d", err, s.Champion, len(s.Series))
	}
	for i := 0; i < 3; i++ {
		lg.Offseason()
		for _, tm := range lg.Teams {
			if len(tm.Roster) != RosterSize {
				t.Fatalf("team %d has %d players", tm.ID, len(tm.Roster))
			}
		}
		playSeason(lg)
	}
}

func TestSeasonDeterminism(t *testing.T) {
	run := func() []int {
		lg := GenerateLeague(5)
		var ch []int
		for i := 0; i < 3; i++ {
			playSeason(lg)
			ch = append(ch, lg.Season.Champion)
			lg.Offseason()
		}
		return ch
	}
	a, b := run(), run()
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("champions differ: %v vs %v", a, b)
		}
	}
}

// longRun is what TestLongStability measures for one league.
type longRun struct {
	start    float64   // rotation average of the new league
	avgs     []float64 // rotation average at the end of each season
	lo90     int       // fewest and most players rated 90+ in any season
	hi90     int
	bad90    []int // seasons where the 90+ count left [2, max90]
	diffs    []int // peak overall minus draft-day potential, top-10 picks of the first 21 classes
	last     *Season
	finalAvg float64
}

func playLongRun(seed uint64, seasons, max90 int) longRun {
	lg := GenerateLeague(seed)
	r := longRun{start: rotationAvg(lg), lo90: 99}
	type pk struct{ pot, peak int }
	picks := map[int]*pk{}
	track := func() {
		for _, p := range lg.allPlayers() {
			if k := picks[p.ID]; k != nil {
				k.peak = max(k.peak, p.Overall())
			}
		}
	}
	for y := 0; y < seasons; y++ {
		playSeason(lg)
		r.avgs = append(r.avgs, rotationAvg(lg))
		n := count90(lg)
		r.lo90, r.hi90 = min(r.lo90, n), max(r.hi90, n)
		if n < 2 || n > max90 {
			r.bad90 = append(r.bad90, y)
		}
		track()
		if y == seasons-1 {
			r.last = lg.Season
		}
		rep := lg.Offseason()
		for _, d := range rep.Draft {
			if d.Pick <= 10 && y <= 20 { // later classes have not peaked yet
				picks[d.PlayerID] = &pk{d.Potential, d.Overall}
			}
		}
		track()
	}
	for _, k := range picks {
		r.diffs = append(r.diffs, k.peak-k.pot)
	}
	r.finalAvg = rotationAvg(lg)
	return r
}

// TestLongStability plays 30 seasons on five seeds and checks the league does
// not drift. The rotation average is measured on healthy players at the end
// of a season, so one season can swing with the injury count; drift is
// therefore judged on the average of the last ten seasons, and a single
// season only has to stay inside a wider guard band.
func TestLongStability(t *testing.T) {
	const (
		seasons    = 30
		driftBand  = 2.0 // last-ten-season average vs the starting value
		seasonBand = 4.0 // any single season vs the starting value
		// Players rated 90+ in any season: at least 2, and at most 15 on seed 1
		// (the long-standing check). Seeds 2 to 5 were first measured on
		// 2026-11-15 and peak between 11 and 16, so they get a guard of 18.
		max90Seed1 = 15
		max90Other = 18
	)
	seeds := []uint64{1, 2, 3, 4, 5}
	runs := make([]longRun, len(seeds))
	var wg sync.WaitGroup
	for i, seed := range seeds {
		wg.Add(1)
		go func() {
			defer wg.Done()
			limit := max90Other
			if seed == 1 {
				limit = max90Seed1
			}
			runs[i] = playLongRun(seed, seasons, limit)
		}()
	}
	wg.Wait()

	near, tot, sum := 0, 0, 0
	hist := make([]int, 20)
	for i, r := range runs {
		seed := seeds[i]
		late, worst := 0.0, 0.0
		for y, a := range r.avgs {
			if y >= seasons-10 {
				late += a / 10
			}
			worst = math.Max(worst, math.Abs(a-r.start))
			if math.Abs(a-r.start) > seasonBand {
				t.Errorf("seed %d season %d: rotation avg %.2f vs start %.2f", seed, y, a, r.start)
			}
		}
		if math.Abs(late-r.start) > driftBand {
			t.Errorf("seed %d: last ten seasons average %.2f vs start %.2f (drift)", seed, late, r.start)
		}
		for _, y := range r.bad90 {
			t.Errorf("seed %d season %d: 90+ count outside its band (range over the run %d-%d)", seed, y, r.lo90, r.hi90)
		}
		for _, d := range r.diffs {
			tot++
			sum += d
			hist[min(19, max(0, (d+40)/4))]++
			if d >= -4 && d <= 4 {
				near++
			}
		}
		t.Logf("seed %d: start %.2f, last-ten avg %.2f, final %.2f, worst single season %.2f, 90+ range %d-%d",
			seed, r.start, late, r.finalAvg, worst, r.lo90, r.hi90)
	}
	checkAverages(t, runs[0].last)
	t.Logf("hist (4-wide bins from -30): %v", hist)
	t.Logf("mean peak-pot %.1f; top-10 picks within +-4 of draft potential at peak: %d/%d (%.0f%%)",
		float64(sum)/float64(tot), near, tot, 100*float64(near)/float64(tot))
}

func checkAverages(t *testing.T, s *Season) {
	var pts, hw, reb, ast, tov float64
	var fgm, fga, tpm, tpa, ftm, fta float64
	for _, g := range s.Games {
		pts += float64(g.HomeScore + g.AwayScore)
		if g.HomeScore > g.AwayScore {
			hw++
		}
	}
	for _, p := range s.Stats {
		fgm += float64(p.FGM)
		fga += float64(p.FGA)
		tpm += float64(p.TPM)
		tpa += float64(p.TPA)
		ftm += float64(p.FTM)
		fta += float64(p.FTA)
		reb += float64(p.Reb)
		ast += float64(p.Ast)
		tov += float64(p.Tov)
	}
	n := float64(len(s.Games) * 2)
	check := func(name string, v, lo, hi float64) {
		if v < lo || v > hi {
			t.Errorf("final season %s = %.2f outside [%.0f, %.0f]", name, v, lo, hi)
		}
	}
	check("points", pts/n, 108, 118)
	check("FG%", 100*fgm/fga, 45, 49)
	check("3P%", 100*tpm/tpa, 34, 38)
	check("FT%", 100*ftm/fta, 74, 80)
	check("rebounds", reb/n, 41, 47)
	check("assists", ast/n, 23, 29)
	check("turnovers", tov/n, 12, 15)
	check("3PA", tpa/n, 30, 40)
	check("FTA", fta/n, 18, 26)
	check("home win%", 100*hw/float64(len(s.Games)), 54, 60)
}

func TestTeamBalance(t *testing.T) {
	for _, seed := range []uint64{7, 2} {
		lg := GenerateLeague(seed)
		for y := 0; y < 5; y++ {
			s := playSeasonStats(lg)
			for id, w := range s.Wins {
				if w > 70 || w < 12 {
					t.Errorf("seed %d year %d: team %d went %d-%d", seed, s.Year, id, w, s.Losses[id])
				}
			}
		}
	}
}

func playSeasonStats(lg *League) *Season {
	s := lg.StartSeason()
	lg.SimRegularSeason()
	lg.SimPlayoffs()
	lg.Offseason()
	return s
}

func TestUserGameRecordingKeepsResults(t *testing.T) {
	a, b := GenerateLeague(21), GenerateLeague(21)
	b.UserTeamID = (a.UserTeamID + 7) % NumTeams
	a.StartSeason()
	b.StartSeason()
	for d := 0; d < 5; d++ {
		a.SimDay()
		b.SimDay()
		if a.LastBox == nil || len(a.LastBox.Events) == 0 {
			t.Fatalf("day %d: no events in LastBox", d)
		}
	}
	for i := 0; i < 5*GamesPerDay; i++ {
		ga, gb := a.Season.Games[i], b.Season.Games[i]
		if ga.HomeScore != gb.HomeScore || ga.AwayScore != gb.AwayScore {
			t.Fatalf("game %d differs: %d-%d vs %d-%d", i, ga.HomeScore, ga.AwayScore, gb.HomeScore, gb.AwayScore)
		}
	}
	for id, ps := range a.Season.Stats {
		if *ps != *b.Season.Stats[id] {
			t.Fatalf("stats differ for player %d", id)
		}
	}
}
