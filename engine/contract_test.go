package engine

import (
	"strings"
	"testing"
)

func testPlayer(ovr, age, pot int) *Player {
	r := ovr
	p := &Player{Age: age, Ratings: Ratings{r, r, r, r, r, r, r}, TeamID: -1}
	p.Potential = max(pot, p.Overall())
	return p
}

func TestMarketSalary(t *testing.T) {
	prev := 0
	for o := 25; o <= 99; o++ {
		s := MarketSalary(testPlayer(o, 27, o))
		if s < prev || s < Cap.MinSalary || s > Cap.MaxSalary || s%100 != 0 {
			t.Fatalf("ovr %d: salary %d (prev %d)", o, s, prev)
		}
		prev = s
	}
	if MarketSalary(testPlayer(40, 27, 40)) != Cap.MinSalary || MarketSalary(testPlayer(99, 27, 99)) != Cap.MaxSalary {
		t.Error("ends of the curve are not min and max")
	}
	if MarketSalary(testPlayer(70, 35, 70)) >= MarketSalary(testPlayer(70, 28, 70)) {
		t.Error("old players should be discounted")
	}
	if MarketSalary(testPlayer(60, 21, 72)) <= MarketSalary(testPlayer(60, 21, 60)) {
		t.Error("young high-potential players should get a premium")
	}
	for _, c := range [][3]int{{50, 27, 50}, {60, 27, 60}, {66, 27, 66}, {72, 27, 72}, {80, 27, 80}, {70, 34, 70}, {60, 21, 72}} {
		t.Logf("ovr %d age %d pot %d: %d", c[0], c[1], c[2], MarketSalary(testPlayer(c[0], c[1], c[2])))
	}
}

func avgPayroll(lg *League) (avg float64, high, overCap, overTax, overCeil int) {
	sum := 0
	for _, tm := range lg.Teams {
		pay := lg.Payroll(tm.ID)
		sum += pay
		high = max(high, pay)
		if pay > Cap.SalaryCap {
			overCap++
		}
		if pay > Cap.TaxLine {
			overTax++
		}
		if pay > Cap.HardCeiling {
			overCeil++
		}
	}
	return float64(sum) / float64(len(lg.Teams)), high, overCap, overTax, overCeil
}

func TestInitialContracts(t *testing.T) {
	for seed := uint64(1); seed <= 3; seed++ {
		lg := GenerateLeague(seed)
		avg, high, _, overTax, _ := avgPayroll(lg)
		ratio := avg / float64(Cap.SalaryCap)
		t.Logf("seed %d: avg payroll %.0f (%.0f%% of cap), highest %d", seed, avg, ratio*100, high)
		if ratio < 0.90 || ratio > 1.05 || overTax != 0 {
			t.Errorf("seed %d: payroll ratio %.2f, %d teams over the tax line", seed, ratio, overTax)
		}
		for _, tm := range lg.Teams {
			for _, p := range tm.Roster {
				if p.Contract.Years < 1 || p.Contract.Years > Cap.MaxContractYears || p.Contract.Salary < Cap.MinSalary {
					t.Fatalf("bad contract %+v", p.Contract)
				}
			}
		}
		for _, p := range lg.FreeAgents {
			if p.Contract != (Contract{}) {
				t.Fatal("free agent with contract")
			}
		}
	}
	a, b := GenerateLeague(4), GenerateLeague(4)
	for i := range a.Teams {
		for j := range a.Teams[i].Roster {
			if a.Teams[i].Roster[j].Contract != b.Teams[i].Roster[j].Contract {
				t.Fatal("same seed gave different contracts")
			}
		}
	}
}

