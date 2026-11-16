package engine

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

var finderWantKeys = []string{"any", "players", "picks", "young", "relief"}

// userBest returns the user's players, best first.
func userBest(lg *League) []*Player { return sortedByOvr(lg.Teams[lg.UserTeamID]) }

func someUserPicks(lg *League, n int) []Asset {
	var out []Asset
	for _, k := range lg.PicksOwnedBy(lg.UserTeamID) {
		if len(out) < n {
			out = append(out, PickOf(k.Year, k.Round, k.Original))
		}
	}
	return out
}

func checkFinder(t *testing.T, lg *League, give []Asset, want string, r FindResult) {
	t.Helper()
	user := lg.UserTeamID
	seen := map[int]bool{}
	for i, o := range r.Offers {
		if o.Team == user || seen[o.Team] {
			t.Fatalf("%s: bad or duplicate team %d", want, o.Team)
		}
		seen[o.Team] = true
		d := o.Deal(user, give)
		if v := EvaluateDeal(lg, d, user); !v.Valid || !v.Accept {
			t.Fatalf("%s: offer from %d not accepted: %+v", want, o.Team, v)
		}
		if len(o.Receive) == 0 || o.Note == "" {
			t.Fatalf("%s: bad offer %+v", want, o)
		}
		if want != "relief" && i > 0 && o.Value > r.Offers[i-1].Value+1e-9 {
			t.Fatalf("%s: offers not sorted by value", want)
		}
		for _, a := range o.Receive {
			p := lg.FindPlayer(a.PlayerID)
			switch want {
			case "players":
				if a.Kind != PlayerAsset {
					t.Fatalf("players-only offer has a pick")
				}
			case "picks":
				if a.Kind != PickAsset {
					t.Fatalf("picks-only offer has a player")
				}
			case "young":
				if a.Kind == PlayerAsset && p.Age > AssistTuning.YoungAge {
					t.Fatalf("young offer has %s aged %d", p.Name, p.Age)
				}
			}
		}
		if want == "relief" {
			if o.SalaryIn >= o.SalaryOut {
				t.Fatalf("relief offer takes in %d, sends %d", o.SalaryIn, o.SalaryOut)
			}
			if i > 0 && o.SalaryOut-o.SalaryIn > r.Offers[i-1].SalaryOut-r.Offers[i-1].SalaryIn {
				t.Fatalf("relief offers not sorted by salary saved")
			}
		}
	}
	for _, p := range r.Passed {
		if p.Reason == "" || seen[p.Team] || p.Team == user {
			t.Fatalf("bad pass %+v", p)
		}
	}
	if len(r.Offers)+len(r.Passed) != len(lg.Teams)-1 {
		t.Fatalf("%d offers + %d passed != %d teams", len(r.Offers), len(r.Passed), len(lg.Teams)-1)
	}
}

func TestFindOffersRules(t *testing.T) {
	total := 0
	for _, seed := range []uint64{1, 2} {
		for _, halve := range []bool{false, true} {
			lg := realSalaryLeague(seed)
			if halve {
				lg = tradeLeague(seed)
			}
			ps := userBest(lg)
			sels := [][]Asset{{PlayerOf(ps[0].ID)}, {PlayerOf(ps[5].ID)}, {PlayerOf(ps[1].ID), PlayerOf(ps[3].ID)},
				append([]Asset{PlayerOf(ps[2].ID)}, someUserPicks(lg, 2)...)}
			for _, give := range sels {
				for _, want := range finderWantKeys {
					r, err := lg.FindOffers(give, want)
					if err != nil {
						t.Fatal(err)
					}
					if r.Possible != (len(r.Offers) > 0) || (!r.Possible && r.Reason == "") {
						t.Fatalf("bad Possible/Reason: %+v", r)
					}
					checkFinder(t, lg, give, want, r)
					total += len(r.Offers)
				}
			}
		}
	}
	t.Logf("%d offers checked", total)
	if total == 0 {
		t.Fatal("no offers at all")
	}
}

func TestFindOffersExecutes(t *testing.T) {
	lg := tradeLeague(5)
	give := []Asset{PlayerOf(userBest(lg)[1].ID)}
	r, err := lg.FindOffers(give, "any")
	if err != nil || !r.Possible {
		t.Fatalf("want offers, got %+v %v", r, err)
	}
	o := r.Offers[0]
	if _, err := ExecuteDeal(lg, o.Deal(0, give), 0); err != nil {
		t.Fatalf("offer refused: %v", err)
	}
	if lg.findRosterPlayer(o.Team, give[0].PlayerID) == nil {
		t.Fatal("player did not arrive")
	}
}

