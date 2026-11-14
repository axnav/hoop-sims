package engine

import (
	"reflect"
	"strings"
	"testing"
)

func dealLeague() *League {
	lg := tradeLeague(11)
	for i := 0; i < 8; i++ {
		lg.FreeAgents = append(lg.FreeAgents, GenerateRookie(NewRNG(uint64(i)), 90000+i, 2025))
		lg.FreeAgents[i].TeamID = -1
	}
	return lg
}

func pl(from, to, id int) Move { return Move{from, to, PlayerOf(id)} }
func pk(from, to, year, round, orig int) Move {
	return Move{from, to, PickOf(year, round, orig)}
}

func wantProblem(t *testing.T, lg *League, d Deal, sub string) {
	t.Helper()
	v := EvaluateDeal(lg, d, 0)
	if v.Valid || !strings.Contains(strings.ToLower(v.Problem), strings.ToLower(sub)) {
		t.Fatalf("want problem containing %q, got %+v", sub, v)
	}
}

func TestDealValidation(t *testing.T) {
	lg := dealLeague()
	mine, theirs := sortedByOvr(lg.Teams[0]), sortedByOvr(lg.Teams[5])
	other := sortedByOvr(lg.Teams[7])
	y := lg.Year
	// wrong owner
	wantProblem(t, lg, Deal{[]Move{pl(0, 5, theirs[0].ID), pl(5, 0, mine[0].ID)}}, "not on the team")
	wantProblem(t, lg, Deal{[]Move{pk(0, 5, y, 1, 3), pl(5, 0, theirs[0].ID)}}, "not owned")
	// duplicate asset
	wantProblem(t, lg, Deal{[]Move{pl(0, 5, mine[0].ID), pl(0, 5, mine[0].ID), pl(5, 0, theirs[0].ID)}}, "twice")
	// a team that only receives
	wantProblem(t, lg, Deal{[]Move{pl(0, 5, mine[0].ID), pl(5, 0, theirs[0].ID), pl(0, 7, mine[1].ID)}}, "send and receive")
	// four teams
	wantProblem(t, lg, Deal{[]Move{pl(0, 5, mine[0].ID), pl(5, 7, theirs[0].ID), pl(7, 9, other[0].ID),
		pl(9, 0, sortedByOvr(lg.Teams[9])[0].ID)}}, "2 to 3 teams")
	// too many picks
	var many []Move
	for yr := y; yr < y+PickYearsAhead; yr++ {
		many = append(many, pk(0, 5, yr, 1, 0))
	}
	many = append(many, pk(0, 5, y, 2, 0), pl(5, 0, theirs[0].ID))
	wantProblem(t, lg, Deal{many}, "at most 4 draft picks")
	// closed window
	lg.Season.Day = TradeDeadlineDay + 1
	wantProblem(t, lg, Deal{[]Move{pl(0, 5, mine[0].ID), pl(5, 0, theirs[0].ID)}}, "deadline")
}

func TestDealPickForPlayer(t *testing.T) {
	lg := dealLeague()
	theirs := sortedByOvr(lg.Teams[5])
	y := lg.Year
	// four firsts for their worst player: accepted
	var ms []Move
	for yr := y; yr < y+PickYearsAhead; yr++ {
		ms = append(ms, pk(0, 5, yr, 1, 0))
	}
	ms = append(ms, pl(5, 0, theirs[len(theirs)-1].ID))
	if v := EvaluateDeal(lg, Deal{ms}, 0); !v.Valid || !v.Accept || len(v.Teams) != 1 || v.Teams[0].Reason == "" {
		t.Fatalf("generous pick offer: %+v", v)
	}
	// a second for their best player: refused
	bad := Deal{[]Move{pk(0, 5, y, 2, 0), pl(5, 0, theirs[0].ID)}}
	v := EvaluateDeal(lg, bad, 0)
	if !v.Valid || v.Accept || v.Teams[0].Reason == "" {
		t.Fatalf("lopsided pick offer: %+v", v)
	}
	if _, err := ExecuteDeal(lg, bad, 0); err == nil {
		t.Fatal("refused deal executed")
	}
}

