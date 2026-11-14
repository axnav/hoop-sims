package engine

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func tradeLeague(seed uint64) *League {
	lg := GenerateLeague(seed)
	lg.UserTeamID = 0
	lg.Season = lg.StartSeason()
	return lg
}

func sortedByOvr(t *Team) []*Player {
	ps := append([]*Player(nil), t.Roster...)
	sort.SliceStable(ps, func(i, j int) bool { return ps[i].Overall() > ps[j].Overall() })
	return ps
}

func checkConsistent(t *testing.T, lg *League, total int) {
	t.Helper()
	seen := map[int]bool{}
	n := 0
	for _, tm := range lg.Teams {
		if len(tm.Roster) != RosterSize {
			t.Fatalf("%s has %d players", tm.FullName(), len(tm.Roster))
		}
		for _, p := range tm.Roster {
			if seen[p.ID] || p.TeamID != tm.ID {
				t.Fatalf("player %d duplicated or wrong TeamID", p.ID)
			}
			seen[p.ID] = true
			n++
		}
	}
	for _, p := range lg.FreeAgents {
		if seen[p.ID] || p.TeamID != -1 {
			t.Fatalf("free agent %d duplicated or has team", p.ID)
		}
		seen[p.ID] = true
		n++
	}
	if n != total {
		t.Fatalf("players %d, want %d", n, total)
	}
}

func TestTradeAcceptRefuse(t *testing.T) {
	lg := tradeLeague(11)
	for i := 0; i < 8; i++ {
		lg.FreeAgents = append(lg.FreeAgents, GenerateRookie(NewRNG(uint64(i)), 90000+i, 2025))
		lg.FreeAgents[i].TeamID = -1
	}
	total := 30*RosterSize + len(lg.FreeAgents)
	mine, theirs := sortedByOvr(lg.Teams[0]), sortedByOvr(lg.Teams[5])
	// Their best for my worst: refused.
	bad := TradeOffer{0, 5, []int{mine[len(mine)-1].ID}, []int{theirs[0].ID}}
	if v := EvaluateTrade(lg, bad); !v.Valid || v.Accept || v.Reason == "" {
		t.Fatalf("lopsided offer: %+v", v)
	}
	// My best for their worst: accepted.
	good := TradeOffer{0, 5, []int{mine[0].ID}, []int{theirs[len(theirs)-1].ID}}
	if v := EvaluateTrade(lg, good); !v.Accept {
		t.Fatalf("generous offer refused: %+v", v)
	}
	// Counter suggestions are accepted.
	mid := TradeOffer{0, 5, []int{mine[len(mine)-1].ID}, []int{theirs[3].ID}}
	if c, ok := SuggestCounter(lg, mid); ok {
		if v := EvaluateTrade(lg, c); !v.Accept {
			t.Fatalf("counter not accepted: %+v", v)
		}
	}
	// Execute 2-for-1 and 1-for-2; stay consistent; others untouched.
	before := map[int][]int{}
	for _, tm := range lg.Teams {
		for _, p := range tm.Roster {
			before[tm.ID] = append(before[tm.ID], p.ID)
		}
	}
	two := TradeOffer{0, 5, []int{mine[0].ID, mine[1].ID}, []int{theirs[len(theirs)-1].ID}}
	res, err := ExecuteTrade(lg, two)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Released) != 1 || len(lg.Transactions) != 1 {
		t.Fatalf("result %+v", res)
	}
	checkConsistent(t, lg, total)
	for _, tm := range lg.Teams {
		if tm.ID == 0 || tm.ID == 5 {
			continue
		}
		var now []int
		for _, p := range tm.Roster {
			now = append(now, p.ID)
		}
		if !reflect.DeepEqual(now, before[tm.ID]) {
			t.Fatalf("team %d changed", tm.ID)
		}
	}
	if _, err := ExecuteTrade(lg, bad); err == nil {
		t.Fatal("refused trade executed")
	}
	// 1-for-2 where the other side signs a free agent.
	m2, t2 := sortedByOvr(lg.Teams[0]), sortedByOvr(lg.Teams[5])
	if _, err := ExecuteTrade(lg, TradeOffer{0, 5, []int{m2[0].ID}, []int{t2[len(t2)-1].ID, t2[len(t2)-2].ID}}); err != nil {
		t.Fatal(err)
	}
	checkConsistent(t, lg, total)
	if len(TradeBlock(lg, 5)) != RosterSize {
		t.Fatal("trade block size")
	}
}

