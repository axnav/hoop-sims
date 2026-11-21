package engine

import (
	"fmt"
	"sort"
	"strings"
)

// Find an upgrade: for one of the user's players, list better players at his
// position on computer teams, say which ones their team is shopping, and find
// the cheapest package that includes the user's player and that the other team
// accepts. Rules only: read-only, no RNG, and the hidden Impact is never read
// (visible overall and the same valuation WhatWouldItTake uses).
//
// The package search is WhatWouldItTake's (takeSearch) with the user's player
// required in every package, so every package returned passes the same salary
// rules, value test and evaluateDealBase that loading it later will.

// UpgradeTuning holds the numbers behind FindUpgrades. Edit here.
var UpgradeTuning = struct {
	// MinGain: a candidate's visible overall must beat the player's by this much.
	MinGain int
	// MaxCandidates: most candidates listed (each one gets a full package search).
	MaxCandidates int
	// MaxInjuryDays: a candidate out for more games than this is left out
	// (injuries count games, not days).
	MaxInjuryDays int
	// ShopBonus: points added to the gain when ranking candidates for the cut
	// to MaxCandidates, for a player the other team is shopping or selling.
	ShopBonus float64
}{
	MinGain: 2, MaxCandidates: 12, MaxInjuryDays: 20, ShopBonus: 2,
}

// Upgrade is one better player on a computer team.
type Upgrade struct {
	Player     *Player
	Team       int
	Direction  TeamStatus
	Gain       int // his visible overall minus the user's player's
	Shopped    bool
	ShopReason string
	Possible   bool
	Reason     string       // set when Possible is false
	Package    *TakePackage // set when Possible is true
}

// UpgradeResult is the answer of FindUpgrades.
type UpgradeResult struct {
	Player   *Player
	Possible bool   // false when trades are closed (Reason says so)
	Reason   string // why not possible, or why the list is empty
	Upgrades []Upgrade
}

// FindUpgrades looks for upgrades over one of the user's players. The error is
// for a player who is not on the user's team.
func (lg *League) FindUpgrades(playerID int) (UpgradeResult, error) {
	user := lg.UserTeamID
	p := lg.findRosterPlayer(user, playerID)
	if p == nil {
		return UpgradeResult{}, fmt.Errorf("That player is not on your team.")
	}
	res := UpgradeResult{Player: p, Upgrades: []Upgrade{}}
	if ok, why := TradeWindowOpen(lg); !ok {
		res.Reason = why
		return res, nil
	}
	if lg.Draft != nil && p.Contract.Years < 1 {
		res.Reason = p.Name + " has an expiring contract and cannot be traded during the draft."
		return res, nil
	}
	res.Possible = true
	t := &UpgradeTuning
	cf := lg.contendFactors()

	// advice for each computer team, computed once and only when needed
	type shopInfo struct {
		shopped bool
		reason  string
	}
	shop := map[int]map[int]shopInfo{}
	shopOf := func(q *Player) shopInfo {
		m, ok := shop[q.TeamID]
		if !ok {
			m = map[int]shopInfo{}
			rows, _ := lg.advisorFor(q.TeamID)
			for _, r := range rows {
				si := shopInfo{shopped: r.Advice != AdviceKeep}
				if si.shopped {
					// the strongest reason to move him; the advisor writes "your",
					// but here the team is theirs
					si.reason = "They would move him for the right offer."
					if len(r.Against) > 0 {
						si.reason = theirWords(r.Against[0])
					}
				}
				m[r.Player.ID] = si
			}
			shop[q.TeamID] = m
		}
		return m[q.ID]
	}

	type cand struct {
		p     *Player
		gain  int
		score float64
		si    shopInfo
	}
	var cands []cand
	for tid, tm := range lg.Teams {
		if tid == user {
			continue
		}
		for _, q := range tm.Roster {
			gain := q.Overall() - p.Overall()
			if gain < t.MinGain || !q.CanPlay(p.NatPos()) {
				continue
			}
			if q.Injury != nil && q.Injury.GamesLeft > t.MaxInjuryDays {
				continue
			}
			c := cand{p: q, gain: gain, si: shopOf(q)}
			c.score = float64(gain)
			if c.si.shopped {
				c.score += t.ShopBonus
			}
			cands = append(cands, c)
		}
	}
	sort.SliceStable(cands, func(i, j int) bool {
		if cands[i].score != cands[j].score {
			return cands[i].score > cands[j].score
		}
		return cands[i].p.ID < cands[j].p.ID
	})
	if len(cands) > t.MaxCandidates {
		cands = cands[:t.MaxCandidates]
	}

	for _, c := range cands {
		u := Upgrade{Player: c.p, Team: c.p.TeamID, Direction: statusFor(cf[c.p.TeamID]),
			Gain: c.gain, Shopped: c.si.shopped, ShopReason: c.si.reason}
		if lg.Draft != nil && c.p.Contract.Years < 1 {
			u.Reason = c.p.Name + " has an expiring contract and cannot be traded during the draft."
		} else {
			r := lg.takeSearch(TakeResult{Player: c.p, Team: c.p.TeamID}, c.p, p, cf, true)
			if r.Possible && len(r.Packages) > 0 {
				u.Possible = true
				pk := r.Packages[0]
				u.Package = &pk
			} else {
				u.Reason = r.Reason
			}
		}
		res.Upgrades = append(res.Upgrades, u)
	}
	sort.SliceStable(res.Upgrades, func(i, j int) bool {
		a, b := res.Upgrades[i], res.Upgrades[j]
		switch {
		case a.Possible != b.Possible:
			return a.Possible
		case a.Shopped != b.Shopped:
			return a.Shopped
		case a.Gain != b.Gain:
			return a.Gain > b.Gain
		}
		return a.Player.ID < b.Player.ID
	})
	if len(res.Upgrades) == 0 {
		res.Reason = "Nobody at his position on another team is clearly better."
	}
	return res, nil
}

// theirWords turns an advisor reason about "your" team into one about theirs.
func theirWords(s string) string {
	s = strings.ReplaceAll(s, " your ", " their ")
	if strings.HasPrefix(s, "Your ") {
		s = "Their " + s[5:]
	}
	return s
}
