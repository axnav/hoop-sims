package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"sort"
	"text/tabwriter"

	"hoopsim/engine"
)

func printBox(b *engine.TeamBox) {
	fmt.Printf("\n%s  %d\n", b.Team.FullName(), b.Score)
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Name\tPos\tOvr\tMIN\tPTS\tREB\tAST\tTOV\tFG\t3P\tFT")
	for _, l := range b.Lines {
		if l.Min == 0 {
			continue
		}
		p := l.Player
		fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%d\t%d\t%d\t%d\t%d-%d\t%d-%d\t%d-%d\n",
			p.Name, p.PosLabel(), p.Overall(), l.Min, l.Pts, l.Reb, l.Ast, l.Tov,
			l.FGM, l.FGA, l.TPM, l.TPA, l.FTM, l.FTA)
	}
	w.Flush()
}

// noInSeasonDev is set by -noinseason and copied into every league built here.
var noInSeasonDev bool

func main() {
	seed := flag.Uint64("seed", 1, "random seed")
	games := flag.Int("games", 2000, "number of league-average games to simulate")
	mode := flag.String("mode", "sanity", "sanity, seasons, ratings, develop or resign")
	seasons := flag.Int("seasons", 10, "number of seasons to simulate (mode seasons)")
	reveal := flag.Bool("reveal", false, "debug: show hidden Impact in the prospect tracker")
	noInSeason := flag.Bool("noinseason", false, "switch off in-season player development")
	flag.Parse()
	noInSeasonDev = *noInSeason

	if *mode == "seasons" {
		runSeasons(*seed, *seasons, *reveal)
		return
	}
	if *mode == "develop" {
		runDevelop(*seed, *seasons, 4)
		return
	}
	if *mode == "resign" {
		runResign(*seed, *seasons)
		return
	}
	if *mode == "ratings" {
		runRatings(*seed)
		return
	}

	lg := engine.GenerateLeague(*seed)
	r := engine.NewRNG(*seed)
	g := engine.SimGame(r, lg.Teams[0], lg.Teams[1])
	printBox(g.Home)
	printBox(g.Away)
	if g.Overtimes > 0 {
		fmt.Printf("\nFinal (%dOT): %d-%d\n", g.Overtimes, g.Home.Score, g.Away.Score)
	} else {
		fmt.Printf("\nFinal: %d-%d\n", g.Home.Score, g.Away.Score)
	}

	var pts, fgm, fga, tpm, tpa, ftm, fta, reb, ast, tov, homeWins, hi int
	for i := 0; i < *games; i++ {
		a := r.IntN(len(lg.Teams))
		b := r.IntN(len(lg.Teams) - 1)
		if b >= a {
			b++
		}
		res := engine.SimGame(r, lg.Teams[a], lg.Teams[b])
		if res.Home.Score > res.Away.Score {
			homeWins++
		}
		for _, tb := range []*engine.TeamBox{res.Home, res.Away} {
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
				hi = max(hi, l.Pts)
			}
		}
	}
	n := float64(*games * 2)
	pct := func(m, a int) float64 { return 100 * float64(m) / float64(a) }
	fmt.Printf("\nLeague averages per team per game (%d games):\n", *games)
	fmt.Printf("PTS %.1f  FG%% %.1f  3P%% %.1f  FT%% %.1f  REB %.1f  AST %.1f  TOV %.1f  3PA %.1f  FTA %.1f  HomeWin%% %.1f  HighPts %d\n",
		float64(pts)/n, pct(fgm, fga), pct(tpm, tpa), pct(ftm, fta), float64(reb)/n,
		float64(ast)/n, float64(tov)/n, float64(tpa)/n, float64(fta)/n,
		100*float64(homeWins)/float64(*games), hi)
}

func percentile(values []int, p float64) int {
	if len(values) == 0 {
		return 0
	}
	sort.Ints(values)
	i := int(math.Round(float64(len(values)-1) * p))
	return values[i]
}

func printDistribution(label string, values []int) {
	if len(values) == 0 {
		return
	}
	fmt.Printf("%-18s n=%3d  min=%2d  p10=%2d  p25=%2d  median=%2d  p75=%2d  p90=%2d  max=%2d\n",
		label, len(values), percentile(values, 0), percentile(values, 0.10), percentile(values, 0.25),
		percentile(values, 0.50), percentile(values, 0.75), percentile(values, 0.90), percentile(values, 1))
}

