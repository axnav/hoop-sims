package main

import (
	"fmt"
	"sort"

	"hoopsim/engine"
)

// runResign plays n offseasons hands-off and prints, per offseason, the league
// payroll against the cap, how many contracts ran out, how many of those
// players were re-signed, how many reached free agency, how many said they
// would test free agency, and how many extensions the user's team made.
//
// The user's team re-signs its expiring players rated 58 or better (a stand-in
// for a manager) and, before each offseason, extends every last-year player
// who will talk. Computer teams never extend.
func runResign(seed uint64, n int) {
	lg := engine.GenerateLeague(seed)
	lg.UserTeamID = 0
	lg.Options.NoInSeasonDev = noInSeasonDev
	cap := float64(engine.Cap.SalaryCap) * float64(len(lg.Teams))
	fmt.Printf("%-6s %9s %9s %9s %9s %9s %9s\n", "year", "payroll%", "expiring", "resigned", "extended", "test", "reachedFA")
	for y := 0; y < n; y++ {
		year := lg.Year
		lg.StartSeason()
		lg.SimRegularSeason()
		lg.SimPlayoffs()
		extended := 0
		for _, p := range lg.Teams[lg.UserTeamID].Roster { // NEW
			if sal, _, err := lg.ExtensionAsk(lg.UserTeamID, p.ID); err == nil && lg.Extend(lg.UserTeamID, p.ID, sal, 3) == nil { // NEW
				extended++ // NEW
			} // NEW
		} // NEW
		lg.BeginOffseason()
		expiring := map[int]bool{}
		test := 0
		for _, t := range lg.Teams {
			for _, p := range lg.Expiring(t.ID) {
				expiring[p.ID] = true
				if lg.StanceOf(p).Key == "test" { // NEW
					test++ // NEW
				} // NEW
			}
		}
		lg.DraftAutoAll()
		exp := lg.Expiring(lg.UserTeamID)
		sort.SliceStable(exp, func(i, j int) bool { return exp[i].Overall() > exp[j].Overall() })
		for _, p := range exp {
			if p.Overall() >= 58 {
				lg.ReSign(lg.UserTeamID, p.ID)
			}
		}
		if err := lg.BeginFreeAgency(); err != nil {
			fmt.Println("error:", err)
			return
		}
		reached := 0
		for _, p := range lg.FreeAgents {
			if expiring[p.ID] {
				reached++
			}
		}
		lg.FinishOffseason()
		total := 0
		for _, t := range lg.Teams {
			total += lg.Payroll(t.ID)
		}
		fmt.Printf("%-6d %8.1f%% %9d %9d %9d %9d %9d\n", year, 100*float64(total)/cap, len(expiring), len(expiring)-reached, extended, test, reached)
	}
}
