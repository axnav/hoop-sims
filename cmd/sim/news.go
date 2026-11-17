package main

import (
	"fmt"
	"sort"

	"hoopsim/engine"
)

// runNews simulates n seasons and prints how many news stories each kind made
// per season (a season here is its regular season, playoffs, offseason and
// the next summer's trades), to tune the volume. Then the headline stories and
// award winners of the last season.
func runNews(seed uint64, n int) {
	engine.NewsTuning.Kept = 1 << 30 // keep every story while measuring
	lg := engine.GenerateLeague(seed)
	lg.Options.NoInSeasonDev = noInSeasonDev
	kinds := []string{engine.NwTrade, engine.NwSigning, engine.NwRelease, engine.NwInjury, engine.NwRequest, engine.NwGame,
		engine.NwStreak, engine.NwPlayoffs, engine.NwDraft, engine.NwAward, engine.NwRetire, engine.NwCoach, engine.NwLeague}
	fmt.Printf("%-6s", "season")
	for _, k := range kinds {
		fmt.Printf(" %9s", k)
	}
	fmt.Printf(" %7s\n", "total")
	last := 0
	var lastItems []engine.NewsItem
	var s *engine.Season
	for i := 0; i < n; i++ {
		s = lg.StartSeason()
		year := s.Year
		lg.SimRegularSeason()
		lg.SimPlayoffs()
		lg.Offseason()
		var items []engine.NewsItem
		for _, it := range lg.News {
			if it.ID > last {
				items = append(items, it)
			}
		}
		if len(lg.News) > 0 {
			last = lg.News[len(lg.News)-1].ID
		}
		count := map[string]int{}
		for _, it := range items {
			count[it.Kind]++
		}
		fmt.Printf("%-6d", year)
		for _, k := range kinds {
			fmt.Printf(" %9d", count[k])
		}
		fmt.Printf(" %7d\n", len(items))
		lastItems = items
	}
	fmt.Printf("\nHeadline stories of the last season:\n")
	shown := 0
	for _, it := range lastItems {
		if it.Big && shown < 15 {
			fmt.Printf("  [%s %s] %s\n", it.Stage, it.Kind, it.Text)
			shown++
		}
	}
	big := 0
	for _, it := range lastItems {
		if it.Big {
			big++
		}
	}
	fmt.Printf("  (%d headline stories in all)\n", big)

	rec := lg.Records[len(lg.Records)-1]
	name := func(a engine.Award) string {
		if a.Name == "" {
			return "-"
		}
		return fmt.Sprintf("%s (%s) %.1f/%.1f/%.1f in %d GP", a.Name, lg.Teams[a.TeamID].Name, a.PPG, a.RPG, a.APG, a.GP)
	}
	fmt.Printf("\nAwards %d:\n", rec.Year)
	fmt.Println(" MVP:", name(rec.MVP))
	fmt.Println(" Finals MVP:", name(rec.FinalsMVP))
	fmt.Println(" Scoring leader:", name(rec.ScoringLeader))
	fmt.Println(" Rookie of the Year:", name(rec.ROY))
	fmt.Println(" Defensive Player of the Year:", name(rec.DPOY))
	fmt.Println(" Sixth Man:", name(rec.SixthMan))
	fmt.Println(" Most Improved:", name(rec.MIP))
	fmt.Printf(" Coach of the Year: %s (%s) %d-%d\n", rec.Coach.Name, lg.Teams[rec.Coach.TeamID].Name, rec.Coach.W, rec.Coach.L)
	for i, tm := range rec.AllLeague {
		fmt.Printf(" All-League team %d:", i+1)
		for _, a := range tm {
			fmt.Printf(" %s;", a.Name)
		}
		fmt.Println()
	}
	fmt.Print(" All-Rookie:")
	for _, a := range rec.AllRookie {
		fmt.Printf(" %s;", a.Name)
	}
	fmt.Println()
	_ = sort.Ints
}
