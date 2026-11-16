package engine

import (
	"errors"
	"fmt"
	"sort"
)

// Player motivations, stance on re-signing and contract extensions.
//
// Motivations are derived (nothing saved) from the league seed and the
// player's ID. The stance is derived from the current roster, so it moves as
// the team changes.

// Level is how strongly a player cares about one thing.
type Level int

const (
	Low Level = iota
	Medium
	High
)

func (l Level) String() string { return [...]string{"Low", "Medium", "High"}[l] }

// Motivation is what a player cares about in a team.
type Motivation struct{ Money, Winning, Role, Loyalty Level }

// motivationOffset is the RNG stream: one hashed stream per player (90000 is
// the next free ten-thousand; see RULES.md section 3).
const motivationOffset = 90000

// MotivationTuning holds the numbers behind motivations and stances.
var MotivationTuning = struct {
	LowShare, HighShare float64    // share of players with a Low / High level per trait (rest Medium)
	TraitWeight         [3]float64 // how much a Low / Medium / High trait counts in a stance score
	FAWeight            [3]float64 // the same for the free-agent appeal parts (Low 0.75, Medium 1, High 1.3)
	LoyaltyBonus        [3]float64 // free-agent appeal bonus (fraction) for his previous team, by Loyalty

	Base       float64 // everyone starts a little inclined to stay
	StarterPts float64 // top-five player
	BenchPts   float64 // ninth man or lower (negative)
	ShortPts   float64 // plays at least ShortMin fewer minutes than his rank deserves (negative)
	ShortMin   int
	ContendPts float64 // team direction points: Contender, Buyer, Seller, Rebuilding
	BuyerPts   float64
	SellerPts  float64
	RebuildPts float64
	MoneyPts   float64 // negative: his market value is more than the team can pay
	TenurePts  float64 // loyalty points at LoyaltyYears with the team (scaled between)
	TenureFrom int     // years with the team where tenure points are zero
	TenureFull int     // years where they are at their maximum

	ResignAt  float64 // score at or above: "resign"
	TestBelow float64 // score below: "test"
}{
	LowShare: 0.25, HighShare: 0.25,
	TraitWeight:  [3]float64{0.5, 1, 1.6},
	FAWeight:     [3]float64{0.75, 1, 1.3},
	LoyaltyBonus: [3]float64{0, 0.02, 0.05},
	Base:         0.35, StarterPts: 0.35, BenchPts: -0.5, ShortPts: -0.5, ShortMin: 6,
	ContendPts: 0.5, BuyerPts: 0.2, SellerPts: -0.2, RebuildPts: -0.6,
	MoneyPts: -0.4, TenurePts: 0.6, TenureFrom: 2, TenureFull: 8,
	ResignAt: 0.5, TestBelow: -0.15,
}

// MotivationOf is the player's motivations, the same in every call.
func (lg *League) MotivationOf(p *Player) Motivation {
	r := NewRNG(mix64(lg.Seed, uint64(motivationOffset), uint64(p.ID)))
	t := &MotivationTuning
	pick := func() Level {
		x := r.Float64()
		switch {
		case x < t.LowShare:
			return Low
		case x >= 1-t.HighShare:
			return High
		}
		return Medium
	}
	return Motivation{Money: pick(), Winning: pick(), Role: pick(), Loyalty: pick()}
}

// rankOnTeam is the player's place on his roster by overall (0 = best; ties:
// lower ID first).
func rankOnTeam(tm *Team, p *Player) int {
	order := append([]*Player(nil), tm.Roster...)
	sort.SliceStable(order, func(i, j int) bool {
		if a, b := order[i].Overall(), order[j].Overall(); a != b {
			return a > b
		}
		return order[i].ID < order[j].ID
	})
	rank := 0
	for i, q := range order {
		if q == p {
			rank = i
		}
	}
	return rank
}

