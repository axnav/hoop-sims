package engine

import "testing"

func isPerm30(o []int) bool {
	seen := map[int]bool{}
	for _, t := range o {
		seen[t] = true
	}
	return len(o) == 30 && len(seen) == 30
}

func TestLotteryOrders(t *testing.T) {
	lg := finishedSeasonLeague(4)
	st := lg.Season.Standings()
	lg.BeginOffseason()
	d := lg.Draft
	if !isPerm30(d.Order) || !isPerm30(d.Order2) {
		t.Fatalf("orders are not permutations: %v %v", d.Order, d.Order2)
	}
	for i := 0; i < 30; i++ {
		if d.Order2[i] != st[29-i] {
			t.Fatalf("round 2 slot %d: %d, want %d", i+1, d.Order2[i], st[29-i])
		}
	}
	for i := 0; i < 14; i++ {
		if lg.Season.MadePlayoffs(d.Order[i]) {
			t.Fatalf("playoff team %d in slot %d", d.Order[i], i+1)
		}
	}
	if len(d.Lottery) != 14 {
		t.Fatalf("lottery rows %d", len(d.Lottery))
	}
	for _, row := range d.Lottery {
		if d.Order[row.Slot-1] != row.Team || row.Change != row.Seed-row.Slot || row.Odds != LotteryOdds[row.Seed-1] || row.Owner != row.Team {
			t.Fatalf("bad row %+v", row)
		}
	}
	// Same seed, same lottery.
	lg2 := finishedSeasonLeague(4)
	lg2.BeginOffseason()
	for i := range d.Order {
		if d.Order[i] != lg2.Draft.Order[i] {
			t.Fatal("same seed gave a different lottery")
		}
	}
}

func TestLotteryFrequencies(t *testing.T) {
	lg := GenerateLeague(1)
	rev := make([]int, 30)
	for i := range rev {
		rev[i] = i // team i has the (i+1)-th worst record
	}
	playoff := func(t int) bool { return t >= 14 }
	const n = 4000
	var top [14]int
	var worstSlot5Plus int
	for i := 0; i < n; i++ {
		r := NewRNG(uint64(1000 + i))
		order, _ := lg.lotteryOrder(r, rev, playoff)
		if !isPerm30(order) {
			t.Fatal("not a permutation")
		}
		if order[0] < 14 {
			top[order[0]]++
		}
		for s, tm := range order {
			if tm == 0 && s > 4 {
				worstSlot5Plus++
			}
			if s < 14 && tm >= 14 {
				t.Fatal("playoff team in lottery slots")
			}
		}
	}
	if worstSlot5Plus != 0 {
		t.Errorf("worst team fell below slot 5 %d times", worstSlot5Plus)
	}
	for i := 0; i < 14; i++ {
		got, want := float64(top[i])/n*100, float64(LotteryOdds[i])/10
		if got < want-2.5 || got > want+2.5 {
			t.Errorf("seed %d wins lottery %.1f%%, want about %.1f%%", i+1, got, want)
		}
	}
	t.Logf("slot-1 frequency of three worst: %.1f%% %.1f%% %.1f%%", float64(top[0])/n*100, float64(top[1])/n*100, float64(top[2])/n*100)
}

func TestLotteryTradedPickKeepsSlot(t *testing.T) {
	// Covered with a finished season in TestTransferredPickSelects; here check
	// the row's Owner follows a trade.
	lg := finishedSeasonLeague(6)
	st := lg.Season.Standings()
	worst, buyer := st[29], st[0]
	if err := lg.TransferPick(lg.Year, 1, worst, worst, buyer); err != nil {
		t.Fatal(err)
	}
	lg.BeginOffseason()
	for _, row := range lg.Draft.Lottery {
		if row.Team == worst && row.Owner != buyer {
			t.Errorf("row owner %d, want %d", row.Owner, buyer)
		}
	}
}
