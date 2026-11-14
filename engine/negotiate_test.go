package engine

import "testing"

func TestDeadlineConstant(t *testing.T) {
	if TradeDeadlineDay != 50 {
		t.Fatal("deadline should be day 50")
	}
	lg := tradeLeague(2)
	lg.Season.Day = TradeDeadlineDay
	if ok, _ := TradeWindowOpen(lg); !ok {
		t.Fatal("closed on the deadline day")
	}
	lg.Season.Day++
	if ok, _ := TradeWindowOpen(lg); ok {
		t.Fatal("open after the deadline")
	}
}

func TestTiers(t *testing.T) {
	lg := tradeLeague(1)
	valid := map[string]bool{"Franchise": true, "Star": true, "Starter": true, "Rotation": true, "Bench": true, "Prospect": true}
	counts := map[string]int{}
	total := 0
	for _, tm := range lg.Teams {
		for _, p := range tm.Roster {
			tier := PlayerTier(p)
			if !valid[tier] {
				t.Fatalf("bad tier %q", tier)
			}
			counts[tier]++
			total++
		}
	}
	t.Logf("tiers over %d players: %v", total, counts)
	if counts["Franchise"] > 15 || counts["Star"] == 0 || counts["Starter"] == 0 || counts["Rotation"] == 0 || counts["Bench"] == 0 {
		t.Fatalf("odd spread: %v", counts)
	}
	cf := lg.contendFactors()
	weak, strong := 0, 0
	for i := range cf {
		if cf[i] < cf[weak] {
			weak = i
		}
		if cf[i] > cf[strong] {
			strong = i
		}
	}
	y := lg.Year
	if lg.PickTier(y, 1, weak) != "Lottery pick" || lg.PickTier(y, 1, strong) != "Late first" || lg.PickTier(y, 2, weak) != "Second-rounder" {
		t.Fatal("pick tiers out of order")
	}
	if lg.PickValue(y, 1, weak, 0) <= lg.PickValue(y, 1, strong, 0) {
		t.Fatal("tier order disagrees with value")
	}
}

func TestNegotiationAsk(t *testing.T) {
	lg := dealLeague()
	mine := sortedByOvr(lg.Teams[0])
	closeFound := 0
	for tid := 1; tid < 30 && closeFound < 3; tid++ {
		theirs := sortedByOvr(lg.Teams[tid])
		for _, u := range mine {
			for _, q := range theirs[2:] {
				d := Deal{[]Move{pl(0, tid, u.ID), pl(tid, 0, q.ID)}}
				v := EvaluateDeal(lg, d, 0)
				if !v.Valid {
					continue
				}
				tv := v.Teams[0]
				if tv.Accept {
					if tv.Mood != "accept" || tv.Ask != nil {
						t.Fatal("accepting verdict has wrong mood or ask")
					}
					continue
				}
				if tv.AskText == "" || (tv.Mood != "close" && tv.Mood != "far" && tv.Mood != "no") {
					t.Fatalf("bad refusal: %+v", tv)
				}
				if tv.Ask != nil {
					nd := Deal{append(append([]Move(nil), d.Moves...), Move{0, tid, *tv.Ask})}
					if r := EvaluateDeal(lg, nd, 0); !r.Valid || !r.Teams[0].Accept {
						t.Fatalf("asked asset does not make them accept: %+v", tv)
					}
					if tv.Mood == "close" {
						closeFound++
					}
				}
			}
		}
	}
	if closeFound == 0 {
		t.Fatal("never saw a close deal with an ask")
	}
	// hopeless
	theirs := sortedByOvr(lg.Teams[5])
	v := EvaluateDeal(lg, Deal{[]Move{pl(0, 5, mine[12].ID), pl(5, 0, theirs[0].ID)}}, 0)
	tv := v.Teams[0]
	if tv.Accept || (tv.Mood != "far" && tv.Mood != "no") || tv.AskText == "" {
		t.Fatalf("hopeless deal: %+v", tv)
	}
}

// userRatio is what the user gets over what the user gives, on the user's valuation.
func userRatio(lg *League, d Deal) float64 {
	user := lg.UserTeamID
	cf := lg.contendFactors()
	var out, in []*Player
	pickV := 0.0
	for _, m := range d.Moves {
		if m.Kind == PlayerAsset {
			if m.From == user {
				out = append(out, lg.findRosterPlayer(user, m.PlayerID))
			} else {
				in = append(in, lg.findRosterPlayer(m.From, m.PlayerID))
			}
		} else if m.To == user {
			pickV += lg.pickValue(m.Asset, user, cf)
		}
	}
	g, gt, _ := lg.sideEval(user, out, in, cf)
	return (gt + pickV) / g
}

