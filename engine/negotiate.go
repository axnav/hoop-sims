package engine

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// Tier thresholds, from the visible rating distribution of a generated league
// (about 390 players: p62 = 66, p90 = 77, max 99). Only visible ratings.
const (
	tierFranchiseOvr = 90 // a handful per league
	tierStarOvr      = 77 // about the top 10%
	tierStarterOvr   = 66 // about five per team
	tierRotationOvr  = 57 // the next three or so per team
	// Prospect: young, rated low now, visible potential well above it.
	tierProspectMaxAge  = 22
	tierProspectMaxOvr  = 62
	tierProspectPotGap  = 10
	tierProspectMinPot  = 68
	pickLotterySlots    = 14 // weakest 14 original teams: lottery
	pickMidFirstSlotEnd = 23 // slots 14..22 mid first, the rest late first
)

// PlayerTier is a plain-language label from visible information only.
func PlayerTier(p *Player) string {
	ovr := p.Overall()
	switch {
	case ovr >= tierFranchiseOvr:
		return "Franchise"
	case ovr >= tierStarOvr:
		return "Star"
	case ovr >= tierStarterOvr:
		return "Starter"
	case p.Age <= tierProspectMaxAge && ovr < tierProspectMaxOvr &&
		p.Potential >= ovr+tierProspectPotGap && p.Potential >= tierProspectMinPot:
		return "Prospect"
	case ovr >= tierRotationOvr:
		return "Rotation"
	}
	return "Bench"
}

// PickTier labels a draft pick by the expected slot of its original team.
func (lg *League) PickTier(year, round, original int) string {
	if round != 1 {
		return "Second-rounder"
	}
	if original < 0 || original >= len(lg.Teams) {
		return "Late first"
	}
	cf := lg.contendFactors()
	slot := int(math.Round(cf[original] * float64(len(lg.Teams)-1))) // 0 = weakest
	switch {
	case slot < pickLotterySlots:
		return "Lottery pick"
	case slot < pickMidFirstSlotEnd:
		return "Mid first"
	}
	return "Late first"
}

// Negotiation and proposal tunables.
const (
	CloseFraction        = 0.85 // "close": team gets at least this share of what it needs
	noFraction           = 0.5  // below this, a refused core player is a flat "no"
	ProposalChancePerDay = 1.0 / 8
	MaxPendingProposals  = 2
	ProposalLifeDays     = 6
	proposalBandLo       = 0.90 // user gets at least this share of what the user gives
	proposalBandHi       = 1.15
	proposalOffset       = 20000 // RNG stream, apart from days, playoffs, offseason
	proposalMaxFound     = 60
)

// Options are league settings. The zero value is the default behaviour.
type Options struct {
	NoDeadlineStop bool // true: multi-day sims do not pause before the deadline
	NoOfferStop    bool // true: multi-day sims do not pause when an offer arrives
	NoInjuries     bool // true: nobody gets hurt (existing injuries still heal)
	Commissioner   bool // true: EditPlayer is allowed (commissioner.go)
}

// Proposal is a trade offer from a computer team to the user.
type Proposal struct {
	ID      int
	Team    int // the proposing team
	Deal    Deal
	Day     int // season day it was made (days played)
	Expires int // dropped once this many days have been played
	Message string
}

func (lg *League) userValue(a Asset, user int, cf []float64) float64 {
	if a.Kind == PickAsset {
		return lg.pickValue(a, user, cf)
	}
	if p := lg.findRosterPlayer(user, a.PlayerID); p != nil {
		return lg.value(p, user, cf)
	}
	return 0
}

func (lg *League) askSentence(a Asset, user int) string {
	if a.Kind == PickAsset {
		rd := "first"
		if a.Round == 2 {
			rd = "second"
		}
		if a.Original == user {
			return fmt.Sprintf("Add your %d %s-round pick and we have a deal.", a.Year, rd)
		}
		return fmt.Sprintf("Add the %d %s-round pick from %s and we have a deal.", a.Year, rd, lg.Teams[a.Original].FullName())
	}
	return fmt.Sprintf("Add %s and we have a deal.", lg.assetName(a))
}

