package main

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"hoopsim/engine"
)

// runAttributes prints one team's seven ratings, 22 attributes and 7
// tendencies, then league-wide checks (mean and SD of each attribute,
// per-position means of Dunk, Speed, Three tendency and Post Up).
func runAttributes(seed uint64, real bool, season, team string) {
	var lg *engine.League
	if real {
		y, err := strconv.Atoi(season)
		if err != nil {
			fmt.Fprintln(os.Stderr, "season must be a year")
			os.Exit(1)
		}
		lg, err = engine.NewRealLeagueSeason("data/real", seed, y)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	} else {
		lg = engine.GenerateLeague(seed)
	}
	var players []*engine.Player
	for _, t := range lg.Teams {
		players = append(players, t.Roster...)
	}
	keys := engine.AttrKeys()
	tends := engine.TendencyKeys()

	if team == "" {
		team = lg.Teams[0].Name
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)
	for _, t := range lg.Teams {
		if !strings.Contains(strings.ToLower(t.FullName()), strings.ToLower(team)) {
			continue
		}
		fmt.Printf("%s\n", t.FullName())
		short := func(s string) string {
			f := strings.Fields(s)
			out := ""
			for _, x := range f {
				out += x[:1]
			}
			if len(f) == 1 {
				out = s
				if len(out) > 5 {
					out = out[:5]
				}
			}
			return out
		}
		hdr := "Name\tPos\tOvr\tIns Mid Thr Ply Reb Def Ath\t"
		for _, k := range keys {
			hdr += short(k.Label) + " "
		}
		hdr += "\t"
		for _, k := range tends {
			hdr += short(k[1]) + " "
		}
		fmt.Fprintln(w, hdr)
		for _, p := range t.Roster {
			r := p.Ratings
			row := fmt.Sprintf("%s\t%s\t%d\t%3d %3d %3d %3d %3d %3d %3d\t", p.Name, p.PosLabel(), p.Overall(),
				r.Inside, r.Mid, r.Three, r.Playmaking, r.Rebounding, r.Defense, r.Athleticism)
			for _, a := range p.Attributes() {
				row += fmt.Sprintf("%d ", a.Value)
			}
			row += "\t"
			for _, v := range p.TendencyValues() {
				row += fmt.Sprintf("%d ", v.Value)
			}
			fmt.Fprintln(w, row)
		}
		w.Flush()
		fmt.Println("columns:", func() string {
			s := []string{}
			for _, k := range keys {
				s = append(s, k.Label)
			}
			return strings.Join(s, ", ")
		}())
		fmt.Println("tendencies:", func() string {
			s := []string{}
			for _, k := range tends {
				s = append(s, k[1])
			}
			return strings.Join(s, ", ")
		}())
		fmt.Println()
	}

	// League-wide.
	fmt.Println("league-wide: mean and SD of each attribute and tendency")
	for i, k := range keys {
		m, sd := meanSD(players, func(p *engine.Player) float64 { return float64(p.AttrOf(i)) })
		fmt.Fprintf(w, "%s\t%.1f\t%.1f\n", k.Label, m, sd)
	}
	for i, k := range tends {
		m, sd := meanSD(players, func(p *engine.Player) float64 { return float64(p.Tendencies[i]) })
		fmt.Fprintf(w, "tendency %s\t%.1f\t%.1f\n", k[1], m, sd)
	}
	w.Flush()
	fmt.Println("\nper position means")
	fmt.Fprintln(w, "Pos\tDunk\tSpeed\tThree tend.\tPost-up tend.\tn")
	for pos := engine.PG; pos <= engine.C; pos++ {
		var sub []*engine.Player
		for _, p := range players {
			if p.Pos == pos {
				sub = append(sub, p)
			}
		}
		d, _ := meanSD(sub, func(p *engine.Player) float64 { return float64(p.AttrOf(engine.AttrDunk)) })
		sp, _ := meanSD(sub, func(p *engine.Player) float64 { return float64(p.AttrOf(engine.AttrSpeed)) })
		th, _ := meanSD(sub, func(p *engine.Player) float64 { return float64(p.Tendencies[engine.TendThree]) })
		pu, _ := meanSD(sub, func(p *engine.Player) float64 { return float64(p.Tendencies[engine.TendPostUp]) })
		fmt.Fprintf(w, "%s\t%.1f\t%.1f\t%.1f\t%.1f\t%d\n", pos, d, sp, th, pu, len(sub))
	}
	w.Flush()

	if real {
		fmt.Println("\nreal-data spot checks (league rank of 1 = highest among rostered players)")
		spot := func(name string, idx []int, tidx []int) {
			for _, p := range players {
				if p.Name != name {
					continue
				}
				fmt.Printf("%-18s %s ovr %d:", name, p.PosLabel(), p.Overall())
				for _, i := range idx {
					fmt.Printf("  %s %d (#%d)", keys[i].Label, p.AttrOf(i), rank(players, func(q *engine.Player) float64 { return float64(q.AttrOf(i)) }, float64(p.AttrOf(i))))
				}
				for _, i := range tidx {
					fmt.Printf("  %s tend %d (#%d)", tends[i][1], p.Tendencies[i], rank(players, func(q *engine.Player) float64 { return float64(q.Tendencies[i]) }, float64(p.Tendencies[i])))
				}
				fmt.Println()
			}
		}
		A := engine.AttrFreeThrow
		spot("Stephen Curry", []int{A, engine.AttrThreePoint}, []int{engine.TendThree})
		spot("Rudy Gobert", []int{engine.AttrBlock, engine.AttrInteriorD, engine.AttrDefRebound, engine.AttrSpeed}, nil)
		spot("Clint Capela", []int{engine.AttrBlock, engine.AttrInteriorD, engine.AttrDefRebound, engine.AttrSpeed}, nil)
		spot("Russell Westbrook", []int{engine.AttrSpeed, engine.AttrDunk, engine.AttrDrawFoul}, nil)
		spot("Draymond Green", []int{engine.AttrPassVision}, nil)
		spot("DeAndre Jordan", []int{engine.AttrFreeThrow}, nil)
	}
}

func meanSD(ps []*engine.Player, f func(*engine.Player) float64) (float64, float64) {
	if len(ps) == 0 {
		return 0, 0
	}
	s, s2 := 0.0, 0.0
	for _, p := range ps {
		v := f(p)
		s += v
		s2 += v * v
	}
	n := float64(len(ps))
	m := s / n
	return m, math.Sqrt(math.Max(0, s2/n-m*m))
}

func rank(ps []*engine.Player, f func(*engine.Player) float64, v float64) int {
	vals := make([]float64, len(ps))
	for i, p := range ps {
		vals[i] = f(p)
	}
	sort.Sort(sort.Reverse(sort.Float64Slice(vals)))
	for i, x := range vals {
		if x <= v {
			return i + 1
		}
	}
	return len(vals)
}
