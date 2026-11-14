package engine

import "testing"

func statusCounts(lg *League) map[string]int {
	c := map[string]int{}
	for _, s := range lg.TeamStatuses() {
		c[s.Key]++
	}
	return c
}

func TestTeamStatuses(t *testing.T) {
	lg := tradeLeague(3)
	check := func(when string) {
		sts := lg.TeamStatuses()
		if len(sts) != len(lg.Teams) {
			t.Fatalf("%s: %d statuses", when, len(sts))
		}
		cf := lg.contendFactors()
		rank := map[string]int{"rebuilding": 0, "seller": 1, "buyer": 2, "contender": 3}
		for i, s := range sts {
			if _, ok := rank[s.Key]; !ok || s.Label == "" || s.Wants == "" {
				t.Fatalf("%s: team %d bad status %+v", when, i, s)
			}
			if s != lg.TeamStatusOf(i) {
				t.Fatalf("%s: TeamStatusOf differs for %d", when, i)
			}
			for j := range sts {
				if cf[i] > cf[j] && rank[s.Key] < rank[sts[j].Key] {
					t.Fatalf("%s: team %d (cf %.2f) lower band than %d (cf %.2f)", when, i, cf[i], j, cf[j])
				}
			}
		}
		counts := statusCounts(lg)
		for _, k := range []string{"contender", "buyer", "seller", "rebuilding"} {
			if counts[k] == 0 {
				t.Fatalf("%s: no %s teams: %v", when, k, counts)
			}
		}
		t.Logf("%s counts: %v", when, counts)
	}
	check("fresh")
	a := lg.TeamStatuses()
	b := lg.TeamStatuses()
	for i := range a {
		if a[i] != b[i] {
			t.Fatal("not deterministic")
		}
	}
	lg.SimDays(40)
	check("day 40")
}
