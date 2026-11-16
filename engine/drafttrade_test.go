package engine

import (
	"path/filepath"
	"reflect"
	"testing"
)

// userPick returns the user's own round-1 pick slot (0-based) in a draft.
func slotOf(lg *League, round, orig int) int {
	k, _ := lg.draftSlotIndex(lg.Year, round, orig)
	return k
}

func draftOffers(lg *League) []Proposal {
	var out []Proposal
	for _, p := range lg.PendingProposals() {
		if p.DraftPick > 0 {
			out = append(out, p)
		}
	}
	return out
}

func TestDraftNightWindow(t *testing.T) {
	lg := draftLeague(11)
	if ok, why := TradeWindowOpen(lg); !ok || why != "Draft night: trades are open." {
		t.Fatalf("window during draft: %v %q", ok, why)
	}
	lg.DraftAutoAll()
	if ok, _ := TradeWindowOpen(lg); ok {
		t.Fatal("window should close once the last pick is made")
	}
}

func TestDraftNightPickTradeAndUsedPick(t *testing.T) {
	var lg *League
	var user, mine, other, os int
	for seed := uint64(11); seed < 40 && other == 0; seed++ {
		lg = draftLeague(seed)
		user = lg.UserTeamID
		other, os = pickPair(lg)
	}
	if other == 0 {
		t.Fatal("no seed with a later computer pick found")
	}
	us := 0
	for i, o := range lg.Draft.Order {
		if o == user {
			us, mine = i, o
		}
	}
	_ = us
	deal := Deal{[]Move{pk(user, other, lg.Year, 1, mine), pk(user, other, lg.Year, 2, user), pk(other, user, lg.Year, 1, other)}}
	v := EvaluateDeal(lg, deal, user)
	if !v.Valid || !v.Accept {
		t.Fatalf("deal should work: %+v", v)
	}
	// value units: an earlier slot is worth more
	if lg.PickValue(lg.Year, 1, mine, user) <= lg.PickValue(lg.Year, 1, other, user) {
		t.Fatal("earlier slot should be worth more")
	}
	if _, err := ExecuteDeal(lg, deal, user); err != nil {
		t.Fatal(err)
	}
	if o, _ := lg.PickOwner(lg.Year, 1, other); o != user {
		t.Fatal("pick did not move")
	}
	// the draft runs until that slot; the new owner selects
	for lg.Draft.Made < os {
		lg.DraftPick(-1)
	}
	if lg.DraftOnClock() != user {
		t.Fatalf("on the clock: %d, want user", lg.DraftOnClock())
	}
	lg.DraftPick(-1)
	if last := lg.Draft.Results[len(lg.Draft.Results)-1]; last.TeamID != user {
		t.Fatalf("pick went to %d", last.TeamID)
	}
	// that pick (and every earlier one) is used now: not listed, not tradable
	for _, p := range lg.PicksOwnedBy(user) {
		if p.Year == lg.Year && p.Round == 1 && p.Original == other {
			t.Fatal("used pick still listed")
		}
	}
	if err := lg.TransferPick(lg.Year, 1, other, user, 3); err == nil {
		t.Fatal("transfer of a used pick should fail")
	}
	wantProblem(t, lg, Deal{[]Move{pk(user, 5, lg.Year, 1, other), pl(5, user, lg.Teams[5].Roster[0].ID)}}, "already been used")
}

func TestDraftNightDrafteeTradableNoTrim(t *testing.T) {
	lg := draftLeague(11)
	user := lg.UserTeamID
	for lg.DraftOnClock() != user && !lg.DraftDone() {
		lg.DraftPick(-1)
	}
	lg.DraftPick(-1)
	rookie := lg.Draft.Results[len(lg.Draft.Results)-1]
	var rp *Player
	for _, p := range lg.Teams[user].Roster {
		if p.ID == rookie.PlayerID {
			rp = p
		}
	}
	if rp == nil {
		t.Fatal("rookie not on the roster")
	}
	found := false
	for _, a := range TradeBlock(lg, user) {
		if a.PlayerID == rp.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("just-drafted player not on the trade block")
	}
	faBefore := len(lg.FreeAgents)
	dead := map[int]int{}
	for i, tm := range lg.Teams {
		dead[i] = len(tm.DeadMoney)
	}
	var done bool
	for tid := range lg.Teams {
		if tid == user {
			continue
		}
		for _, p := range lg.PicksOwnedBy(tid) {
			d := Deal{[]Move{pl(user, tid, rp.ID), pk(tid, user, p.Year, p.Round, p.Original)}}
			if v := EvaluateDeal(lg, d, user); v.Valid && v.Accept {
				res, err := ExecuteDeal(lg, d, user)
				if err != nil {
					t.Fatal(err)
				}
				if len(res.Released) != 0 || len(res.Signed) != 0 {
					t.Fatalf("draft night trade changed rosters: %+v", res)
				}
				done = true
			}
			if done {
				break
			}
		}
		if done {
			break
		}
	}
	if !done {
		t.Fatal("no team took the rookie for a pick")
	}
	if len(lg.FreeAgents) != faBefore {
		t.Fatal("free agents changed during the draft")
	}
	for i, tm := range lg.Teams {
		if len(tm.DeadMoney) != dead[i] {
			t.Fatalf("dead money created for team %d", i)
		}
	}
	// the old trade path also leaves rosters alone
	for _, tm := range lg.Teams {
		for _, p := range tm.Roster {
			if p.Contract.Years < 1 {
				if err := lg.draftTradeProblem([]*Player{p}); err == nil {
					t.Fatal("expiring contract should not be tradable")
				}
				return
			}
		}
	}
}