func TestProposalsGenerated(t *testing.T) {
	total, seeds := 0, 8
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		lg := tradeLeague(seed)
		for lg.Season.Day < TradeDeadlineDay+5 {
			lg.SimDay()
			ps := lg.PendingProposals()
			if len(ps) > MaxPendingProposals {
				t.Fatal("too many pending")
			}
			if lg.ProposalsCreatedLastDay() > 0 {
				p := ps[len(ps)-1]
				total += lg.ProposalsCreatedLastDay()
				if p.Day != lg.Season.Day || p.Day >= TradeDeadlineDay || p.Message == "" {
					t.Fatalf("bad proposal %+v", p)
				}
				if err := ValidateDeal(lg, p.Deal, 0); err != nil {
					t.Fatal(err)
				}
				if v := evaluateDealBase(lg, p.Deal, 0); !v.Accept {
					t.Fatal("proposer would not accept its own proposal")
				}
				if r := userRatio(lg, p.Deal); r < proposalBandLo-1e-9 || r > proposalBandHi+1e-9 {
					t.Fatalf("user ratio %.3f outside band", r)
				}
			}
		}
		if len(lg.PendingProposals()) != 0 {
			t.Fatal("proposals survive past the deadline")
		}
	}
	t.Logf("%d proposals over %d seasons (%.1f per season, %.1f days each)", total, seeds, float64(total)/float64(seeds), float64(TradeDeadlineDay)/(float64(total)/float64(seeds)))
	if total < seeds || total > seeds*15 {
		t.Fatalf("implausible proposal count %d", total)
	}
}

// pendingLeague sims until at least one proposal is pending.
func pendingLeague(t *testing.T) *League {
	for seed := uint64(1); seed < 30; seed++ {
		lg := tradeLeague(seed)
		for lg.Season.Day < TradeDeadlineDay-1 {
			lg.SimDay()
			if len(lg.Proposals) > 0 {
				return lg
			}
		}
	}
	t.Fatal("no proposal found")
	return nil
}

func TestProposalAcceptRejectDrop(t *testing.T) {
	lg := pendingLeague(t)
	p := lg.PendingProposals()[0]
	var target int
	for _, m := range p.Deal.Moves {
		if m.From == 0 && m.Kind == PlayerAsset {
			target = m.PlayerID
		}
	}
	if _, err := lg.AcceptProposal(p.ID); err != nil {
		t.Fatal(err)
	}
	if lg.findRosterPlayer(p.Team, target) == nil || lg.proposalIndex(p.ID) >= 0 {
		t.Fatal("accept did not execute or remove")
	}
	if _, err := lg.AcceptProposal(p.ID); err == nil {
		t.Fatal("accepted twice")
	}
	// reject
	lg = pendingLeague(t)
	id := lg.Proposals[0].ID
	if !lg.RejectProposal(id) || lg.RejectProposal(id) || len(lg.Proposals) != 0 && lg.proposalIndex(id) >= 0 {
		t.Fatal("reject")
	}
	// expired
	lg = pendingLeague(t)
	lg.Proposals[0].Expires = lg.Season.Day
	lg.updateProposals()
	if len(lg.Proposals) != 0 && lg.Proposals[0].Expires == lg.Season.Day {
		t.Fatal("expired proposal kept")
	}
	// invalidated: the target leaves the user's roster
	lg = pendingLeague(t)
	p = lg.Proposals[0]
	for _, m := range p.Deal.Moves {
		if m.From == 0 && m.Kind == PlayerAsset {
			pp := lg.findRosterPlayer(0, m.PlayerID)
			var keep []*Player
			for _, q := range lg.Teams[0].Roster {
				if q != pp {
					keep = append(keep, q)
				}
			}
			lg.Teams[0].Roster = keep
		}
	}
	lg.dropBadProposals()
	if lg.proposalIndex(p.ID) >= 0 {
		t.Fatal("invalid proposal kept")
	}
}

func TestProposalsDoNotChangeGames(t *testing.T) {
	a, b := tradeLeague(4), tradeLeague(4)
	for i := 0; i < 60; i++ {
		a.SimDay()
		b.SimDay()
		b.Proposals = nil
	}
	for i := range a.Season.Wins {
		if a.Season.Wins[i] != b.Season.Wins[i] || a.Season.PointDiff[i] != b.Season.PointDiff[i] {
			t.Fatal("proposals changed game results")
		}
	}
}

func TestDeadlinePause(t *testing.T) {
	for _, off := range []bool{false, true} {
		lg := tradeLeague(6)
		lg.Options.NoDeadlineStop = off
		fires := 0
		for lg.Season.Day < TradeDeadlineDay+10 {
			if lg.ShouldPauseForDeadline() {
				if lg.Season.Day != TradeDeadlineDay || lg.ShouldPauseForDeadline() {
					t.Fatal("fired on the wrong day or twice")
				}
				fires++
			}
			lg.SimDay()
		}
		if want := map[bool]int{false: 1, true: 0}[off]; fires != want {
			t.Fatalf("NoDeadlineStop=%v fired %d times", off, fires)
		}
	}
}