func TestSigningRules(t *testing.T) {
	lg := GenerateLeague(3)
	tid := 4
	pay := lg.Payroll(tid)
	// cap room: fits under the cap or not
	if err := lg.CanSign(tid, Cap.SalaryCap-pay, SignCapRoom); pay < Cap.SalaryCap && err != nil && Cap.SalaryCap-pay >= Cap.MinSalary {
		t.Errorf("exact fit refused: %v", err)
	}
	if err := lg.CanSign(tid, max(Cap.MinSalary, Cap.SalaryCap-pay+100), SignCapRoom); err == nil || !strings.Contains(err.Error(), "cap room") {
		t.Errorf("over-the-cap signing allowed: %v", err)
	}
	// push the team over the cap by hand: own and minimum still work, up to the ceiling
	lg.Teams[tid].Roster[0].Contract.Salary += Cap.SalaryCap - pay + 5000
	if lg.CapRoom(tid) >= 0 {
		t.Fatal("setup: still has room")
	}
	if err := lg.CanSign(tid, 5000, SignOwn); err != nil {
		t.Errorf("own signing over the cap refused: %v", err)
	}
	if err := lg.CanSign(tid, Cap.MinSalary, SignMinimum); err != nil {
		t.Errorf("minimum over the cap refused: %v", err)
	}
	if err := lg.CanSign(tid, Cap.MinSalary+100, SignMinimum); err == nil {
		t.Error("non-minimum 'minimum' allowed")
	}
	if err := lg.CanSign(tid, Cap.MinSalary-100, SignOwn); err == nil {
		t.Error("below-minimum allowed")
	}
	if err := lg.CanSign(tid, Cap.MaxSalary+100, SignOwn); err == nil {
		t.Error("above-maximum allowed")
	}
	lg.Teams[tid].Roster[0].Contract.Salary = Cap.HardCeiling - 1000
	if err := lg.CanSign(tid, 5000, SignOwn); err == nil || !strings.Contains(err.Error(), "hard ceiling") {
		t.Errorf("own signing past the ceiling allowed: %v", err)
	}
	if err := lg.CanSign(tid, Cap.MinSalary, SignMinimum); err == nil {
		t.Error("minimum past the ceiling allowed")
	}
	if err := lg.CanSign(tid, 5000, SignDraft); err != nil {
		t.Errorf("draft signing refused: %v", err)
	}
	// Sign: years, full roster, free agent removal
	fa := GenerateRookie(NewRNG(1), 9000, 2025)
	fa.TeamID = -1
	lg.FreeAgents = append(lg.FreeAgents, fa)
	if err := lg.Sign(tid, fa, Cap.MinSalary, 1, SignMinimum); err == nil || !strings.Contains(err.Error(), "full roster") {
		t.Errorf("full roster not refused: %v", err)
	}
	lg.Teams[tid].Roster = lg.Teams[tid].Roster[1:]
	if err := lg.Sign(tid, fa, Cap.MinSalary, 5, SignMinimum); err == nil {
		t.Error("5-year contract allowed")
	}
	if err := lg.Sign(tid, fa, Cap.MinSalary, 1, SignMinimum); err != nil {
		t.Fatal(err)
	}
	if fa.TeamID != tid || fa.Contract != (Contract{Cap.MinSalary, 1}) || len(lg.Teams[tid].Roster) != RosterSize {
		t.Error("signed player not on roster with contract")
	}
	for _, p := range lg.FreeAgents {
		if p == fa {
			t.Error("still in free agency")
		}
	}
}

func TestRookieScale(t *testing.T) {
	first, last, second := RookieContract(1, 1), RookieContract(30, 1), RookieContract(31, 2)
	if first.Salary != Cap.RookieTop || last.Salary != Cap.RookieLastFirst || first.Years != 4 {
		t.Errorf("first %+v last %+v", first, last)
	}
	if second.Salary != Cap.MinSalary || second.Years != 2 {
		t.Errorf("second round %+v", second)
	}
	for k := 2; k <= 30; k++ {
		if RookieContract(k, 1).Salary > RookieContract(k-1, 1).Salary {
			t.Fatalf("rookie scale rises at pick %d", k)
		}
	}
	// drafted players get it
	lg := GenerateLeague(5)
	lg.UserTeamID = 0
	lg.StartSeason()
	lg.SimRegularSeason()
	lg.SimPlayoffs()
	lg.BeginOffseason()
	lg.DraftAutoAll()
	for _, r := range lg.Draft.Results {
		var p *Player
		for _, q := range lg.Teams[r.TeamID].Roster {
			if q.ID == r.PlayerID {
				p = q
			}
		}
		if p == nil || p.Contract != RookieContract(r.Pick, r.Round) {
			t.Fatalf("pick %d: contract %+v", r.Pick, p)
		}
	}
}