// findAsk returns the lowest-value (to the user) single extra user asset that
// makes team tid accept.
func (lg *League) findAsk(tid int, deal Deal, user int, cf []float64) (Asset, bool) {
	inDeal := map[Asset]bool{}
	for _, m := range deal.Moves {
		inDeal[m.Asset] = true
	}
	var cands []Asset
	for _, p := range lg.Teams[user].Roster {
		cands = append(cands, PlayerOf(p.ID))
	}
	for _, k := range lg.PicksOwnedBy(user) {
		cands = append(cands, PickOf(k.Year, k.Round, k.Original))
	}
	var best Asset
	bestVal, found := 0.0, false
	for _, a := range cands {
		if inDeal[a] {
			continue
		}
		val := lg.userValue(a, user, cf)
		if found && val >= bestVal {
			continue
		}
		nd := Deal{Moves: append(append([]Move(nil), deal.Moves...), Move{user, tid, a})}
		v := evaluateDealBase(lg, nd, user)
		if !v.Valid {
			continue
		}
		for _, t := range v.Teams {
			if t.Team == tid && t.Accept {
				best, bestVal, found = a, val, true
			}
		}
	}
	return best, found
}

// negotiate fills Mood, Ask and AskText on one team's verdict.
func (lg *League) negotiate(tv *DealTeamVerdict, deal Deal, user int, cf []float64) {
	if tv.Accept {
		tv.Mood = "accept"
		return
	}
	ratio := 0.0
	if tv.Gives*tv.Needed > 0 {
		ratio = tv.Gets / (tv.Gives * tv.Needed)
	}
	tv.Mood = "far"
	if ratio >= CloseFraction {
		tv.Mood = "close"
	}
	if a, ok := lg.findAsk(tv.Team, deal, user, cf); ok {
		tv.Ask = &a
		tv.AskText = lg.askSentence(a, user)
		return
	}
	un := lg.untouchables(tv.Team, cf)
	for _, m := range deal.Moves {
		if m.From == tv.Team && m.Kind == PlayerAsset && un[m.PlayerID] && ratio < noFraction {
			tv.Mood = "no"
			tv.AskText = fmt.Sprintf("We will not move %s at any reasonable price; ask about someone else.", lg.findRosterPlayer(tv.Team, m.PlayerID).Name)
			return
		}
	}
	short := tv.Gives*tv.Needed - tv.Gets
	switch {
	case short >= 25:
		tv.AskText = "No single piece will do it: we would want a star-level player back, or a top first-round pick plus more."
	case short >= 8:
		tv.AskText = "No single piece will do it: we want a first-round pick or a starter-level player back, and probably more than one."
	default:
		tv.AskText = "No single piece will do it: we would want a couple of smaller assets, such as second-round picks or bench players."
	}
}

// ---- proposals from computer teams ----

func (lg *League) dropBadProposals() {
	open, _ := TradeWindowOpen(lg)
	day := lg.currentDay()
	var kept []Proposal
	for _, p := range lg.Proposals {
		if open && day < p.Expires && ValidateDeal(lg, p.Deal, lg.UserTeamID) == nil {
			kept = append(kept, p)
		}
	}
	lg.Proposals = kept
}

// updateProposals runs at the end of each SimDay: drops stale proposals and
// may create one new one.
func (lg *League) updateProposals() {
	s := lg.Season
	lg.dropBadProposals()
	if open, _ := TradeWindowOpen(lg); !open || s.Day >= TradeDeadlineDay || len(lg.Proposals) >= MaxPendingProposals {
		return
	}
	r := lg.dayRNG(s.Year, proposalOffset+s.Day)
	if !r.Chance(ProposalChancePerDay) {
		return
	}
	cf := lg.contendFactors()
	for _, tid := range r.Perm(len(lg.Teams)) {
		if tid == lg.UserTeamID {
			continue
		}
		deal, msg, ok := lg.buildProposal(tid, cf, r)
		if !ok {
			continue
		}
		lg.NextProposalID++
		lg.Proposals = append(lg.Proposals, Proposal{ID: lg.NextProposalID, Team: tid, Deal: deal,
			Day: s.Day, Expires: s.Day + ProposalLifeDays, Message: msg})
		lg.newProposals++
		return
	}
}