// rolePts is the role score of a player on a team: points for being a
// starter, minus for a deep bench place or for fewer minutes than his rank
// deserves (an injured player gets no minutes penalty). short says the
// minutes shortfall applied. pms are the team's effective minutes (nil: they
// are worked out here). The one copy of the logic, used by the stance and by
// morale.
func (lg *League) rolePts(tm *Team, p *Player, pms []PlayerMinutes) (pts float64, reason string, short bool) {
	t := &MotivationTuning
	rank := rankOnTeam(tm, p)
	switch {
	case rank < 5:
		pts, reason = t.StarterPts, "Is a starter"
	case rank >= 8:
		pts, reason = t.BenchPts, "Wants a bigger role"
	}
	if rank < len(minutesByRank) && !p.Injured() {
		if pms == nil {
			pms = tm.EffectiveMinutes()
		}
		got := 0
		for _, pm := range pms {
			if pm.Player == p {
				got = pm.Min
			}
		}
		if minutesByRank[rank]-got >= t.ShortMin {
			pts, reason, short = pts+t.ShortPts, "Wants a bigger role", true
		}
	}
	return
}

// Stance is a player's attitude to re-signing with his team.
type Stance struct {
	Key, Label string
	Reasons    []string
}

var errTestsFA = errors.New("He wants to test free agency.")

// StanceOf is the stance of a player on a team (anyone else gets "open" with
// no reasons). Deterministic: no randomness beyond MotivationOf.
func (lg *League) StanceOf(p *Player) Stance {
	t := &MotivationTuning
	open := Stance{"open", "Open to it", nil}
	if p == nil || p.TeamID < 0 || p.TeamID >= len(lg.Teams) {
		return open
	}
	tm := lg.Teams[p.TeamID]
	m := lg.MotivationOf(p)
	w := func(l Level) float64 { return t.TraitWeight[l] }

	type part struct {
		pts    float64
		reason string
	}
	var parts []part
	add := func(pts float64, reason string) {
		if reason != "" {
			parts = append(parts, part{pts, reason})
		}
	}

	// Role: rank by overall on the roster, against the minutes he gets.
	pts, reason, _ := lg.rolePts(tm, p, nil)
	add(pts*w(m.Role), reason)

	// Team direction.
	switch lg.TeamStatusOf(tm.ID).Key {
	case "contender":
		add(t.ContendPts*w(m.Winning), "Likes that the team is contending")
	case "buyer":
		add(t.BuyerPts*w(m.Winning), "Likes that the team is in the playoff race")
	case "seller":
		add(t.SellerPts*w(m.Winning), "Worries the team is falling out of the race")
	default:
		add(t.RebuildPts*w(m.Winning), "Does not want to wait through a rebuild")
	}

	// Years with the team (0 = unknown = neutral).
	if p.TeamSince > 0 {
		yrs := max(0, lg.Year-p.TeamSince)
		span := float64(t.TenureFull - t.TenureFrom)
		f := float64(min(yrs, t.TenureFull)-t.TenureFrom) / span
		if f > 1 {
			f = 1
		}
		r := ""
		switch {
		case yrs >= t.TenureFrom+1:
			r = fmt.Sprintf("Has been here %d years", yrs)
		case yrs == 0:
			r = "Just arrived"
		}
		add(t.TenurePts*f*w(m.Loyalty), r)
	}

	// Money: can the team pay his market value?
	if lg.Payroll(tm.ID)-p.Contract.counted()+MarketSalary(p) > Cap.TaxLine {
		add(t.MoneyPts*w(m.Money), "The team may not be able to pay what he can get")
	}

	// Morale: an unhappy player leans toward leaving, a happy one toward staying.
	if pts, reason := moraleStancePts(p); pts != 0 {
		add(pts, reason)
	}

	score := t.Base
	for _, x := range parts {
		score += x.pts
	}
	key, label := "open", "Open to it"
	switch {
	case score >= t.ResignAt:
		key, label = "resign", "Will re-sign"
	case score < t.TestBelow:
		key, label = "test", "Will test free agency"
	}
	// Reasons: the biggest pulls in the direction of the verdict first; at
	// most three.
	sort.SliceStable(parts, func(i, j int) bool {
		a, b := parts[i].pts, parts[j].pts
		if score < t.ResignAt {
			return a < b
		}
		return a > b
	})
	var rs []string
	for _, x := range parts {
		if len(rs) < 3 && x.pts != 0 {
			rs = append(rs, x.reason)
		}
	}
	return Stance{key, label, rs}
}