func TestTradeWindow(t *testing.T) {
	lg := tradeLeague(3)
	if ok, _ := TradeWindowOpen(lg); !ok {
		t.Fatal("closed at start")
	}
	lg.Season.Day = TradeDeadlineDay
	if ok, _ := TradeWindowOpen(lg); !ok {
		t.Fatal("closed on the deadline day")
	}
	lg.Season.Day = TradeDeadlineDay + 1
	ok, why := TradeWindowOpen(lg)
	if ok || why == "" {
		t.Fatal("open after deadline")
	}
	mine, theirs := sortedByOvr(lg.Teams[0]), sortedByOvr(lg.Teams[5])
	v := EvaluateTrade(lg, TradeOffer{0, 5, []int{mine[0].ID}, []int{theirs[12].ID}})
	if v.Valid || v.Problem == "" {
		t.Fatal("evaluated while closed")
	}
	lg.Season.Day = 20
	lg.Season.PlayoffsStarted = true
	if ok, _ := TradeWindowOpen(lg); ok {
		t.Fatal("open in playoffs")
	}
	lg.Season.PlayoffsStarted = false
	lg.Draft = &Draft{}
	if ok, _ := TradeWindowOpen(lg); ok {
		t.Fatal("open during draft")
	}
}

func TestTradeSaveLoad(t *testing.T) {
	lg := tradeLeague(5)
	mine, theirs := sortedByOvr(lg.Teams[0]), sortedByOvr(lg.Teams[7])
	if _, err := ExecuteTrade(lg, TradeOffer{0, 7, []int{mine[0].ID}, []int{theirs[12].ID}}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "l.json")
	if err := lg.Save(path); err != nil {
		t.Fatal(err)
	}
	lg2, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(lg.Transactions, lg2.Transactions) || len(lg2.Transactions) != 1 {
		t.Fatal("transactions differ")
	}
	for i := range lg.Teams {
		for j, p := range lg.Teams[i].Roster {
			if lg2.Teams[i].Roster[j].ID != p.ID {
				t.Fatal("rosters differ")
			}
		}
	}
	os.Remove(path)
}

func TestAITradesDeterministicAndLegal(t *testing.T) {
	run := func() ([]TradeResult, *League) {
		lg := tradeLeague(21)
		var all []TradeResult
		for y := 0; y < 6; y++ {
			all = append(all, lg.SimAITrades(NewRNG(uint64(100+y)))...)
		}
		return all, lg
	}
	a, lg := run()
	b, _ := run()
	if !reflect.DeepEqual(a, b) {
		t.Fatal("AI trades not reproducible")
	}
	checkConsistent(t, lg, 30*RosterSize)
	for _, tr := range a {
		if tr.TeamA == lg.UserTeamID || tr.TeamB == lg.UserTeamID {
			t.Fatal("AI trade involved the user")
		}
	}
	t.Logf("AI trades in 6 calls: %d", len(a))
}

func TestTradesWithSeasons(t *testing.T) {
	lg := GenerateLeague(8)
	lg.Season = lg.StartSeason()
	n := 0
	for y := 0; y < 6; y++ {
		lg.SimRegularSeason()
		lg.Season.Champion = 0
		lg.Offseason()
		n += len(lg.Transactions)
		lg.Season = lg.StartSeason()
		checkConsistent(t, lg, 30*RosterSize+len(lg.FreeAgents))
	}
	t.Logf("transactions over 6 offseasons: %d", len(lg.Transactions))
}

func TestRealTrades(t *testing.T) {
	lg := realLeagueOrSkip(t, 2)
	lg.UserTeamID = 0
	lg.Season = lg.StartSeason()
	mine := sortedByOvr(lg.Teams[0])
	theirs := sortedByOvr(lg.Teams[4])
	v := EvaluateTrade(lg, TradeOffer{0, 4, []int{mine[len(mine)-1].ID}, []int{theirs[0].ID}})
	if !v.Valid || v.Accept {
		t.Fatalf("star for scrub: %+v", v)
	}
	v = EvaluateTrade(lg, TradeOffer{0, 4, []int{mine[0].ID}, []int{theirs[0].ID}})
	if !v.Valid {
		t.Fatalf("star for star: %+v", v)
	}
}

func TestTradeValueExamples(t *testing.T) {
	mk := func(ovr, pot, age int) *Player {
		p := &Player{Age: age, Pos: SF, Potential: pot}
		p.Ratings = Ratings{ovr, ovr, ovr, ovr, ovr, ovr, ovr}
		return p
	}
	tm := &Team{}
	for _, c := range [][3]int{{92, 92, 27}, {80, 90, 21}, {78, 78, 34}, {75, 75, 27}, {60, 60, 27}} {
		p := mk(c[0], c[1], c[2])
		t.Logf("ovr %d pot %d age %d -> %.1f (rated %d)", c[0], c[1], c[2], tradeValue(p, 0.5, nil), p.Overall())
	}
	_ = tm
}
