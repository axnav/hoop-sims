package engine

import "testing"

func TestDeterminism(t *testing.T) {
	lg := GenerateLeague(7)
	a := SimGame(NewRNG(42), lg.Teams[0], lg.Teams[1])
	b := SimGame(NewRNG(42), lg.Teams[0], lg.Teams[1])
	if a.Home.Score != b.Home.Score || a.Away.Score != b.Away.Score || a.Overtimes != b.Overtimes {
		t.Fatalf("not deterministic: %d-%d vs %d-%d", a.Home.Score, a.Away.Score, b.Home.Score, b.Away.Score)
	}
}

func TestInvariants(t *testing.T) {
	lg := GenerateLeague(3)
	r := NewRNG(5)
	for i := 0; i < 200; i++ {
		x := r.IntN(30)
		y := (x + 1 + r.IntN(29)) % 30
		g := SimGame(r, lg.Teams[x], lg.Teams[y])
		if g.Home.Score == g.Away.Score {
			t.Fatal("tied game")
		}
		for _, tb := range []*TeamBox{g.Home, g.Away} {
			pts, min := 0, 0
			for _, l := range tb.Lines {
				pts += l.Pts
				min += l.Min
			}
			if pts != tb.Score {
				t.Fatalf("score %d != sum of points %d", tb.Score, pts)
			}
			if min != 240 {
				t.Fatalf("minutes %d != 240", min)
			}
		}
	}
}

func TestLeagueAverages(t *testing.T) {
	lg := GenerateLeague(1)
	r := NewRNG(1)
	var pts, fgm, fga, tpm, tpa, ftm, fta, reb, ast, tov, hw int
	const games = 2000
	for i := 0; i < games; i++ {
		a := r.IntN(30)
		b := (a + 1 + r.IntN(29)) % 30
		g := SimGame(r, lg.Teams[a], lg.Teams[b])
		if g.Home.Score > g.Away.Score {
			hw++
		}
		for _, tb := range []*TeamBox{g.Home, g.Away} {
			pts += tb.Score
			for _, l := range tb.Lines {
				fgm += l.FGM
				fga += l.FGA
				tpm += l.TPM
				tpa += l.TPA
				ftm += l.FTM
				fta += l.FTA
				reb += l.Reb
				ast += l.Ast
				tov += l.Tov
			}
		}
	}
	n := float64(games * 2)
	check := func(name string, v, lo, hi float64) {
		if v < lo || v > hi {
			t.Errorf("%s = %.2f outside [%.0f, %.0f]", name, v, lo, hi)
		}
	}
	check("points", float64(pts)/n, 108, 118)
	check("FG%", 100*float64(fgm)/float64(fga), 45, 49)
	check("3P%", 100*float64(tpm)/float64(tpa), 34, 38)
	check("FT%", 100*float64(ftm)/float64(fta), 74, 80)
	check("rebounds", float64(reb)/n, 41, 47)
	check("assists", float64(ast)/n, 23, 29)
	check("turnovers", float64(tov)/n, 12, 15)
	check("3PA", float64(tpa)/n, 30, 40)
	check("FTA", float64(fta)/n, 18, 26)
	check("home win%", 100*float64(hw)/games, 54, 60)
}
