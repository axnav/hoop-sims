package engine

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

// realSalaryLeague is a generated league with the real contracts (the trade
// tests halve salaries; here the cap rules matter), user team 0.
func realSalaryLeague(seed uint64) *League {
	lg := GenerateLeague(seed)
	lg.UserTeamID = 0
	lg.Season = lg.StartSeason()
	return lg
}

func TestNeedsMatchDirection(t *testing.T) {
	lg := realSalaryLeague(3)
	for tid := range lg.Teams {
		n := lg.NeedsOf(tid)
		if !reflect.DeepEqual(n, lg.NeedsOf(tid)) {
			t.Fatalf("team %d: needs change between calls", tid)
		}
		st := lg.TeamStatusOf(tid)
		if n.Status != st {
			t.Fatalf("team %d: status %v, want %v", tid, n.Status, st)
		}
		if len(n.Values) == 0 || len(n.Values) > 4 || len(n.Lacks) > 4 || n.Summary == "" {
			t.Fatalf("team %d: bad needs %+v", tid, n)
		}
		switch st.Key {
		case "contender", "buyer":
			if n.Values[0] != "Win-now veterans" {
				t.Fatalf("team %d (%s): values %v", tid, st.Key, n.Values)
			}
		case "rebuilding":
			if n.Values[0] != "Young players with upside" {
				t.Fatalf("team %d (%s): values %v", tid, st.Key, n.Values)
			}
		case "seller":
			if n.Values[0] != "Draft picks" {
				t.Fatalf("team %d (%s): values %v", tid, st.Key, n.Values)
			}
		}
		t.Log(tid, st.Key, "|", n.Summary)
	}
}

func TestNeedsSeeThinPosition(t *testing.T) {
	lg := realSalaryLeague(3)
	// Make team 5 thin at center: nobody there can play it.
	for _, p := range lg.Teams[5].Roster {
		if p.Pos == C {
			p.Pos = PF
		}
		var alt []Position
		for _, a := range p.AltPos {
			if a != C {
				alt = append(alt, a)
			}
		}
		p.AltPos = alt
	}
	found := false
	for _, l := range lg.NeedsOf(5).Lacks {
		if strings.Contains(l, "Center depth") {
			found = true
		}
	}
	if !found {
		t.Fatalf("lacks %v should include center depth", lg.NeedsOf(5).Lacks)
	}
}

// checkTake verifies every package of a result with the real rules.
func checkTake(t *testing.T, lg *League, r TakeResult) {
	t.Helper()
	for _, pk := range r.Packages {
		d := pk.Deal(lg.UserTeamID, r.Team)
		if err := ValidateDeal(lg, d, lg.UserTeamID); err != nil {
			t.Fatalf("%s: package %q fails ValidateDeal: %v", r.Player.Name, pk.Label, err)
		}
		if v := EvaluateDeal(lg, d, lg.UserTeamID); !v.Valid || !v.Accept {
			t.Fatalf("%s: package %q not accepted: %+v", r.Player.Name, pk.Label, v)
		}
		if len(pk.Give) == 0 || len(pk.Receive) != 1 || pk.Receive[0] != PlayerOf(r.Player.ID) {
			t.Fatalf("bad package shape %+v", pk)
		}
		picks := 0
		for _, a := range pk.Give {
			if a.Kind == PickAsset {
				picks++
			}
		}
		if len(pk.Give)-picks > MaxTradePlayers || picks > AssistTuning.MaxPicks {
			t.Fatalf("package too big: %+v", pk)
		}
	}
}

func TestWhatWouldItTakeAccepted(t *testing.T) {
	total, possible := 0, 0
	for _, seed := range []uint64{1, 2, 3} {
		for _, halve := range []bool{false, true} {
			var lg *League
			if halve {
				lg = tradeLeague(seed)
			} else {
				lg = realSalaryLeague(seed)
			}
			for _, tid := range []int{1, 7, 14, 22} {
				for _, p := range lg.Teams[tid].Roster {
					r, err := lg.WhatWouldItTake(p.ID)
					if err != nil {
						t.Fatal(err)
					}
					total++
					if r.Possible {
						possible++
						checkTake(t, lg, r)
						if !r.Packages[0].Cheapest {
							t.Fatalf("first package should be the cheapest")
						}
					} else if r.Reason == "" {
						t.Fatalf("seed %d %s: impossible without a reason", seed, p.Name)
					}
				}
			}
		}
	}
	t.Logf("%d of %d players have a package", possible, total)
	if possible < total/3 {
		t.Fatalf("only %d of %d players have a package", possible, total)
	}
}

