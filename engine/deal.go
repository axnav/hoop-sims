package engine

import (
	"fmt"
	"math"
	"strings"
)

// Deal limits.
const (
	MaxDealTeams = 3 // teams in one deal
	// MaxDealPicks is the most draft picks one team may send out in a deal.
	// (Players sent out by one team are limited by MaxTradePlayers.)
	MaxDealPicks = 4
)

// Pick value tunables. Values are in the same units as tradeValue: a 60-rated
// player is about 5, a 70 about 20, a 75 about 35, an 80 about 55.
const (
	// Round 1 value runs from pickR1Late (the last pick of the round) up to
	// pickR1Late+pickR1Spread (the first pick), following weakness^pickR1Curve,
	// where weakness is 1 for the weakest original team and 0 for the strongest.
	// This puts picks 1-3 near a strong young starter (about 27-32), a mid
	// first near 11 and a late first near 8 (a rotation player).
	pickR1Late   = 8.0
	pickR1Spread = 24.0
	pickR1Curve  = 3.0
	// Round 2 value runs from pickR2Late to pickR2Late+pickR2Spread, linearly.
	pickR2Late   = 1.0
	pickR2Spread = 1.5
	// Each draft further away is worth this fraction of the one before.
	pickYearDiscount = 0.88
	// The valuing team's contend factor c (0 rebuilding, 1 contending) scales
	// the value by pickTiltBase - pickTiltSlope*c: rebuilders 1.25, contenders 0.75.
	pickTiltBase  = 1.25
	pickTiltSlope = 0.5
)

// AssetKind says what an Asset is.
type AssetKind int

const (
	PlayerAsset AssetKind = iota
	PickAsset
)

// Asset is one tradable thing: a player (PlayerID) or a draft pick (Year,
// Round, Original). Unused fields stay zero so Assets compare and work as map keys.
type Asset struct {
	Kind                  AssetKind
	PlayerID              int
	Year, Round, Original int
}

// PlayerOf and PickOf build assets.
func PlayerOf(id int) Asset { return Asset{Kind: PlayerAsset, PlayerID: id} }
func PickOf(year, round, original int) Asset {
	return Asset{Kind: PickAsset, Year: year, Round: round, Original: original}
}

// Move sends one asset from one team to another.
type Move struct {
	From, To int
	Asset
}

// Deal is a set of moves among 2 to MaxDealTeams teams.
type Deal struct {
	Moves []Move
}

// DealTeamVerdict is one computer team's answer. Gives/Gets are on that team's
// own valuation; Needed is the multiple of Gives that Gets must reach.
type DealTeamVerdict struct {
	Team        int
	Accept      bool
	Reason      string
	Gives, Gets float64
	Needed      float64

	// Negotiation (negotiate.go). Mood is "accept", "close", "far" or "no".
	// When the team does not accept, Ask is the cheapest single extra asset
	// from the user's team that would make it accept (nil if none is enough)
	// and AskText says so in a plain sentence.
	Mood    string
	Ask     *Asset
	AskText string
}

// DealVerdict is the answer to a whole deal. Valid is false (with Problem set)
// if the deal breaks a rule; otherwise Teams has one entry per computer team
// and Accept is true only if all of them accept.
type DealVerdict struct {
	Valid   bool
	Problem string
	Accept  bool
	Teams   []DealTeamVerdict
}

// PickValueFor is the value of a draft pick to a team. yearsAway is 0 for the
// next draft; originalContend is the contend factor (0 weak .. 1 strong) of the
// pick's original team, which sets the expected slot; valuerContend is the
// contend factor of the team doing the valuing.
func PickValueFor(round, yearsAway int, originalContend, valuerContend float64) float64 {
	w := math.Max(0, math.Min(1, 1-originalContend))
	var v float64
	if round == 1 {
		v = pickR1Late + pickR1Spread*math.Pow(w, pickR1Curve)
	} else {
		v = pickR2Late + pickR2Spread*w
	}
	v *= math.Pow(pickYearDiscount, float64(max(0, yearsAway)))
	return v * (pickTiltBase - pickTiltSlope*math.Max(0, math.Min(1, valuerContend)))
}

func (lg *League) pickValue(a Asset, tid int, cf []float64) float64 {
	return PickValueFor(a.Round, a.Year-lg.Year, cf[a.Original], cf[tid])
}

// PickValue is what a draft pick is worth to the given team right now, in the
// same units as TradeValue for players.
func (lg *League) PickValue(year, round, original, valuerTeam int) float64 {
	if original < 0 || original >= len(lg.Teams) || valuerTeam < 0 || valuerTeam >= len(lg.Teams) {
		return 0
	}
	return lg.pickValue(PickOf(year, round, original), valuerTeam, lg.contendFactors())
}

