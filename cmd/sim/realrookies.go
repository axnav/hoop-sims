package main

import (
	"fmt"
	"os"
	"sort"

	"hoopsim/engine"
)

// runRealRookies is the measuring tool for real prospects and young stars
// (HANDOFF section 23, parts B and C). A start before 2025 measures how far
// the real draft classes of 2015-2023 grow against their real peak; a start
// of 2025 or 2026 prints the ceilings of the newest classes and the count of
// 90+ and 85+ players per simulated year against the fictional league.
func runRealRookies(start, seasons, seeds int) {
	if start >= 2025 {
		realRookiesFuture(start, seasons, seeds)
		return
	}
	realRookiesPast(start, seasons, seeds)
}

func everyone(lg *engine.League) []*engine.Player {
	var ps []*engine.Player
	for _, t := range lg.Teams {
		ps = append(ps, t.Roster...)
	}
	return append(ps, lg.FreeAgents...)
}

type rookieRec struct {
	name              string
	year, pick        int
	pot0, simPeak, rp int
}

func realRookiesPast(start, seasons, seeds int) {
	last := min(start+seasons-1, 2026)
	// Real peak per player over the seasons the sim covers (part A ratings).
	realPeak := map[string]int{}
	for s := start; s <= last; s++ {
		lg, err := engine.NewRealLeagueSeason("data/real", 1, s)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		for _, p := range everyone(lg) {
			if o := p.Overall(); o > realPeak[p.RealID] {
				realPeak[p.RealID] = o
			}
		}
	}
	res := map[string][]rookieRec{}
	var order []string
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		lg, err := engine.NewRealLeagueSeason("data/real", seed, start)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		tr := map[string]*rookieRec{}
		seen := map[string]bool{}
		note := func(p *engine.Player) {
			if p.RealID == "" {
				return
			}
			r := tr[p.RealID]
			if r == nil {
				// First sight is before any simulated draft, so the real
				// draft year and pick are still on the player.
				if seen[p.RealID] {
					return
				}
				seen[p.RealID] = true
				if p.DraftYear < 2015 || p.DraftYear > 2023 || p.DraftPick < 1 {
					return
				}
				r = &rookieRec{name: p.Name, year: p.DraftYear, pick: p.DraftPick, pot0: p.Potential}
				tr[p.RealID] = r
			}
			if o := p.Overall(); o > r.simPeak {
				r.simPeak = o
			}
		}
		for _, cls := range lg.DraftClasses {
			for _, p := range cls {
				note(p)
			}
		}
		for i := 0; i < seasons; i++ {
			for _, p := range everyone(lg) {
				note(p)
			}
			for _, p := range lg.DraftClasses[lg.Year] {
				note(p)
			}
			lg.StartSeason()
			lg.SimRegularSeason()
			lg.SimPlayoffs()
			lg.Offseason()
		}
		for _, p := range everyone(lg) {
			note(p)
		}
		for id, r := range tr {
			if _, ok := res[id]; !ok {
				order = append(order, id)
			}
			r.rp = realPeak[id]
			res[id] = append(res[id], *r)
		}
	}
	sort.Slice(order, func(i, j int) bool {
		a, b := res[order[i]][0], res[order[j]][0]
		if a.year != b.year {
			return a.year < b.year
		}
		return a.pick < b.pick
	})
	fmt.Println("year pick name                      realPeak | ceiling per seed | simPeak per seed | worst gap")
	var n85, allIn, mostIn, n75, hi3 int
	var short85, short75, cnt85, cnt75 float64
	for _, id := range order {
		rs := res[id]
		r0 := rs[0]
		if r0.rp == 0 {
			continue
		}
		shown := r0.pick <= 14 || r0.rp >= 75
		line := fmt.Sprintf("%d %3d %-25s %3d |", r0.year, r0.pick, r0.name, r0.rp)
		for _, r := range rs {
			line += fmt.Sprintf(" %3d", r.pot0)
		}
		line += " |"
		worst, within, at90 := 0, 0, 0
		for _, r := range rs {
			line += fmt.Sprintf(" %3d", r.simPeak)
			g := max(0, r.rp-r.simPeak)
			worst = max(worst, g)
			if g <= 5 {
				within++
			}
			if r.simPeak >= 90 {
				at90++
			}
			switch {
			case r.rp >= 85:
				short85 += float64(g)
				cnt85++
			case r.rp >= 75:
				short75 += float64(g)
				cnt75++
			}
		}
		if shown {
			fmt.Printf("%s | %+d\n", line, -worst)
		}
		switch {
		case r0.rp >= 85:
			n85++
			if within == len(rs) {
				allIn++
			}
			if within*2 > len(rs) {
				mostIn++
			}
			if at90 == len(rs) && r0.rp < 92 {
				hi3++
			}
		case r0.rp >= 75:
			n75++
		}
	}
	mean := func(a, b float64) float64 {
		if b == 0 {
			return 0
		}
		return a / b
	}
	fmt.Printf("SUMMARY start %d, %d seasons, %d seeds: real peak 85+: %d players, within 5 in all seeds %d, in most seeds %d, mean shortfall %.1f; "+
		"real peak 75-84: %d players, mean shortfall %.1f; 90+ in every seed with real peak under 92: %d\n",
		start, seasons, seeds, n85, allIn, mostIn, mean(short85, cnt85), n75, mean(short75, cnt75), hi3)
}

