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

// focusMode is set by -focus: auto (computer picks, the default) or balanced (develop mode resets everyone to Balanced).
var focusMode = "auto"

func main() {
	seed := flag.Uint64("seed", 1, "random seed")
	games := flag.Int("games", 2000, "number of league-average games to simulate")
	mode := flag.String("mode", "sanity", "sanity, seasons, ratings, badges, coaches, morale, develop, resign or gm")
	seasons := flag.Int("seasons", 10, "number of seasons to simulate (mode seasons)")
	reveal := flag.Bool("reveal", false, "debug: show hidden Impact in the prospect tracker")
	noInSeason := flag.Bool("noinseason", false, "switch off in-season player development")
	focus := flag.String("focus", "auto", "develop mode: auto (computer teams pick a training focus) or balanced (everyone Balanced)")
	flag.Parse()
	noInSeasonDev = *noInSeason
	focusMode = *focus

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
	if *mode == "gm" {
		runGM()
		return
	}
	if *mode == "ratings" {
		runRatings(*seed)
		return
	}
	if *mode == "badges" {
		runBadges(*seed)
		return
	}
	if *mode == "morale" {
		runMorale(*seed, *seasons)
		return
	}
	if *mode == "coaches" {
		runCoaches(*seed, *seasons, *games)
		return
	}

	lg := engine.GenerateLeague(*seed)
	r := engine.NewRNG(*seed)
	g := lg.SimGame(r, lg.Teams[0], lg.Teams[1])
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
		res := lg.SimGame(r, lg.Teams[a], lg.Teams[b])
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

// runBadges prints the badge spread of a fresh league: tier counts, badges per
// player (overall 80+, other starters, bench) by group, and the Legend list.
func runBadges(seed uint64) {
	lg := engine.GenerateLeague(seed)
	groups := []string{"finishing", "shooting", "playmaking", "defense", "rebounding", "general", "personality"}
	type band struct {
		n, total int
		byGroup  map[string]int
		min, max int
	}
	newBand := func() *band { return &band{byGroup: map[string]int{}, min: 999} }
	bands := map[string]*band{"80+": newBand(), "starters": newBand(), "bench": newBand()}
	var tiers [6]int
	var legends []string
	for _, t := range lg.Teams {
		isStarter := map[int]bool{}
		for i, p := range t.Rotation() {
			if i < 5 {
				isStarter[p.ID] = true
			}
		}
		for _, p := range t.Roster {
			bs := lg.BadgesOf(p)
			name := "bench"
			if p.Overall() >= 80 {
				name = "80+"
			} else if isStarter[p.ID] {
				name = "starters"
			}
			b := bands[name]
			b.n++
			b.total += len(bs)
			b.min, b.max = min(b.min, len(bs)), max(b.max, len(bs))
			for _, x := range bs {
				b.byGroup[x.Group]++
				tiers[x.Tier]++
				if x.Tier == engine.Legend {
					legends = append(legends, fmt.Sprintf("%s (%s, %d) %s", p.Name, p.PosLabel(), p.Overall(), x.Name))
				}
			}
		}
	}
	fmt.Printf("Skill badges in the league: Bronze %d, Silver %d, Gold %d, Hall of Fame %d, Legend %d\n", tiers[1], tiers[2], tiers[3], tiers[4], tiers[5])
	for _, name := range []string{"80+", "starters", "bench"} {
		b := bands[name]
		if b.n == 0 {
			continue
		}
		fmt.Printf("%-8s (%3d players): average %.1f badges (min %d, max %d); per player by group:", name, b.n, float64(b.total)/float64(b.n), b.min, b.max)
		for _, g := range groups {
			fmt.Printf(" %s %.1f", g, float64(b.byGroup[g])/float64(b.n))
		}
		fmt.Println()
	}
	sort.Strings(legends)
	fmt.Printf("Legend badges (%d):\n", len(legends))
	for _, l := range legends {
		fmt.Println("  " + l)
	}
}

// runCoaches reports on coaches, systems and fit: the spread of coach ratings
// and team fit, the systems computer teams run, each system forced on every
// team (league numbers and pace), then seasons of wins against team fit and the
// coaches replaced in each offseason.
func runCoaches(seed uint64, seasons, games int) {
	lg := engine.GenerateLeague(seed)
	spread := func(label string, v []float64) {
		sort.Float64s(v)
		fmt.Printf("%-22s min %.1f  median %.1f  max %.1f\n", label, v[0], v[len(v)/2], v[len(v)-1])
	}
	var off, def, dev, comb []float64
	for _, t := range lg.Teams {
		c := t.Coach
		off, def, dev, comb = append(off, float64(c.Offense)), append(def, float64(c.Defense)), append(dev, float64(c.Development)), append(comb, c.Overall())
	}
	fmt.Println("Coach rating spread (30 coaches):")
	spread("Offense", off)
	spread("Defense", def)
	spread("Development", dev)
	spread("Average of the three", comb)
	counts := func() string {
		n := map[string]int{}
		for _, t := range lg.Teams {
			n[t.SystemKey()]++
		}
		out := ""
		for _, sy := range engine.Systems {
			out += fmt.Sprintf(" %s %d", sy.Key, n[sy.Key])
		}
		return out
	}
	fits := func() []float64 {
		var v []float64
		for _, t := range lg.Teams {
			v = append(v, lg.TeamFit(t))
		}
		return v
	}
	fmt.Println("\nSystems run by computer teams at league creation:" + counts())
	spread("Team fit (score)", fits())
	var grades = map[string]int{}
	var scores []float64
	for _, t := range lg.Teams {
		for _, pm := range t.EffectiveMinutes() {
			if pm.Min >= 20 {
				f := lg.PlayerFit(t, pm.Player)
				grades[f.Grade()]++
				scores = append(scores, f.Score)
			}
		}
	}
	spread("Starter fit (20+ min)", scores)
	fmt.Printf("Starter grades: A %d, B %d, C %d, D %d\n", grades["A"], grades["B"], grades["C"], grades["D"])

	fmt.Printf("\nEvery team forced onto one system (%d games each, same random pairs):\n", games)
	fmt.Printf("%-14s %6s %6s %6s %6s %6s %6s %8s\n", "system", "PTS", "FG%", "3P%", "3PA", "poss", "teamFit", "fitSpread")
	saved := make([]string, len(lg.Teams))
	for i, t := range lg.Teams {
		saved[i] = t.System
	}
	for _, sy := range append([]engine.System{{Key: ""}}, engine.Systems...) {
		for i, t := range lg.Teams {
			t.System = saved[i]
			if sy.Key != "" {
				lg.SetSystem(t.ID, sy.Key)
			}
		}
		r := engine.NewRNG(seed + 99)
		var pts, fgm, fga, tpm, tpa, tov, fta, oreb int
		_ = oreb
		for i := 0; i < games; i++ {
			a := r.IntN(len(lg.Teams))
			b := r.IntN(len(lg.Teams) - 1)
			if b >= a {
				b++
			}
			res := lg.SimGame(r, lg.Teams[a], lg.Teams[b])
			for _, tb := range []*engine.TeamBox{res.Home, res.Away} {
				pts += tb.Score
				for _, l := range tb.Lines {
					fgm, fga, tpm, tpa, tov, fta = fgm+l.FGM, fga+l.FGA, tpm+l.TPM, tpa+l.TPA, tov+l.Tov, fta+l.FTA
				}
			}
		}
		n := float64(games * 2)
		fv := fits()
		sort.Float64s(fv)
		label := sy.Key
		if label == "" {
			label = "(as chosen)"
		}
		// possessions per team, approximately: shots + turnovers + 0.44 x free-throw attempts
		poss := (float64(fga+tov) + 0.44*float64(fta)) / n
		fmt.Printf("%-14s %6.1f %6.1f %6.1f %6.1f %6.1f %6.1f %4.0f-%.0f\n", label, float64(pts)/n, 100*float64(fgm)/float64(fga),
			100*float64(tpm)/float64(tpa), float64(tpa)/n, poss, mean(fv), fv[0], fv[len(fv)-1])
	}
	for i, t := range lg.Teams {
		t.System = saved[i]
	}

	// Strength: one team runs the system, the other is Balanced, same random
	// pairs; margin per game of the system team (home advantage cancels against
	// the Balanced-vs-Balanced row; coaches and fit included).
	fmt.Printf("\nOne team on the system against a Balanced opponent (%d games each, margin of the system team):\n", games)
	for _, t := range lg.Teams {
		lg.SetSystem(t.ID, "balanced")
	}
	var base float64
	for _, sy := range engine.Systems {
		r := engine.NewRNG(seed + 7)
		tot := 0.0
		for i := 0; i < games; i++ {
			a := r.IntN(len(lg.Teams))
			b := r.IntN(len(lg.Teams) - 1)
			if b >= a {
				b++
			}
			lg.SetSystem(a, sy.Key)
			res := lg.SimGame(r, lg.Teams[a], lg.Teams[b])
			tot += float64(res.Home.Score - res.Away.Score)
			lg.SetSystem(a, "balanced")
		}
		m := tot / float64(games)
		if sy.Key == "balanced" {
			base = m
		}
		fmt.Printf("%-14s %+5.2f\n", sy.Key, m-base)
	}
	for i, t := range lg.Teams {
		t.System = saved[i]
	}

	fmt.Printf("\n%d seasons (wins against team fit at the start of the season):\n", seasons)
	var xs, ys, os []float64
	for i := 0; i < seasons; i++ {
		s := lg.StartSeason()
		fv := fits()
		lg.SimRegularSeason()
		for _, t := range lg.Teams {
			tw, tm := 0.0, 0.0
			for _, pm := range t.EffectiveMinutes() {
				tw += float64(pm.Min * pm.Player.Overall())
				tm += float64(pm.Min)
			}
			os = append(os, tw/tm)
			xs = append(xs, fv[t.ID])
			ys = append(ys, float64(s.Wins[t.ID]))
		}
		lg.SimPlayoffs()
		before := map[int]*engine.Coach{}
		for _, t := range lg.Teams {
			before[t.ID] = t.Coach
		}
		lg.Offseason()
		changed, offFav := 0, 0
		for _, t := range lg.Teams {
			if t.Coach != before[t.ID] {
				changed++
			}
			if !t.OnFavourite() {
				offFav++
			}
		}
		fmt.Printf("%d: coaches replaced %d, teams off their coach's favourite %d, systems:%s\n", s.Year, changed, offFav, counts())
	}
	fmt.Printf("Correlation of wins with team fit over %d team-seasons: %.2f\n", len(xs), corr(xs, ys))
	// Roster quality drives both wins and fit: take it out of both first.
	rx, ry := resid(xs, os), resid(ys, os)
	fmt.Printf("Same, after taking out the roster's average overall: %.2f\n", corr(rx, ry))
}

// resid is y minus its straight-line fit on x.
func resid(y, x []float64) []float64 {
	mx, my := mean(x), mean(y)
	var sxy, sxx float64
	for i := range x {
		sxy += (x[i] - mx) * (y[i] - my)
		sxx += (x[i] - mx) * (x[i] - mx)
	}
	b := sxy / sxx
	out := make([]float64, len(y))
	for i := range y {
		out[i] = y[i] - my - b*(x[i]-mx)
	}
	return out
}

func mean(v []float64) float64 {
	s := 0.0
	for _, x := range v {
		s += x
	}
	return s / float64(len(v))
}

func corr(x, y []float64) float64 {
	mx, my := mean(x), mean(y)
	var sxy, sxx, syy float64
	for i := range x {
		sxy += (x[i] - mx) * (y[i] - my)
		sxx += (x[i] - mx) * (x[i] - mx)
		syy += (y[i] - my) * (y[i] - my)
	}
	return sxy / math.Sqrt(sxx*syy)
}

// runMorale reports the mood spread by label at mid-season and at the end of
// each season, and the trade requests (league wide and on the user's team
// that does nothing: the offseason is the autopilot).
func runMorale(seed uint64, seasons int) {
	lg := engine.GenerateLeague(seed)
	labels := []string{"Furious", "Unhappy", "Content", "Happy", "Thrilled"}
	spread := func(when string) {
		n := map[string]int{}
		sum, tot, lo, hi := 0, 0, 0, 0
		asking := 0
		for _, t := range lg.Teams {
			for _, p := range t.Roster {
				n[engine.MoraleLabel(p.Morale)]++
				sum += p.Morale
				tot++
				lo, hi = min(lo, p.Morale), max(hi, p.Morale)
				if p.TradeRequest > 0 {
					asking++
				}
			}
		}
		fmt.Printf("  %-11s mean %+.1f  min %+d  max %+d  asking %d |", when, float64(sum)/float64(tot), lo, hi, asking)
		for _, l := range labels {
			fmt.Printf(" %s %d", l, n[l])
		}
		fmt.Println()
	}
	var reqs, withdrawn, traded, userReqs []int
	for y := 0; y < seasons; y++ {
		lg.StartSeason()
		year := lg.Year
		before := len(lg.Events)
		_ = before
		seen := 0
		for lg.SimDay() {
			if lg.Season.Day == 41 {
				fmt.Printf("Season %d\n", y+1)
				spread("day 41")
			}
		}
		spread("season end")
		var r, w, t, u int
		for _, e := range lg.Events {
			if e.Year != year {
				continue
			}
			switch e.Type {
			case engine.EvTradeRequest:
				r++
				if e.TeamID == lg.UserTeamID {
					u++
				}
			case engine.EvRequestWithdrawn:
				w++
			case engine.EvRequestTraded:
				t++
			}
		}
		_ = seen
		reqs, withdrawn, traded, userReqs = append(reqs, r), append(withdrawn, w), append(traded, t), append(userReqs, u)
		fmt.Printf("  requests %d (user team %d), withdrawn %d, traded %d\n", r, u, w, t)
		if err := lg.SimPlayoffs(); err != nil {
			fmt.Println("playoffs:", err)
			return
		}
		lg.Offseason()
	}
	sum := func(v []int) (s int) {
		for _, x := range v {
			s += x
		}
		return
	}
	fmt.Printf("\nOver %d seasons: requests %d (%.1f a season), user team %d, withdrawn %d, traded %d\n",
		seasons, sum(reqs), float64(sum(reqs))/float64(seasons), sum(userReqs), sum(withdrawn), sum(traded))
}