// runRatings summarizes the current league and draft class rating scales.
// It is a calibration aid, not a gameplay simulation.
func runRatings(seed uint64) {
	lg := engine.GenerateLeague(seed)
	var leagueOVR, leagueUsage []int
	byPosOVR := [5][]int{}
	byPosUsage := [5][]int{}
	for _, t := range lg.Teams {
		for _, p := range t.Roster {
			leagueOVR = append(leagueOVR, p.Overall())
			leagueUsage = append(leagueUsage, p.ShotUsage())
			byPosOVR[p.Pos] = append(byPosOVR[p.Pos], p.Overall())
			byPosUsage[p.Pos] = append(byPosUsage[p.Pos], p.ShotUsage())
		}
	}
	fmt.Printf("Generated league rating distribution (seed %d; primary position)\n", seed)
	printDistribution("Overall", leagueOVR)
	printDistribution("Shot tendency", leagueUsage)
	for pos := engine.PG; pos <= engine.C; pos++ {
		fmt.Printf("\n%s\n", pos)
		printDistribution("Overall", byPosOVR[pos])
		printDistribution("Shot tendency", byPosUsage[pos])
	}

	r := engine.NewRNG(seed + 1000003)
	rookies := make([]*engine.Player, engine.Progression.DraftSize)
	for i := range rookies {
		rookies[i] = engine.GenerateRookie(r, lg.NextID+i, lg.Year)
	}
	sort.SliceStable(rookies, func(i, j int) bool {
		a := engine.Progression.DraftOvrWeight*float64(rookies[i].Overall()) + engine.Progression.DraftPotWeight*float64(rookies[i].Potential)
		b := engine.Progression.DraftOvrWeight*float64(rookies[j].Overall()) + engine.Progression.DraftPotWeight*float64(rookies[j].Potential)
		return a > b
	})
	fmt.Println("\nGenerated rookie ratings by approximate draft tier (Overall / Potential)")
	for _, tier := range []struct {
		name       string
		start, end int
	}{{"Picks 1-5", 0, 5}, {"Picks 6-14", 5, 14}, {"Picks 15-30", 14, 30}, {"Picks 31-60", 30, len(rookies)}} {
		var ovr, pot []int
		for _, p := range rookies[tier.start:tier.end] {
			ovr = append(ovr, p.Overall())
			pot = append(pot, p.Potential)
		}
		printDistribution(tier.name+" OVR", ovr)
		printDistribution(tier.name+" POT", pot)
	}
}

type snap struct {
	year, age, ovr, pot int
	ok                  bool
}

func runSeasons(seed uint64, n int, reveal bool) {
	lg := engine.GenerateLeague(seed)
	lg.Options.NoInSeasonDev = noInSeasonDev
	// Track the three highest-potential players under 22.
	var prospects []*engine.Player
	for _, t := range lg.Teams {
		for _, p := range t.Roster {
			if p.Age < 22 {
				prospects = append(prospects, p)
			}
		}
	}
	sort.SliceStable(prospects, func(i, j int) bool {
		if prospects[i].Potential != prospects[j].Potential {
			return prospects[i].Potential > prospects[j].Potential
		}
		return prospects[i].ID < prospects[j].ID
	})
	prospects = prospects[:3]
	hist := make([][]snap, len(prospects))
	record := func() {
		byID := map[int]*engine.Player{}
		for _, t := range lg.Teams {
			for _, p := range t.Roster {
				byID[p.ID] = p
			}
		}
		for _, p := range lg.FreeAgents {
			byID[p.ID] = p
		}
		for i, pr := range prospects {
			if p := byID[pr.ID]; p != nil {
				hist[i] = append(hist[i], snap{lg.Year, p.Age, p.Overall(), p.Potential, true})
			} else {
				hist[i] = append(hist[i], snap{year: lg.Year})
			}
		}
	}
	name := func(id int) string { return lg.Teams[id].FullName() }
	for i := 0; i < n; i++ {
		record()
		s := lg.StartSeason()
		lg.SimRegularSeason()
		lg.SimPlayoffs()
		best := s.Standings()[0]
		fin := s.Series[len(s.Series)-1]
		loser := fin.High
		if loser == s.Champion {
			loser = fin.Low
		}
		cw, lw := fin.HighWins, fin.LowWins
		if s.Champion == fin.Low {
			cw, lw = lw, cw
		}
		fmt.Printf("== %d ==\n", s.Year)
		fmt.Printf("Best record: %s %d-%d | Champion: %s over %s %d-%d\n",
			name(best), s.Wins[best], s.Losses[best], name(s.Champion), name(loser), cw, lw)
		line := func(label string, a engine.Award) {
			fmt.Printf("%s: %s (%s) %.1f pts %.1f reb %.1f ast in %d GP\n", label, a.Name, lg.Teams[a.TeamID].Name, a.PPG, a.RPG, a.APG, a.GP)
		}
		line("MVP", s.Awards.MVP)
		line("Scoring leader", s.Awards.ScoringLeader)
		rep := lg.Offseason()
		fmt.Print("Retiring:")
		for j, r := range rep.Retirements {
			if j == 3 {
				break
			}
			fmt.Printf(" %s (%d, ovr %d);", r.Name, r.Age, r.Overall)
		}
		fmt.Print("\nTop picks:")
		for _, d := range rep.Draft[:3] {
			fmt.Printf(" #%d %s (%d, ovr %d, pot %d, %s);", d.Pick, d.Name, d.Age, d.Overall, d.Potential, lg.Teams[d.TeamID].Name)
		}
		fmt.Println()
	}
	record()
	fmt.Println("\nProspect tracker (age/overall/potential by year):")
	for i, pr := range prospects {
		extra := ""
		if reveal {
			extra = fmt.Sprintf("  [Impact %+d]", pr.Impact)
		}
		fmt.Printf("%s%s\n ", pr.Name, extra)
		for _, h := range hist[i] {
			if h.ok {
				fmt.Printf(" %d: %d/%d/%d", h.year, h.age, h.ovr, h.pot)
			} else {
				fmt.Printf(" %d: out", h.year)
			}
		}
		fmt.Println()
	}
}