func (lg *League) shortAsset(a Asset, owner int) string {
	if a.Kind == PlayerAsset {
		if p := lg.findRosterPlayer(owner, a.PlayerID); p != nil {
			return p.Name
		}
		return "a player"
	}
	rd := "first"
	if a.Round == 2 {
		rd = "second"
	}
	if a.Original == owner {
		return fmt.Sprintf("our %d %s-round pick", a.Year, rd)
	}
	return fmt.Sprintf("the %d %s-round pick from %s", a.Year, rd, lg.Teams[a.Original].FullName())
}

func joinList(xs []string) string {
	switch len(xs) {
	case 0:
		return ""
	case 1:
		return xs[0]
	}
	return strings.Join(xs[:len(xs)-1], ", ") + " and " + xs[len(xs)-1]
}

func (lg *League) buildProposal(tid int, cf []float64, r *RNG) (Deal, string, bool) {
	user := lg.UserTeamID
	contender := cf[tid] >= 0.5
	var targets []*Player
	for _, p := range lg.Teams[user].Roster {
		if contender && p.Overall() >= 58 {
			targets = append(targets, p)
		} else if !contender && p.Age <= 25 && p.Potential >= 64 {
			targets = append(targets, p)
		}
	}
	sort.SliceStable(targets, func(i, j int) bool {
		a, b := targets[i], targets[j]
		if contender {
			if a.Overall() != b.Overall() {
				return a.Overall() > b.Overall()
			}
		} else {
			if a.Potential != b.Potential {
				return a.Potential > b.Potential
			}
			if a.Age != b.Age {
				return a.Age < b.Age
			}
		}
		return a.ID < b.ID
	})
	if len(targets) > 3 {
		targets = targets[:3]
	}
	for _, target := range targets {
		deal, assets, ok := lg.findPackage(tid, target, cf, r)
		if !ok {
			continue
		}
		var names []string
		for _, a := range assets {
			names = append(names, lg.shortAsset(a, tid))
		}
		var msg string
		if contender {
			msg = fmt.Sprintf("We are pushing for the playoffs and %s would help us right away; we will send %s.", target.Name, joinList(names))
		} else {
			msg = fmt.Sprintf("We are building for the future and love the upside of %s; we will send %s.", target.Name, joinList(names))
		}
		return deal, msg, true
	}
	return Deal{}, "", false
}