func (lg *League) assetName(a Asset) string {
	if a.Kind == PickAsset {
		rd := "first"
		if a.Round == 2 {
			rd = "second"
		}
		return fmt.Sprintf("the %d %s-round pick from %s", a.Year, rd, lg.Teams[a.Original].FullName())
	}
	for _, t := range lg.Teams {
		for _, p := range t.Roster {
			if p.ID == a.PlayerID {
				return describe(p)
			}
		}
	}
	return "a player"
}

// findRosterPlayer returns the player with that ID on a team, or nil.
func (lg *League) findRosterPlayer(tid, id int) *Player {
	for _, p := range lg.Teams[tid].Roster {
		if p.ID == id {
			return p
		}
	}
	return nil
}

// ValidateDeal checks every rule and returns a user-readable error, or nil.
func ValidateDeal(lg *League, deal Deal, userTeam int) error {
	if ok, why := TradeWindowOpen(lg); !ok {
		return fmt.Errorf("%s", why)
	}
	n := len(lg.Teams)
	sends, gets := map[int]bool{}, map[int]bool{}
	players, picks := map[int]int{}, map[int]int{}
	net := map[int]int{}
	seen := map[Asset]bool{}
	var order []int
	note := func(t int) {
		if _, ok := sends[t]; !ok {
			if _, ok2 := gets[t]; !ok2 {
				order = append(order, t)
			}
		}
	}
	if len(deal.Moves) == 0 {
		return fmt.Errorf("The deal is empty.")
	}
	for _, m := range deal.Moves {
		if m.From < 0 || m.From >= n || m.To < 0 || m.To >= n {
			return fmt.Errorf("That is not a valid team.")
		}
		if m.From == m.To {
			return fmt.Errorf("A team cannot trade an asset to itself.")
		}
		if seen[m.Asset] {
			return fmt.Errorf("The same player or pick is listed twice.")
		}
		seen[m.Asset] = true
		note(m.From)
		note(m.To)
		sends[m.From], gets[m.To] = true, true
		switch m.Kind {
		case PlayerAsset:
			if lg.findRosterPlayer(m.From, m.PlayerID) == nil {
				return fmt.Errorf("A player in the deal is not on the team that is sending him.")
			}
			players[m.From]++
			net[m.From]--
			net[m.To]++
		case PickAsset:
			own, ok := lg.PickOwner(m.Year, m.Round, m.Original)
			if !ok {
				return fmt.Errorf("A draft pick in the deal does not exist.")
			}
			if own != m.From {
				return fmt.Errorf("A draft pick in the deal is not owned by the team that is sending it.")
			}
			picks[m.From]++
		default:
			return fmt.Errorf("Unknown asset in the deal.")
		}
	}
	if len(order) < 2 || len(order) > MaxDealTeams {
		return fmt.Errorf("A deal needs 2 to %d teams.", MaxDealTeams)
	}
	inDeal := false
	for _, t := range order {
		if t == userTeam {
			inDeal = true
		}
		if !sends[t] || !gets[t] {
			return fmt.Errorf("Every team in the deal must both send and receive something (%s does not).", lg.Teams[t].FullName())
		}
		if players[t] > MaxTradePlayers {
			return fmt.Errorf("A team can send at most %d players in one deal.", MaxTradePlayers)
		}
		if picks[t] > MaxDealPicks {
			return fmt.Errorf("A team can send at most %d draft picks in one deal.", MaxDealPicks)
		}
	}
	// Rosters stay at RosterSize: a team left short signs free agents, and a
	// team left over releases players, who become free agents the short
	// teams can sign. So the deal only fails if the shortage is bigger than
	// the free agents on hand plus the players the deal itself releases.
	short, over := 0, 0
	for _, t := range order {
		size := len(lg.Teams[t].Roster) + net[t]
		if size < RosterSize {
			short += RosterSize - size
		} else {
			over += size - RosterSize
		}
	}
	if short > len(lg.FreeAgents)+over {
		return fmt.Errorf("Not enough free agents to fill the short roster.")
	}
	if !inDeal {
		return fmt.Errorf("Deals must include your team.")
	}
	for _, t := range order {
		var out, in []*Player
		for _, m := range deal.Moves {
			if m.Kind != PlayerAsset {
				continue
			}
			if m.From == t {
				out = append(out, lg.findRosterPlayer(m.From, m.PlayerID))
			}
			if m.To == t {
				in = append(in, lg.findRosterPlayer(m.From, m.PlayerID))
			}
		}
		if err := lg.salaryProblem(t, out, in); err != nil {
			return err
		}
	}
	return nil
}

func dealTeams(deal Deal) []int {
	var order []int
	seen := map[int]bool{}
	for _, m := range deal.Moves {
		for _, t := range []int{m.From, m.To} {
			if !seen[t] {
				seen[t] = true
				order = append(order, t)
			}
		}
	}
	return order
}