func TestFindOffersCombine(t *testing.T) {
	lg := tradeLeague(2)
	ps := userBest(lg)
	give := []Asset{PlayerOf(ps[0].ID), PlayerOf(ps[1].ID)}
	r, err := lg.FindOffers(give, "any")
	if err != nil || !r.Possible {
		t.Fatalf("combination: %+v %v", r, err)
	}
	checkFinder(t, lg, give, "any", r)
	one, _ := lg.FindOffers(give[:1], "any")
	if r.GiveValue <= one.GiveValue {
		t.Fatalf("two players should be worth more than one")
	}
}

func TestFindOffersRefusals(t *testing.T) {
	lg := tradeLeague(3)
	ps := userBest(lg)
	mine := []Asset{PlayerOf(ps[0].ID)}
	if _, err := lg.FindOffers(mine, "bogus"); err == nil {
		t.Fatal("unknown want should be an error")
	}
	if ValidFindWant("bogus") || !ValidFindWant("relief") {
		t.Fatal("ValidFindWant wrong")
	}
	foreign := lg.Teams[4].Roster[0].ID
	if _, err := lg.FindOffers([]Asset{PlayerOf(foreign)}, "any"); err == nil {
		t.Fatal("foreign player should be an error")
	}
	if _, err := lg.FindOffers([]Asset{PlayerOf(-77)}, "any"); err == nil {
		t.Fatal("unknown player should be an error")
	}
	k := lg.PicksOwnedBy(4)[0]
	if _, err := lg.FindOffers([]Asset{PickOf(k.Year, k.Round, k.Original)}, "any"); err == nil {
		t.Fatal("foreign pick should be an error")
	}
	if r, err := lg.FindOffers(nil, "any"); err != nil || r.Possible || r.Reason == "" {
		t.Fatalf("empty selection: %+v %v", r, err)
	}
	many := []Asset{PlayerOf(ps[0].ID), PlayerOf(ps[1].ID), PlayerOf(ps[2].ID), PlayerOf(ps[3].ID)}
	if r, err := lg.FindOffers(many, "any"); err != nil || r.Possible || r.Reason == "" {
		t.Fatalf("too many players: %+v %v", r, err)
	}
	lg.Season.Day = TradeDeadlineDay + 1
	r, err := lg.FindOffers(mine, "any")
	if err != nil || r.Possible || r.Reason == "" {
		t.Fatalf("closed window: %+v %v", r, err)
	}
}

func TestFindOffersDraftNight(t *testing.T) {
	lg := draftLeague(11)
	ps := userBest(lg)
	var tradable, expiring *Player
	for _, p := range ps {
		if p.Contract.Years >= 1 && tradable == nil {
			tradable = p
		}
		if p.Contract.Years < 1 && expiring == nil {
			expiring = p
		}
	}
	if tradable == nil {
		t.Skip("no tradable player")
	}
	give := []Asset{PlayerOf(tradable.ID)}
	r, err := lg.FindOffers(give, "any")
	if err != nil {
		t.Fatal(err)
	}
	checkFinder(t, lg, give, "any", r)
	if expiring != nil {
		r, _ := lg.FindOffers([]Asset{PlayerOf(expiring.ID)}, "any")
		if r.Possible || r.Reason == "" {
			t.Fatalf("expiring contract on draft night: %+v", r)
		}
	}
}

func TestFindOffersPureAndDeterministic(t *testing.T) {
	lg := realSalaryLeague(3)
	dir := t.TempDir()
	save := func(name string) []byte {
		p := filepath.Join(dir, name)
		if err := lg.Save(p); err != nil {
			t.Fatal(err)
		}
		b, _ := os.ReadFile(p)
		return b
	}
	before := save("a.json")
	give := append([]Asset{PlayerOf(userBest(lg)[0].ID)}, someUserPicks(lg, 1)...)
	r1, _ := lg.FindOffers(give, "any")
	r2, _ := lg.FindOffers(give, "any")
	if !reflect.DeepEqual(r1, r2) {
		t.Fatal("two calls differ")
	}
	if string(before) != string(save("b.json")) {
		t.Fatal("FindOffers changed the league")
	}
}

func TestFindOffersSpeed(t *testing.T) {
	lg := realSalaryLeague(1)
	ps := userBest(lg)
	for _, want := range finderWantKeys {
		give := append([]Asset{PlayerOf(ps[0].ID), PlayerOf(ps[1].ID), PlayerOf(ps[2].ID)}, someUserPicks(lg, 2)...)
		start := time.Now()
		if _, err := lg.FindOffers(give, want); err != nil {
			t.Fatal(err)
		}
		if d := time.Since(start); d > time.Second {
			t.Fatalf("%s: took %v", want, d)
		} else {
			t.Logf("%s: %v", want, d)
		}
	}
}
