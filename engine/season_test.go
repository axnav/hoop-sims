package engine

import "testing"

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

func TestLongStability(t *testing.T) {
	lg := GenerateLeague(1)
	start := rotationAvg(lg)
	type pk struct{ pot, peak int }
	picks := map[int]*pk{}
	lo90, hi90 := 99, 0
	for y := 0; y < 30; y++ {
		playSeason(lg)
		s := lg.Season
		avg := rotationAvg(lg)
		if avg < start-2 || avg > start+2 {
			t.Errorf("season %d: rotation avg %.2f vs start %.2f", y, avg, start)
		}
		n := count90(lg)
		lo90, hi90 = min(lo90, n), max(hi90, n)
		if n < 2 || n > 15 {
			t.Errorf("season %d: %d players at 90+", y, n)
		}
		for _, p := range lg.allPlayers() {
			if k := picks[p.ID]; k != nil {
				k.peak = max(k.peak, p.Overall())
			}
		}
		if y == 29 {
			checkAverages(t, s)
		}
		rep := lg.Offseason()
		for _, d := range rep.Draft {
			if d.Pick <= 10 && y <= 20 { // later classes have not peaked yet
				picks[d.PlayerID] = &pk{d.Potential, d.Overall}
			}
		}
		for _, p := range lg.allPlayers() {
			if k := picks[p.ID]; k != nil {
				k.peak = max(k.peak, p.Overall())
			}
		}
	}
	near, tot, sum := 0, 0, 0
	hist := make([]int, 20)
	for _, k := range picks {
		tot++
		sum += k.peak - k.pot
		hist[min(19, max(0, (k.peak-k.pot+40)/4))]++
		if d := k.peak - k.pot; d >= -4 && d <= 4 {
			near++
		}
	}
	t.Logf("hist (4-wide bins from -30): %v", hist)
	t.Logf("mean peak-pot %.1f", float64(sum)/float64(tot))
	t.Logf("start avg %.2f final avg %.2f, 90+ range %d-%d, top-10 picks within +-4 of draft potential at peak: %d/%d (%.0f%%)",
		start, rotationAvg(lg), lo90, hi90, near, tot, 100*float64(near)/float64(tot))
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
