package engine

import (
	"fmt"
	"sort"
)

// The trade finder: the user picks some of his own players and draft picks and
// asks "what would each team give me for these?". It is the mirror of
// WhatWouldItTake. Read-only and random-free: nothing is saved, no RNG, and the
// hidden Impact is never read (only the valuation helpers the assistant uses).
//
// For each computer team it lists packages from that team's roster and picks,
// best value to the user first, and returns the first one that passes the
// salary rules and evaluateDealBase, so loading an offer is always accepted.

// FinderTuning holds the numbers behind FindOffers. Edit here.
var FinderTuning = struct {
	MaxTheirPlayers int     // most players one team gives back
	MaxTheirPicks   int     // most picks one team gives back
	MaxOffers       int     // most offers returned (29 teams in the default league)
	PieceCost       float64 // each asset coming back costs this when ranking (fewer pieces are better)
	MinReturn       float64 // an offer worth less than this share of what the user gives is skipped (not for cap relief)
	PoolPlayers     int     // most of a team's players considered (best by value to the user)
	PoolPicks       int     // most of a team's picks considered
	MaxTries        int     // full deal checks per team before giving up
	MaxSalaryChecks int     // cheap salary checks per team before giving up
}{
	MaxTheirPlayers: 3, MaxTheirPicks: 2, MaxOffers: 29, PieceCost: 0.5, MinReturn: 0.6,
	PoolPlayers: 14, PoolPicks: 8, MaxTries: 40, MaxSalaryChecks: 4000,
}

// Finder "want" keys.
var finderWants = []struct{ Key, Label string }{
	{"any", "Anything"}, {"players", "Players"}, {"picks", "Picks"}, {"young", "Young players"}, {"relief", "Cap relief"},
}

// ValidFindWant says whether key is a known want filter.
func ValidFindWant(key string) bool {
	for _, w := range finderWants {
		if w.Key == key {
			return true
		}
	}
	return false
}

// FindWants lists the want filters (key, label) in display order.
func FindWants() [][2]string {
	var out [][2]string
	for _, w := range finderWants {
		out = append(out, [2]string{w.Key, w.Label})
	}
	return out
}

// FindOffer is what one computer team would give for the user's assets.
type FindOffer struct {
	Team                int     // the computer team
	Receive             []Asset // what the user gets
	Value               float64 // user's own valuation of Receive
	GiveValue           float64 // user's own valuation of what he gives
	SalaryIn, SalaryOut int     // thousands: salary the user takes in / sends out
	Note                string
}

// Deal is the offer as a deal between the user and the team.
func (o FindOffer) Deal(user int, give []Asset) Deal {
	var d Deal
	for _, a := range give {
		d.Moves = append(d.Moves, Move{user, o.Team, a})
	}
	for _, a := range o.Receive {
		d.Moves = append(d.Moves, Move{o.Team, user, a})
	}
	return d
}

// FindPass is a team with no offer, and why.
type FindPass struct {
	Team   int
	Reason string
}

// FindResult is the answer of FindOffers.
type FindResult struct {
	Possible  bool
	Reason    string // set when Possible is false
	GiveValue float64
	Offers    []FindOffer // at most one per team, best first
	Passed    []FindPass
}

type finderCand struct {
	a      Asset
	p      *Player // nil for a pick
	mine   float64 // value to the user
	theirs float64 // value to the team that gives it
	core   bool    // one of the team's top two
	salary int
}

// finderCombo is a package as indexes into the pool (players first, then picks).
type finderCombo struct {
	ps       [3]int
	np       int
	ks       [2]int
	nk       int
	mine     float64
	score    float64
	salaryIn int
}