// ExtensionAsk is what the player wants to extend, and the longest deal he
// accepts. Allowed for a player on the team in his last contract year (one
// year left, no extension yet) during the season (playoffs included) or the
// offseason before free agency starts; refused for a player who will test
// free agency or whose ask is more than ExtensionMaxRaise times his salary.
func (lg *League) ExtensionAsk(teamID, playerID int) (salary, maxYears int, err error) {
	if teamID < 0 || teamID >= len(lg.Teams) {
		return 0, 0, fmt.Errorf("That is not a valid team.")
	}
	p := lg.findRosterPlayer(teamID, playerID)
	if p == nil {
		return 0, 0, fmt.Errorf("That player is not on your team.")
	}
	if lg.FA != nil {
		return 0, 0, fmt.Errorf("Free agency has begun, so extensions are closed.")
	}
	if p.Contract.NextYears > 0 {
		return 0, 0, fmt.Errorf("%s has already agreed to an extension.", p.Name)
	}
	if p.Contract.Years != 1 {
		return 0, 0, fmt.Errorf("He can only extend in the last year of his contract.")
	}
	st := lg.StanceOf(p)
	if st.Key == "test" {
		return 0, 0, errTestsFA
	}
	salary = MarketSalary(p)
	if f := moraleAskFactor(p); f != 1 {
		salary = min(Cap.MaxSalary, int(float64(salary)*f/100+0.5)*100)
	}
	if st.Key == "open" {
		salary = min(Cap.MaxSalary, int(float64(salary)*Cap.ExtensionOpenMult/100+0.5)*100)
	}
	if salary > lg.extensionLimit(p) {
		return 0, 0, fmt.Errorf("He wants more than an extension allows (at most %d%% of his salary).", int(Cap.ExtensionMaxRaise*100+0.5))
	}
	return salary, Cap.MaxContractYears, nil
}

// extensionLimit is the largest first-year salary an extension may have.
func (lg *League) extensionLimit(p *Player) int {
	return min(Cap.MaxSalary, int(float64(p.Contract.Salary)*Cap.ExtensionMaxRaise))
}

// ExtensionLimit is the largest salary an extension of this player may have
// (for screens).
func (lg *League) ExtensionLimit(p *Player) int { return lg.extensionLimit(p) }

// Extend agrees an extension that starts when the current contract runs out.
// The salary must be at least his ask (and within the limit); the years 1 to
// Cap.MaxContractYears. Rule for payroll: the team's payroll for next season
// (contracts with more than one year left plus extensions, plus this one)
// must stay at or under the hard ceiling; there is no cap-room test, as with
// re-signing.
func (lg *League) Extend(teamID, playerID, salary, years int) error {
	ask, _, err := lg.ExtensionAsk(teamID, playerID)
	if err != nil {
		return err
	}
	p := lg.findRosterPlayer(teamID, playerID)
	if years < 1 || years > Cap.MaxContractYears {
		return fmt.Errorf("A contract must run 1 to %d years.", Cap.MaxContractYears)
	}
	if salary < ask {
		return fmt.Errorf("He wants at least %s.", fmtM(ask))
	}
	if salary > lg.extensionLimit(p) {
		return fmt.Errorf("An extension can start at most at %s (%d%% of his salary).", fmtM(lg.extensionLimit(p)), int(Cap.ExtensionMaxRaise*100+0.5))
	}
	next := salary
	for _, q := range lg.Teams[teamID].Roster {
		switch {
		case q == p:
		case q.Contract.Years > 1:
			next += q.Contract.Salary
		case q.Contract.NextYears > 0:
			next += q.Contract.NextSalary
		}
	}
	if next > Cap.HardCeiling {
		return fmt.Errorf("Next season's payroll would be %s, over the hard ceiling of %s.", fmtM(next), fmtM(Cap.HardCeiling))
	}
	p.Contract.NextSalary, p.Contract.NextYears = salary, years
	lg.moraleReSigned(p)
	return nil
}