func TestTradeUpOfferOnTheClock(t *testing.T) {
	for seed := uint64(1); seed <= 20; seed++ {
		lg := draftLeague(seed)
		if len(draftOffers(lg)) != 0 {
			t.Fatal("offer before the user is on the clock")
		}
		for !lg.DraftDone() && lg.DraftOnClock() != lg.UserTeamID {
			lg.DraftPick(-1)
			if len(draftOffers(lg)) != 0 {
				t.Fatal("offer while a computer team is on the clock")
			}
		}
		if lg.DraftDone() {
			continue
		}
		lg.EnsureDraftOffer()
		offs := draftOffers(lg)
		if len(offs) == 0 {
			continue
		}
		if len(offs) != 1 {
			t.Fatalf("%d offers for one pick", len(offs))
		}
		lg.EnsureDraftOffer()
		if len(draftOffers(lg)) != 1 {
			t.Fatal("a second call added another offer")
		}
		// deterministic: same seed, same offer
		lg2 := draftLeague(seed)
		lg2.DraftAutoUntilUser()
		o2 := draftOffers(lg2)
		if len(o2) != 1 || !reflect.DeepEqual(o2[0].Deal, offs[0].Deal) || o2[0].Team != offs[0].Team {
			t.Fatalf("offer not deterministic: %+v vs %+v", o2, offs)
		}
		// save/load mid-draft keeps the owner, the offer and the "already checked" mark
		path := filepath.Join(t.TempDir(), "s.json")
		if err := lg.Save(path); err != nil {
			t.Fatal(err)
		}
		got, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		g := draftOffers(got)
		if len(g) != 1 || !reflect.DeepEqual(g[0].Deal, offs[0].Deal) || got.Draft.OfferChecked != lg.Draft.OfferChecked {
			t.Fatalf("offer lost in the round trip: %+v", g)
		}
		got.EnsureDraftOffer()
		if len(draftOffers(got)) != 1 {
			t.Fatal("loaded draft made another offer")
		}
		// accept: the pick moves and the other team is on the clock
		o := offs[0]
		pick := o.Deal.Moves[0]
		if _, err := lg.AcceptProposal(o.ID); err != nil {
			t.Fatal(err)
		}
		if own, _ := lg.PickOwner(pick.Year, pick.Round, pick.Original); own != o.Team {
			t.Fatalf("pick owner %d, want %d", own, o.Team)
		}
		if lg.DraftOnClock() != o.Team {
			t.Fatalf("on the clock %d, want %d", lg.DraftOnClock(), o.Team)
		}
		if len(draftOffers(lg)) != 0 {
			t.Fatal("offer still pending after accept")
		}
		lg.EnsureDraftOffer()
		if len(draftOffers(lg)) != 0 {
			t.Fatal("a new offer for a pick the user no longer holds")
		}
		// reject leaves no second offer for the same pick
		lg2.RejectProposal(o2[0].ID)
		lg2.EnsureDraftOffer()
		if len(draftOffers(lg2)) != 0 {
			t.Fatal("offer came back after a reject")
		}
		// the offer ends when the user picks
		lg3 := draftLeague(seed)
		lg3.DraftAutoUntilUser()
		lg3.DraftPick(-1)
		if len(draftOffers(lg3)) != 0 {
			t.Fatal("offer outlived the pick")
		}
		t.Logf("seed %d: offer from team %d, %d assets", seed, o.Team, len(o.Deal.Moves)-1)
		return
	}
	t.Fatal("no seed in 1..20 produced a trade-up offer")
}

// pickPair finds a computer team whose round-1 slot is at least 6 later than
// the user's (0 and 0 if there is none).
func pickPair(lg *League) (other, slot int) {
	us := -1
	for i, o := range lg.Draft.Order {
		if o == lg.UserTeamID {
			us = i
		}
	}
	for i, o := range lg.Draft.Order {
		if o != lg.UserTeamID && us >= 0 && i >= us+6 && i > slot {
			other, slot = o, i
		}
	}
	return
}