// FindOffers asks every computer team what it would give for the user's assets.
// want is one of the keys of FindWants. An error is for an unknown want or an
// asset that does not exist or is not the user's; every other refusal is
// Possible=false with a Reason.
func (lg *League) FindOffers(give []Asset, want string) (FindResult, error) {
	if !ValidFindWant(want) {
		return FindResult{}, fmt.Errorf("Unknown filter %q.", want)
	}
	user := lg.UserTeamID
	var givePlayers []*Player
	seen := map[Asset]bool{}
	nPicks := 0
	for _, a := range give {
		if seen[a] {
			return FindResult{}, fmt.Errorf("The same player or pick is listed twice.")
		}
		seen[a] = true
		switch a.Kind {
		case PlayerAsset:
			p := lg.findRosterPlayer(user, a.PlayerID)
			if p == nil {
				return FindResult{}, fmt.Errorf("That player is not on your team.")
			}
			givePlayers = append(givePlayers, p)
		case PickAsset:
			own, ok := lg.PickOwner(a.Year, a.Round, a.Original)
			if !ok || own != user || lg.pickUsed(a.Year, a.Round, a.Original) {
				return FindResult{}, fmt.Errorf("That draft pick is not yours.")
			}
			nPicks++
		default:
			return FindResult{}, fmt.Errorf("Unknown asset.")
		}
	}
	res := FindResult{}
	if ok, why := TradeWindowOpen(lg); !ok {
		res.Reason = why
		return res, nil
	}
	switch {
	case len(give) == 0:
		res.Reason = "Tick the players and picks you want to offer."
		return res, nil
	case len(givePlayers) > MaxTradePlayers:
		res.Reason = fmt.Sprintf("You can offer at most %d players in one deal.", MaxTradePlayers)
		return res, nil
	case nPicks > MaxDealPicks:
		res.Reason = fmt.Sprintf("You can offer at most %d draft picks in one deal.", MaxDealPicks)
		return res, nil
	}
	if lg.Draft != nil {
		for _, p := range givePlayers {
			if p.Contract.Years < 1 {
				res.Reason = p.Name + " has an expiring contract and cannot be traded during the draft."
				return res, nil
			}
		}
	}

	cf := lg.contendFactors()
	t := &FinderTuning
	for _, a := range give {
		if a.Kind == PlayerAsset {
			res.GiveValue += lg.value(lg.findRosterPlayer(user, a.PlayerID), user, cf)
		} else {
			res.GiveValue += lg.pickValue(a, user, cf)
		}
	}
	giveSalary := sumSalary(givePlayers)

	for tid := range lg.Teams {
		if tid == user {
			continue
		}
		offer, why := lg.findForTeam(tid, give, givePlayers, giveSalary, res.GiveValue, want, cf)
		if offer == nil {
			res.Passed = append(res.Passed, FindPass{tid, why})
			continue
		}
		res.Offers = append(res.Offers, *offer)
	}
	sort.SliceStable(res.Offers, func(i, j int) bool {
		a, b := res.Offers[i], res.Offers[j]
		if want == "relief" {
			if sa, sb := a.SalaryOut-a.SalaryIn, b.SalaryOut-b.SalaryIn; sa != sb {
				return sa > sb
			}
		}
		return a.Value > b.Value
	})
	if len(res.Offers) > t.MaxOffers {
		res.Offers = res.Offers[:t.MaxOffers]
	}
	res.Possible = len(res.Offers) > 0
	if !res.Possible {
		res.Reason = "No team would make an offer for these."
	}
	return res, nil
}