// findPackage looks for what team tid can send (at most one player and three
// picks) for target so that tid accepts and the user's side comes out between
// proposalBandLo and proposalBandHi of what the user gives, on the user's valuation.
func (lg *League) findPackage(tid int, target *Player, cf []float64, r *RNG) (Deal, []Asset, bool) {
	user := lg.UserTeamID
	un := lg.untouchables(tid, cf)
	var ps []*Player
	for _, p := range lg.Teams[tid].Roster {
		if !un[p.ID] {
			ps = append(ps, p)
		}
	}
	picks := lg.PicksOwnedBy(tid)
	var combos [][]DraftPick
	combos = append(combos, nil)
	for i := range picks {
		combos = append(combos, []DraftPick{picks[i]})
		for j := i + 1; j < len(picks); j++ {
			combos = append(combos, []DraftPick{picks[i], picks[j]})
			for k := j + 1; k < len(picks); k++ {
				combos = append(combos, []DraftPick{picks[i], picks[j], picks[k]})
			}
		}
	}
	give := lg.value(target, user, cf)
	type cand struct {
		deal   Deal
		assets []Asset
		ratio  float64
	}
	var found []cand
	for pi := -1; pi < len(ps) && len(found) < proposalMaxFound; pi++ {
		var pl *Player
		if pi >= 0 {
			pl = ps[pi]
		}
		for _, pk := range combos {
			if len(found) >= proposalMaxFound {
				break
			}
			if pl == nil && len(pk) == 0 {
				continue
			}
			moves := []Move{{user, tid, PlayerOf(target.ID)}}
			var assets []Asset
			var in []*Player
			cheap, pickV := 0.0, 0.0
			if pl != nil {
				moves = append(moves, Move{tid, user, PlayerOf(pl.ID)})
				assets = append(assets, PlayerOf(pl.ID))
				in = []*Player{pl}
				cheap += lg.value(pl, user, cf)
			}
			for _, k := range pk {
				a := PickOf(k.Year, k.Round, k.Original)
				moves = append(moves, Move{tid, user, a})
				assets = append(assets, a)
				v := lg.pickValue(a, user, cf)
				pickV += v
				cheap += v
			}
			if cheap/give < proposalBandLo-0.1 || cheap/give > proposalBandHi+0.1 {
				continue // cheap prefilter, with slack for the position penalty
			}
			g, gt, _ := lg.sideEval(user, []*Player{target}, in, cf)
			ratio := (gt + pickV) / g
			if ratio < proposalBandLo || ratio > proposalBandHi {
				continue
			}
			d := Deal{Moves: moves}
			if v := evaluateDealBase(lg, d, user); !v.Valid || !v.Accept {
				continue
			}
			found = append(found, cand{d, assets, ratio})
		}
	}
	if len(found) == 0 {
		return Deal{}, nil, false
	}
	sort.SliceStable(found, func(i, j int) bool {
		if len(found[i].assets) != len(found[j].assets) {
			return len(found[i].assets) < len(found[j].assets)
		}
		return math.Abs(found[i].ratio-1.02) < math.Abs(found[j].ratio-1.02)
	})
	c := found[r.IntN(min(3, len(found)))]
	return c.deal, c.assets, true
}

// PendingProposals lists the proposals waiting for the user, oldest first.
func (lg *League) PendingProposals() []Proposal {
	return append([]Proposal(nil), lg.Proposals...)
}

// ProposalsCreatedLastDay is how many proposals the most recent SimDay created
// (so a multi-day loop can stop when it is above zero).
func (lg *League) ProposalsCreatedLastDay() int { return lg.newProposals }

func (lg *League) proposalIndex(id int) int {
	for i, p := range lg.Proposals {
		if p.ID == id {
			return i
		}
	}
	return -1
}

// AcceptProposal re-validates and executes a proposal, then removes it.
func (lg *League) AcceptProposal(id int) (TradeResult, error) {
	i := lg.proposalIndex(id)
	if i < 0 {
		return TradeResult{}, fmt.Errorf("That offer is no longer available.")
	}
	p := lg.Proposals[i]
	if lg.currentDay() >= p.Expires {
		lg.dropBadProposals()
		return TradeResult{}, fmt.Errorf("That offer has expired.")
	}
	res, err := ExecuteDeal(lg, p.Deal, lg.UserTeamID)
	if err != nil {
		lg.dropBadProposals()
		return TradeResult{}, err
	}
	lg.Proposals = append(lg.Proposals[:i:i], lg.Proposals[i+1:]...)
	lg.dropBadProposals()
	return res, nil
}

// RejectProposal removes a proposal; false if there is no such ID.
func (lg *League) RejectProposal(id int) bool {
	i := lg.proposalIndex(id)
	if i < 0 {
		return false
	}
	lg.Proposals = append(lg.Proposals[:i:i], lg.Proposals[i+1:]...)
	return true
}

// ShouldPauseForDeadline reports true once per season, when the next day to
// be played is the last one with the trade window open (days played ==
// TradeDeadlineDay) and Options.NoDeadlineStop is off. It records that it
// fired, so call it only when you really intend to pause.
func (lg *League) ShouldPauseForDeadline() bool {
	s := lg.Season
	if lg.Options.NoDeadlineStop || s == nil || s.Year != lg.Year || s.PlayoffsStarted || s.Done() {
		return false
	}
	if s.Day != TradeDeadlineDay || lg.DeadlineStopYear == lg.Year {
		return false
	}
	lg.DeadlineStopYear = lg.Year
	return true
}
