package engine

import (
	"fmt"
	"math"
	"sort"
)

// Draft night trades (RULES.md: offset 130000). Trades are open while the
// draft is running. This file holds what is special about them: which picks
// are still tradable, expiring contracts, and the computer teams' offers to
// trade up for the pick the user has on the clock.

// DraftTradeTuning shapes the trade-up offers. A computer team offers when its
// favourite prospect (by its own noisy view) is clearly better than what it
// expects to get at its own, later pick.
var DraftTradeTuning = struct {
	Chance  float64 // chance that an eligible user pick draws an offer at all
	MinGain float64 // draft-score points its favourite beats the prospect it expects to get later
	MaxJump int     // most slots a team will trade up
	Offset  uint64  // random stream
}{Chance: 0.6, MinGain: 2.0, MaxJump: 12, Offset: 130000}

// draftSlotIndex returns the 0-based overall slot of a current-year pick while
// a draft is running.
func (lg *League) draftSlotIndex(year, round, original int) (int, bool) {
	d := lg.Draft
	if d == nil || year != lg.Year || (round != 1 && round != 2) {
		return 0, false
	}
	lg.ensureDraft()
	n := len(d.Order)
	order := d.Order
	if round == 2 {
		order = d.Order2
	}
	for i, o := range order {
		if o == original {
			return (round-1)*n + i, true
		}
	}
	return 0, false
}

// pickUsed reports whether a current-year pick has already been selected.
func (lg *League) pickUsed(year, round, original int) bool {
	k, ok := lg.draftSlotIndex(year, round, original)
	return ok && k < lg.Draft.Made
}

// DraftSlot is the 1-based overall slot of an unused current-year pick during
// the draft (false at any other time).
func (lg *League) DraftSlot(year, round, original int) (int, bool) {
	k, ok := lg.draftSlotIndex(year, round, original)
	if !ok || k < lg.Draft.Made {
		return 0, false
	}
	return k + 1, true
}

// draftTradeProblem refuses players who cannot move on draft night: an
// expiring contract would just walk away.
func (lg *League) draftTradeProblem(sets ...[]*Player) error {
	if lg.Draft == nil {
		return nil
	}
	for _, ps := range sets {
		for _, p := range ps {
			if p.Contract.Years < 1 {
				return fmt.Errorf("%s has an expiring contract and cannot be traded during the draft.", p.Name)
			}
		}
	}
	return nil
}

// EnsureDraftOffer may create a trade-up offer for the user's pick on the
// clock. It is safe to call any number of times: each pick is checked once
// (saved in Draft.OfferChecked) and the draw is hashed from the pick number.
func (lg *League) EnsureDraftOffer() {
	d := lg.Draft
	if d == nil || lg.DraftDone() || lg.DraftOnClock() != lg.UserTeamID || d.OfferChecked == d.Made+1 {
		return
	}
	d.OfferChecked = d.Made + 1
	k := d.Made // 0-based slot on the clock
	user := lg.UserTeamID
	r := NewRNG(mix64(lg.Seed, uint64(lg.Year), DraftTradeTuning.Offset, uint64(k)))
	if !r.Chance(DraftTradeTuning.Chance) {
		return
	}
	orig, round := d.slot(k)
	cf := lg.contendFactors()
	mine := PickOf(lg.Year, round, orig)
	myVal := lg.pickValue(mine, user, cf)
	if myVal <= 0 || len(d.Pool) < 2 {
		return
	}
	for _, tid := range r.Perm(len(lg.Teams)) {
		if tid == user {
			continue
		}
		deal, ok := lg.tradeUpPackage(tid, mine, myVal, cf)
		if !ok {
			continue
		}
		lg.NextProposalID++
		lg.Proposals = append(lg.Proposals, Proposal{ID: lg.NextProposalID, DraftPick: k + 1, Team: tid, Deal: deal,
			Day: lg.currentDay(), Expires: lg.currentDay() + 1,
			Message: fmt.Sprintf("There is a prospect still on the board we like. We want to move up to pick #%d and will send you picks for it.", k+1)})
		return
	}
}

// likesToMoveUp says whether team tid has a clear reason to trade up from its
// next pick to slot k: its favourite prospect (by its own view) is clearly
// better than the one it expects to still have at its own pick.
func (lg *League) likesToMoveUp(tid, k int) bool {
	d := lg.Draft
	later := -1
	for _, p := range lg.PicksOwnedBy(tid) {
		if i, ok := lg.draftSlotIndex(p.Year, p.Round, p.Original); ok && i > k && (later < 0 || i < later) {
			later = i
		}
	}
	if later < 0 || later-k > DraftTradeTuning.MaxJump {
		return false
	}
	s := make([]float64, len(d.Pool))
	for i, p := range d.Pool {
		s[i] = lg.computerDraftScore(tid, p)
	}
	sort.Sort(sort.Reverse(sort.Float64Slice(s)))
	return s[0]-s[min(later-k, len(s)-1)] >= DraftTradeTuning.MinGain
}

// tradeUpPackage finds picks team tid would send for the user's pick: up to
// three, worth between 95% and 135% of it on the user's own valuation, that
// the team accepts. The fewest assets wins, then the closest to 110%.
func (lg *League) tradeUpPackage(tid int, mine Asset, myVal float64, cf []float64) (Deal, bool) {
	user := lg.UserTeamID
	if !lg.likesToMoveUp(tid, lg.Draft.Made) {
		return Deal{}, false
	}
	picks := lg.PicksOwnedBy(tid)
	type cand struct {
		deal  Deal
		n     int
		ratio float64
	}
	var found []cand
	try := func(ps ...DraftPick) {
		sum := 0.0
		moves := []Move{{user, tid, mine}}
		for _, p := range ps {
			a := PickOf(p.Year, p.Round, p.Original)
			sum += lg.pickValue(a, user, cf)
			moves = append(moves, Move{tid, user, a})
		}
		ratio := sum / myVal
		if ratio < 0.95 || ratio > 1.35 {
			return
		}
		d := Deal{Moves: moves}
		if v := evaluateDealBase(lg, d, user); !v.Valid || !v.Accept {
			return
		}
		found = append(found, cand{d, len(ps), ratio})
	}
	for i := range picks {
		try(picks[i])
		for j := i + 1; j < len(picks); j++ {
			try(picks[i], picks[j])
			for k := j + 1; k < len(picks); k++ {
				try(picks[i], picks[j], picks[k])
			}
		}
	}
	if len(found) == 0 {
		return Deal{}, false
	}
	sort.SliceStable(found, func(a, b int) bool {
		if found[a].n != found[b].n {
			return found[a].n < found[b].n
		}
		return math.Abs(found[a].ratio-1.1) < math.Abs(found[b].ratio-1.1)
	})
	return found[0].deal, true
}
