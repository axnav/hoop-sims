package main

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"hoopsim/engine"
)

// runGM measures the assistant GM. For three seeds it plays six seasons for
// one team three ways and prints regular-season wins per season, the final
// payroll, the roster size and the number of log entries per area:
//
//	A  hands-off user, stepped flow as the web app leaves a user who only
//	   presses continue: the draft is auto-picked for the user (DraftAutoAll),
//	   his expiring players are released to free agency (BeginFreeAgency),
//	   nobody is re-signed, no offers are made, then FinishOffseason.
//	B  the same with every GM auto switch on (plan balanced; the trade-alert
//	   switch stays off here because it only alerts and costs time).
//	C  the same with plan rebuild.
//
// Not the Offseason() autopilot path: that one re-signs the user's players
// like a computer team and would hide what the assistant adds.
//
// Environment (all optional): GM_ONLY letters of the variants to run (default ABCD), GM_TEAM user team id (default 0), GM_SEED0 first
// seed (default 1), GM_SEEDS number of seeds (default 3), GM_VERBOSE=1 prints
// one line per season for the GM variants: payroll, cap room when free agency
// opens, re-signs, extensions, free-agency signings (and how many above the
// minimum), draft picks, the players lost to free agency with their overall,
// the mean overall and age of the top eight at the start of the season.
func runGM() {
	const seasons = 6
	envInt := func(k string, def int) int {
		if v, err := strconv.Atoi(os.Getenv(k)); err == nil {
			return v
		}
		return def
	}
	team := envInt("GM_TEAM", 0)
	seed0 := envInt("GM_SEED0", 1)
	nSeeds := envInt("GM_SEEDS", 3)
	verbose := os.Getenv("GM_VERBOSE") == "1"
	only := os.Getenv("GM_ONLY") // letters of the variants to run, e.g. "BC" (default all)
	engine.GMTuning.LogKeep = 100000 // keep the whole log so the per-season counts are exact

	variants := []struct {
		name string
		set  func(lg *engine.League)
		gm   bool
		comp bool // the computer's own Offseason() autopilot for the user's team
	}{
		{"A hands-off", func(lg *engine.League) {}, false, false},
		{"D computer autopilot", func(lg *engine.League) {}, false, true},
		{"B GM balanced, all auto on", func(lg *engine.League) {
			lg.SetGMPlan("balanced")
			lg.SetGMAuto(engine.GMAuto{Lineup: true, Training: true, Contracts: true, FreeAgency: true, Draft: true})
		}, true, false},
		{"C GM rebuild, all auto on", func(lg *engine.League) {
			lg.SetGMPlan("rebuild")
			lg.SetGMAuto(engine.GMAuto{Lineup: true, Training: true, Contracts: true, FreeAgency: true, Draft: true})
		}, true, false},
	}
	sums := make([]float64, len(variants))
	for s := 0; s < nSeeds; s++ {
		seed := uint64(seed0 + s)
		fmt.Printf("seed %d team %d\n", seed, team)
		for vi, v := range variants {
			if only != "" && !strings.Contains(only, v.name[:1]) {
				continue
			}
			lg := engine.GenerateLeague(seed)
			lg.UserTeamID = team
			lg.Options.NoInSeasonDev = noInSeasonDev
			v.set(lg)
			var wins []string
			total := 0
			logSeen := 0
			for y := 0; y < seasons; y++ {
				lg.StartSeason()
				top8, age8 := topEight(lg, team)
				lg.SimRegularSeason()
				w := lg.Season.Wins[team]
				wins = append(wins, fmt.Sprint(w))
				total += w
				lg.SimPlayoffs()
				if v.comp {
					lg.Offseason()
					continue
				}
				lg.BeginOffseason()
				lg.DraftAutoUntilUser()
				lg.DraftAutoAll()
				type gone struct{ id, ovr, age int }
				var exp []gone
				for _, p := range lg.Expiring(team) {
					exp = append(exp, gone{p.ID, p.Overall(), p.Age})
				}
				lg.BeginFreeAgency()
				room := lg.CapRoom(team)
				pool := map[int]bool{}
				for _, p := range lg.FreeAgents {
					pool[p.ID] = true
				}
				var lost []string
				for _, g := range exp {
					if pool[g.id] {
						lost = append(lost, fmt.Sprintf("%d(%dy)", g.ovr, g.age))
					}
				}
				sort.Strings(lost)
				lg.FinishOffseason()
				signed, signedAbove := 0, 0
				for _, p := range lg.Teams[team].Roster {
					if pool[p.ID] {
						signed++
						if p.Contract.Salary > engine.Cap.MinSalary {
							signedAbove++
						}
					}
				}
				if verbose && v.gm {
					entries := lg.GMLogEntries()
					cnt := map[string]int{}
					for _, e := range entries[logSeen:] {
						switch {
						case strings.HasPrefix(e.Text, "Re-signed"):
							cnt["resign"]++
						case strings.HasPrefix(e.Text, "Extended"):
							cnt["extend"]++
						case strings.HasPrefix(e.Text, "Offered"):
							cnt["offers"]++
						case e.Area == "draft":
							cnt["draft"]++
						}
					}
					logSeen = len(entries)
					fmt.Printf("    %s y%d wins %2d top8 %.1f age %.1f | next summer: room $%.1fM resign %d extend %d offers %d FA signed %d (above min %d) draft %d lost %s | payroll after $%.1fM roster %d\n",
						v.name[:1], y+1, w, top8, age8, float64(room)/1000, cnt["resign"], cnt["extend"], cnt["offers"], signed, signedAbove, cnt["draft"],
						strings.Join(lost, ","), float64(lg.Payroll(team))/1000, len(lg.Teams[team].Roster))
				}
			}
			sums[vi] += float64(total) / seasons
			areas := map[string]int{}
			for _, e := range lg.GMLogEntries() {
				areas[e.Area]++
			}
			fmt.Printf("  %-28s wins %s  avg %.1f  payroll $%.1fM  roster %d  log %d (lineup %d, training %d, contracts %d, fa %d, draft %d)\n",
				v.name, strings.Join(wins, " "), float64(total)/seasons, float64(lg.Payroll(team))/1000, len(lg.Teams[team].Roster),
				len(lg.GMLogEntries()), areas["lineup"], areas["training"], areas["contracts"], areas["free_agency"], areas["draft"])
		}
	}
	fmt.Printf("mean wins per season over the %d seeds:\n", nSeeds)
	for i, v := range variants {
		fmt.Printf("  %-28s %.1f\n", v.name, sums[i]/float64(nSeeds))
	}
}

// topEight is the mean overall and the mean age of a team's eight best players.
func topEight(lg *engine.League, team int) (float64, float64) {
	ps := append([]*engine.Player(nil), lg.Teams[team].Roster...)
	sort.SliceStable(ps, func(i, j int) bool { return ps[i].Overall() > ps[j].Overall() })
	if len(ps) > 8 {
		ps = ps[:8]
	}
	o, a := 0.0, 0.0
	for _, p := range ps {
		o += float64(p.Overall())
		a += float64(p.Age)
	}
	n := float64(max(1, len(ps)))
	return o / n, a / n
}
