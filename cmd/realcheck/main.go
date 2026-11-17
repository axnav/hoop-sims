// Command realcheck validates the real-data league: it sims a real start
// season (flag -season, default 2011 = 2010-11) many times and compares against the real results.
package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"sort"

	"hoopsim/engine"
)

func main() {
	runs := flag.Int("runs", 30, "number of simulated seasons")
	dir := flag.String("data", "data/real", "real data directory")
	season := flag.Int("season", 2011, "real start season, ending year (2011 = 2010-11, up to 2026)")
	flag.Parse()

	realW, err := engine.RealTeamWins(*dir, *season)
	check(err)
	realPG, err := engine.RealPerGameStats(*dir, *season)
	check(err)
	realPPG, err := engine.RealLeaguePPG(*dir, *season)
	check(err)
	abbr := engine.RealTeamAbbrevsFor(*season)

	simW := make([]float64, engine.NumTeams)
	type acc struct {
		name              string
		pts, reb, ast, gp float64
	}
	sim := map[string]*acc{}
	simPts := 0.0
	var first *engine.League
	for run := 0; run < *runs; run++ {
		lg, err := engine.NewRealLeagueSeason(*dir, uint64(1000+run), *season)
		check(err)
		if run == 0 {
			first = lg
		}
		ids := map[int]string{}
		for _, t := range lg.Teams {
			for _, p := range t.Roster {
				ids[p.ID] = p.RealID
			}
		}
		lg.StartSeason()
		lg.SimRegularSeason()
		for i, w := range lg.Season.Wins {
			simW[i] += float64(w)
		}
		for _, g := range lg.Season.Games {
			simPts += float64(g.HomeScore + g.AwayScore)
		}
		for id, ps := range lg.Season.Stats {
			a := sim[ids[id]]
			if a == nil {
				a = &acc{name: ps.Name}
				sim[ids[id]] = a
			}
			a.pts += float64(ps.Pts)
			a.reb += float64(ps.Reb)
			a.ast += float64(ps.Ast)
			a.gp += float64(ps.GP)
		}
	}
	n := float64(*runs)
	simPPG := simPts / n / float64(engine.NumTeams*engine.SeasonGames) // per team... see below
	simPPG = simPts / n / float64(engine.NumTeams*engine.SeasonGames) * 1
	// simPts counts both teams per game: 1230 games, 2 teams each.
	simPPG = simPts / n / float64(engine.NumTeams*engine.SeasonGames)

	type trow struct {
		abbr         string
		real, simAvg float64
	}
	var rows []trow
	for i, a := range abbr {
		rows = append(rows, trow{a, float64(realW[a]), simW[i] / n})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].real > rows[j].real })
	fmt.Printf("%-4s %5s %6s %6s\n", "team", "real", "sim", "diff")
	var xs, ys []float64
	mae := 0.0
	for _, r := range rows {
		fmt.Printf("%-4s %5.0f %6.1f %+6.1f\n", r.abbr, r.real, r.simAvg, r.simAvg-r.real)
		xs = append(xs, r.real)
		ys = append(ys, r.simAvg)
		mae += math.Abs(r.simAvg - r.real)
	}
	fmt.Printf("win correlation %.3f   MAE %.2f wins   (sim win SD %.1f vs real %.1f)\n\n", corr(xs, ys), mae/30, sd(ys), sd(xs))

	scale := realPPG / simPPG
	fmt.Printf("League PPG per team: real %.1f, sim %.1f (gap %+.1f, scale %.3f)\n\n", realPPG, simPPG, simPPG-realPPG, scale)

	type prow struct {
		id   string
		name string
		v    float64
	}
	top := func(label string, n int, val func(engine.RealPerGame) float64, simv func(*acc) float64) {
		var ps []prow
		for id, r := range realPG {
			if r.G >= 50 {
				ps = append(ps, prow{id, r.Name, val(r)})
			}
		}
		sort.Slice(ps, func(i, j int) bool { return ps[i].v > ps[j].v })
		fmt.Printf("%-18s %7s %7s %7s\n", label, "real", "sim", "scaled")
		for _, p := range ps[:n] {
			s := 0.0
			if a := sim[p.id]; a != nil && a.gp > 0 {
				s = simv(a)
			}
			sc := 1.0
			if label == "Top scorers" {
				sc = scale
			}
			fmt.Printf("%-18s %7.1f %7.1f %7.1f\n", p.name, p.v, s, s*sc)
		}
		fmt.Println()
	}
	top("Top scorers", 15, func(r engine.RealPerGame) float64 { return r.PPG }, func(a *acc) float64 { return a.pts / a.gp })
	top("Top rebounders", 10, func(r engine.RealPerGame) float64 { return r.RPG }, func(a *acc) float64 { return a.reb / a.gp })
	top("Top assists", 10, func(r engine.RealPerGame) float64 { return r.APG }, func(a *acc) float64 { return a.ast / a.gp })

	// Talent summary.
	var all []*engine.Player
	rotSum, rotN := 0.0, 0
	for _, t := range first.Teams {
		rot := append([]*engine.Player(nil), t.Roster...)
		sort.Slice(rot, func(i, j int) bool { return rot[i].Overall() > rot[j].Overall() })
		for _, p := range rot[:10] {
			rotSum += float64(p.Overall())
			rotN++
		}
		all = append(all, t.Roster...)
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].Overall() > all[j].Overall() })
	n90 := 0
	for _, p := range all {
		if p.Overall() >= 90 {
			n90++
		}
	}
	fmt.Printf("Top-10 rotation average Overall: %.1f   90+ players: %d\n", rotSum/float64(rotN), n90)
	fmt.Printf("%-20s %-3s %-6s %4s %4s | Ins Mid 3pt Ply Reb Def Ath | Imp  Pot\n", "Top 15", "Pos", "Team", "Age", "Ovr")
	for _, p := range all[:15] {
		r := p.Ratings
		fmt.Printf("%-20s %-5s %-6s %4d %4d | %3d %3d %3d %3d %3d %3d %3d | %+3d %4d\n", p.Name, p.PosLabel(), abbr[p.TeamID], p.Age, p.Overall(),
			r.Inside, r.Mid, r.Three, r.Playmaking, r.Rebounding, r.Defense, r.Athleticism, p.Impact, p.Potential)
	}
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func sd(v []float64) float64 {
	m := 0.0
	for _, x := range v {
		m += x
	}
	m /= float64(len(v))
	s := 0.0
	for _, x := range v {
		s += (x - m) * (x - m)
	}
	return math.Sqrt(s / float64(len(v)))
}

func corr(a, b []float64) float64 {
	ma, mb := 0.0, 0.0
	for i := range a {
		ma += a[i]
		mb += b[i]
	}
	ma /= float64(len(a))
	mb /= float64(len(b))
	var sab, saa, sbb float64
	for i := range a {
		sab += (a[i] - ma) * (b[i] - mb)
		saa += (a[i] - ma) * (a[i] - ma)
		sbb += (b[i] - mb) * (b[i] - mb)
	}
	return sab / math.Sqrt(saa*sbb)
}
