package main

import (
	"fmt"
	"os"
	"sort"
	"strconv"

	"hoopsim/engine"
)

// runOwner measures the owner goals and trust. For seeds 1..5 (OWNER_SEEDS to
// change, OWNER_SEED0 for the first) team 0 (OWNER_TEAM) plays eight seasons
// (OWNER_SEASONS) three ways, with firing on; when fired the user takes the first
// job and goes on:
//
//	A  hands-off, the stepped flow of cmd/sim/gm.go variant A (draft auto-picked,
//	   expiring players released to free agency, nobody re-signed).
//	B  the assistant GM on auto, plan balanced (as gm.go variant B).
//	C  a bad GM: like A, but every expiring player is re-signed at his ask and
//	   the best free agents are bid for until the payroll passes the tax line.
//
// Per season it prints wins, trust start -> end, goals met, profit, budget and
// payroll; then the seasons until the first firing per variant.
func runOwner() {
	envInt := func(k string, def int) int {
		if v, err := strconv.Atoi(os.Getenv(k)); err == nil {
			return v
		}
		return def
	}
	team := envInt("OWNER_TEAM", 0)
	seed0 := envInt("OWNER_SEED0", 1)
	nSeeds := envInt("OWNER_SEEDS", 5)
	seasons := envInt("OWNER_SEASONS", 8)
	verbose := os.Getenv("OWNER_QUIET") == ""

	variants := []string{"A hands-off", "B assistant GM (balanced, auto)", "C bad GM (re-sign all, bid to the tax)"}
	firstFire := make([][]int, len(variants)) // seasons until the first firing, per run that had one
	fired := make([]int, len(variants))
	played := make([]int, len(variants))
	metSum := make([]int, len(variants))
	goalSum := make([]int, len(variants))
	winSum := make([]int, len(variants))
	profSum := make([]int, len(variants))
	kindMet := make([][3]int, len(variants))
	for s := 0; s < nSeeds; s++ {
		seed := uint64(seed0 + s)
		fmt.Printf("seed %d team %d\n", seed, team)
		for vi, name := range variants {
			lg := engine.GenerateLeague(seed)
			lg.UserTeamID = team
			lg.Options.NoInSeasonDev = noInSeasonDev
			if vi == 1 {
				lg.SetGMPlan("balanced")
				lg.SetGMAuto(engine.GMAuto{Lineup: true, Training: true, Contracts: true, FreeAgency: true, Draft: true})
			}
			first := 0
			fmt.Printf("  %s (owner %s, %s, %s)\n", name, lg.OwnerOf(team).Name, lg.OwnerOf(team).Personality, lg.OwnerOf(team).Market)
			for y := 1; y <= seasons; y++ {
				lg.StartSeason()
				o := lg.Owner
				budget, payStart := o.Budget, lg.Payroll(lg.UserTeamID)
				cur := lg.UserTeamID
				lg.SimRegularSeason()
				lg.SimPlayoffs()
				pay := lg.Payroll(cur)
				lg.BeginOffseason()
				row := o.Ledger[len(o.Ledger)-1]
				met := 0
				for gi, g := range row.Goals {
					if g.Met {
						met++
						kindMet[vi][gi]++
					}
				}
				played[vi]++
				metSum[vi] += met
				goalSum[vi] += len(row.Goals)
				winSum[vi] += row.Wins
				profSum[vi] += row.Profit
				tag := ""
				if o.Fired {
					tag = "  FIRED"
					fired[vi]++
					if first == 0 {
						first = y
						firstFire[vi] = append(firstFire[vi], y)
					}
				}
				if verbose {
					fmt.Printf("    y%d team %2d wins %2d  trust %3d -> %3d  goals %d/3  profit %7.1fM  budget %6.1fM payroll start %6.1fM end %6.1fM  %s%s\n",
						y, cur, row.Wins, row.TrustStart, row.TrustEnd, met, float64(row.Profit)/1000, float64(budget)/1000,
						float64(payStart)/1000, float64(pay)/1000, row.Playoffs, tag)
				}
				if o.Fired {
					jobs := lg.OwnerJobs()
					if _, err := lg.TakeJob(jobs[0]); err != nil {
						panic(err)
					}
				}
				// the offseason, stepped
				lg.DraftAutoUntilUser()
				lg.DraftAutoAll()
				if vi == 2 {
					exp := lg.Expiring(lg.UserTeamID)
					for _, p := range exp {
						lg.ReSign(lg.UserTeamID, p.ID)
					}
				}
				lg.BeginFreeAgency()
				if vi == 2 {
					badBids(lg)
				}
				lg.FinishOffseason()
			}
		}
	}
	fmt.Println("totals per variant (all seeds):")
	for vi, name := range variants {
		n := float64(max(1, played[vi]))
		sort.Ints(firstFire[vi])
		fmt.Printf("  %-40s wins/season %.1f  goals met %.2f/3  profit %.1fM/season  met by kind (results/money/building) %.0f%%/%.0f%%/%.0f%%  firings %d in %d seasons  first firing in season %v (of %d seeds)\n",
			name, float64(winSum[vi])/n, float64(metSum[vi])/n*3/(float64(goalSum[vi])/n), float64(profSum[vi])/n/1000,
			100*float64(kindMet[vi][0])/n, 100*float64(kindMet[vi][1])/n, 100*float64(kindMet[vi][2])/n, fired[vi], played[vi], firstFire[vi], nSeeds)
	}
}

// badBids bids for the best free agents at their asking price until the payroll
// passes the tax line or an offer is refused.
func badBids(lg *engine.League) {
	user := lg.UserTeamID
	pool := append([]*engine.Player(nil), lg.FreeAgents...)
	sort.SliceStable(pool, func(i, j int) bool { return pool[i].Overall() > pool[j].Overall() })
	for _, p := range pool {
		if len(lg.Teams[user].Roster)+len(lg.OffersBy(user)) >= engine.RosterSize || lg.Payroll(user) > engine.Cap.TaxLine+8000 {
			break
		}
		salary, years := lg.FreeAgentAsk(p)
		lg.MakeOffer(user, p.ID, salary, years)
	}
}
