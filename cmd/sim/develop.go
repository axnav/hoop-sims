package main

import (
	"fmt"

	"hoopsim/engine"
)

// developMinYears is how many seasons a draft class needs after its draft to
// count in the measurement (a 19-year-old reaches his prime at 27).
const developMinYears = 8

// runDevelop measures the development tuning target: the share of top-10
// draft picks whose best overall ends within 4 points of their draft-day
// potential (target 55 to 65%). It runs `leagues` leagues of n seasons each,
// starting at seed, and also prints how many 90+ players and what average
// overall each league ends with.
func runDevelop(seed uint64, n, leagues int) {
	type pick struct {
		p         *engine.Player
		pot, peak int
		year      int
	}
	within, total, sumDiff := 0, 0, 0
	for k := 0; k < leagues; k++ {
		lg := engine.GenerateLeague(seed + uint64(k))
		lg.Options.NoInSeasonDev = noInSeasonDev
		var picks []*pick
		for i := 0; i < n; i++ {
			lg.StartSeason()
			lg.SimRegularSeason()
			lg.SimPlayoffs()
			year := lg.Year
			lg.Offseason()
			for _, pk := range picks {
				pk.peak = max(pk.peak, pk.p.Overall())
			}
			for _, t := range lg.Teams {
				for _, p := range t.Roster {
					if p.DraftYear == year && p.DraftPick >= 1 && p.DraftPick <= 10 {
						picks = append(picks, &pick{p: p, pot: p.Potential, peak: p.Overall(), year: i})
					}
				}
			}
		}
		lw, lt := 0, 0
		for _, pk := range picks {
			if n-1-pk.year < developMinYears {
				continue
			}
			lt++
			d := pk.peak - pk.pot
			sumDiff += d
			if d >= -4 && d <= 4 {
				lw++
			}
		}
		stars, sum, cnt := 0, 0, 0
		for _, t := range lg.Teams {
			for _, p := range t.Roster {
				cnt++
				sum += p.Overall()
				if p.Overall() >= 90 {
					stars++
				}
			}
		}
		fmt.Printf("seed %d: top-10 picks within 4 of draft-day potential %d/%d (%.0f%%) | after %d seasons: %d players 90+, average overall %.1f\n",
			seed+uint64(k), lw, lt, 100*float64(lw)/float64(max(lt, 1)), n, stars, float64(sum)/float64(max(cnt, 1)))
		within += lw
		total += lt
	}
	fmt.Printf("ALL: %d/%d (%.1f%%) within 4 points; mean (peak - potential) %.2f; in-season development %s\n",
		within, total, 100*float64(within)/float64(max(total, 1)), float64(sumDiff)/float64(max(total, 1)),
		map[bool]string{true: "OFF", false: "ON"}[noInSeasonDev])
}