// A suggested package, proposed as is, goes through.
func TestWhatWouldItTakeExecutes(t *testing.T) {
	lg := tradeLeague(5)
	target := sortedByOvr(lg.Teams[9])[3]
	r, err := lg.WhatWouldItTake(target.ID)
	if err != nil || !r.Possible {
		t.Fatalf("want a package, got %+v %v", r, err)
	}
	pk := r.Packages[len(r.Packages)-1]
	if _, err := ExecuteDeal(lg, pk.Deal(0, 9), 0); err != nil {
		t.Fatalf("suggested package refused: %v", err)
	}
	if lg.findRosterPlayer(0, target.ID) == nil {
		t.Fatal("target did not arrive")
	}
}

func TestWhatWouldItTakeImpossible(t *testing.T) {
	// A franchise player on a weak team, user has little.
	lg := tradeLeague(6)
	star := sortedByOvr(lg.Teams[4])[0]
	star.Ratings = Ratings{97, 97, 97, 97, 97, 97, 97}
	star.Age = 25
	r, _ := lg.WhatWouldItTake(star.ID)
	if r.Possible || !strings.Contains(r.Reason, "franchise") {
		t.Fatalf("want a franchise refusal, got %+v", r)
	}

	// Value is there but the salaries cannot be matched.
	lg = realSalaryLeague(6)
	lg.UserTeamID = 0
	for _, p := range lg.Teams[0].Roster {
		p.Contract.Salary = 10000
	}
	tgt := sortedByOvr(lg.Teams[12])[4]
	tgt.Contract.Salary, tgt.Contract.Years = 49000, 3
	// Their payroll is high too, so they cannot just swallow salary.
	for _, p := range lg.Teams[12].Roster {
		if p != tgt {
			p.Contract.Salary = 12000
		}
	}
	// Give the user a very rich roster so value is not the issue.
	for _, p := range lg.Teams[0].Roster {
		p.Ratings = Ratings{80, 80, 80, 80, 80, 80, 80}
	}
	tgt.Ratings = Ratings{70, 70, 70, 70, 70, 70, 70}
	r, _ = lg.WhatWouldItTake(tgt.ID)
	if r.Possible || !strings.Contains(r.Reason, "Salary can't be matched") {
		t.Fatalf("want a salary refusal, got possible=%v reason=%q", r.Possible, r.Reason)
	}

	// Nothing to offer.
	lg = tradeLeague(6)
	for _, p := range lg.Teams[0].Roster {
		p.Ratings = Ratings{35, 35, 35, 35, 35, 35, 35}
		p.Potential = 40
	}
	lg.Picks = nil
	tgt = sortedByOvr(lg.Teams[3])[2]
	r, _ = lg.WhatWouldItTake(tgt.ID)
	if r.Possible || r.Reason == "" {
		t.Fatalf("want a reason, got %+v", r)
	}
	t.Log(r.Reason)

	// Errors.
	if _, err := lg.WhatWouldItTake(lg.Teams[0].Roster[0].ID); err == nil {
		t.Fatal("own player should be an error")
	}
	if _, err := lg.WhatWouldItTake(-5); err == nil {
		t.Fatal("unknown player should be an error")
	}
	lg.Season.Day = TradeDeadlineDay + 1
	r, _ = lg.WhatWouldItTake(tgt.ID)
	if r.Possible || !strings.Contains(r.Reason, "deadline") {
		t.Fatalf("closed window: %+v", r)
	}
}

func TestWhatWouldItTakeSkipsOwnCore(t *testing.T) {
	lg := tradeLeague(8)
	cf := lg.contendFactors()
	core := lg.untouchables(0, cf)
	target := sortedByOvr(lg.Teams[10])[5]
	r, _ := lg.WhatWouldItTake(target.ID)
	if !r.Possible {
		t.Skip("no package")
	}
	for _, pk := range r.Packages {
		for _, a := range pk.Give {
			if a.Kind == PlayerAsset && core[a.PlayerID] {
				t.Fatalf("package includes your core player: %+v", pk)
			}
		}
	}
}

func TestWhatWouldItTakeSpeed(t *testing.T) {
	lg := realSalaryLeague(4)
	worst, sum, n := time.Duration(0), time.Duration(0), 0
	for _, tid := range []int{2, 11, 20} {
		for _, p := range lg.Teams[tid].Roster {
			start := time.Now()
			if _, err := lg.WhatWouldItTake(p.ID); err != nil {
				t.Fatal(err)
			}
			d := time.Since(start)
			sum += d
			n++
			worst = max(worst, d)
		}
	}
	t.Logf("average %v, worst %v over %d players", sum/time.Duration(n), worst, n)
	if sum/time.Duration(n) > 200*time.Millisecond || worst > 2*time.Second {
		t.Fatalf("too slow: average %v, worst %v", sum/time.Duration(n), worst)
	}
}