func TestTradeSalaryRules(t *testing.T) {
	lg := tradeLeague(11)
	a, b := lg.Teams[0], lg.Teams[5]
	// Put both teams well over the cap and make one side far bigger.
	for _, tm := range []*Team{a, b} {
		for _, p := range tm.Roster {
			p.Contract = Contract{12000, 2}
		}
	}
	big := a.Roster[0]
	big.Contract.Salary = 30000
	small := b.Roster[0]
	small.Contract.Salary = 5000
	if lg.CapRoom(5) >= 0 {
		t.Fatal("setup: team 5 under the cap")
	}
	bad := Deal{[]Move{pl(0, 5, big.ID), pl(5, 0, small.ID)}}
	err := ValidateDeal(lg, bad, 0)
	if err == nil || !strings.Contains(err.Error(), "take back") || !strings.Contains(err.Error(), "$30.0M for $5.0M") {
		t.Fatalf("mismatch not refused with message: %v", err)
	}
	t.Log(err)
	// matched: same salary both ways
	small.Contract.Salary = 30000
	if err := ValidateDeal(lg, Deal{[]Move{pl(0, 5, big.ID), pl(5, 0, small.ID)}}, 0); err != nil {
		t.Errorf("matched deal refused: %v", err)
	}
	// within the 125% + 250 allowance
	small.Contract.Salary = 24000
	if err := ValidateDeal(lg, Deal{[]Move{pl(0, 5, big.ID), pl(5, 0, small.ID)}}, 0); err != nil {
		t.Errorf("deal inside allowance refused: %v", err)
	}
	// old two-team path agrees
	small.Contract.Salary = 5000
	if v := EvaluateTrade(lg, TradeOffer{0, 5, []int{big.ID}, []int{small.ID}}); v.Valid {
		t.Error("two-team path allowed a salary mismatch")
	}
	// hard ceiling applies even to a "matched" deal that grows payroll
	for _, p := range b.Roster {
		p.Contract = Contract{14500, 2}
	}
	b.Roster[0].Contract.Salary = 14500
	a.Roster[0].Contract.Salary = 14500
	a.Roster[1].Contract.Salary = 14500
	// both sides over the cap and above ceiling-after for team 5
	if err := lg.salaryProblem(5, []*Player{b.Roster[0]}, []*Player{{Contract: Contract{49000, 1}}}); err == nil {
		t.Error("ceiling not enforced")
	}
}

func TestContractValueAdjust(t *testing.T) {
	others := []*Player{}
	p := testPlayer(70, 27, 70)
	base := tradeValue(p, 0.5, others)
	market := MarketSalary(p)
	p.Contract = Contract{market, 3}
	fair := tradeValue(p, 0.5, others)
	p.Contract = Contract{Cap.MinSalary, 4}
	bargain := tradeValue(p, 0.5, others)
	p.Contract = Contract{Cap.MaxSalary, 4}
	over := tradeValue(p, 0.5, others)
	if !(bargain > fair && fair > over) {
		t.Errorf("bargain %.2f fair %.2f overpay %.2f", bargain, fair, over)
	}
	if bargain > base*(1+Cap.ContractSwing)+1e-9 || over < base*(1-Cap.ContractSwing)-1e-9 {
		t.Errorf("adjustment out of bounds: base %.2f bargain %.2f over %.2f", base, bargain, over)
	}
	p.Contract = Contract{market, 0}
	if rebuild, contend := tradeValue(p, 0, others), tradeValue(p, 1, others); rebuild >= contend {
		t.Errorf("expiring player not worth less to a rebuilder: %.2f vs %.2f", rebuild, contend)
	}
}

func TestOffseasonContracts(t *testing.T) {
	lg := GenerateLeague(8)
	lg.UserTeamID = 0
	lg.StartSeason()
	lg.SimRegularSeason()
	lg.SimPlayoffs()
	before := map[int]int{}
	for _, tm := range lg.Teams {
		for _, p := range tm.Roster {
			before[p.ID] = p.Contract.Years
		}
	}
	wantTax := map[int]int{}
	for _, tm := range lg.Teams {
		wantTax[tm.ID] = lg.TaxBill(tm.ID)
	}
	lg.BeginOffseason()
	if lg.Teams[3].TaxPaid[lg.Year] != wantTax[3] {
		t.Error("tax not recorded")
	}
	expiring := map[int]bool{}
	for _, tm := range lg.Teams {
		for _, p := range tm.Roster {
			if p.Contract.Years != before[p.ID]-1 {
				t.Fatalf("years %d -> %d", before[p.ID], p.Contract.Years)
			}
		}
		for _, p := range lg.Expiring(tm.ID) {
			expiring[p.ID] = true
		}
	}
	if len(expiring) == 0 {
		t.Fatal("nobody expiring")
	}
	// the user can re-sign one of his own; releasing is allowed in the offseason
	if exp := lg.Expiring(0); len(exp) > 0 {
		if err := lg.ReSign(0, exp[0].ID); err != nil || exp[0].Contract.Years < 1 {
			t.Fatalf("resign: %v %+v", err, exp[0].Contract)
		}
		if err := lg.ReSign(0, exp[0].ID); err == nil {
			t.Error("re-signed a player who is not expiring")
		}
		delete(expiring, exp[0].ID)
	}
	if err := lg.Release(0, lg.Teams[0].Roster[0].ID); err != nil {
		t.Fatal(err)
	}
	lg.DraftAutoAll()
	lg.FinishOffseason()
	kept, left := 0, 0
	for _, tm := range lg.Teams {
		if len(tm.Roster) != RosterSize {
			t.Fatalf("team %d has %d players", tm.ID, len(tm.Roster))
		}
		for _, p := range tm.Roster {
			if p.Contract.Years < 1 || p.Contract.Salary < Cap.MinSalary {
				t.Fatalf("player %d without contract: %+v", p.ID, p.Contract)
			}
			if tm.ID != 0 && expiring[p.ID] && before[p.ID] == 1 && p.TeamID == tm.ID {
				kept++
			}
		}
	}
	for id := range expiring {
		_ = id
	}
	for _, p := range lg.FreeAgents {
		if p.Contract != (Contract{}) {
			t.Fatalf("free agent has a contract %+v", p.Contract)
		}
		if expiring[p.ID] {
			left++
		}
	}
	t.Logf("expiring %d; computer teams kept %d by re-signing or signing elsewhere; %d in the pool", len(expiring), kept, left)
	if left == 0 {
		t.Error("no expiring player reached free agency")
	}
}