// findForTeam returns the best accepted package of team tid, or the reason there is none.
func (lg *League) findForTeam(tid int, give []Asset, givePlayers []*Player, giveSalary int, giveValue float64, want string, cf []float64) (*FindOffer, string) {
	user := lg.UserTeamID
	t := &FinderTuning
	un := lg.untouchables(tid, cf)

	// Value of the user's offer to the team.
	var pickIn float64
	for _, a := range give {
		if a.Kind == PickAsset {
			pickIn += lg.pickValue(a, tid, cf)
		}
	}
	_, getsP, _ := lg.sideEval(tid, nil, givePlayers, cf)
	gets := getsP + pickIn

	// The pool: their players and picks that fit the filter.
	var players, picks []*finderCand
	for _, p := range lg.Teams[tid].Roster {
		if lg.Draft != nil && p.Contract.Years < 1 {
			continue
		}
		if want == "picks" || (want == "young" && p.Age > AssistTuning.YoungAge) {
			continue
		}
		players = append(players, &finderCand{a: PlayerOf(p.ID), p: p, mine: lg.value(p, user, cf),
			theirs: lg.value(p, tid, cf), core: un[p.ID], salary: p.Contract.counted()})
	}
	if want != "players" {
		for _, k := range lg.PicksOwnedBy(tid) {
			a := PickOf(k.Year, k.Round, k.Original)
			picks = append(picks, &finderCand{a: a, mine: lg.pickValue(a, user, cf), theirs: lg.pickValue(a, tid, cf)})
		}
	}
	trim := func(cs []*finderCand, n int) []*finderCand {
		sort.SliceStable(cs, func(i, j int) bool { return cs[i].mine > cs[j].mine })
		if len(cs) > n {
			cs = cs[:n]
		}
		return cs
	}
	players, picks = trim(players, t.PoolPlayers), trim(picks, t.PoolPicks)
	if len(players)+len(picks) == 0 {
		return nil, "Nothing to offer that fits your filter."
	}
	pool := append(append([]*finderCand(nil), players...), picks...)
	np := len(players)

	// Enumerate.
	var pickSets [][]int
	pickSets = append(pickSets, nil)
	for i := range picks {
		pickSets = append(pickSets, []int{np + i})
		if t.MaxTheirPicks >= 2 {
			for j := i + 1; j < len(picks); j++ {
				pickSets = append(pickSets, []int{np + i, np + j})
			}
		}
	}
	var playerSets [][]int
	playerSets = append(playerSets, nil)
	for i := 0; i < np; i++ {
		playerSets = append(playerSets, []int{i})
		if t.MaxTheirPlayers < 2 {
			continue
		}
		for j := i + 1; j < np; j++ {
			playerSets = append(playerSets, []int{i, j})
			if t.MaxTheirPlayers < 3 {
				continue
			}
			for k := j + 1; k < np; k++ {
				playerSets = append(playerSets, []int{i, j, k})
			}
		}
	}
	relief := want == "relief"
	young := want == "young"
	baseNeed, coreNeed := 1+TradeMargin, 1+UntouchableMargin
	var combos []finderCombo
	valueOK := false
	for _, ps := range playerSets {
		var pm, pt float64
		var pSal int
		core := false
		for _, i := range ps {
			pm += pool[i].mine
			pt += pool[i].theirs
			pSal += pool[i].salary
			core = core || pool[i].core
		}
		for _, ks := range pickSets {
			n := len(ps) + len(ks)
			if n == 0 {
				continue
			}
			if want == "players" && len(ps) == 0 || want == "picks" && len(ps) > 0 {
				continue
			}
			if young && len(ps) == 0 && len(ks) == 0 {
				continue
			}
			m, th := pm, pt
			for _, i := range ks {
				m += pool[i].mine
				th += pool[i].theirs
			}
			need := baseNeed
			if core {
				need = coreNeed
			}
			if th*need > gets { // necessary: position penalty and roster only make it harder
				continue
			}
			valueOK = true
			if relief {
				if pSal >= giveSalary {
					continue
				}
			} else if m < t.MinReturn*giveValue {
				continue
			}
			c := finderCombo{np: len(ps), nk: len(ks), mine: m, salaryIn: pSal}
			copy(c.ps[:], ps)
			copy(c.ks[:], ks)
			c.score = m - t.PieceCost*float64(n)
			if relief {
				c.score = float64(giveSalary-pSal)*1000 + c.score // saved salary first, value breaks ties
			}
			combos = append(combos, c)
		}
	}
	if len(combos) == 0 {
		switch {
		case !valueOK:
			return nil, "Does not value your offer enough."
		case relief:
			return nil, "Nothing to offer that saves you salary."
		}
		return nil, "Nothing to offer that is worth enough to you."
	}
	sort.SliceStable(combos, func(i, j int) bool { return combos[i].score > combos[j].score })

	salaryFail, tries := false, 0
	for ci, c := range combos {
		if ci >= t.MaxSalaryChecks || tries >= t.MaxTries {
			break
		}
		var out []*Player
		var recv []Asset
		for _, i := range c.ps[:c.np] {
			out = append(out, pool[i].p)
			recv = append(recv, pool[i].a)
		}
		for _, i := range c.ks[:c.nk] {
			recv = append(recv, pool[i].a)
		}
		if lg.salaryProblem(user, givePlayers, out) != nil || lg.salaryProblem(tid, out, givePlayers) != nil {
			salaryFail = true
			continue
		}
		tries++
		offer := FindOffer{Team: tid, Receive: recv}
		if v := evaluateDealBase(lg, offer.Deal(user, give), user); !v.Valid || !v.Accept {
			continue
		}
		offer.Value = c.mine
		offer.GiveValue = giveValue
		offer.SalaryIn, offer.SalaryOut = c.salaryIn, giveSalary
		offer.Note = lg.findNote(tid, givePlayers, give, out, recv)
		return &offer, ""
	}
	if salaryFail && tries == 0 {
		return nil, "Cannot match the salaries."
	}
	return nil, "Does not value your offer enough."
}

// findNote is one plain sentence about why the team makes this offer.
func (lg *League) findNote(tid int, givePlayers []*Player, give []Asset, out []*Player, recv []Asset) string {
	key := lg.TeamStatusOf(tid).Key
	rebuild := key == "rebuilding" || key == "seller"
	vet, young, picks := false, false, false
	for _, p := range givePlayers {
		switch {
		case p.Age >= AssistTuning.VeteranAge && p.Overall() >= tierRotationOvr:
			vet = true
		case p.Age <= AssistTuning.YoungAge && p.Potential > p.Overall():
			young = true
		}
	}
	for _, a := range give {
		if a.Kind == PickAsset {
			picks = true
		}
	}
	switch {
	case !rebuild && vet:
		return "They value your veteran for a playoff push."
	case rebuild && young:
		return "They value your young player with upside."
	case rebuild && picks:
		return "They value your draft pick."
	case len(out) == 0:
		return "They pay in draft picks."
	}
	allPicks := true
	for _, a := range recv {
		allPicks = allPicks && a.Kind == PickAsset
	}
	if allPicks {
		return "They pay in draft picks."
	}
	return "Fair value for both sides."
}