func countStars(lg *engine.League) (n90, n85 int) {
	for _, p := range everyone(lg) {
		if o := p.Overall(); o >= 90 {
			n90++
			n85++
		} else if o >= 85 {
			n85++
		}
	}
	return
}

func realRookiesFuture(start, seasons, seeds int) {
	type yc struct{ n90, n85 float64 }
	real := make([]yc, seasons)
	fict := make([]yc, seasons)
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		lg, err := engine.NewRealLeagueSeason("data/real", seed, start)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		line := func(label string, ps []*engine.Player) {
			sort.SliceStable(ps, func(i, j int) bool { return ps[i].DraftPick < ps[j].DraftPick })
			s := fmt.Sprintf("seed %d %s:", seed, label)
			for _, p := range ps {
				if p.DraftPick <= 5 {
					s += fmt.Sprintf(" #%d %s %d/%d;", p.DraftPick, p.Name, p.Overall(), p.Potential)
				}
			}
			fmt.Println(s)
		}
		for _, y := range []int{start - 1, start} {
			var ps []*engine.Player
			for _, p := range everyone(lg) {
				if p.DraftYear == y && p.DraftPick >= 1 {
					ps = append(ps, p)
				}
			}
			ps = append(ps, lg.DraftClasses[y]...)
			line(fmt.Sprintf("%d class (overall/ceiling)", y), ps)
		}
		fl := engine.GenerateLeague(seed)
		for i := 0; i < seasons; i++ {
			a, b := countStars(lg)
			real[i].n90 += float64(a)
			real[i].n85 += float64(b)
			a, b = countStars(fl)
			fict[i].n90 += float64(a)
			fict[i].n85 += float64(b)
			for _, l := range []*engine.League{lg, fl} {
				l.StartSeason()
				l.SimRegularSeason()
				l.SimPlayoffs()
				l.Offseason()
			}
		}
	}
	f := float64(seeds)
	fmt.Println("year  real 90+ / 85+   fictional 90+ / 85+   (mean per seed, at the start of each season)")
	var r90, r85, f90, f85 float64
	for i := 0; i < seasons; i++ {
		fmt.Printf("%d  %5.1f / %5.1f    %5.1f / %5.1f\n", start+i, real[i].n90/f, real[i].n85/f, fict[i].n90/f, fict[i].n85/f)
		r90 += real[i].n90 / f
		r85 += real[i].n85 / f
		f90 += fict[i].n90 / f
		f85 += fict[i].n85 / f
	}
	n := float64(seasons)
	fmt.Printf("SUMMARY start %d, %d seasons, %d seeds: mean per year real 90+ %.1f 85+ %.1f; fictional 90+ %.1f 85+ %.1f\n",
		start, seasons, seeds, r90/n, r85/n, f90/n, f85/n)
}
