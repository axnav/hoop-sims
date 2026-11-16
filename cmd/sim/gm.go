package main

import (
	"fmt"
	"strings"

	"hoopsim/engine"
)

// runGM measures the assistant GM. For seeds 1 to 3 it plays six seasons for
// team 0 three ways and prints regular-season wins per season, the final
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
func runGM() {
	const seasons = 6
	variants := []struct {
		name string
		set  func(lg *engine.League)
	}{
		{"A hands-off", func(lg *engine.League) {}},
		{"B GM balanced, all auto on", func(lg *engine.League) {
			lg.SetGMPlan("balanced")
			lg.SetGMAuto(engine.GMAuto{Lineup: true, Training: true, Contracts: true, FreeAgency: true, Draft: true})
		}},
		{"C GM rebuild, all auto on", func(lg *engine.League) {
			lg.SetGMPlan("rebuild")
			lg.SetGMAuto(engine.GMAuto{Lineup: true, Training: true, Contracts: true, FreeAgency: true, Draft: true})
		}},
	}
	sums := make([]float64, len(variants))
	for seed := uint64(1); seed <= 3; seed++ {
		fmt.Printf("seed %d\n", seed)
		for vi, v := range variants {
			lg := engine.GenerateLeague(seed)
			lg.UserTeamID = 0
			lg.Options.NoInSeasonDev = noInSeasonDev
			v.set(lg)
			var wins []string
			total := 0
			for y := 0; y < seasons; y++ {
				lg.StartSeason()
				lg.SimRegularSeason()
				w := lg.Season.Wins[0]
				wins = append(wins, fmt.Sprint(w))
				total += w
				lg.SimPlayoffs()
				lg.BeginOffseason()
				lg.DraftAutoUntilUser()
				lg.DraftAutoAll()
				lg.BeginFreeAgency()
				lg.FinishOffseason()
			}
			sums[vi] += float64(total) / seasons
			areas := map[string]int{}
			for _, e := range lg.GMLogEntries() {
				areas[e.Area]++
			}
			fmt.Printf("  %-28s wins %s  avg %.1f  payroll $%.1fM  roster %d  log %d (lineup %d, training %d, contracts %d, fa %d, draft %d)\n",
				v.name, strings.Join(wins, " "), float64(total)/seasons, float64(lg.Payroll(0))/1000, len(lg.Teams[0].Roster),
				len(lg.GMLogEntries()), areas["lineup"], areas["training"], areas["contracts"], areas["free_agency"], areas["draft"])
		}
	}
	fmt.Println("mean wins per season over the three seeds:")
	for i, v := range variants {
		fmt.Printf("  %-28s %.1f\n", v.name, sums[i]/3)
	}
}