func TestComputerReSign(t *testing.T) {
	lg := GenerateLeague(9)
	lg.UserTeamID = 0
	lg.StartSeason()
	lg.SimRegularSeason()
	lg.SimPlayoffs()
	lg.BeginOffseason()
	lg.DraftAutoAll()
	// force several expiring on team 7: one star, one scrub
	tm := lg.Teams[7]
	star, scrub := tm.Roster[0], tm.Roster[1]
	star.Ratings = Ratings{72, 72, 72, 72, 72, 72, 72}
	star.Age = 26
	scrub.Ratings = Ratings{40, 40, 40, 40, 40, 40, 40}
	star.Contract.Years, scrub.Contract.Years = 0, 0
	lg.reSignComputerTeams()
	if star.Contract.Years < 1 {
		t.Error("computer team did not re-sign its star")
	}
	if scrub.Contract.Years >= 1 {
		t.Error("computer team re-signed a scrub")
	}
	lg.releaseExpiring()
	if scrub.TeamID != -1 || scrub.Contract != (Contract{}) {
		t.Error("expiring scrub not released")
	}
}

// TestSixSeasons plays six seasons and reports payroll after each offseason.
func TestSixSeasons(t *testing.T) {
	lg := GenerateLeague(5)
	lg.UserTeamID = 0
	var avgs []float64
	for y := 0; y < 6; y++ {
		avg, high, oc, ot, oh := avgPayroll(lg)
		avgs = append(avgs, avg)
		t.Logf("season %d start: avg %.0f (%.0f%% of cap) highest %d, over cap %d, over tax %d, over ceiling %d",
			lg.Year, avg, 100*avg/float64(Cap.SalaryCap), high, oc, ot, oh)
		if oh > 0 {
			t.Logf("  NOTE: %d teams over the hard ceiling", oh)
		}
		for _, tm := range lg.Teams {
			if len(tm.Roster) != RosterSize {
				t.Fatalf("team %d has %d players", tm.ID, len(tm.Roster))
			}
			for _, p := range tm.Roster {
				if p.Contract.Years < 1 {
					t.Fatalf("player %d has no contract", p.ID)
				}
			}
		}
		if avg < 0.75*float64(Cap.SalaryCap) || avg > 1.15*float64(Cap.SalaryCap) {
			t.Errorf("average payroll %.0f out of band", avg)
		}
		lg.StartSeason()
		lg.SimRegularSeason()
		lg.SimPlayoffs()
		// Offseason by hand: the user's team gets no computer offers and is
		// filled only at FinishOffseason, with one-year minimum deals.
		lg.BeginOffseason()
		lg.DraftAutoAll()
		if err := lg.BeginFreeAgency(); err != nil {
			t.Fatal(err)
		}
		for lg.FA.DaysLeft() > 0 {
			lg.AdvanceFreeAgency()
		}
		for _, s := range lg.FA.Log {
			if s.Team == lg.UserTeamID {
				t.Fatalf("user team signed %s without offering", s.Name)
			}
		}
		had := map[int]bool{}
		for _, p := range lg.Teams[lg.UserTeamID].Roster {
			had[p.ID] = true
		}
		lg.FinishOffseason()
		if n := len(lg.Teams[lg.UserTeamID].Roster); n != RosterSize {
			t.Fatalf("user team has %d players after the offseason", n)
		}
		for _, p := range lg.Teams[lg.UserTeamID].Roster {
			if !had[p.ID] && p.Contract != (Contract{Cap.MinSalary, 1}) {
				t.Fatalf("auto-filled user player not on a one-year minimum: %+v", p.Contract)
			}
		}
	}
	t.Logf("average payrolls: %.0f", avgs)
}