// EvaluateDeal asks every computer team in the deal whether it accepts. Each
// team judges only its own side, with the same rule as EvaluateTrade (value in
// vs value out, times the margin; more if it gives up a core player), with
// draft picks valued by PickValueFor.
func EvaluateDeal(lg *League, deal Deal, userTeam int) DealVerdict {
	v := evaluateDealBase(lg, deal, userTeam)
	if v.Valid {
		cf := lg.contendFactors()
		for i := range v.Teams {
			lg.negotiate(&v.Teams[i], deal, userTeam, cf)
		}
	}
	return v
}

// evaluateDealBase is EvaluateDeal without the negotiation fields.
func evaluateDealBase(lg *League, deal Deal, userTeam int) DealVerdict {
	if err := ValidateDeal(lg, deal, userTeam); err != nil {
		return DealVerdict{Problem: err.Error()}
	}
	cf := lg.contendFactors()
	v := DealVerdict{Valid: true, Accept: true}
	for _, tid := range dealTeams(deal) {
		if tid == userTeam {
			continue
		}
		var out, in []*Player
		var outAssets []Asset
		var pickOut, pickIn float64
		for _, m := range deal.Moves {
			if m.From == tid {
				outAssets = append(outAssets, m.Asset)
				if m.Kind == PlayerAsset {
					out = append(out, lg.findRosterPlayer(tid, m.PlayerID))
				} else {
					pickOut += lg.pickValue(m.Asset, tid, cf)
				}
			}
			if m.To == tid {
				if m.Kind == PlayerAsset {
					in = append(in, lg.findRosterPlayer(m.From, m.PlayerID))
				} else {
					pickIn += lg.pickValue(m.Asset, tid, cf)
				}
			}
		}
		gives, gets, need := lg.sideEval(tid, out, in, cf)
		gives += pickOut
		gets += pickIn
		tv := DealTeamVerdict{Team: tid, Gives: gives, Gets: gets, Needed: need, Accept: gets >= gives*need}
		name := lg.Teams[tid].FullName()
		un := lg.untouchables(tid, cf)
		// the most valuable asset the team is giving up, for the message
		var best Asset
		bestVal, core := -1.0, false
		for _, a := range outAssets {
			var val float64
			if a.Kind == PlayerAsset {
				val = lg.value(lg.findRosterPlayer(tid, a.PlayerID), tid, cf)
			} else {
				val = lg.pickValue(a, tid, cf)
			}
			if val > bestVal {
				best, bestVal = a, val
				core = a.Kind == PlayerAsset && un[a.PlayerID]
			}
		}
		switch {
		case tv.Accept:
			tv.Reason = name + " accept: the deal comes out ahead for them on value."
		case core && need > 1+TradeMargin:
			tv.Reason = fmt.Sprintf("%s see %s as a core player and want a much bigger offer.", name, lg.assetName(best))
		default:
			tv.Reason = fmt.Sprintf("%s want more for %s.", name, lg.assetName(best))
		}
		if !tv.Accept {
			v.Accept = false
		}
		v.Teams = append(v.Teams, tv)
	}
	return v
}

// ExecuteDeal validates and evaluates the deal and, if every computer team
// accepts, carries it out and logs it in lg.Transactions.
func ExecuteDeal(lg *League, deal Deal, userTeam int) (TradeResult, error) {
	v := evaluateDealBase(lg, deal, userTeam)
	if !v.Valid {
		return TradeResult{}, fmt.Errorf("%s", v.Problem)
	}
	if !v.Accept {
		var why []string
		for _, t := range v.Teams {
			if !t.Accept {
				why = append(why, t.Reason)
			}
		}
		return TradeResult{}, fmt.Errorf("%s", strings.Join(why, " "))
	}
	teams := dealTeams(deal)
	res := TradeResult{Year: lg.Year, Day: lg.currentDay(), TeamA: teams[0], TeamB: teams[1], Teams: teams}
	type pm struct {
		p  *Player
		to int
	}
	var moving []pm
	gone := map[int]bool{}
	for _, m := range deal.Moves {
		if m.Kind == PlayerAsset {
			p := lg.findRosterPlayer(m.From, m.PlayerID)
			moving = append(moving, pm{p, m.To})
			gone[p.ID] = true
			res.Moved = append(res.Moved, TxPlayer{p.ID, p.Name, m.From, m.To})
		} else {
			if err := lg.TransferPick(m.Year, m.Round, m.Original, m.From, m.To); err != nil {
				return TradeResult{}, err // cannot happen after validation
			}
			res.Picks = append(res.Picks, TxPick{m.Year, m.Round, m.Original, m.From, m.To})
		}
	}
	for _, tid := range teams {
		var keep []*Player
		for _, p := range lg.Teams[tid].Roster {
			if !gone[p.ID] {
				keep = append(keep, p)
			}
		}
		lg.Teams[tid].Roster = keep
	}
	for _, x := range moving {
		x.p.TeamID = x.to
		x.p.TeamSince = lg.Year
		lg.Teams[x.to].Roster = append(lg.Teams[x.to].Roster, x.p)
	}
	lg.fixRosters(teams, &res)
	lg.Transactions = append(lg.Transactions, res)
	return res, nil
}