func advice(rows []AdvisorRow, id int) AdvisorRow {
	for _, r := range rows {
		if r.Player.ID == id {
			return r
		}
	}
	return AdvisorRow{}
}

func TestAdvisor(t *testing.T) {
	lg := realSalaryLeague(9)
	// the user runs the weakest team
	weak := 0
	for i := range lg.Teams {
		if lg.contendFactors()[i] < lg.contendFactors()[weak] {
			weak = i
		}
	}
	lg.UserTeamID = weak
	if k := lg.TeamStatusOf(weak).Key; k != "rebuilding" {
		t.Fatalf("weakest team is %s", k)
	}
	ro := sortedByOvr(lg.Teams[weak])
	vet, kid, other := ro[2], ro[0], ro[4]
	vet.Age, vet.Contract = 34, Contract{Salary: 30000, Years: 3}
	vet.Ratings = Ratings{72, 72, 72, 72, 72, 72, 72}
	kid.Age, kid.Experience = 21, 2
	kid.Ratings = Ratings{82, 82, 82, 82, 82, 82, 82}
	kid.Potential = 90
	other.TradeRequest = lg.Year*100 + 3
	other.Morale = -30

	rows, summary := lg.AdvisorOf()
	if len(rows) != len(lg.Teams[weak].Roster) || summary == "" {
		t.Fatalf("rows %d, summary %q", len(rows), summary)
	}
	if a := advice(rows, vet.ID); a.Advice != AdviceSell {
		t.Fatalf("old expensive vet on a rebuilding team: %+v", a)
	}
	if a := advice(rows, kid.ID); a.Advice != AdviceKeep {
		t.Fatalf("young cornerstone: %+v", a)
	}
	if a := advice(rows, other.ID); a.Advice == AdviceKeep {
		t.Fatalf("requester must not be Keep: %+v", a)
	}
	for _, r := range rows {
		if len(r.Reasons) < 1 || len(r.Reasons) > 3 {
			t.Fatalf("%s: reasons %v", r.Player.Name, r.Reasons)
		}
		if r.Advice != AdviceKeep && r.Advice != AdviceShop && r.Advice != AdviceSell {
			t.Fatalf("bad advice %q", r.Advice)
		}
	}
	rows2, summary2 := lg.AdvisorOf()
	if summary != summary2 || len(rows2) != len(rows) {
		t.Fatal("advisor is not deterministic")
	}
	for i := range rows {
		if rows[i].Advice != rows2[i].Advice || !reflect.DeepEqual(rows[i].Reasons, rows2[i].Reasons) {
			t.Fatal("advisor is not deterministic")
		}
	}
	for _, r := range rows {
		t.Log(r.Player.Name, r.Player.Age, r.Player.Overall(), r.Advice, r.Reasons)
	}
	t.Log(summary)
}

func TestAdvisorContenderKeepsVeteran(t *testing.T) {
	lg := realSalaryLeague(9)
	strong := 0
	for i := range lg.Teams {
		if lg.contendFactors()[i] > lg.contendFactors()[strong] {
			strong = i
		}
	}
	lg.UserTeamID = strong
	ro := sortedByOvr(lg.Teams[strong])
	v := ro[3]
	v.Age, v.Contract = 32, Contract{Salary: MarketSalary(v), Years: 2}
	rows, _ := lg.AdvisorOf()
	if a := advice(rows, v.ID); a.Advice == AdviceSell {
		t.Fatalf("productive vet on a contender should not be Sell: %+v", a)
	}
}

// On draft night the packages still pass the real rules (expiring contracts
// and used picks are never part of one).
func TestWhatWouldItTakeDraftNight(t *testing.T) {
	lg := draftLeague(11)
	n := 0
	for _, tid := range []int{3, 8, 15} {
		for _, p := range lg.Teams[tid].Roster {
			r, err := lg.WhatWouldItTake(p.ID)
			if err != nil {
				t.Fatal(err)
			}
			if r.Possible {
				n++
				checkTake(t, lg, r)
			} else if r.Reason == "" {
				t.Fatalf("%s: impossible without a reason", p.Name)
			}
		}
	}
	if n == 0 {
		t.Fatal("no package on draft night")
	}
}