func TestDealThreeTeams(t *testing.T) {
	lg := dealLeague()
	total := 30*RosterSize + len(lg.FreeAgents)
	m0, m5, m7 := sortedByOvr(lg.Teams[0]), sortedByOvr(lg.Teams[5]), sortedByOvr(lg.Teams[7])
	y := lg.Year
	d := Deal{[]Move{
		pl(0, 5, m0[0].ID),
		pk(0, 7, y, 1, 0),
		pk(0, 7, y+1, 1, 0),
		pl(5, 7, m5[len(m5)-1].ID),
		pl(7, 0, m7[len(m7)-1].ID),
	}}
	if v := EvaluateDeal(lg, d, 0); !v.Accept || len(v.Teams) != 2 {
		t.Fatalf("three-team deal refused: %+v", v)
	}
	res, err := ExecuteDeal(lg, d, 0)
	if err != nil {
		t.Fatal(err)
	}
	checkConsistent(t, lg, total)
	if len(res.Teams) != 3 || len(res.Picks) != 2 || len(lg.Transactions) != 1 {
		t.Fatalf("log %+v", res)
	}
	owner := func(id int) int {
		for _, tm := range lg.Teams {
			for _, p := range tm.Roster {
				if p.ID == id {
					return tm.ID
				}
			}
		}
		return -1
	}
	if owner(m0[0].ID) != 5 || owner(m5[len(m5)-1].ID) != 7 || owner(m7[len(m7)-1].ID) != 0 {
		t.Fatal("players went to the wrong teams")
	}
	for _, yr := range []int{y, y + 1} {
		if o, _ := lg.PickOwner(yr, 1, 0); o != 7 {
			t.Fatalf("pick %d owner %d, want 7", yr, o)
		}
	}
	if o, _ := lg.PickOwner(y+2, 1, 0); o != 0 {
		t.Fatal("untraded pick moved")
	}
}

func TestDealMatchesEvaluateTrade(t *testing.T) {
	lg := dealLeague()
	r := NewRNG(5)
	accepts, refuses := 0, 0
	for i := 0; i < 120; i++ {
		to := 1 + r.IntN(29)
		ng, nt := 1+r.IntN(MaxTradePlayers), 1+r.IntN(MaxTradePlayers)
		pick := func(tid, n int) []int {
			used := map[int]bool{}
			var ids []int
			for len(ids) < n {
				k := r.IntN(RosterSize)
				if !used[k] {
					used[k] = true
					ids = append(ids, lg.Teams[tid].Roster[k].ID)
				}
			}
			return ids
		}
		offer := TradeOffer{0, to, pick(0, ng), pick(to, nt)}
		tv := EvaluateTrade(lg, offer)
		var ms []Move
		for _, id := range offer.Give {
			ms = append(ms, pl(0, to, id))
		}
		for _, id := range offer.Get {
			ms = append(ms, pl(to, 0, id))
		}
		dv := EvaluateDeal(lg, Deal{ms}, 0)
		if tv.Valid != dv.Valid || (tv.Valid && tv.Accept != dv.Accept) {
			t.Fatalf("offer %+v: trade %+v deal %+v", offer, tv, dv)
		}
		if tv.Accept {
			accepts++
		} else {
			refuses++
		}
	}
	if accepts == 0 || refuses == 0 {
		t.Fatalf("test not diverse: %d accepts %d refuses", accepts, refuses)
	}
}

func TestPickValueOrdering(t *testing.T) {
	// weaker original team (low contend) means an earlier slot, worth more
	if !(PickValueFor(1, 0, 0.0, 0.5) > PickValueFor(1, 0, 0.5, 0.5) && PickValueFor(1, 0, 0.5, 0.5) > PickValueFor(1, 0, 1.0, 0.5)) {
		t.Fatal("earlier slot should be worth more")
	}
	if PickValueFor(2, 0, 0.0, 0.5) >= PickValueFor(1, 0, 1.0, 0.5) {
		t.Fatal("best second should be below worst first")
	}
	for _, c := range []float64{0, 0.5, 1} {
		if PickValueFor(1, 0, c, 0.5) <= PickValueFor(2, 0, c, 0.5) {
			t.Fatal("round 1 should beat round 2")
		}
		if PickValueFor(1, 0, c, 0.5) <= PickValueFor(1, 1, c, 0.5) || PickValueFor(1, 1, c, 0.5) <= PickValueFor(1, 3, c, 0.5) {
			t.Fatal("sooner should beat later")
		}
		if PickValueFor(1, 0, c, 0.0) <= PickValueFor(1, 0, c, 1.0) {
			t.Fatal("rebuilding team should value a pick more")
		}
	}
	t.Logf("top-3 first (rebuilder %.1f, mid %.1f, contender %.1f); mid first %.1f; late first %.1f; worst second %.1f; best second %.1f",
		PickValueFor(1, 0, 0.03, 0), PickValueFor(1, 0, 0.03, 0.5), PickValueFor(1, 0, 0.03, 1),
		PickValueFor(1, 0, 0.5, 0.5), PickValueFor(1, 0, 1, 0.5), PickValueFor(2, 0, 0, 0.5), PickValueFor(2, 0, 1, 0.5))
}

func TestDealDeterministic(t *testing.T) {
	lg := dealLeague()
	theirs := sortedByOvr(lg.Teams[5])
	d := Deal{[]Move{pk(0, 5, lg.Year, 1, 0), pl(5, 0, theirs[4].ID)}}
	a, b := EvaluateDeal(lg, d, 0), EvaluateDeal(lg, d, 0)
	if len(a.Teams) != 1 || !reflect.DeepEqual(a.Teams[0], b.Teams[0]) || a.Accept != b.Accept {
		t.Fatalf("%+v vs %+v", a, b)
	}
}
